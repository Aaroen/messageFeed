package service

import (
	"context"
	"messagefeed/internal/domain"
	"strings"
	"time"
)

type agentButtonControlResult struct {
	Plan        domain.AgentPlan
	Task        domain.AgentScheduledTask
	Status      string
	Summary     string
	ControlType string
	Changed     bool
	Metadata    domain.AgentJSON
}

// handleWeChatButtonCallback 将企微按钮动作转换为计划控制动作。
func (s *AgentConversationService) handleWeChatButtonCallback(
	ctx context.Context,
	account domain.ExternalAccount,
	inbound domain.AgentInboundMessage,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
) (bool, ReceiveWeChatWorkAppMessageResult, error) {
	if s == nil || s.repository == nil || input.Provider != domain.AgentProviderWeChatWorkApp {
		return false, ReceiveWeChatWorkAppMessageResult{}, nil
	}
	actionKey := normalizeAgentButtonCallbackKey(input.EventKey)
	handler := agentButtonCallbackHandler(actionKey)
	if actionKey == "" || handler == "" {
		return false, ReceiveWeChatWorkAppMessageResult{}, nil
	}
	plans, err := s.repository.ListAgentPlans(ctx, account.UserID, session.ID, 0, 1)
	if err != nil {
		return false, ReceiveWeChatWorkAppMessageResult{}, err
	}
	plan := domain.AgentPlan{}
	if len(plans) > 0 {
		plan = plans[0]
	}
	control, err := s.applyAgentButtonDirectControl(ctx, account.UserID, session.ID, actionKey, handler, plan, input)
	if err != nil {
		return false, ReceiveWeChatWorkAppMessageResult{}, err
	}
	plan = control.Plan
	reply := s.agentButtonCallbackReply(ctx, input, actionKey, handler, control)
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		EventType: "agent.button_direct_control",
		Status:    control.Status,
		Message:   control.Summary,
		Metadata: domain.AgentJSON{
			"event_key":           input.EventKey,
			"action_key":          actionKey,
			"handler":             handler,
			"plan_id":             plan.ID,
			"scheduled_task_id":   control.Task.ID,
			"control_type":        control.ControlType,
			"changed":             control.Changed,
			"control_metadata":    control.Metadata,
			"provider_message_id": input.ProviderMessageID,
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: s.now().UTC(),
	})
	result, err := s.finishTurnWithReply(ctx, account, inbound, session, turn, input, reply, nil, "button_callback")
	result.Plan = plan
	return true, result, err
}

// applyAgentButtonDirectControl 执行审批、重试、恢复和取消等显式控制。
func (s *AgentConversationService) applyAgentButtonDirectControl(ctx context.Context, userID int64, sessionID int64, actionKey string, handler string, plan domain.AgentPlan, input ReceiveWeChatWorkAppMessageInput) (agentButtonControlResult, error) {
	now := s.now().UTC()
	result := agentButtonControlResult{
		Plan:        plan,
		Status:      "succeeded",
		Summary:     "wechat work button callback opened control entry",
		ControlType: "control_entry",
		Metadata: domain.AgentJSON{
			"handler": handler,
		},
	}
	if plan.ID < 1 {
		result.Status = "no_plan"
		result.Summary = "wechat work button callback has no associated agent plan"
		result.ControlType = "no_associated_plan"
		return result, nil
	}
	switch actionKey {
	case "approval":
		result.ControlType = "approval"
		if plan.Status == domain.AgentPlanStatusAwaitingApproval {
			updated, err := s.repository.UpdateAgentPlanStatus(ctx, userID, plan.ID, domain.AgentPlanStatusApproved, now, "")
			if err != nil {
				return result, err
			}
			result.Plan = updated
			result.Changed = true
			result.Summary = "wechat work approval button approved agent plan"
			result.Metadata["from_status"] = string(plan.Status)
			result.Metadata["to_status"] = string(domain.AgentPlanStatusApproved)
			return result, nil
		}
		result.Status = "skipped"
		result.Summary = "approval button callback skipped because plan is not awaiting approval"
		result.Metadata["plan_status"] = string(plan.Status)
		return result, nil
	case "retry_plan":
		result.ControlType = "retry"
		if plan.Status != domain.AgentPlanStatusFailed {
			result.Status = "skipped"
			result.Summary = "retry button callback skipped because plan is not failed"
			result.Metadata["plan_status"] = string(plan.Status)
			return result, nil
		}
		queued, skipped, exhausted := 0, 0, 0
		for _, step := range plan.Steps {
			if step.Status != domain.AgentPlanStepStatusFailed {
				skipped++
				continue
			}
			updatedStep, retryErr := prepareAgentPlanStepRetry(step, "wechat button retry", now)
			if retryErr != nil {
				if appErr, ok := retryErr.(*domain.AppError); ok && appErr.Code == "agent_plan_step_retry_exhausted" {
					exhausted++
				} else {
					skipped++
				}
				continue
			}
			if _, err := s.repository.UpdateAgentPlanStepStatus(ctx, userID, updatedStep); err != nil {
				return result, err
			}
			queued++
		}
		result.Metadata["queued"] = queued
		result.Metadata["skipped"] = skipped
		result.Metadata["exhausted"] = exhausted
		if queued == 0 {
			result.Status = "skipped"
			result.Summary = "retry button callback found no retryable failed steps"
			return result, nil
		}
		updated, err := s.repository.UpdateAgentPlanStatus(ctx, userID, plan.ID, domain.AgentPlanStatusExecuting, now, "")
		if err != nil {
			return result, err
		}
		result.Plan = updated
		result.Changed = true
		result.Summary = "wechat work retry button queued failed plan steps"
		return result, nil
	case "recover_plan":
		result.ControlType = "recover"
		if plan.Status != domain.AgentPlanStatusExecuting && plan.Status != domain.AgentPlanStatusFailed {
			result.Status = "skipped"
			result.Summary = "recovery button callback skipped because plan is not recoverable"
			result.Metadata["plan_status"] = string(plan.Status)
			return result, nil
		}
		recoveredSteps := 0
		for _, step := range plan.Steps {
			if step.Status != domain.AgentPlanStepStatusExecuting {
				continue
			}
			step.Status = domain.AgentPlanStepStatusApproved
			step.OutputSummary = "recovered from wechat button"
			step.ErrorMessage = ""
			step.StartedAt = nil
			step.CompletedAt = nil
			step.UpdatedAt = now
			metadata := cloneServiceAgentJSON(step.RetryMetadata)
			metadata["previous_status"] = string(domain.AgentPlanStepStatusExecuting)
			metadata["recovered_at"] = now.Format(time.RFC3339)
			metadata["recover_reason"] = "wechat button recovery"
			step.RetryMetadata = metadata
			if _, err := s.repository.UpdateAgentPlanStepStatus(ctx, userID, step); err != nil {
				return result, err
			}
			recoveredSteps++
		}
		if recoveredSteps == 0 && plan.Status == domain.AgentPlanStatusFailed {
			result.Status = "skipped"
			result.Summary = "recovery button callback found no interrupted executing steps"
			return result, nil
		}
		plan.Metadata = cloneServiceAgentJSON(plan.Metadata)
		recoveryMetadata := buildAgentPlanRecoveryMetadata(plan, recoveredSteps, "wechat button recovery", now)
		plan.Metadata["recovery"] = recoveryMetadata
		updated, err := s.repository.UpdateAgentPlanStatus(ctx, userID, plan.ID, domain.AgentPlanStatusExecuting, now, "")
		if err != nil {
			return result, err
		}
		updated, err = s.repository.UpdateAgentPlanMetadata(ctx, userID, plan.ID, plan.Metadata, now)
		if err != nil {
			return result, err
		}
		result.Plan = updated
		result.Changed = true
		result.Summary = "wechat work recovery button recovered agent plan"
		result.Metadata["recovered_steps"] = recoveredSteps
		return result, nil
	case "cancel_scheduled_task":
		result.ControlType = "cancel_scheduled_task"
		tasks, err := s.repository.ListAgentScheduledTasks(ctx, domain.AgentScheduledTaskListOptions{UserID: userID, Limit: 50})
		if err != nil {
			return result, err
		}
		for _, task := range tasks {
			if task.PlanID != plan.ID || !agentScheduledTaskCancelable(task.Status) {
				continue
			}
			task.Status = domain.AgentScheduledTaskStatusCanceled
			task.LastError = ""
			task.NextRunAt = nil
			task.CompletedAt = &now
			task.UpdatedAt = now
			updated, err := s.repository.UpdateAgentScheduledTask(ctx, task)
			if err != nil {
				return result, err
			}
			result.Task = updated
			result.Changed = true
			result.Summary = "wechat work cancel button canceled scheduled task"
			result.Metadata["task_status"] = string(updated.Status)
			return result, nil
		}
		result.Status = "skipped"
		result.Summary = "cancel button callback found no cancelable scheduled task"
		return result, nil
	case "view_progress", "view_final_report":
		result.ControlType = "view"
		result.Summary = "wechat work button callback opened agent progress view"
		return result, nil
	default:
		return result, nil
	}
}

func agentScheduledTaskCancelable(status domain.AgentScheduledTaskStatus) bool {
	return status == domain.AgentScheduledTaskStatusQueued ||
		status == domain.AgentScheduledTaskStatusRunning ||
		status == domain.AgentScheduledTaskStatusInputRequired
}

func normalizeAgentButtonCallbackKey(eventKey string) string {
	key := strings.TrimSpace(strings.ToLower(eventKey))
	if key == "" {
		return ""
	}
	for _, separator := range []string{"?", "&", "=", ":", "|"} {
		if index := strings.Index(key, separator); index > 0 {
			key = key[:index]
			break
		}
	}
	switch key {
	case "progress", "view":
		return "view_progress"
	case "approve", "approval":
		return "approval"
	case "retry":
		return "retry_plan"
	case "recover", "recovery":
		return "recover_plan"
	case "cancel":
		return "cancel_scheduled_task"
	case "report", "final_report":
		return "view_final_report"
	default:
		return key
	}
}

func (s *AgentConversationService) agentButtonCallbackReply(ctx context.Context, input ReceiveWeChatWorkAppMessageInput, actionKey string, handler string, control agentButtonControlResult) string {
	plan := control.Plan
	return s.generateAgentWeChatFeedbackText(ctx, agentWeChatFeedbackRequest{
		Stage:       "button_callback",
		UserMessage: input.TextContent,
		Plan:        plan,
		ProgressURL: s.agentPlanURLIfAvailable(plan.ID),
		Control: agentWeChatFeedbackControl{
			ActionKey:           actionKey,
			Handler:             handler,
			Type:                control.ControlType,
			Status:              control.Status,
			Summary:             control.Summary,
			Changed:             control.Changed,
			PlanID:              plan.ID,
			ScheduledTaskID:     control.Task.ID,
			ScheduledTaskStatus: string(control.Task.Status),
			Metadata:            control.Metadata,
		},
	})
}
