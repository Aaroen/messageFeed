package service

import (
	"context"
	"fmt"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
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
	reply string,
	observations []agent.CapabilityObservation,
	auditStatus string,
) (ReceiveWeChatWorkAppMessageResult, error) {
	now := s.now().UTC()
	reply = sanitizeAgentReportText(reply)
	if strings.TrimSpace(reply) == "" {
		cause := domain.NewAppError(domain.ErrorKindUnavailable, "agent_direct_reply_empty", "agent direct reply is empty", "service.agent.finish_turn", true, nil)
		return s.failTurnWithFeedback(ctx, account, inbound, session, turn, input, domain.AgentPlan{}, cause), nil
	}
	_, _ = s.repository.AppendTranscriptEntry(ctx, domain.AgentTranscriptEntry{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		Role:      domain.AgentTranscriptRoleAssistant,
		Content:   reply,
		Metadata: domain.AgentJSON{
			"observations": agent.ObservationMetadata(observations),
		},
		CreatedAt: now,
	})
	finishedAt := now
	turn.Status = domain.AgentTurnStatusSucceeded
	turn.OutputText = reply
	turn.FinishedAt = &finishedAt
	turn, _ = s.repository.UpdateTurn(ctx, turn)

	sendResult := notifier.WeChatWorkSendResult{}
	sendCount := 0
	replyChunks := splitUTF8Bytes(reply, notifier.WeChatWorkTextByteLimit)
	if s.shouldSendWeChatWorkNotification(ctx, account.UserID, input, "final") {
		var err error
		sendResult, sendCount, err = s.sendWeChatWorkReply(ctx, input.ExternalUserID, reply)
		if err != nil {
			_, _ = s.repository.UpdateInboundMessageStatus(ctx, account.UserID, inbound.ID, domain.AgentInboundMessageStatusFailed, s.now().UTC())
			return ReceiveWeChatWorkAppMessageResult{ExternalAccount: account, InboundMessage: inbound, Session: session, Turn: turn, Reply: reply}, err
		}
	}
	inbound, _ = s.repository.UpdateInboundMessageStatus(ctx, account.UserID, inbound.ID, domain.AgentInboundMessageStatusSucceeded, s.now().UTC())
	if auditStatus == "" {
		auditStatus = "succeeded"
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		EventType: "agent.turn_reply",
		Status:    auditStatus,
		Message:   "agent turn completed with direct reply",
		Metadata: domain.AgentJSON{
			"provider_message_id": input.ProviderMessageID,
			"send_count":          sendCount,
			"reply_bytes":         len([]byte(reply)),
			"text_chunks":         len(replyChunks),
			"text_chunk_bytes":    utf8ByteLengths(replyChunks),
			"observations":        agent.ObservationMetadata(observations),
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: s.now().UTC(),
	})
	return ReceiveWeChatWorkAppMessageResult{
		ExternalAccount: account,
		InboundMessage:  inbound,
		Session:         session,
		Turn:            turn,
		Reply:           reply,
		SendResult:      sendResult,
	}, nil
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
) ReceiveWeChatWorkAppMessageResult {
	if cause == nil {
		cause = fmt.Errorf("agent turn failed")
	}
	now := s.now().UTC()
	failedTurn := turn
	failedTurn.Status = domain.AgentTurnStatusFailed
	failedTurn.ErrorMessage = cause.Error()
	failedTurn.FinishedAt = &now
	if failedTurn.ID > 0 {
		if updated, err := s.repository.UpdateTurn(ctx, failedTurn); err == nil {
			failedTurn = updated
		}
	}
	if inbound.ID > 0 {
		if updated, err := s.repository.UpdateInboundMessageStatus(ctx, account.UserID, inbound.ID, domain.AgentInboundMessageStatusFailed, now); err == nil {
			inbound = updated
		}
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    failedTurn.ID,
		UserID:    account.UserID,
		EventType: "agent.turn_failed",
		Status:    "failed",
		Message:   cause.Error(),
		Metadata: domain.AgentJSON{
			"provider_message_id": input.ProviderMessageID,
			"plan_id":             plan.ID,
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: now,
	})
	result := s.sendTurnFailureFeedback(ctx, account, inbound, session, turn, failedTurn, input, plan, cause)
	result.InboundMessage = inbound
	result.Plan = plan
	result.Turn = failedTurn
	return result
}

func (s *AgentConversationService) sendTurnFailureFeedback(
	ctx context.Context,
	account domain.ExternalAccount,
	inbound domain.AgentInboundMessage,
	session domain.AgentSession,
	originalTurn domain.AgentTurn,
	failedTurn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
	plan domain.AgentPlan,
	cause error,
) ReceiveWeChatWorkAppMessageResult {
	if failedTurn.ID == 0 {
		failedTurn = originalTurn
	}
	reply := s.generateAgentWeChatFeedbackText(ctx, agentWeChatFeedbackRequest{
		Stage:       "failed",
		UserMessage: input.TextContent,
		Plan:        plan,
		ErrorText:   truncateError(cause.Error(), 500),
		Cause:       cause,
		ProgressURL: s.agentPlanURLIfAvailable(plan.ID),
	})
	if !s.processInline {
		reply = s.agentTurnCompletionReply(ctx, plan, reply)
	}
	reply = sanitizeAgentReportText(reply)
	sendResult := notifier.WeChatWorkSendResult{}
	sendCount := 0
	sendStatus := "skipped"
	finalDelivery := agentWeChatFinalReportDeliveryResult{}
	if s.shouldSendWeChatWorkNotification(ctx, account.UserID, input, "failure") {
		var sendErr error
		finalDelivery, sendErr = s.sendWeChatWorkFinalReportDelivery(ctx, input.ExternalUserID, plan, reply, "failed")
		sendResult = finalDelivery.SendResult
		sendCount = finalDelivery.SendCount
		if sendErr != nil {
			sendStatus = "failed"
		} else {
			sendStatus = "succeeded"
		}
	}
	now := s.now().UTC()
	_, _ = s.repository.AppendTranscriptEntry(ctx, domain.AgentTranscriptEntry{
		SessionID: session.ID,
		TurnID:    failedTurn.ID,
		UserID:    account.UserID,
		Role:      domain.AgentTranscriptRoleAssistant,
		Content:   reply,
		Metadata: domain.AgentJSON{
			"fallback":       true,
			"failure_reason": truncateError(cause.Error(), 500),
			"send_status":    sendStatus,
		},
		CreatedAt: now,
	})
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    failedTurn.ID,
		UserID:    account.UserID,
		EventType: "agent.turn_failure_feedback",
		Status:    sendStatus,
		Message:   "agent turn failed and fallback feedback was generated",
		Metadata: domain.AgentJSON{
			"provider_message_id": input.ProviderMessageID,
			"send_count":          sendCount,
			"failure_reason":      truncateError(cause.Error(), 500),
			"message_type":        finalDelivery.DeliveryMode,
			"template_status":     finalDelivery.TemplateStatus,
			"text_status":         finalDelivery.TextStatus,
			"template_error":      finalDelivery.TemplateError,
			"text_error":          finalDelivery.TextError,
			"progress_url":        finalDelivery.ProgressURL,
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: now,
	})
	return ReceiveWeChatWorkAppMessageResult{
		ExternalAccount: account,
		InboundMessage:  inbound,
		Session:         session,
		Turn:            failedTurn,
		Reply:           reply,
		SendResult:      sendResult,
	}
}

func (s *AgentConversationService) failTurn(ctx context.Context, userID int64, sessionID int64, turn domain.AgentTurn, input ReceiveWeChatWorkAppMessageInput, cause error) (ReceiveWeChatWorkAppMessageResult, error) {
	now := s.now().UTC()
	turn.Status = domain.AgentTurnStatusFailed
	turn.ErrorMessage = cause.Error()
	turn.FinishedAt = &now
	if turn.ID > 0 {
		_, _ = s.repository.UpdateTurn(ctx, turn)
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: sessionID,
		TurnID:    turn.ID,
		UserID:    userID,
		EventType: "wechat_work.reply_failed",
		Status:    "failed",
		Message:   cause.Error(),
		Metadata:  domain.AgentJSON{"provider_message_id": input.ProviderMessageID},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: now,
	})
	return ReceiveWeChatWorkAppMessageResult{Turn: turn}, cause
}
