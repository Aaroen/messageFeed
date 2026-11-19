package service

import (
	"context"
	"errors"
	"fmt"
	"messagefeed/internal/domain"
	"strings"
	"time"
)

const (
	defaultAgentWorkerLease       = 2 * time.Minute
	defaultAgentWorkerLimit       = 10
	agentWorkerCancellationPoll   = time.Second
	agentWorkerLeaseRenewInterval = 30 * time.Second
)

type RunAgentWorkerOnceInput struct {
	WorkerID      string
	Limit         int
	LeaseDuration time.Duration
}

type AgentWorkerResult struct {
	ClaimedCount int
	Succeeded    int
	Failed       int
	Canceled     int
	Skipped      int
}

func (s *AgentConversationService) RunAgentWorkerOnce(ctx context.Context, input RunAgentWorkerOnceInput) (AgentWorkerResult, error) {
	workerRepository, ok := s.repository.(AgentTurnWorkerRepository)
	if !ok {
		return AgentWorkerResult{}, fmt.Errorf("agent worker repository is not configured")
	}
	workerID := strings.TrimSpace(input.WorkerID)
	if workerID == "" {
		workerID = "agent-worker"
	}
	leaseDuration := input.LeaseDuration
	if leaseDuration <= 0 {
		leaseDuration = defaultAgentWorkerLease
	}
	limit := input.Limit
	if limit <= 0 {
		limit = defaultAgentWorkerLimit
	}
	claimed, err := workerRepository.ClaimQueuedAgentTurns(ctx, domain.AgentTurnClaimInput{
		Now:           s.now().UTC(),
		WorkerID:      workerID,
		Limit:         limit,
		LeaseDuration: leaseDuration,
	})
	if err != nil {
		return AgentWorkerResult{}, err
	}
	statuses := make(chan domain.AgentTurn, len(claimed))
	for _, turn := range claimed {
		go func() {
			processErr := s.processClaimedAgentTurn(ctx, workerRepository, turn)
			latest, err := workerRepository.GetAgentTurn(ctx, turn.UserID, turn.ID)
			if err != nil || processErr != nil && latest.Status != domain.AgentTurnStatusSucceeded {
				latest.Status = domain.AgentTurnStatusFailed
			}
			statuses <- latest
		}()
	}
	result := AgentWorkerResult{ClaimedCount: len(claimed)}
	for range claimed {
		turnStatus := <-statuses
		switch turnStatus.Status {
		case domain.AgentTurnStatusSucceeded:
			result.Succeeded++
		case domain.AgentTurnStatusFailed:
			if turnStatus.CancelRequested {
				result.Canceled++
			} else {
				result.Failed++
			}
		default:
			result.Skipped++
		}
	}
	return result, nil
}

func (s *AgentConversationService) processClaimedAgentTurn(ctx context.Context, repository AgentTurnWorkerRepository, turn domain.AgentTurn) error {
	if s.sessionTaskLocker == nil {
		return s.processClaimedAgentTurnUnlocked(ctx, repository, turn)
	}
	entered := false
	lockName := fmt.Sprintf("agent-session:%d", turn.SessionID)
	lockErr := s.sessionTaskLocker.WithLock(ctx, lockName, defaultAgentWorkerLease, func(lockCtx context.Context) error {
		entered = true
		return s.processClaimedAgentTurnUnlocked(lockCtx, repository, turn)
	})
	if entered {
		return lockErr
	}
	// Another worker owns this session. Return the claimed turn to the queue
	// without consuming an attempt or marking the business task as failed.
	return s.requeueClaimedAgentTurn(ctx, repository, turn)
}

func (s *AgentConversationService) requeueClaimedAgentTurn(ctx context.Context, repository AgentTurnWorkerRepository, turn domain.AgentTurn) error {
	workerID := turn.LockedBy
	turn.Status = domain.AgentTurnStatusQueued
	turn.FinishedAt = nil
	turn.LockedBy = ""
	turn.LockedAt = nil
	turn.LeaseUntil = nil
	_, err := repository.UpdateTurnIfOwned(ctx, turn, workerID)
	return err
}

func (s *AgentConversationService) processClaimedAgentTurnUnlocked(ctx context.Context, repository AgentTurnWorkerRepository, turn domain.AgentTurn) error {
	ctx, stopLeaseRenewal := s.renewAgentTurnLease(ctx, turn)
	defer stopLeaseRenewal()
	if turn.CancelRequested {
		reason := strings.TrimSpace(turn.CancelReason)
		if reason == "" {
			reason = "agent turn canceled"
		}
		return s.failClaimedAgentTurn(ctx, repository, turn, fmt.Errorf("%s", reason))
	}
	ctx, _, stopWatch := s.watchAgentProcess(ctx, turn)
	defer stopWatch()
	inbound, err := repository.GetInboundMessage(ctx, turn.UserID, turn.InboundMessageID)
	if err != nil {
		return s.failClaimedAgentTurn(ctx, repository, turn, err)
	}
	selectedCtx, err := agentSelectionFromInbound(ctx, inbound)
	if err != nil {
		return s.failClaimedAgentTurn(ctx, repository, turn, err)
	}
	ctx = selectedCtx
	session, err := repository.GetAgentSession(ctx, turn.UserID, turn.SessionID)
	if err != nil {
		return s.failClaimedAgentTurn(ctx, repository, turn, err)
	}
	account := domain.ExternalAccount{
		ID:             inbound.ExternalAccountID,
		UserID:         inbound.UserID,
		Provider:       inbound.Provider,
		CorpID:         inbound.CorpID,
		AgentID:        inbound.AgentID,
		ExternalUserID: inbound.ExternalUserID,
		BindingStatus:  domain.ExternalAccountBindingStatusActive,
	}
	input := agentInputFromInbound(inbound)
	if input.Provider == domain.AgentProviderWeChatWorkApp {
		s.sendWeChatWorkTaskAcceptedFeedback(ctx, account, session, turn, input)
	}

	ctx = context.WithValue(ctx, agentTurnRoutingKey{}, &agentTurnRouting{})
	if handled, handledResult, processErr := s.handleMultiTurnMessage(ctx, account, inbound, session, turn, input); processErr != nil {
		if handledResult.Turn.ID > 0 {
			return s.failClaimedAgentTurn(ctx, repository, handledResult.Turn, processErr)
		}
		failedResult, finishErr := s.failTurnWithFeedback(ctx, account, inbound, session, turn, input, domain.AgentPlan{}, processErr)
		if finishErr != nil {
			return s.failClaimedAgentTurn(ctx, repository, turn, finishErr)
		}
		turn = failedResult.Turn
	} else if handled {
		turn = handledResult.Turn
		if !s.persistentQueue && input.Provider == domain.AgentProviderWeb && handledResult.Turn.ID > 0 {
			if reportErr := s.sendWebAgentTaskFinalReport(ctx, account, inbound, session, handledResult.Turn, handledResult.Plan, handledResult.Reply, input); reportErr != nil {
				return s.failClaimedAgentTurn(ctx, repository, turn, reportErr)
			}
		}
	} else {
		processed, processErr := s.processTurn(ctx, account, inbound, session, turn, input)
		if processed.Turn.ID > 0 {
			turn = processed.Turn
		}
		if !s.persistentQueue && input.Provider == domain.AgentProviderWeb && processErr == nil {
			if reportErr := s.sendWebAgentTaskFinalReport(ctx, account, inbound, session, processed.Turn, processed.Plan, processed.Reply, input); reportErr != nil {
				processErr = reportErr
			}
		}
		if processErr != nil {
			return s.failClaimedAgentTurn(ctx, repository, turn, processErr)
		}
	}
	return nil
}

func (s *AgentConversationService) failClaimedAgentTurn(ctx context.Context, repository AgentTurnWorkerRepository, turn domain.AgentTurn, cause error) error {
	if cause == nil {
		cause = fmt.Errorf("agent worker failed")
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	current, err := repository.GetAgentTurn(ctx, turn.UserID, turn.ID)
	if err != nil {
		return err
	}
	if current.Status == domain.AgentTurnStatusSucceeded || current.Status == domain.AgentTurnStatusFailed {
		return cause
	}
	if current.LockedBy != turn.LockedBy || current.AttemptCount != turn.AttemptCount {
		return errors.Join(cause, domain.ErrConflict)
	}
	_, err = s.commitFailedAgentTurn(ctx, current, cause)
	return errors.Join(cause, err)
}

func (s *AgentConversationService) commitFailedAgentTurn(ctx context.Context, turn domain.AgentTurn, cause error) (domain.AgentTurnCompletionResult, error) {
	now := s.now().UTC()
	turn.Status = domain.AgentTurnStatusFailed
	turn.ErrorMessage = cause.Error()
	turn.ErrorCode = agentTurnErrorCode(cause)
	turn.OutputText = fmt.Sprintf("%s（调用编号：%d）", agentUserFacingFailureReason(cause.Error()), turn.ID)
	turn.DeliveryStatus = "not_required"
	if turn.CancelRequested && strings.TrimSpace(turn.CancelReason) != "" {
		turn.ErrorMessage = turn.CancelReason
	}
	plans, err := s.repository.ListAgentPlans(ctx, turn.UserID, turn.SessionID, turn.ID, 1)
	if err != nil {
		return domain.AgentTurnCompletionResult{}, err
	}
	completion := domain.AgentTurnCompletion{Turn: turn, WorkerID: turn.LockedBy, CompletedAt: now}
	if len(plans) > 0 && agentPlanCanStop(plans[0].Status) {
		plan, err := s.repository.GetAgentPlan(ctx, turn.UserID, plans[0].ID)
		if err != nil {
			return domain.AgentTurnCompletionResult{}, err
		}
		plan.Status = domain.AgentPlanStatusFailed
		plan.ErrorMessage = turn.ErrorMessage
		for index, step := range plan.Steps {
			plan.Steps[index] = convergeUnobservedPlanStep(step, now)
		}
		plan.Metadata = cloneApprovalMetadata(plan.Metadata)
		if turn.CancelRequested || cause.Error() == "用户停止执行" {
			plan.Metadata["stop"] = domain.AgentJSON{"reason": turn.ErrorMessage, "requested_at": now.Format(time.RFC3339), "requested_by_user_id": turn.UserID}
		}
		plan.Metadata["execution_completeness"] = buildAgentExecutionCompletenessMetadata(plan, now)
		completion.Plan = plan
	}
	return s.repository.CommitAgentTurnResult(ctx, completion)
}

func agentInputFromInbound(inbound domain.AgentInboundMessage) ReceiveWeChatWorkAppMessageInput {
	input := ReceiveWeChatWorkAppMessageInput{
		Provider:          inbound.Provider,
		ProviderMessageID: inbound.ProviderMessageID,
		CorpID:            inbound.CorpID,
		AgentID:           inbound.AgentID,
		ExternalUserID:    inbound.ExternalUserID,
		ChatID:            inbound.ChatID,
		ChatType:          inbound.ChatType,
		MsgType:           inbound.MsgType,
		TextContent:       inbound.TextContent,
		RequestID:         inbound.RequestID,
		TraceID:           inbound.TraceID,
	}
	if inbound.Payload != nil {
		input.EventType, _ = inbound.Payload["event_type"].(string)
		input.EventKey, _ = inbound.Payload["event_key"].(string)
		input.RawXML, _ = inbound.Payload["raw_xml"].(string)
	}
	return input
}

func (s *AgentConversationService) watchAgentTurnCancellation(ctx context.Context, turn domain.AgentTurn, process *agentActiveProcess) func() {
	store, ok := s.repository.(interface {
		GetAgentTurn(context.Context, int64, int64) (domain.AgentTurn, error)
	})
	if !ok || process == nil || turn.ID < 1 {
		return func() {}
	}
	watchCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(agentWorkerCancellationPoll)
		defer ticker.Stop()
		for {
			select {
			case <-watchCtx.Done():
				return
			case <-ticker.C:
				latest, err := store.GetAgentTurn(watchCtx, turn.UserID, turn.ID)
				if err == nil && latest.CancelRequested {
					process.requestStop()
					return
				}
			}
		}
	}()
	return cancel
}

func (s *AgentConversationService) renewAgentTurnLease(ctx context.Context, turn domain.AgentTurn) (context.Context, context.CancelFunc) {
	store, ok := s.repository.(AgentTurnWorkerRepository)
	if !ok || turn.LockedBy == "" {
		return ctx, func() {}
	}
	renewCtx, cancel := context.WithCancel(ctx)
	leaseDuration := defaultAgentWorkerLease
	if turn.LeaseUntil != nil && turn.LockedAt != nil && turn.LeaseUntil.After(*turn.LockedAt) {
		leaseDuration = turn.LeaseUntil.Sub(*turn.LockedAt)
	}
	go func() {
		ticker := time.NewTicker(max(time.Millisecond, min(agentWorkerLeaseRenewInterval, leaseDuration/3)))
		defer ticker.Stop()
		for {
			select {
			case <-renewCtx.Done():
				return
			case now := <-ticker.C:
				if err := store.RenewAgentTurnLease(renewCtx, turn.UserID, turn.ID, turn.LockedBy, turn.AttemptCount, now.Add(leaseDuration), now); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	return renewCtx, cancel
}
