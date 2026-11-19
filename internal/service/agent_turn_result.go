package service

import (
	"context"
	"fmt"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"messagefeed/internal/metrics"
	"messagefeed/internal/notifier"
	"strings"
)

// finishTurnWithReply 记录直接回复并关闭当前 turn。
func (s *AgentConversationService) finishTurnWithReply(
	ctx context.Context,
	account domain.ExternalAccount,
	inbound domain.AgentInboundMessage,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
	plan domain.AgentPlan,
	reply string,
	observations []agent.CapabilityObservation,
	auditStatus string,
) (ReceiveWeChatWorkAppMessageResult, error) {
	now := s.now().UTC()
	reply = sanitizeAgentReportText(reply)
	if strings.TrimSpace(reply) == "" {
		cause := domain.NewAppError(domain.ErrorKindUnavailable, "agent_direct_reply_empty", "agent direct reply is empty", "service.agent.finish_turn", true, nil)
		if auditStatus == "failed" {
			return ReceiveWeChatWorkAppMessageResult{}, cause
		}
		return s.failTurnWithFeedback(ctx, account, inbound, session, turn, input, plan, cause)
	}
	failureRequested := auditStatus == "failed"
	failureMessage := turn.ErrorMessage
	turn.Status = domain.AgentTurnStatusSucceeded
	turn.OutputText = reply
	turn.ErrorMessage = ""
	if failureRequested {
		turn.Status = domain.AgentTurnStatusFailed
		turn.ErrorMessage = failureMessage
	}
	completionPlan := domain.AgentPlan{}
	controllerRunID := int64(0)
	if plan.ID > 0 && plan.TurnID == turn.ID && plan.FailedAt == nil && (failureRequested || plan.Status == domain.AgentPlanStatusCompleted || plan.Status == domain.AgentPlanStatusFailed) {
		plan.Metadata = cloneApprovalMetadata(plan.Metadata)
		delete(plan.Metadata, "answer_quality")
		delete(plan.Metadata, "result_quality")
		if failureRequested {
			plan.Status = domain.AgentPlanStatusFailed
			plan.ErrorMessage = turn.ErrorMessage
			for index, step := range plan.Steps {
				plan.Steps[index] = convergeUnobservedPlanStep(step, now)
			}
		}
		if plan.Status == domain.AgentPlanStatusFailed {
			turn.Status = domain.AgentTurnStatusFailed
			turn.ErrorMessage = plan.ErrorMessage
		}
		plan.Metadata["execution_completeness"] = buildAgentExecutionCompletenessMetadata(plan, now)
		plan.Metadata["handoff"] = buildAgentHandoffMetadata(plan, s.agentNotificationPreference(ctx, account.UserID), now)
		plan.Metadata["runtime_observability"] = buildAgentRuntimeObservabilityMetadata(plan, metadataMap(plan.Metadata, "admission_policy"), now)
		completionPlan = plan
		controllerRunID = plan.ControllerRunID
	}
	if turn.Status == domain.AgentTurnStatusFailed {
		if !failureRequested {
			reply = s.generateAgentWeChatFeedbackText(ctx, agentWeChatFeedbackRequest{Stage: "failed", UserMessage: input.TextContent, Plan: plan, ErrorText: turn.ErrorMessage, ProgressURL: s.agentPlanURLIfAvailable(plan.ID)})
		}
		turn.OutputText = reply
		auditStatus = "failed"
	}
	turn.DeliveryStatus = "not_required"
	notificationKind := "final"
	if turn.Status == domain.AgentTurnStatusFailed {
		notificationKind = "failure"
	}
	shouldNotify := s.shouldSendWeChatWorkNotification(ctx, account.UserID, input, notificationKind)
	if shouldNotify {
		turn.DeliveryStatus = "pending"
	}
	now = s.now().UTC()
	completed, err := s.repository.CommitAgentTurnResult(ctx, domain.AgentTurnCompletion{
		Turn: turn, Plan: completionPlan, ControllerRunID: controllerRunID, WorkerID: s.workerID, CompletedAt: now,
		TranscriptMetadata: domain.AgentJSON{"model_provider": turn.ModelProvider, "model": turn.Model, "observations": agent.ObservationMetadata(observations)},
	})
	if err != nil {
		return ReceiveWeChatWorkAppMessageResult{ExternalAccount: account, InboundMessage: inbound, Session: session, Plan: plan}, err
	}
	turn, inbound = completed.Turn, completed.InboundMessage
	if completed.Plan.ID > 0 {
		plan = completed.Plan
	}
	sendResult := notifier.WeChatWorkSendResult{}
	sendCount := 0
	finalDelivery := agentWeChatFinalReportDeliveryResult{}
	replyChunks := splitUTF8Bytes(reply, notifier.WeChatWorkTextByteLimit)
	if shouldNotify {
		if completionPlan.ID > 0 {
			delivery, sendErr := s.sendWeChatWorkFinalReportDelivery(ctx, input.ExternalUserID, plan, reply, string(plan.Status))
			finalDelivery = delivery
			sendResult, sendCount, err = delivery.SendResult, delivery.SendCount, sendErr
			traceStatus := domain.AgentTraceEventSucceeded
			if sendErr != nil {
				traceStatus = domain.AgentTraceEventFailed
			}
			s.recordAgentNotificationTraceEvent(ctx, input, account, session, turn, plan, "final_report_delivery", traceStatus, sendCount, sendErr, agentWeChatFinalReportMetadata(delivery))
		} else {
			sendResult, sendCount, err = s.sendWeChatWorkReply(ctx, input.ExternalUserID, reply)
		}
		status, message := "succeeded", ""
		if err != nil {
			status, message = "failed", err.Error()
		}
		updated, deliveryErr := s.repository.UpdateAgentTurnDelivery(ctx, account.UserID, turn.ID, status, message, s.now().UTC())
		if deliveryErr != nil {
			return ReceiveWeChatWorkAppMessageResult{ExternalAccount: account, InboundMessage: inbound, Session: session, Turn: turn, Plan: plan, Reply: reply}, deliveryErr
		}
		turn = updated
		if err != nil {
			return ReceiveWeChatWorkAppMessageResult{ExternalAccount: account, InboundMessage: inbound, Session: session, Turn: turn, Plan: plan, Reply: reply}, err
		}
	}
	if auditStatus == "" {
		auditStatus = "succeeded"
	}
	eventType, eventMessage := "agent.turn_reply", "agent turn completed with direct reply"
	if completionPlan.ID > 0 && turn.Status == domain.AgentTurnStatusSucceeded {
		eventType, eventMessage = "agent.turn_completed", "agent turn completed"
		if sendCount > 0 {
			eventType, eventMessage = "wechat_work.reply_sent", "wechat work reply sent"
		}
	}
	if turn.Status == domain.AgentTurnStatusFailed {
		eventType, eventMessage = "agent.turn_failed", turn.ErrorMessage
	}
	metrics.AgentReplyBytes.WithLabelValues(input.Provider, auditStatus).Observe(float64(len([]byte(reply))))
	metrics.AgentReplyChunksTotal.WithLabelValues(input.Provider, auditStatus).Add(float64(sendCount))
	_, err = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		EventType: eventType,
		Status:    auditStatus,
		Message:   eventMessage,
		Metadata: domain.AgentJSON{
			"provider_message_id": input.ProviderMessageID,
			"send_count":          sendCount,
			"reply_bytes":         len([]byte(reply)),
			"text_chunks":         len(replyChunks),
			"text_chunk_bytes":    utf8ByteLengths(replyChunks),
			"observations":        agent.ObservationMetadata(observations),
			"message_type":        finalDelivery.DeliveryMode,
			"template_status":     finalDelivery.TemplateStatus,
			"text_status":         finalDelivery.TextStatus,
			"template_error":      finalDelivery.TemplateError,
			"text_error":          finalDelivery.TextError,
			"progress_url":        finalDelivery.ProgressURL,
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: s.now().UTC(),
	})
	if err != nil {
		return ReceiveWeChatWorkAppMessageResult{ExternalAccount: account, InboundMessage: inbound, Session: session, Turn: turn, Plan: plan, Reply: reply, SendResult: sendResult}, err
	}
	if turn.Status == domain.AgentTurnStatusFailed {
		_, err = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{UserID: account.UserID, SessionID: session.ID, TurnID: turn.ID, EventType: "agent.turn_failure_feedback", Status: "sent", Message: "agent turn failure feedback generated", Metadata: domain.AgentJSON{"failure_reason": turn.ErrorMessage, "delivery_status": turn.DeliveryStatus}, RequestID: input.RequestID, TraceID: input.TraceID, CreatedAt: s.now().UTC()})
	}
	return ReceiveWeChatWorkAppMessageResult{
		ExternalAccount: account,
		InboundMessage:  inbound,
		Session:         session,
		Turn:            turn,
		Plan:            plan,
		Reply:           reply,
		SendResult:      sendResult,
	}, err
}

// failTurnWithFeedback 关闭失败 turn，并按通知策略生成降级反馈。
func (s *AgentConversationService) failTurnWithFeedback(
	ctx context.Context,
	account domain.ExternalAccount,
	inbound domain.AgentInboundMessage,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
	plan domain.AgentPlan,
	cause error,
) (ReceiveWeChatWorkAppMessageResult, error) {
	if cause == nil {
		cause = fmt.Errorf("agent turn failed")
	}
	turn.ErrorMessage = cause.Error()
	reply := s.generateAgentWeChatFeedbackText(ctx, agentWeChatFeedbackRequest{Stage: "failed", UserMessage: input.TextContent, Plan: plan, ErrorText: cause.Error(), Cause: cause, ProgressURL: s.agentPlanURLIfAvailable(plan.ID)})
	return s.finishTurnWithReply(ctx, account, inbound, session, turn, input, plan, reply, nil, "failed")
}
