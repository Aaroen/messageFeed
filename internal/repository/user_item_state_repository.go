package repository

import (
	"context"
	"messagefeed/internal/domain"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserItemStateRepository struct {
	db *gorm.DB
}

func NewUserItemStateRepository(db *gorm.DB) *UserItemStateRepository {
	return &UserItemStateRepository{db: db}
}

type userItemStateModel struct {
	ID          int64 `gorm:"primaryKey"`
	UserID      int64 `gorm:"not null"`
	ItemID      int64 `gorm:"not null"`
	IsRead      bool
	ReadAt      *time.Time
	IsFavorite  bool
	FavoritedAt *time.Time
	IsHidden    bool
	HiddenAt    *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (userItemStateModel) TableName() string {
	return "user_item_states"
}

func (r *UserItemStateRepository) UpdateState(ctx context.Context, update domain.UserItemStateUpdate) (domain.UserItemState, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.user_item_state.update_state", "upsert", "user_item_states")
	var opErr error
	defer func() { finish(opErr) }()

	var result userItemStateModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureItemBelongsToUser(ctx, tx, update.UserID, update.ItemID); err != nil {
			return err
		}

		now := update.Now.UTC()
		if now.IsZero() {
			now = time.Now().UTC()
		}

		model := userItemStateModel{
			UserID: update.UserID,
			ItemID: update.ItemID,
		}
		applyUserItemStateUpdate(&model, update, now)

		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "item_id"}},
			DoUpdates: clause.Assignments(userItemStateUpsertAssignments(update, now)),
		}).Create(&model).Error; err != nil {
			return err
		}

		if err := tx.WithContext(ctx).
			Where("user_id = ? AND item_id = ?", update.UserID, update.ItemID).
			First(&result).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		opErr = mapRepositoryError(err)
		return domain.UserItemState{}, opErr
	}
	return userItemStateModelToDomain(result), nil
}

func ensureItemBelongsToUser(ctx context.Context, db *gorm.DB, userID int64, itemID int64) error {
	var count int64
	if err := db.WithContext(ctx).
		Model(&itemModel{}).
		Joins("JOIN sources ON sources.id = items.source_id").
		Where("items.id = ? AND sources.user_id = ?", itemID, userID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func userItemStateUpsertAssignments(update domain.UserItemStateUpdate, now time.Time) map[string]interface{} {
	assignments := map[string]interface{}{
		"updated_at": now,
	}
	if update.IsRead != nil {
		assignments["is_read"] = *update.IsRead
		assignments["read_at"] = nil
		if *update.IsRead {
			assignments["read_at"] = now
		}
	}
	if update.IsFavorite != nil {
		assignments["is_favorite"] = *update.IsFavorite
		assignments["favorited_at"] = nil
		if *update.IsFavorite {
			assignments["favorited_at"] = now
		}
	}
	if update.IsHidden != nil {
		assignments["is_hidden"] = *update.IsHidden
		assignments["hidden_at"] = nil
		if *update.IsHidden {
			assignments["hidden_at"] = now
		}
	}
	return assignments
}

func applyUserItemStateUpdate(model *userItemStateModel, update domain.UserItemStateUpdate, now time.Time) {
	if update.IsRead != nil {
		model.IsRead = *update.IsRead
		if *update.IsRead {
			model.ReadAt = &now
		} else {
			model.ReadAt = nil
		}
	}
	if update.IsFavorite != nil {
		model.IsFavorite = *update.IsFavorite
		if *update.IsFavorite {
			model.FavoritedAt = &now
		} else {
			model.FavoritedAt = nil
		}
	}
	if update.IsHidden != nil {
		model.IsHidden = *update.IsHidden
		if *update.IsHidden {
			model.HiddenAt = &now
		} else {
			model.HiddenAt = nil
		}
	}
}

func userItemStateModelToDomain(model userItemStateModel) domain.UserItemState {
	return domain.UserItemState{
		ID:          model.ID,
		UserID:      model.UserID,
		ItemID:      model.ItemID,
		IsRead:      model.IsRead,
		ReadAt:      model.ReadAt,
		IsFavorite:  model.IsFavorite,
		FavoritedAt: model.FavoritedAt,
		IsHidden:    model.IsHidden,
		HiddenAt:    model.HiddenAt,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}
}
