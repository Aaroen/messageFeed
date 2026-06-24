package repository

import (
	"context"
	"messagefeed/internal/domain"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AgentApprovalRepository struct {
	db *gorm.DB
}

func NewAgentApprovalRepository(db *gorm.DB) *AgentApprovalRepository {
	return &AgentApprovalRepository{db: db}
}

type agentApprovalModel struct {
	ID                int64 `gorm:"primaryKey"`
	PlanID            *int64
	UserID            int64 `gorm:"not null"`
	ExternalAccountID *int64
	ApprovalTokenHash string
	Channel           string
	Status            string
	ExpiresAt         time.Time
	DecidedAt         *time.Time
	RequestID         string
	TraceID           string
	Metadata          domain.AgentJSON `gorm:"column:metadata_json;serializer:json;type:jsonb;not null"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (agentApprovalModel) TableName() string { return "agent_approvals" }

func (r *AgentApprovalRepository) GetByTokenHash(ctx context.Context, userID int64, tokenHash string) (domain.AgentApproval, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_approval.get", "select", "agent_approvals")
	var opErr error
	defer func() { finish(opErr) }()

	var model agentApprovalModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND approval_token_hash = ?", userID, strings.TrimSpace(tokenHash)).
		First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentApproval{}, opErr
	}
	return agentApprovalModelToDomain(model), nil
}

func (r *AgentApprovalRepository) Decide(ctx context.Context, userID int64, tokenHash string, status domain.AgentApprovalStatus, now time.Time) (domain.AgentApproval, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_approval.decide", "update", "agent_approvals")
	var opErr error
	defer func() { finish(opErr) }()

	var model agentApprovalModel
	result := r.db.WithContext(ctx).
		Model(&model).
		Clauses(clause.Returning{}).
		Where(
			"user_id = ? AND approval_token_hash = ? AND status = ? AND expires_at > ?",
			userID,
			strings.TrimSpace(tokenHash),
			string(domain.AgentApprovalStatusPending),
			now.UTC(),
		).
		Updates(map[string]any{
			"status":     string(status),
			"decided_at": now.UTC(),
			"updated_at": now.UTC(),
		})
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.AgentApproval{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.AgentApproval{}, opErr
	}
	return agentApprovalModelToDomain(model), nil
}

func (r *AgentApprovalRepository) UpdateAgentPlanStatusForApproval(ctx context.Context, userID int64, planID int64, status domain.AgentPlanStatus, now time.Time) error {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_approval.update_plan_status", "update", "agent_plans")
	var opErr error
	defer func() { finish(opErr) }()

	if status != domain.AgentPlanStatusApproved && status != domain.AgentPlanStatusRejected {
		opErr = domain.ErrInvalidInput
		return opErr
	}
	updates := map[string]any{
		"status":     string(status),
		"updated_at": now.UTC(),
	}
	if status == domain.AgentPlanStatusApproved {
		updates["approved_at"] = now.UTC()
	} else {
		updates["rejected_at"] = now.UTC()
	}
	result := r.db.WithContext(ctx).
		Model(&agentPlanModel{}).
		Where("id = ? AND user_id = ? AND status IN ?", planID, userID, []string{string(domain.AgentPlanStatusDraft), string(domain.AgentPlanStatusAwaitingApproval)}).
		Updates(updates)
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return opErr
	}
	return nil
}

func agentApprovalModelToDomain(model agentApprovalModel) domain.AgentApproval {
	return domain.AgentApproval{
		ID:                model.ID,
		PlanID:            model.PlanID,
		UserID:            model.UserID,
		ExternalAccountID: model.ExternalAccountID,
		ApprovalTokenHash: model.ApprovalTokenHash,
		Channel:           model.Channel,
		Status:            domain.AgentApprovalStatus(model.Status),
		ExpiresAt:         model.ExpiresAt,
		DecidedAt:         model.DecidedAt,
		RequestID:         model.RequestID,
		TraceID:           model.TraceID,
		Metadata:          cloneAgentJSON(model.Metadata),
		CreatedAt:         model.CreatedAt,
		UpdatedAt:         model.UpdatedAt,
	}
}
