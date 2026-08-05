package service

import (
	"context"
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
	s.SetWorkerID(workerID)
	claimed, err := workerRepository.ClaimQueuedAgentTurns(ctx, domain.AgentTurnClaimInput{
		Now:           s.now().UTC(),
		WorkerID:      workerID,
		Limit:         limit,
		LeaseDuration: leaseDuration,
	})
	if err != nil {
		return AgentWorkerResult{}, err
	}
	result := AgentWorkerResult{ClaimedCount: len(claimed)}
	for _, turn := range claimed {
		if err := s.processClaimedAgentTurn(ctx, workerRepository, turn); err != nil {
			latest, getErr := workerRepository.GetAgentTurn(ctx, turn.UserID, turn.ID)
			if getErr == nil && latest.CancelRequested {
				result.Canceled++
			} else {
				result.Failed++
			}
			continue
		}
		switch turnStatus, _ := workerRepository.GetAgentTurn(ctx, turn.UserID, turn.ID); turnStatus.Status {
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
	workerID := strings.TrimSpace(s.workerID)
	turn.Status = domain.AgentTurnStatusQueued
	turn.FinishedAt = nil
	turn.LockedBy = ""
	turn.LockedAt = nil
	turn.LeaseUntil = nil
	_, err := repository.UpdateTurnIfOwned(ctx, turn, workerID)
	return err
}

func (s *AgentConversationService) processClaimedAgentTurnUnlocked(ctx context.Context, repository AgentTurnWorkerRepository, turn domain.AgentTurn) error {
	stopLeaseRenewal := s.renewAgentTurnLease(ctx, turn)
	defer stopLeaseRenewal()
	if turn.CancelRequested {
		reason := strings.TrimSpace(turn.CancelReason)
		if reason == "" {
			reason = "agent turn canceled"
		}
		return s.failClaimedAgentTurn(ctx, repository, turn, fmt.Errorf("%s", reason))
	}
	inbound, err := repository.GetInboundMessage(ctx, turn.UserID, turn.InboundMessageID)
	if err != nil {
		return s.failClaimedAgentTurn(ctx, repository, turn, err)
	}
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

	entry := s.userTranscriptForTurn(ctx, turn, inbound)
	defer s.captureMemoryCandidateFromTranscriptAsync(ctx, entry)
	if handled, handledResult, processErr := s.handleMultiTurnMessage(ctx, account, inbound, session, turn, input); processErr != nil {
		if failErr := s.failTurnWithFeedback(ctx, account, inbound, session, turn, input, domain.AgentPlan{}, processErr); failErr.Turn.ID > 0 {
			turn = failErr.Turn
		}
	} else if handled {
		turn = handledResult.Turn
		if input.Provider == domain.AgentProviderWeb && handledResult.Turn.ID > 0 {
			_ = s.sendWebAgentTaskFinalReport(ctx, account, inbound, session, handledResult.Turn, handledResult.Plan, handledResult.Reply, input)
		}
	} else {
		processed, processErr := s.processTurn(ctx, account, inbound, session, turn, input)
		if processed.Turn.ID > 0 {
			turn = processed.Turn
		}
		if input.Provider == domain.AgentProviderWeb && processErr == nil {
			if reportErr := s.sendWebAgentTaskFinalReport(ctx, account, inbound, session, processed.Turn, processed.Plan, processed.Reply, input); reportErr != nil {
				processErr = reportErr
			}
		}
		if processErr != nil {
			return s.failClaimedAgentTurn(ctx, repository, turn, processErr)
		}
	}
	return s.releaseClaimedAgentTurn(ctx, repository, turn)
}

func (s *AgentConversationService) failClaimedAgentTurn(ctx context.Context, repository AgentTurnWorkerRepository, turn domain.AgentTurn, cause error) error {
	if cause == nil {
		cause = fmt.Errorf("agent worker failed")
	}
	now := s.now().UTC()
	turn.Status = domain.AgentTurnStatusFailed
	turn.ErrorMessage = cause.Error()
	turn.FinishedAt = &now
	turn.LockedBy = ""
	turn.LockedAt = nil
	turn.LeaseUntil = nil
	if _, err := repository.UpdateTurnIfOwned(ctx, turn, strings.TrimSpace(s.workerID)); err != nil {
		return err
	}
	_, _ = s.repository.UpdateInboundMessageStatus(ctx, turn.UserID, turn.InboundMessageID, domain.AgentInboundMessageStatusFailed, now)
	return cause
}

func (s *AgentConversationService) releaseClaimedAgentTurn(ctx context.Context, repository AgentTurnWorkerRepository, turn domain.AgentTurn) error {
	workerID := strings.TrimSpace(s.workerID)
	turn.LockedBy = ""
	turn.LockedAt = nil
	turn.LeaseUntil = nil
	if turn.Status == domain.AgentTurnStatusQueued {
		turn.Status = domain.AgentTurnStatusFailed
	}
	_, err := repository.UpdateTurnIfOwned(ctx, turn, workerID)
	return err
}

func (s *AgentConversationService) userTranscriptForTurn(ctx context.Context, turn domain.AgentTurn, inbound domain.AgentInboundMessage) domain.AgentTranscriptEntry {
	entries, _ := s.repository.ListRecentTranscriptEntries(ctx, domain.AgentTranscriptListOptions{UserID: turn.UserID, SessionID: turn.SessionID, Limit: 50})
	for _, entry := range entries {
		if entry.TurnID == turn.ID && entry.Role == domain.AgentTranscriptRoleUser {
			return entry
		}
	}
	return domain.AgentTranscriptEntry{
		SessionID: turn.SessionID,
		TurnID:    turn.ID,
		UserID:    turn.UserID,
		Role:      domain.AgentTranscriptRoleUser,
		Content:   inbound.TextContent,
		Metadata:  domain.AgentJSON{"provider_message_id": inbound.ProviderMessageID},
		CreatedAt: inbound.CreatedAt,
	}
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

func (s *AgentConversationService) renewAgentTurnLease(ctx context.Context, turn domain.AgentTurn) func() {
	store, ok := s.repository.(interface {
		RenewAgentTurnLease(context.Context, int64, int64, string, time.Time, time.Time) error
	})
	if !ok || strings.TrimSpace(s.workerID) == "" {
		return func() {}
	}
	renewCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(agentWorkerLeaseRenewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-renewCtx.Done():
				return
			case now := <-ticker.C:
				_ = store.RenewAgentTurnLease(renewCtx, turn.UserID, turn.ID, s.workerID, now.Add(defaultAgentWorkerLease), now)
			}
		}
	}()
	return cancel
}
