package service

import (
	"context"
	"fmt"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"messagefeed/internal/notifier"
	"messagefeed/internal/observability"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

func (s *AgentConversationService) sessionLock(sessionID int64) *sync.Mutex {
	s.lockMu.Lock()
	defer s.lockMu.Unlock()
	if s.sessionLocks == nil {
		s.sessionLocks = map[int64]*sync.Mutex{}
	}
	lock := s.sessionLocks[sessionID]
	if lock == nil {
		lock = &sync.Mutex{}
		s.sessionLocks[sessionID] = lock
	}
	return lock
}

// Record 将 agent 内核审计事件落到业务审计表。
func (s *AgentConversationService) Record(ctx context.Context, event agent.AuditEvent) error {
	if s == nil || s.repository == nil {
		return nil
	}
	_, err := s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: event.SessionID,
		TurnID:    event.TurnID,
		UserID:    event.UserID,
		EventType: event.EventType,
		Status:    event.Status,
		Message:   event.Message,
		Metadata:  event.Metadata,
		RequestID: event.RequestID,
		TraceID:   event.TraceID,
		CreatedAt: event.CreatedAt,
	})
	return err
}

func (s *AgentConversationService) sendWeChatWorkReply(ctx context.Context, toUser string, reply string) (notifier.WeChatWorkSendResult, int, error) {
	chunks := splitUTF8Bytes(reply, notifier.WeChatWorkTextByteLimit)
	ctx, span := observability.StartSpan(ctx, "service.agent.send_wechat_work_reply",
		attribute.Int("agent.reply_chunks", len(chunks)),
		attribute.Int("agent.reply_bytes", len([]byte(reply))),
	)
	var sendErr error
	defer func() {
		status := "success"
		if sendErr != nil {
			status = "failed"
		}
		span.SetAttributes(attribute.String("agent.reply_send.status", status))
		observability.EndSpan(span, sendErr)
	}()

	var result notifier.WeChatWorkSendResult
	for i, chunk := range chunks {
		var err error
		span.SetAttributes(attribute.Int("agent.reply_chunk_index", i+1))
		result, err = s.sender.SendText(ctx, notifier.WeChatWorkTextMessage{
			ToUser:  toUser,
			Content: chunk,
		})
		if err != nil {
			sendErr = err
			return result, i, err
		}
	}
	return result, len(chunks), nil
}

func splitUTF8Bytes(value string, limit int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if limit <= 0 || len(value) <= limit {
		return []string{value}
	}
	chunks := make([]string, 0, len(value)/limit+1)
	var builder strings.Builder
	currentBytes := 0
	for _, r := range value {
		part := string(r)
		partBytes := len(part)
		if currentBytes > 0 && currentBytes+partBytes > limit {
			chunks = append(chunks, strings.TrimSpace(builder.String()))
			builder.Reset()
			currentBytes = 0
		}
		builder.WriteString(part)
		currentBytes += partBytes
	}
	if tail := strings.TrimSpace(builder.String()); tail != "" {
		chunks = append(chunks, tail)
	}
	return chunks
}

func normalizeReceiveWeChatWorkInput(input ReceiveWeChatWorkAppMessageInput) ReceiveWeChatWorkAppMessageInput {
	input.Provider = strings.TrimSpace(input.Provider)
	if input.Provider == "" {
		input.Provider = domain.AgentProviderWeChatWorkApp
	}
	input.ProviderMessageID = strings.TrimSpace(input.ProviderMessageID)
	input.CorpID = strings.TrimSpace(input.CorpID)
	input.AgentID = strings.TrimSpace(input.AgentID)
	input.ExternalUserID = strings.TrimSpace(input.ExternalUserID)
	input.ChatID = strings.TrimSpace(input.ChatID)
	if input.ChatID == "" {
		input.ChatID = input.ExternalUserID
	}
	input.ChatType = strings.TrimSpace(input.ChatType)
	if input.ChatType == "" {
		input.ChatType = "direct"
	}
	input.MsgType = strings.TrimSpace(input.MsgType)
	input.TextContent = strings.TrimSpace(input.TextContent)
	input.EventType = strings.TrimSpace(input.EventType)
	input.EventKey = strings.TrimSpace(input.EventKey)
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.TraceID = strings.TrimSpace(input.TraceID)
	return input
}

func validateReceiveWeChatWorkInput(input ReceiveWeChatWorkAppMessageInput) error {
	if input.ProviderMessageID == "" {
		return fmt.Errorf("%w: provider message id is required", domain.ErrInvalidInput)
	}
	if input.CorpID == "" {
		return fmt.Errorf("%w: corp id is required", domain.ErrInvalidInput)
	}
	if input.AgentID == "" {
		return fmt.Errorf("%w: agent id is required", domain.ErrInvalidInput)
	}
	if input.ExternalUserID == "" {
		return fmt.Errorf("%w: external user id is required", domain.ErrInvalidInput)
	}
	if input.MsgType == "" {
		return fmt.Errorf("%w: message type is required", domain.ErrInvalidInput)
	}
	if input.MsgType == "text" && input.TextContent == "" {
		return fmt.Errorf("%w: text content is required", domain.ErrInvalidInput)
	}
	return nil
}

func weChatWorkSessionKey(input ReceiveWeChatWorkAppMessageInput) string {
	return input.CorpID + ":" + input.AgentID + ":" + input.ExternalUserID
}

func (s *AgentConversationService) shouldSendWeChatWorkReply(input ReceiveWeChatWorkAppMessageInput) bool {
	return s != nil &&
		s.sender != nil &&
		input.Provider == domain.AgentProviderWeChatWorkApp &&
		strings.TrimSpace(input.ExternalUserID) != ""
}

func (s *AgentConversationService) shouldSendWeChatWorkNotification(ctx context.Context, userID int64, input ReceiveWeChatWorkAppMessageInput, kind string) bool {
	if !s.shouldSendWeChatWorkReply(input) {
		return false
	}
	preference := s.agentNotificationPreference(ctx, userID)
	switch strings.TrimSpace(kind) {
	case "process":
		return preference.ProcessNotificationsEnabled
	case "failure":
		return preference.FailureNotificationsEnabled
	case "recovery":
		return preference.RecoveryNotificationsEnabled
	case "final":
		return preference.FinalReportsEnabled
	default:
		return true
	}
}

func (s *AgentConversationService) agentNotificationPreference(ctx context.Context, userID int64) domain.AgentNotificationPreference {
	if s == nil || s.repository == nil || userID < 1 {
		return defaultAgentNotificationPreference(userID, time.Time{})
	}
	preference, err := s.repository.GetAgentNotificationPreference(ctx, userID)
	if err != nil {
		return defaultAgentNotificationPreference(userID, s.now().UTC())
	}
	return preference
}

func (s *AgentConversationService) agentTaskAdmissionDecision(ctx context.Context, userID int64, entry string, currentScheduledTaskID int64) agentTaskAdmissionDecision {
	now := s.now().UTC()
	preference := s.agentNotificationPreference(ctx, userID)
	var plans []domain.AgentPlan
	var scheduledTasks []domain.AgentScheduledTask
	if s != nil && s.repository != nil && userID > 0 {
		plans, _ = s.repository.ListAgentPlans(ctx, userID, 0, 0, 100)
		scheduledTasks, _ = s.repository.ListAgentScheduledTasks(ctx, domain.AgentScheduledTaskListOptions{UserID: userID, Limit: 100})
	}
	return evaluateAgentTaskAdmission(agentTaskAdmissionInput{
		UserID:                 userID,
		Entry:                  entry,
		Preference:             preference,
		Plans:                  plans,
		ScheduledTasks:         scheduledTasks,
		CurrentScheduledTaskID: currentScheduledTaskID,
		Now:                    now,
	})
}

func normalizeWebAgentChannel(channel string) string {
	channel = strings.TrimSpace(channel)
	if channel == "" {
		return "web"
	}
	return channel
}

func webAgentSessionKey(userID int64, channel string) string {
	return fmt.Sprintf("web:user:%d:%s", userID, normalizeWebAgentChannel(channel))
}

func agentTurnResponse(turn domain.AgentTurn) AgentTurnResponse {
	return AgentTurnResponse{
		ID:               turn.ID,
		SessionID:        turn.SessionID,
		InboundMessageID: turn.InboundMessageID,
		Status:           string(turn.Status),
		InputText:        turn.InputText,
		OutputText:       turn.OutputText,
		ErrorMessage:     turn.ErrorMessage,
		StartedAt:        formatOptionalTime(&turn.StartedAt),
		FinishedAt:       formatOptionalTime(turn.FinishedAt),
		CreatedAt:        formatOptionalTime(&turn.CreatedAt),
		UpdatedAt:        formatOptionalTime(&turn.UpdatedAt),
	}
}
