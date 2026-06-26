package service

import (
	"context"
	"messagefeed/internal/domain"
	"strings"
)

func (s *AgentConversationService) sendWebAgentTaskFinalReport(
	ctx context.Context,
	account domain.ExternalAccount,
	inbound domain.AgentInboundMessage,
	session domain.AgentSession,
	turn domain.AgentTurn,
	plan domain.AgentPlan,
	reply string,
	input ReceiveWeChatWorkAppMessageInput,
) error {
	if s == nil || s.repository == nil || account.UserID < 1 || plan.ID < 1 {
		return nil
	}
	if s.sender == nil {
		s.recordWebAgentTaskFinalReportSkipped(ctx, account, inbound, session, turn, plan, input, "wechat work sender is unavailable")
		return nil
	}
	target, ok, reason := s.webAgentTaskFinalReportTarget(ctx, account.UserID)
	if !ok {
		s.recordWebAgentTaskFinalReportSkipped(ctx, account, inbound, session, turn, plan, input, reason)
		return nil
	}
	delivery, err := s.sendWeChatWorkFinalReportDelivery(ctx, target.ExternalUserID, plan, reply, string(plan.Status))
	status := "succeeded"
	message := "web agent task final report sent to wechat work"
	if err != nil {
		status = "failed"
		message = strings.TrimSpace(err.Error())
		if message == "" {
			message = "web agent task final report delivery failed"
		}
	}
	metadata := agentWeChatFinalReportMetadata(delivery)
	metadata["source_provider"] = domain.AgentProviderWeb
	metadata["provider_message_id"] = input.ProviderMessageID
	metadata["plan_id"] = plan.ID
	metadata["target_account_id"] = target.ID
	metadata["target_corp_id"] = target.CorpID
	metadata["target_agent_id"] = target.AgentID
	metadata["target_external_user_id"] = target.ExternalUserID
	metadata["wechat_msgid"] = delivery.SendResult.MessageID
	metadata["invalid_user"] = delivery.SendResult.InvalidUser
	metadata["send_count"] = delivery.SendCount
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		EventType: "wechat_work.reply_sent",
		Status:    status,
		Message:   message,
		Metadata:  metadata,
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: s.now().UTC(),
	})
	return err
}

func (s *AgentConversationService) webAgentTaskFinalReportTarget(ctx context.Context, userID int64) (domain.ExternalAccount, bool, string) {
	if s == nil || s.repository == nil || userID < 1 {
		return domain.ExternalAccount{}, false, "invalid user"
	}
	accounts, err := s.repository.ListExternalAccounts(ctx, userID)
	if err != nil {
		return domain.ExternalAccount{}, false, err.Error()
	}
	var fallback domain.ExternalAccount
	for _, account := range accounts {
		if account.Provider != domain.AgentProviderWeChatWorkApp ||
			account.BindingStatus != domain.ExternalAccountBindingStatusActive ||
			strings.TrimSpace(account.ExternalUserID) == "" {
			continue
		}
		if fallback.ID == 0 {
			fallback = account
		}
		if account.ActiveAgentSessionID > 0 {
			return account, true, ""
		}
	}
	if fallback.ID > 0 {
		return fallback, true, ""
	}
	return domain.ExternalAccount{}, false, "active wechat work binding not found"
}

func (s *AgentConversationService) recordWebAgentTaskFinalReportSkipped(
	ctx context.Context,
	account domain.ExternalAccount,
	inbound domain.AgentInboundMessage,
	session domain.AgentSession,
	turn domain.AgentTurn,
	plan domain.AgentPlan,
	input ReceiveWeChatWorkAppMessageInput,
	reason string,
) {
	if s == nil || s.repository == nil || account.UserID < 1 || plan.ID < 1 {
		return
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "web task final report delivery skipped"
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		EventType: "agent.web_task_final_report_skipped",
		Status:    "skipped",
		Message:   reason,
		Metadata: domain.AgentJSON{
			"source_provider":     domain.AgentProviderWeb,
			"provider_message_id": input.ProviderMessageID,
			"inbound_message_id":  inbound.ID,
			"plan_id":             plan.ID,
			"progress_url":        s.agentPlanURL(plan.ID),
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: s.now().UTC(),
	})
}
