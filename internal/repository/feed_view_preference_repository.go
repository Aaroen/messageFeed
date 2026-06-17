package repository

import (
	"context"
	"messagefeed/internal/domain"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FeedViewPreferenceRepository struct {
	db *gorm.DB
}

func NewFeedViewPreferenceRepository(db *gorm.DB) *FeedViewPreferenceRepository {
	return &FeedViewPreferenceRepository{db: db}
}

type feedViewPreferenceModel struct {
	ID        int64 `gorm:"primaryKey"`
	UserID    int64 `gorm:"not null"`
	ViewMode  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (feedViewPreferenceModel) TableName() string {
	return "feed_view_preferences"
}

func (r *FeedViewPreferenceRepository) GetByUser(ctx context.Context, userID int64) (domain.FeedViewPreference, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.feed_view_preference.get_by_user", "select", "feed_view_preferences")
	var opErr error
	defer func() { finish(opErr) }()

	var model feedViewPreferenceModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.FeedViewPreference{}, opErr
	}
	return feedViewPreferenceModelToDomain(model), nil
}

func (r *FeedViewPreferenceRepository) Upsert(ctx context.Context, preference domain.FeedViewPreference) (domain.FeedViewPreference, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.feed_view_preference.upsert", "upsert", "feed_view_preferences")
	var opErr error
	defer func() { finish(opErr) }()

	model := feedViewPreferenceModelFromDomain(preference)
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"view_mode":  model.ViewMode,
			"updated_at": model.UpdatedAt,
		}),
	}).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.FeedViewPreference{}, opErr
	}
	result, err := r.GetByUser(ctx, preference.UserID)
	if err != nil {
		opErr = err
		return domain.FeedViewPreference{}, opErr
	}
	return result, nil
}

func feedViewPreferenceModelFromDomain(preference domain.FeedViewPreference) feedViewPreferenceModel {
	return feedViewPreferenceModel{
		ID:        preference.ID,
		UserID:    preference.UserID,
		ViewMode:  string(preference.ViewMode),
		CreatedAt: preference.CreatedAt,
		UpdatedAt: preference.UpdatedAt,
	}
}

func feedViewPreferenceModelToDomain(model feedViewPreferenceModel) domain.FeedViewPreference {
	return domain.FeedViewPreference{
		ID:        model.ID,
		UserID:    model.UserID,
		ViewMode:  domain.FeedViewMode(model.ViewMode),
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}
