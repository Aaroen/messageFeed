package repository

import (
	"context"
	"messagefeed/internal/domain"

	"gorm.io/gorm/clause"
)

func (r *AgentRepository) CreateAgentWorkflow(ctx context.Context, workflow domain.AgentWorkflow) (domain.AgentWorkflow, error) {
	workflow.Status = "draft"
	result := r.db.WithContext(ctx).Table("agent_workflows").Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "source_turn_id"}}, DoNothing: true,
	}).Create(&workflow)
	if result.Error != nil {
		return domain.AgentWorkflow{}, mapRepositoryError(result.Error)
	}
	if result.RowsAffected == 0 {
		if err := r.db.WithContext(ctx).Table("agent_workflows").Where("user_id = ? AND source_turn_id = ?", workflow.UserID, workflow.SourceTurnID).First(&workflow).Error; err != nil {
			return domain.AgentWorkflow{}, mapRepositoryError(err)
		}
	}
	return workflow, nil
}

func (r *AgentRepository) GetAgentWorkflow(ctx context.Context, userID, id int64) (domain.AgentWorkflow, error) {
	var workflow domain.AgentWorkflow
	if err := r.db.WithContext(ctx).Table("agent_workflows").Where("user_id = ? AND id = ?", userID, id).First(&workflow).Error; err != nil {
		return domain.AgentWorkflow{}, mapRepositoryError(err)
	}
	return workflow, nil
}

func (r *AgentRepository) ListAgentWorkflows(ctx context.Context, userID int64) ([]domain.AgentWorkflow, error) {
	var workflows []domain.AgentWorkflow
	if err := r.db.WithContext(ctx).Table("agent_workflows").Where("user_id = ? AND status <> 'archived'", userID).Order("id DESC").Limit(50).Find(&workflows).Error; err != nil {
		return nil, mapRepositoryError(err)
	}
	return workflows, nil
}

func (r *AgentRepository) ReviewAgentWorkflow(ctx context.Context, userID, id int64, status string) error {
	if status != "published" && status != "archived" {
		return domain.ErrInvalidInput
	}
	result := r.db.WithContext(ctx).Table("agent_workflows").Where("user_id = ? AND id = ? AND (status = 'draft' OR status = ? OR status = 'published' AND ? = 'archived')", userID, id, status, status).
		Updates(map[string]any{"status": status, "reviewed_at": clause.Expr{SQL: "NOW()"}})
	if result.Error != nil {
		return mapRepositoryError(result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrConflict
	}
	return nil
}
