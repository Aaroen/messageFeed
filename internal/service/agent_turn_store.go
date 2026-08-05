package service

import (
	"context"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"strings"
	"time"
)

type agentTurnStoreAdapter struct {
	repository AgentConversationRepository
	workerID   string
}

func newAgentTurnStoreAdapter(repository AgentConversationRepository, workerID string) agent.TurnStore {
	return agentTurnStoreAdapter{repository: repository, workerID: strings.TrimSpace(workerID)}
}

func (s agentTurnStoreAdapter) UpdateTurn(ctx context.Context, turn domain.AgentTurn) (domain.AgentTurn, error) {
	return updateAgentTurn(ctx, s.repository, turn, s.workerID)
}

func (s agentTurnStoreAdapter) AppendTranscriptEntry(ctx context.Context, entry domain.AgentTranscriptEntry) (domain.AgentTranscriptEntry, error) {
	return s.repository.AppendTranscriptEntry(ctx, entry)
}

func (s agentTurnStoreAdapter) UpdateInboundMessageStatus(ctx context.Context, userID int64, id int64, status domain.AgentInboundMessageStatus, now time.Time) (domain.AgentInboundMessage, error) {
	return s.repository.UpdateInboundMessageStatus(ctx, userID, id, status, now)
}

func updateAgentTurn(ctx context.Context, repository AgentConversationRepository, turn domain.AgentTurn, workerID string) (domain.AgentTurn, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID != "" {
		if owned, ok := repository.(interface {
			UpdateTurnIfOwned(context.Context, domain.AgentTurn, string) (domain.AgentTurn, error)
		}); ok {
			return owned.UpdateTurnIfOwned(ctx, turn, workerID)
		}
	}
	return repository.UpdateTurn(ctx, turn)
}
