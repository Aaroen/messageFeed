package repository

import (
	"context"
	"messagefeed/internal/domain"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultItemEventClaimLimit = 20
	maxItemEventClaimLimit     = 100
)

type ItemEventRepository struct {
	db *gorm.DB
}

func NewItemEventRepository(db *gorm.DB) *ItemEventRepository {
	return &ItemEventRepository{db: db}
}

type itemEventModel struct {
	ID           int64 `gorm:"primaryKey"`
	UserID       int64 `gorm:"not null"`
	SourceID     int64 `gorm:"not null"`
	ItemID       int64
	EventType    string
	Status       string
	Payload      domain.ItemEventPayload `gorm:"serializer:json;type:jsonb;not null"`
	DedupeKey    string
	AvailableAt  time.Time
	ProcessedAt  *time.Time
	AttemptCount int
	LastError    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (itemEventModel) TableName() string {
	return "item_events"
}

func (r *ItemEventRepository) Create(ctx context.Context, event domain.ItemEvent) (domain.ItemEvent, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.item_event.create", "create", "item_events")
	var opErr error
	defer func() { finish(opErr) }()

	model := itemEventModelFromDomain(event)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.ItemEvent{}, opErr
	}
	return itemEventModelToDomain(model), nil
}

func (r *ItemEventRepository) ClaimPending(ctx context.Context, input domain.ItemEventClaimInput) ([]domain.ItemEvent, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.item_event.claim_pending", "update", "item_events")
	var opErr error
	defer func() { finish(opErr) }()

	input = normalizeItemEventClaimInput(input)
	var models []itemEventModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ids []int64
		if err := tx.WithContext(ctx).
			Model(&itemEventModel{}).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND available_at <= ?", string(domain.ItemEventStatusPending), input.Now).
			Order("available_at ASC, id ASC").
			Limit(input.Limit).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}

		updates := map[string]interface{}{
			"status":        string(domain.ItemEventStatusProcessing),
			"attempt_count": gorm.Expr("attempt_count + ?", 1),
			"updated_at":    input.Now,
		}
		if err := tx.WithContext(ctx).
			Model(&itemEventModel{}).
			Where("id IN ?", ids).
			Updates(updates).Error; err != nil {
			return err
		}

		return tx.WithContext(ctx).
			Where("id IN ?", ids).
			Order("available_at ASC, id ASC").
			Find(&models).Error
	})
	if err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}

	events := make([]domain.ItemEvent, 0, len(models))
	for _, model := range models {
		events = append(events, itemEventModelToDomain(model))
	}
	return events, nil
}

func (r *ItemEventRepository) MarkProcessed(ctx context.Context, userID int64, id int64, now time.Time) (domain.ItemEvent, error) {
	return r.updateStatus(ctx, userID, id, domain.ItemEventStatusProcessed, "", now)
}

func (r *ItemEventRepository) MarkFailed(ctx context.Context, userID int64, id int64, message string, now time.Time) (domain.ItemEvent, error) {
	return r.updateStatus(ctx, userID, id, domain.ItemEventStatusFailed, message, now)
}

func (r *ItemEventRepository) updateStatus(ctx context.Context, userID int64, id int64, status domain.ItemEventStatus, message string, now time.Time) (domain.ItemEvent, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.item_event.update_status", "update", "item_events")
	var opErr error
	defer func() { finish(opErr) }()

	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	updates := map[string]interface{}{
		"status":     string(status),
		"last_error": message,
		"updated_at": now,
	}
	if status == domain.ItemEventStatusProcessed {
		updates["processed_at"] = now
	}

	result := r.db.WithContext(ctx).
		Model(&itemEventModel{}).
		Where("user_id = ? AND id = ?", userID, id).
		Updates(updates)
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.ItemEvent{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.ItemEvent{}, opErr
	}

	var model itemEventModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND id = ?", userID, id).
		First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.ItemEvent{}, opErr
	}
	return itemEventModelToDomain(model), nil
}

func normalizeItemEventClaimInput(input domain.ItemEventClaimInput) domain.ItemEventClaimInput {
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	} else {
		input.Now = input.Now.UTC()
	}
	if input.Limit <= 0 {
		input.Limit = defaultItemEventClaimLimit
	}
	if input.Limit > maxItemEventClaimLimit {
		input.Limit = maxItemEventClaimLimit
	}
	return input
}

func itemEventModelFromDomain(event domain.ItemEvent) itemEventModel {
	return itemEventModel{
		ID:           event.ID,
		UserID:       event.UserID,
		SourceID:     event.SourceID,
		ItemID:       event.ItemID,
		EventType:    string(event.EventType),
		Status:       string(event.Status),
		Payload:      cloneItemEventPayload(event.Payload),
		DedupeKey:    event.DedupeKey,
		AvailableAt:  event.AvailableAt,
		ProcessedAt:  event.ProcessedAt,
		AttemptCount: event.AttemptCount,
		LastError:    event.LastError,
		CreatedAt:    event.CreatedAt,
		UpdatedAt:    event.UpdatedAt,
	}
}

func itemEventModelToDomain(model itemEventModel) domain.ItemEvent {
	return domain.ItemEvent{
		ID:           model.ID,
		UserID:       model.UserID,
		SourceID:     model.SourceID,
		ItemID:       model.ItemID,
		EventType:    domain.ItemEventType(model.EventType),
		Status:       domain.ItemEventStatus(model.Status),
		Payload:      cloneItemEventPayload(model.Payload),
		DedupeKey:    model.DedupeKey,
		AvailableAt:  model.AvailableAt,
		ProcessedAt:  model.ProcessedAt,
		AttemptCount: model.AttemptCount,
		LastError:    model.LastError,
		CreatedAt:    model.CreatedAt,
		UpdatedAt:    model.UpdatedAt,
	}
}

func cloneItemEventPayload(payload domain.ItemEventPayload) domain.ItemEventPayload {
	if payload == nil {
		return nil
	}
	cloned := make(domain.ItemEventPayload, len(payload))
	for key, value := range payload {
		cloned[key] = value
	}
	return cloned
}
