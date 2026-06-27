package service

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"time"
)

// resolveConversationSession 将企业微信外部账号映射到当前可用会话。
func (s *AgentConversationService) resolveConversationSession(ctx context.Context, account domain.ExternalAccount, input ReceiveWeChatWorkAppMessageInput, now time.Time) (domain.AgentSession, error) {
	if account.ActiveAgentSessionID > 0 {
		session, err := s.repository.GetAgentSession(ctx, account.UserID, account.ActiveAgentSessionID)
		if err == nil && session.ExternalAccountID == account.ID && session.Status == domain.AgentSessionStatusActive {
			_ = s.repository.TouchAgentSession(ctx, account.UserID, session.ID, now)
			session.LastActiveAt = now
			return session, nil
		}
		if err != nil && domain.ClassifyError(err) != domain.ErrorKindNotFound {
			return domain.AgentSession{}, err
		}
	}
	return s.repository.GetOrCreateSession(ctx, domain.AgentSession{
		UserID:            account.UserID,
		ExternalAccountID: account.ID,
		Provider:          input.Provider,
		ChannelSessionKey: weChatWorkSessionKey(input),
		Status:            domain.AgentSessionStatusActive,
		Title:             "企业微信对话",
		StartedAt:         now,
		LastActiveAt:      now,
	})
}

// resolveWebConversationSession 将 Web 任务归入指定会话或默认 Web 会话。
func (s *AgentConversationService) resolveWebConversationSession(ctx context.Context, account domain.ExternalAccount, sessionID int64, channel string, now time.Time) (domain.AgentSession, error) {
	if sessionID > 0 {
		session, err := s.repository.GetAgentSession(ctx, account.UserID, sessionID)
		if err != nil {
			return domain.AgentSession{}, err
		}
		if session.Status != domain.AgentSessionStatusActive {
			return domain.AgentSession{}, fmt.Errorf("%w: agent session is not active", domain.ErrInvalidInput)
		}
		_ = s.repository.TouchAgentSession(ctx, account.UserID, session.ID, now)
		session.LastActiveAt = now
		return session, nil
	}
	return s.repository.GetOrCreateSession(ctx, domain.AgentSession{
		UserID:            account.UserID,
		ExternalAccountID: account.ID,
		Provider:          domain.AgentProviderWeb,
		ChannelSessionKey: webAgentSessionKey(account.UserID, channel),
		Status:            domain.AgentSessionStatusActive,
		Title:             "Web Agent 任务",
		StartedAt:         now,
		LastActiveAt:      now,
	})
}
