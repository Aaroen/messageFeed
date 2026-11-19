package repository

import (
	"context"
	"messagefeed/internal/domain"

	"gorm.io/gorm"
)

func (r *AgentRepository) CreateAgentTurnWithTranscript(ctx context.Context, turn domain.AgentTurn, entry domain.AgentTranscriptEntry) (domain.AgentTurn, domain.AgentTranscriptEntry, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		store := NewAgentRepository(tx)
		var err error
		turn, err = store.CreateTurn(ctx, turn)
		if err != nil {
			return err
		}
		entry.TurnID = turn.ID
		entry, err = store.AppendTranscriptEntry(ctx, entry)
		return err
	})
	if err != nil {
		return domain.AgentTurn{}, domain.AgentTranscriptEntry{}, err
	}
	return turn, entry, nil
}
