package repository

import (
	"context"
	"messagefeed/internal/domain"
	"time"

	"gorm.io/gorm"
)

type AlertCandidateRepository struct {
	db *gorm.DB
}

func NewAlertCandidateRepository(db *gorm.DB) *AlertCandidateRepository {
	return &AlertCandidateRepository{db: db}
}

type alertCandidateModel struct {
	ID             int64 `gorm:"primaryKey"`
	UserID         int64 `gorm:"not null"`
	RuleID         int64 `gorm:"not null"`
	ItemEventID    int64
	SourceID       int64 `gorm:"not null"`
	ItemID         int64
	Status         string
	MatchedReasons []string `gorm:"serializer:json;type:jsonb;not null"`
	DedupeKey      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (alertCandidateModel) TableName() string {
	return "alert_candidates"
}

func (r *AlertCandidateRepository) Create(ctx context.Context, candidate domain.AlertCandidate) (domain.AlertCandidate, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.alert_candidate.create", "create", "alert_candidates")
	var opErr error
	defer func() { finish(opErr) }()

	model := alertCandidateModelFromDomain(candidate)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AlertCandidate{}, opErr
	}
	return alertCandidateModelToDomain(model), nil
}

func alertCandidateModelFromDomain(candidate domain.AlertCandidate) alertCandidateModel {
	return alertCandidateModel{
		ID:             candidate.ID,
		UserID:         candidate.UserID,
		RuleID:         candidate.RuleID,
		ItemEventID:    candidate.ItemEventID,
		SourceID:       candidate.SourceID,
		ItemID:         candidate.ItemID,
		Status:         string(candidate.Status),
		MatchedReasons: append([]string(nil), candidate.MatchedReasons...),
		DedupeKey:      candidate.DedupeKey,
		CreatedAt:      candidate.CreatedAt,
		UpdatedAt:      candidate.UpdatedAt,
	}
}

func alertCandidateModelToDomain(model alertCandidateModel) domain.AlertCandidate {
	return domain.AlertCandidate{
		ID:             model.ID,
		UserID:         model.UserID,
		RuleID:         model.RuleID,
		ItemEventID:    model.ItemEventID,
		SourceID:       model.SourceID,
		ItemID:         model.ItemID,
		Status:         domain.AlertCandidateStatus(model.Status),
		MatchedReasons: append([]string(nil), model.MatchedReasons...),
		DedupeKey:      model.DedupeKey,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}
}
