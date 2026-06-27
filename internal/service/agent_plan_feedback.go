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

// approvalRequiredReply 构造需要用户确认时的最小可执行反馈。
func (s *AgentConversationService) approvalRequiredReply(plan domain.AgentPlan, token string) string {
	var builder strings.Builder
	builder.WriteString("该操作需要确认后才能继续。\n计划：")
	builder.WriteString(plan.Summary)
	builder.WriteString("\n状态锚点：approval_required/")
	builder.WriteString(string(plan.Status))
	builder.WriteString("\n影响：")
	builder.WriteString(plan.ImpactSummary)
	builder.WriteString("\n权限：")
	builder.WriteString(planPermissionSummary(plan))
	builder.WriteString("\n预算：")
	builder.WriteString(planBudgetSummary(plan))
	builder.WriteString("\n进度摘要：")
	builder.WriteString(s.agentPlanProgressText(plan))
	builder.WriteString("\n审批地址：")
	builder.WriteString(s.agentApprovalURL(token))
	if plan.ID > 0 {
		builder.WriteString("\n进度地址：")
		builder.WriteString(s.agentPlanURL(plan.ID))
	}
	builder.WriteString("\n下一步：打开审批地址确认或拒绝；如需查看实时执行细节，请打开进度地址。")
	builder.WriteString("\n")
	builder.WriteString(s.agentWeChatActionFallbackText(plan, token))
	return builder.String()
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
	s.sendPlanProgressNotification(ctx, account, session, turn, input, plan, "started", "工作已开始")
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
	reply := s.agentPlanProgressNotificationText(plan, stage, title)
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
}

func (s *AgentConversationService) agentPlanProgressNotificationText(plan domain.AgentPlan, stage string, title string) string {
	if stage == "started" {
		return s.agentPlanStartedReply(plan)
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "进度更新"
	}
	var builder strings.Builder
	builder.WriteString(title)
	builder.WriteString("。\n")
	builder.WriteString("进度：")
	builder.WriteString(s.agentPlanWeChatProgressText(plan))
	builder.WriteString("\n下一步：")
	builder.WriteString(agentProgressNextAction(string(plan.Status), true, plan, nil))
	builder.WriteString("\n详情：")
	builder.WriteString(s.agentPlanURL(plan.ID))
	if failedStep := firstFailedPlanStep(plan); failedStep.ID > 0 {
		builder.WriteString("\n失败步骤：")
		builder.WriteString(planStepLabel(failedStep))
		if failedStep.ErrorMessage != "" {
			builder.WriteString(" / ")
			builder.WriteString(safeSummary(failedStep.ErrorMessage, 160))
		}
	}
	return strings.TrimSpace(builder.String())
}

func (s *AgentConversationService) agentPlanStartedReply(plan domain.AgentPlan) string {
	var builder strings.Builder
	builder.WriteString("已开始处理")
	if strings.TrimSpace(plan.Goal) != "" {
		builder.WriteString("：")
		builder.WriteString(strings.TrimSpace(plan.Goal))
	}
	builder.WriteString("。\n")
	builder.WriteString("进度：")
	builder.WriteString(s.agentPlanWeChatProgressText(plan))
	if plan.ID > 0 {
		builder.WriteString("\n详情：")
		builder.WriteString(s.agentPlanURL(plan.ID))
	}
	return strings.TrimSpace(builder.String())
}

func (s *AgentConversationService) agentTurnCompletionReply(plan domain.AgentPlan, reply string) string {
	reply = strings.TrimSpace(reply)
	if reply != "" {
		return reply
	}
	status := "已完成"
	if plan.Status == domain.AgentPlanStatusFailed {
		status = "处理失败"
	}
	var builder strings.Builder
	builder.WriteString(status)
	if plan.ID > 0 {
		builder.WriteString("。详情：")
		builder.WriteString(s.agentPlanURL(plan.ID))
	}
	return builder.String()
}

func (s *AgentConversationService) agentWeChatActionFallbackText(plan domain.AgentPlan, approvalToken string) string {
	progressURL := s.agentPlanURL(plan.ID)
	approvalURL := progressURL
	if strings.TrimSpace(approvalToken) != "" {
		approvalURL = s.agentApprovalURL(approvalToken)
	}
	actions := []string{
		"view_progress=" + progressURL,
		"approval=" + approvalURL,
		"retry_plan=" + progressURL,
		"recover_plan=" + progressURL,
		"cancel_scheduled_task=" + progressURL,
	}
	return "企微动作组件：" + strings.Join(actions, "；")
}

func (s *AgentConversationService) sendWeChatWorkTaskAcceptedFeedback(
	ctx context.Context,
	account domain.ExternalAccount,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
) (string, notifier.WeChatWorkSendResult, int) {
	reply := agentTaskAcceptedFeedbackText()
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

func agentTaskAcceptedFeedbackText() string {
	return "已收到任务，后台正在处理，请稍等。完成后会在这里返回结果。"
}

func (s *AgentConversationService) agentPlanProgressText(plan domain.AgentPlan) string {
	updatedAt := plan.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = s.now().UTC()
	}
	response := agentPlanResponse(plan, true)
	return AgentProgressTextSummary(AgentProgressSnapshot{
		SubjectType: "plan",
		SubjectID:   plan.ID,
		Status:      string(plan.Status),
		Summary:     plan.Summary,
		NextAction:  agentProgressNextAction(string(plan.Status), true, plan, nil),
		Version:     updatedAt.UnixNano(),
		EventCursor: fmt.Sprintf("plan:%d:%s", plan.ID, updatedAt.UTC().Format(time.RFC3339Nano)),
		UpdatedAt:   formatOptionalTime(&updatedAt),
		Plan:        &response,
	})
}

func (s *AgentConversationService) agentPlanWeChatProgressText(plan domain.AgentPlan) string {
	summary := strings.TrimSpace(plan.Goal)
	if summary == "" {
		summary = strings.TrimSpace(plan.Summary)
	}
	if summary == "" {
		summary = "任务处理中"
	}
	status := "处理中"
	switch plan.Status {
	case domain.AgentPlanStatusCompleted:
		status = "已完成"
	case domain.AgentPlanStatusFailed:
		status = "处理失败"
	case domain.AgentPlanStatusAwaitingApproval:
		status = "等待确认"
	case domain.AgentPlanStatusRejected:
		status = "已拒绝"
	case domain.AgentPlanStatusExecuting, domain.AgentPlanStatusApproved:
		status = "处理中"
	}
	return safeSummary(summary, 120) + "，" + status
}

func planStepLabel(step domain.AgentPlanStep) string {
	title := strings.TrimSpace(step.Title)
	if title == "" {
		title = strings.TrimSpace(step.CapabilityKey)
	}
	if title == "" {
		return "执行计划步骤"
	}
	if step.CapabilityKey == "" {
		return title
	}
	return title + "（" + step.CapabilityKey + "）"
}

func firstFailedPlanStep(plan domain.AgentPlan) domain.AgentPlanStep {
	for _, step := range plan.Steps {
		if step.Status == domain.AgentPlanStepStatusFailed {
			return step
		}
	}
	return domain.AgentPlanStep{}
}

func (s *AgentConversationService) agentPlanURL(planID int64) string {
	path := fmt.Sprintf("/agent/plans/%d", planID)
	if s == nil || s.publicBaseURL == "" {
		return path
	}
	return s.publicBaseURL + path
}

func (s *AgentConversationService) agentApprovalURL(token string) string {
	path := "/agent/approvals/" + strings.TrimSpace(token)
	if s == nil || s.publicBaseURL == "" {
		return path
	}
	return s.publicBaseURL + path
}
