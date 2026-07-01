package service

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/metrics"
	"messagefeed/internal/notifier"
	"strings"
	"time"
)

// approvalRequiredReply 将确认请求的结构化事实交给模型生成用户可见短回复。
func (s *AgentConversationService) approvalRequiredReply(ctx context.Context, input ReceiveWeChatWorkAppMessageInput, plan domain.AgentPlan, token string) string {
	return s.generateAgentWeChatFeedbackText(ctx, agentWeChatFeedbackRequest{
		Stage:       "approval_waiting",
		UserMessage: input.TextContent,
		Plan:        plan,
		ProgressURL: s.agentPlanURLIfAvailable(plan.ID),
		ApprovalURL: s.agentApprovalURL(token),
	})
}

func (s *AgentConversationService) sendPlanStartedFeedback(
	ctx context.Context,
	account domain.ExternalAccount,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
	plan domain.AgentPlan,
) {
	if s == nil || plan.ID == 0 || !s.shouldSendWeChatWorkNotification(ctx, account.UserID, input, "process") {
		return
	}
	s.sendPlanProgressNotification(ctx, account, session, turn, input, plan, "started", "started")
}

func (s *AgentConversationService) startPlanPeriodicProgressNotifications(
	ctx context.Context,
	account domain.ExternalAccount,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
	plan domain.AgentPlan,
) context.CancelFunc {
	if s == nil || s.processInline || plan.ID == 0 || s.progressNotifyInterval <= 0 {
		return func() {}
	}
	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	go func() {
		ticker := time.NewTicker(s.progressNotifyInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				latest := plan
				if loaded, err := s.repository.GetAgentPlan(ctx, account.UserID, plan.ID); err == nil && loaded.ID > 0 {
					latest = loaded
				}
				if agentPlanTerminal(latest.Status) || latest.Status == domain.AgentPlanStatusAwaitingApproval || latest.Status == domain.AgentPlanStatusRejected {
					return
				}
				s.sendPlanProgressNotification(ctx, account, session, turn, input, latest, "periodic_progress", "periodic_progress")
			}
		}
	}()
	return cancel
}

// sendPlanProgressNotification 将关键阶段进度同步到企业微信。
func (s *AgentConversationService) sendPlanProgressNotification(
	ctx context.Context,
	account domain.ExternalAccount,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
	plan domain.AgentPlan,
	stage string,
	title string,
) {
	if s == nil || plan.ID == 0 {
		return
	}
	notificationKind := "process"
	if strings.Contains(stage, "failed") || stage == "failed" {
		notificationKind = "failure"
	}
	if !s.shouldSendWeChatWorkNotification(ctx, account.UserID, input, notificationKind) {
		return
	}
	stage = strings.TrimSpace(stage)
	if stage == "" {
		stage = "progress"
	}
	progressURL := s.agentPlanURL(plan.ID)
	currentStep := planProgressNotificationStep(plan)
	reply := s.agentPlanProgressNotificationText(ctx, input, plan, currentStep, stage, title)
	delivery := s.sendWeChatWorkProgressDelivery(ctx, input.ExternalUserID, plan, stage, title, reply)
	status := "succeeded"
	message := "agent plan progress notification sent"
	if delivery.FallbackStatus == "failed" {
		status = "failed"
		message = delivery.FallbackError
	} else if delivery.DeliveryMode == "text_fallback" {
		message = "agent plan progress notification sent with text fallback"
	}
	eventType := "agent.plan_progress_notification"
	if stage == "started" {
		eventType = "agent.plan_started_feedback"
	} else if stage == "periodic_progress" {
		eventType = "agent.plan_periodic_progress_notification"
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		EventType: eventType,
		Status:    status,
		Message:   message,
		Metadata: domain.AgentJSON{
			"plan_id":             plan.ID,
			"stage":               stage,
			"target_channel":      input.Provider,
			"target_ref":          input.ExternalUserID,
			"provider_message_id": input.ProviderMessageID,
			"wechat_msgid":        delivery.SendResult.MessageID,
			"send_count":          delivery.SendCount,
			"progress_url":        progressURL,
			"message_type":        delivery.DeliveryMode,
			"template_status":     delivery.TemplateStatus,
			"fallback_status":     delivery.FallbackStatus,
			"template_error":      delivery.TemplateError,
			"fallback_error":      delivery.FallbackError,
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: s.now().UTC(),
	})
	traceStatus := domain.AgentTraceEventSucceeded
	var traceErr error
	if status == "failed" {
		traceStatus = domain.AgentTraceEventFailed
		traceErr = fmt.Errorf("%s", message)
	}
	s.recordAgentNotificationTraceEvent(ctx, input, account, session, turn, plan, progressNotificationTraceEventName(stage), traceStatus, delivery.SendCount, traceErr, domain.AgentJSON{
		"stage":           stage,
		"progress_url":    progressURL,
		"message_type":    delivery.DeliveryMode,
		"template_status": delivery.TemplateStatus,
		"fallback_status": delivery.FallbackStatus,
	})
}

func (s *AgentConversationService) agentPlanProgressNotificationText(
	ctx context.Context,
	input ReceiveWeChatWorkAppMessageInput,
	plan domain.AgentPlan,
	step domain.AgentPlanStep,
	stage string,
	title string,
) string {
	return s.generateAgentWeChatFeedbackText(ctx, agentWeChatFeedbackRequest{
		Stage:       stage,
		UserMessage: input.TextContent,
		Plan:        plan,
		Step:        step,
		ErrorText:   firstNonEmptyString(step.ErrorMessage, plan.ErrorMessage),
		ProgressURL: s.agentPlanURL(plan.ID),
	})
}

func (s *AgentConversationService) agentTurnCompletionReply(plan domain.AgentPlan, reply string) string {
	reply = strings.TrimSpace(reply)
	if reply != "" {
		return reply
	}
	key := "completed_empty"
	if plan.Status == domain.AgentPlanStatusFailed {
		key = "failed"
	}
	return finalizeAgentWeChatFeedbackText(renderAgentWeChatFeedbackTemplate(key, agentWeChatFeedbackRequest{
		Stage:       key,
		Plan:        plan,
		ProgressURL: s.agentPlanURLIfAvailable(plan.ID),
	}.templateData()))
}

func (s *AgentConversationService) sendWeChatWorkTaskAcceptedFeedback(
	ctx context.Context,
	account domain.ExternalAccount,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
) (string, notifier.WeChatWorkSendResult, int) {
	request := agentWeChatFeedbackRequest{
		Stage:       "accepted",
		UserMessage: input.TextContent,
	}
	reply := finalizeAgentWeChatFeedbackText(renderAgentWeChatFeedbackTemplate(request.fallbackTemplateKey(), request.templateData()))
	if s == nil || !s.shouldSendWeChatWorkNotification(ctx, account.UserID, input, "process") {
		return reply, notifier.WeChatWorkSendResult{}, 0
	}
	sendResult, sendCount, err := s.sendWeChatWorkReply(ctx, input.ExternalUserID, reply)
	status := "succeeded"
	message := "wechat work task acceptance feedback sent"
	if err != nil {
		status = "failed"
		message = strings.TrimSpace(err.Error())
	}
	metrics.AgentReplyChunksTotal.WithLabelValues(input.Provider, "accepted").Add(float64(sendCount))
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		EventType: "wechat_work.task_accepted_feedback",
		Status:    status,
		Message:   message,
		Metadata: domain.AgentJSON{
			"provider_message_id": input.ProviderMessageID,
			"target_channel":      input.Provider,
			"target_ref":          input.ExternalUserID,
			"wechat_msgid":        sendResult.MessageID,
			"send_count":          sendCount,
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: s.now().UTC(),
	})
	return reply, sendResult, sendCount
}

func planStepLabel(step domain.AgentPlanStep) string {
	title := strings.TrimSpace(step.Title)
	if title == "" {
		title = strings.TrimSpace(step.CapabilityKey)
	}
	if title == "" {
		return "step"
	}
	if step.CapabilityKey == "" {
		return title
	}
	return title + " (" + step.CapabilityKey + ")"
}

func firstFailedPlanStep(plan domain.AgentPlan) domain.AgentPlanStep {
	for _, step := range plan.Steps {
		if step.Status == domain.AgentPlanStepStatusFailed {
			return step
		}
	}
	return domain.AgentPlanStep{}
}

func planProgressNotificationStep(plan domain.AgentPlan) domain.AgentPlanStep {
	if failed := firstFailedPlanStep(plan); failed.ID > 0 {
		return failed
	}
	for index := len(plan.Steps) - 1; index >= 0; index-- {
		if plan.Steps[index].Status == domain.AgentPlanStepStatusExecuting {
			return plan.Steps[index]
		}
	}
	for index := len(plan.Steps) - 1; index >= 0; index-- {
		if plan.Steps[index].Status == domain.AgentPlanStepStatusCompleted {
			return plan.Steps[index]
		}
	}
	return domain.AgentPlanStep{}
}

func progressNotificationTraceEventName(stage string) string {
	if strings.TrimSpace(stage) == "periodic_progress" {
		return "periodic_progress_delivery"
	}
	return "plan_progress_delivery"
}

func agentPlanTerminal(status domain.AgentPlanStatus) bool {
	switch status {
	case domain.AgentPlanStatusCompleted, domain.AgentPlanStatusFailed, domain.AgentPlanStatusExpired:
		return true
	default:
		return false
	}
}

func (s *AgentConversationService) agentPlanURL(planID int64) string {
	path := fmt.Sprintf("/agent/plans/%d", planID)
	if s == nil || s.publicBaseURL == "" {
		return path
	}
	return s.publicBaseURL + path
}

func (s *AgentConversationService) agentPlanURLIfAvailable(planID int64) string {
	if planID < 1 {
		return ""
	}
	return s.agentPlanURL(planID)
}

func (s *AgentConversationService) agentApprovalURL(token string) string {
	path := "/agent/approvals/" + strings.TrimSpace(token)
	if s == nil || s.publicBaseURL == "" {
		return path
	}
	return s.publicBaseURL + path
}
