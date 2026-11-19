package repository

import (
	"context"
	"database/sql"
	"fmt"
	"messagefeed/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *AgentRepository) AppendAgentStreamEvents(ctx context.Context, events []domain.AgentStreamEvent) error {
	if len(events) == 0 {
		return nil
	}
	first := events[0]
	return r.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var session agentSessionModel
		if err := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", first.SessionID, first.UserID).First(&session).Error; err != nil {
			return mapRepositoryError(err)
		}
		var run agentRunModel
		if err := transaction.Where("id = ? AND session_id = ? AND turn_id = ?", first.RunID, first.SessionID, first.TurnID).First(&run).Error; err != nil {
			return mapRepositoryError(err)
		}
		for index, event := range events {
			if event.UserID != first.UserID || event.SessionID != first.SessionID || event.TurnID != first.TurnID || event.RunID != first.RunID {
				return fmt.Errorf("%w: stream event scope differs", domain.ErrInvalidInput)
			}
			if event.Metadata == nil {
				events[index].Metadata = domain.AgentJSON{}
			}
		}
		return transaction.Table("agent_stream_events").Omit("id").Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "run_id"}, {Name: "sequence"}}, DoNothing: true,
		}).Create(&events).Error
	})
}

func (r *AgentRepository) ListAgentStreamEvents(ctx context.Context, userID, sessionID, afterID int64, limit int) ([]domain.AgentStreamEvent, error) {
	var events []domain.AgentStreamEvent
	err := r.db.WithContext(ctx).Table("agent_stream_events").Where("user_id = ? AND session_id = ? AND id > ?", userID, sessionID, afterID).Order("id ASC").Limit(limit).Find(&events).Error
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return events, nil
}

func (r *AgentRepository) GetAgentWorkspace(ctx context.Context, userID, sessionID, beforeTurnID int64) (domain.AgentWorkspaceData, error) {
	return r.getAgentWorkspace(ctx, userID, sessionID, beforeTurnID, true)
}

func (r *AgentRepository) GetAgentWorkspaceState(ctx context.Context, userID, sessionID int64) (domain.AgentWorkspaceData, error) {
	return r.getAgentWorkspace(ctx, userID, sessionID, 0, false)
}

func (r *AgentRepository) getAgentWorkspace(ctx context.Context, userID, sessionID, beforeTurnID int64, includeEvents bool) (domain.AgentWorkspaceData, error) {
	result := domain.AgentWorkspaceData{}
	err := r.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var session agentSessionModel
		if err := transaction.Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
			return mapRepositoryError(err)
		}
		result.Session = agentSessionModelToDomain(session)
		var turns []agentTurnModel
		query := transaction.Select("agent_turns.*, (inbound.payload_json #>> '{model_selection,config_id}')::bigint AS model_config_id").
			Joins("LEFT JOIN agent_inbound_messages AS inbound ON inbound.id = agent_turns.inbound_message_id AND inbound.user_id = agent_turns.user_id AND inbound.provider = 'web'").
			Where("agent_turns.session_id = ? AND agent_turns.user_id = ?", sessionID, userID)
		if beforeTurnID > 0 {
			query = query.Where("agent_turns.id < ?", beforeTurnID)
		}
		if err := query.Order("agent_turns.id DESC").Limit(21).Find(&turns).Error; err != nil {
			return err
		}
		result.HasMore = len(turns) > 20
		if result.HasMore {
			turns = turns[:20]
		}
		turnIDs := make([]int64, 0, len(turns))
		for index := len(turns) - 1; index >= 0; index-- {
			result.Turns = append(result.Turns, agentTurnModelToDomain(turns[index]))
			turnIDs = append(turnIDs, turns[index].ID)
		}
		if len(turnIDs) > 0 {
			var runs []agentRunModel
			if err := transaction.Where("session_id = ? AND turn_id IN ?", sessionID, turnIDs).Order("id ASC").Find(&runs).Error; err != nil {
				return err
			}
			for _, run := range runs {
				result.Runs = append(result.Runs, agentRunModelToDomain(run))
			}
			var plans []agentPlanModel
			if err := transaction.Where("user_id = ? AND session_id = ? AND turn_id IN ?", userID, sessionID, turnIDs).Order("id ASC").Find(&plans).Error; err != nil {
				return err
			}
			for _, plan := range plans {
				result.Plans = append(result.Plans, agentPlanModelToDomain(plan))
			}
			if includeEvents {
				if err := transaction.Table("agent_stream_events").Where("user_id = ? AND session_id = ? AND turn_id IN ?", userID, sessionID, turnIDs).Order("id ASC").Find(&result.Events).Error; err != nil {
					return err
				}
			}
		}
		if !includeEvents {
			return nil
		}
		return transaction.Table("agent_stream_events").Select("COALESCE(MAX(id), 0)").Where("user_id = ? AND session_id = ?", userID, sessionID).Scan(&result.Cursor).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return result, mapRepositoryError(err)
	}
	return result, nil
}
