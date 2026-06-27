package service

import (
	"context"
	"errors"
	"fmt"
	"messagefeed/internal/domain"
	"strings"
	"sync"
	"time"
)

type StopAgentPlanInput struct {
	Reason string `json:"reason"`
}

type StopAgentPlanResult struct {
	Plan    AgentPlanResponse        `json:"plan"`
	Runtime AgentPlanStopRuntimeInfo `json:"runtime"`
}

type AgentPlanStopRuntimeInfo struct {
	PlanID             int64  `json:"plan_id"`
	TurnID             int64  `json:"turn_id"`
	ActiveProcessFound bool   `json:"active_process_found"`
	CancelSignaled     bool   `json:"cancel_signaled"`
	StopConfirmed      bool   `json:"stop_confirmed"`
	Confirmation       string `json:"confirmation"`
	WaitedMillis       int64  `json:"waited_millis"`
}

type agentActiveProcess struct {
	turnID int64
	planID int64
	cancel context.CancelFunc
	done   chan struct{}

	mu            sync.Mutex
	doneOnce      sync.Once
	stopRequested bool
}

// registerAgentProcess 将正在执行的 turn 注册到内存表，供 Web 停止按钮定位并取消。
func (s *AgentConversationService) registerAgentProcess(turnID int64, cancel context.CancelFunc) *agentActiveProcess {
	process := &agentActiveProcess{turnID: turnID, cancel: cancel, done: make(chan struct{})}
	if s == nil || turnID < 1 || cancel == nil {
		return process
	}
	s.activeProcessMu.Lock()
	defer s.activeProcessMu.Unlock()
	if s.activeByTurnID == nil {
		s.activeByTurnID = map[int64]*agentActiveProcess{}
	}
	s.activeByTurnID[turnID] = process
	return process
}

// bindAgentProcessPlan 在计划创建后补充 plan_id 索引，使停止接口可按计划定位执行上下文。
func (s *AgentConversationService) bindAgentProcessPlan(turnID int64, planID int64) {
	if s == nil || turnID < 1 || planID < 1 {
		return
	}
	s.activeProcessMu.Lock()
	defer s.activeProcessMu.Unlock()
	process := s.activeByTurnID[turnID]
	if process == nil {
		return
	}
	process.mu.Lock()
	process.planID = planID
	process.mu.Unlock()
	if s.activeByPlanID == nil {
		s.activeByPlanID = map[int64]*agentActiveProcess{}
	}
	s.activeByPlanID[planID] = process
}

// unregisterAgentProcess 在执行函数退出时关闭 done，停止接口以此确认 goroutine 已结束。
func (s *AgentConversationService) unregisterAgentProcess(process *agentActiveProcess) {
	if process == nil {
		return
	}
	if s != nil {
		s.activeProcessMu.Lock()
		if process.turnID > 0 && s.activeByTurnID[process.turnID] == process {
			delete(s.activeByTurnID, process.turnID)
		}
		if process.planID > 0 && s.activeByPlanID[process.planID] == process {
			delete(s.activeByPlanID, process.planID)
		}
		s.activeProcessMu.Unlock()
	}
	process.doneOnce.Do(func() {
		close(process.done)
	})
}

func (s *AgentConversationService) lookupAgentProcess(plan domain.AgentPlan) *agentActiveProcess {
	if s == nil {
		return nil
	}
	s.activeProcessMu.Lock()
	defer s.activeProcessMu.Unlock()
	if plan.ID > 0 {
		if process := s.activeByPlanID[plan.ID]; process != nil {
			return process
		}
	}
	if plan.TurnID > 0 {
		return s.activeByTurnID[plan.TurnID]
	}
	return nil
}

func (process *agentActiveProcess) requestStop() {
	if process == nil {
		return
	}
	process.mu.Lock()
	process.stopRequested = true
	cancel := process.cancel
	process.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (process *agentActiveProcess) stoppedByUser() bool {
	if process == nil {
		return false
	}
	process.mu.Lock()
	defer process.mu.Unlock()
	return process.stopRequested
}

// cancelAgentProcessAndWait 只在确认 goroutine 退出后返回成功；未确认时由调用方保留原状态。
func (s *AgentConversationService) cancelAgentProcessAndWait(ctx context.Context, plan domain.AgentPlan) (AgentPlanStopRuntimeInfo, error) {
	startedAt := s.now().UTC()
	info := AgentPlanStopRuntimeInfo{
		PlanID: plan.ID,
		TurnID: plan.TurnID,
	}
	process := s.lookupAgentProcess(plan)
	if process == nil {
		info.StopConfirmed = true
		info.Confirmation = "no_active_process_in_current_node"
		return info, nil
	}
	info.ActiveProcessFound = true
	info.CancelSignaled = true
	process.requestStop()
	timer := time.NewTimer(defaultAgentStopWaitTimeout)
	defer timer.Stop()
	select {
	case <-process.done:
		info.StopConfirmed = true
		info.Confirmation = "process_exited_after_cancel"
		info.WaitedMillis = s.now().UTC().Sub(startedAt).Milliseconds()
		return info, nil
	case <-timer.C:
		info.WaitedMillis = defaultAgentStopWaitTimeout.Milliseconds()
		return info, domain.NewAppError(domain.ErrorKindConflict, "agent_stop_not_confirmed", "agent process did not stop before timeout", "service.agent.stop_plan", true, nil)
	case <-ctx.Done():
		info.WaitedMillis = s.now().UTC().Sub(startedAt).Milliseconds()
		return info, ctx.Err()
	}
}

func agentPlanCanStop(status domain.AgentPlanStatus) bool {
	switch status {
	case domain.AgentPlanStatusDraft,
		domain.AgentPlanStatusAwaitingApproval,
		domain.AgentPlanStatusApproved,
		domain.AgentPlanStatusExecuting:
		return true
	default:
		return false
	}
}

func agentRunCanCancel(status domain.AgentRunStatus) bool {
	return status == domain.AgentRunStatusRunning || status == domain.AgentRunStatusInputRequired
}

func agentPlanStepStopStatus(step domain.AgentPlanStep) (domain.AgentPlanStepStatus, bool) {
	switch step.Status {
	case domain.AgentPlanStepStatusPending, domain.AgentPlanStepStatusApproved:
		return domain.AgentPlanStepStatusSkipped, true
	case domain.AgentPlanStepStatusExecuting:
		return domain.AgentPlanStepStatusFailed, true
	default:
		return step.Status, false
	}
}

// StopAgentPlan 是 Web 停止按钮调用的真实停止入口。
// 它先取消并确认当前节点中的执行 goroutine，再收敛 plan/step/run/turn 状态。
func (s *AgentConversationService) StopAgentPlan(ctx context.Context, auth CurrentAuth, planID int64, input StopAgentPlanInput) (StopAgentPlanResult, error) {
	if s == nil || s.repository == nil {
		return StopAgentPlanResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_conversation_unavailable", "agent conversation service is unavailable", "service.agent.stop_plan", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return StopAgentPlanResult{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	if planID < 1 {
		return StopAgentPlanResult{}, fmt.Errorf("%w: plan id is required", domain.ErrInvalidInput)
	}
	plan, err := s.repository.GetAgentPlan(ctx, auth.User.ID, planID)
	if err != nil {
		return StopAgentPlanResult{}, err
	}
	if !agentPlanCanStop(plan.Status) {
		return StopAgentPlanResult{Plan: agentPlanResponse(plan, true), Runtime: AgentPlanStopRuntimeInfo{
			PlanID:        plan.ID,
			TurnID:        plan.TurnID,
			StopConfirmed: true,
			Confirmation:  "plan_already_terminal",
		}}, nil
	}
	stopped, runtimeInfo, err := s.stopExistingAgentPlan(ctx, auth.User.ID, plan, strings.TrimSpace(input.Reason))
	if err != nil {
		return StopAgentPlanResult{Plan: agentPlanResponse(plan, true), Runtime: runtimeInfo}, err
	}
	return StopAgentPlanResult{Plan: agentPlanResponse(stopped, true), Runtime: runtimeInfo}, nil
}

// stopExistingAgentPlan 统一执行停止语义：先确认运行进程退出，再收敛持久化状态。
func (s *AgentConversationService) stopExistingAgentPlan(ctx context.Context, userID int64, plan domain.AgentPlan, reason string) (domain.AgentPlan, AgentPlanStopRuntimeInfo, error) {
	if s == nil || s.repository == nil || plan.ID < 1 {
		return plan, AgentPlanStopRuntimeInfo{PlanID: plan.ID, TurnID: plan.TurnID}, nil
	}
	runtimeInfo, err := s.cancelAgentProcessAndWait(ctx, plan)
	if err != nil {
		return plan, runtimeInfo, err
	}
	cleanupCtx := context.WithoutCancel(ctx)
	latest, err := s.repository.GetAgentPlan(cleanupCtx, userID, plan.ID)
	if err != nil {
		return domain.AgentPlan{}, runtimeInfo, err
	}
	if !agentPlanCanStop(latest.Status) && planStoppedByUser(latest) {
		return latest, runtimeInfo, nil
	}
	stopped, err := s.markAgentPlanStopped(cleanupCtx, userID, latest, reason, runtimeInfo)
	if err != nil {
		return domain.AgentPlan{}, runtimeInfo, err
	}
	return stopped, runtimeInfo, nil
}

// supersedeActivePlanForNewTask 在新用户任务到达时终止旧活动计划。
// 该函数不回复用户，只把旧计划收敛为终态，让当前消息继续进入新的主 Agent 规划流程。
func (s *AgentConversationService) supersedeActivePlanForNewTask(
	ctx context.Context,
	userID int64,
	sessionID int64,
	turnID int64,
	plan domain.AgentPlan,
	input ReceiveWeChatWorkAppMessageInput,
) error {
	if s == nil || s.repository == nil || plan.ID < 1 {
		return nil
	}
	if !agentPlanCanStop(plan.Status) {
		return nil
	}
	reason := "superseded_by_new_user_message"
	stopped, runtimeInfo, err := s.stopExistingAgentPlan(ctx, userID, plan, reason)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	metadata := cloneApprovalMetadata(stopped.Metadata)
	metadata["superseded_by"] = domain.AgentJSON{
		"turn_id":       turnID,
		"message":       safeSummary(input.TextContent, 500),
		"reason":        reason,
		"runtime":       runtimeInfo,
		"superseded_at": now.Format(time.RFC3339),
	}
	if updated, updateErr := s.repository.UpdateAgentPlanMetadata(context.WithoutCancel(ctx), userID, stopped.ID, metadata, now); updateErr == nil {
		stopped = updated
	} else {
		return updateErr
	}
	_, _ = s.repository.CreateAuditLog(context.WithoutCancel(ctx), domain.AgentAuditLog{
		SessionID: sessionID,
		TurnID:    turnID,
		UserID:    userID,
		EventType: "agent.plan_superseded",
		Status:    "superseded",
		Message:   reason,
		Metadata: domain.AgentJSON{
			"plan_id":     stopped.ID,
			"plan_status": string(stopped.Status),
			"runtime":     runtimeInfo,
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: now,
	})
	return nil
}

func (s *AgentConversationService) markAgentPlanStopped(ctx context.Context, userID int64, plan domain.AgentPlan, reason string, runtimeInfo AgentPlanStopRuntimeInfo) (domain.AgentPlan, error) {
	if s == nil || s.repository == nil || plan.ID < 1 {
		return plan, nil
	}
	now := s.now().UTC()
	stopReason := strings.TrimSpace(reason)
	if stopReason == "" {
		stopReason = "用户停止执行"
	}
	for _, step := range plan.Steps {
		nextStatus, changed := agentPlanStepStopStatus(step)
		if !changed {
			continue
		}
		step.Status = nextStatus
		if step.StartedAt == nil && nextStatus != domain.AgentPlanStepStatusSkipped {
			startedAt := now
			step.StartedAt = &startedAt
		}
		if step.CompletedAt == nil {
			completedAt := now
			step.CompletedAt = &completedAt
		}
		if nextStatus == domain.AgentPlanStepStatusFailed {
			step.ErrorMessage = stopReason
		} else {
			step.OutputSummary = stopReason
		}
		if _, err := s.repository.UpdateAgentPlanStepStatus(ctx, userID, step); err != nil {
			return domain.AgentPlan{}, err
		}
	}
	if plan.TurnID > 0 {
		runs, _ := s.repository.ListAgentRunsByTurn(ctx, userID, plan.TurnID)
		for _, run := range runs {
			if !agentRunCanCancel(run.Status) {
				continue
			}
			run.Status = domain.AgentRunStatusCanceled
			run.ErrorMessage = stopReason
			run.ResultRef = "stopped_by_user"
			run.CompletedAt = &now
			run.UpdatedAt = now
			_, _ = s.repository.UpdateAgentRun(ctx, run)
		}
		_, _ = s.repository.UpdateTurn(ctx, domain.AgentTurn{
			ID:           plan.TurnID,
			UserID:       userID,
			Status:       domain.AgentTurnStatusFailed,
			ErrorMessage: stopReason,
			FinishedAt:   &now,
		})
	}
	plan.Metadata = cloneApprovalMetadata(plan.Metadata)
	plan.Metadata["stop"] = domain.AgentJSON{
		"reason":               stopReason,
		"requested_at":         now.Format(time.RFC3339),
		"requested_by_user_id": userID,
		"runtime":              runtimeInfo,
	}
	updated, err := s.repository.UpdateAgentPlanMetadata(ctx, userID, plan.ID, plan.Metadata, now)
	if err != nil {
		return domain.AgentPlan{}, err
	}
	updated, err = s.repository.UpdateAgentPlanStatus(ctx, userID, updated.ID, domain.AgentPlanStatusFailed, now, stopReason)
	if err != nil {
		return domain.AgentPlan{}, err
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: plan.SessionID,
		TurnID:    plan.TurnID,
		UserID:    userID,
		EventType: "agent.plan_stopped",
		Status:    "stopped",
		Message:   stopReason,
		Metadata: domain.AgentJSON{
			"plan_id": plan.ID,
			"runtime": runtimeInfo,
		},
		CreatedAt: now,
	})
	return s.applyAgentPlanTerminalMetadata(ctx, userID, updated), nil
}

func isAgentProcessStopError(ctx context.Context, err error, process *agentActiveProcess) bool {
	if process == nil || !process.stoppedByUser() {
		return false
	}
	if ctx != nil && ctx.Err() != nil {
		return true
	}
	return errors.Is(err, context.Canceled)
}

// finishStoppedAgentProcess 收敛已收到停止信号的执行，不再生成用户最终回复。
func (s *AgentConversationService) finishStoppedAgentProcess(
	ctx context.Context,
	account domain.ExternalAccount,
	inbound domain.AgentInboundMessage,
	session domain.AgentSession,
	turn domain.AgentTurn,
	plan domain.AgentPlan,
) ReceiveWeChatWorkAppMessageResult {
	cleanupCtx := context.WithoutCancel(ctx)
	now := s.now().UTC()
	_, _ = s.repository.UpdateInboundMessageStatus(cleanupCtx, account.UserID, inbound.ID, domain.AgentInboundMessageStatusFailed, now)
	if latestPlan, err := s.repository.GetAgentPlan(cleanupCtx, account.UserID, plan.ID); err == nil {
		runtimeInfo := AgentPlanStopRuntimeInfo{
			PlanID:             latestPlan.ID,
			TurnID:             latestPlan.TurnID,
			ActiveProcessFound: true,
			CancelSignaled:     true,
			StopConfirmed:      true,
			Confirmation:       "process_exited_after_cancel",
		}
		if stopped, stopErr := s.markAgentPlanStopped(cleanupCtx, account.UserID, latestPlan, "用户停止执行", runtimeInfo); stopErr == nil {
			plan = stopped
		}
	}
	turn.Status = domain.AgentTurnStatusFailed
	turn.ErrorMessage = "用户停止执行"
	turn.FinishedAt = &now
	if updatedTurn, err := s.repository.UpdateTurn(cleanupCtx, turn); err == nil {
		turn = updatedTurn
	}
	return ReceiveWeChatWorkAppMessageResult{
		ExternalAccount: account,
		InboundMessage:  inbound,
		Session:         session,
		Turn:            turn,
		Plan:            plan,
	}
}
