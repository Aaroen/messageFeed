package repository

import (
	"context"
	"messagefeed/internal/domain"
	"messagefeed/internal/metrics"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultItemEventClaimLimit = 20
	maxItemEventClaimLimit     = 100
	defaultItemEventLease      = 2 * time.Minute
	itemEventQueueName         = "item_event"
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
	MaxAttempts  int
	LockedBy     string
	LockedAt     *time.Time
	LeaseUntil   *time.Time
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
	claimStarted := time.Now()
	var models []itemEventModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := recoverExpiredItemEvents(tx, input); err != nil {
			return err
		}
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
			"locked_by":     input.WorkerID,
			"locked_at":     input.Now,
			"lease_until":   input.Now.Add(input.LeaseDuration),
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
		metrics.TaskQueueClaimDuration.WithLabelValues(itemEventQueueName).Observe(time.Since(claimStarted).Seconds())
		return nil, opErr
	}
	metrics.TaskQueueClaimDuration.WithLabelValues(itemEventQueueName).Observe(time.Since(claimStarted).Seconds())
	r.observeItemEventQueueState(ctx, input.Now)

	events := make([]domain.ItemEvent, 0, len(models))
	for _, model := range models {
		events = append(events, itemEventModelToDomain(model))
	}
	return events, nil
}

func (r *ItemEventRepository) MarkProcessed(ctx context.Context, userID int64, id int64, now time.Time) (domain.ItemEvent, error) {
	return r.updateStatus(ctx, userID, id, domain.ItemEventStatusProcessed, "", now, "")
}

func (r *ItemEventRepository) MarkProcessedOwned(ctx context.Context, userID int64, id int64, now time.Time, workerID string) (domain.ItemEvent, error) {
	return r.updateStatus(ctx, userID, id, domain.ItemEventStatusProcessed, "", now, strings.TrimSpace(workerID))
}

func (r *ItemEventRepository) MarkFailed(ctx context.Context, userID int64, id int64, message string, now time.Time) (domain.ItemEvent, error) {
	return r.updateStatus(ctx, userID, id, domain.ItemEventStatusFailed, message, now, "")
}

func (r *ItemEventRepository) MarkFailedOwned(ctx context.Context, userID int64, id int64, message string, now time.Time, workerID string) (domain.ItemEvent, error) {
	return r.updateStatus(ctx, userID, id, domain.ItemEventStatusFailed, message, now, strings.TrimSpace(workerID))
}

func (r *ItemEventRepository) updateStatus(ctx context.Context, userID int64, id int64, status domain.ItemEventStatus, message string, now time.Time, workerID string) (domain.ItemEvent, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.item_event.update_status", "update", "item_events")
	var opErr error
	defer func() { finish(opErr) }()

	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	var stored itemEventModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id)
		if workerID != "" {
			query = query.Where("locked_by = ? AND status = ?", workerID, string(domain.ItemEventStatusProcessing))
		}
		if err := query.Clauses(clause.Locking{Strength: "UPDATE"}).First(&stored).Error; err != nil {
			return err
		}
		updates := map[string]interface{}{"last_error": message, "updated_at": now, "locked_by": "", "locked_at": nil, "lease_until": nil}
		if status == domain.ItemEventStatusProcessed {
			updates["status"] = string(domain.ItemEventStatusProcessed)
			updates["processed_at"] = now
		} else if stored.AttemptCount < stored.MaxAttempts {
			updates["status"] = string(domain.ItemEventStatusPending)
			updates["available_at"] = now.Add(itemEventRetryDelay(stored.AttemptCount))
			metrics.TaskQueueRetriesTotal.WithLabelValues(itemEventQueueName).Inc()
		} else {
			updates["status"] = string(domain.ItemEventStatusFailed)
			updates["processed_at"] = nil
			metrics.TaskQueueDeadLettersTotal.WithLabelValues(itemEventQueueName).Inc()
		}
		return tx.WithContext(ctx).Model(&itemEventModel{}).Where("id = ?", id).Updates(updates).Error
	})
	if err != nil {
		opErr = mapRepositoryError(err)
		return domain.ItemEvent{}, opErr
	}
	if err := r.db.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).First(&stored).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.ItemEvent{}, opErr
	}
	return itemEventModelToDomain(stored), nil
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
	input.WorkerID = strings.TrimSpace(input.WorkerID)
	if input.WorkerID == "" {
		input.WorkerID = "item-event-worker"
	}
	if input.LeaseDuration <= 0 {
		input.LeaseDuration = defaultItemEventLease
	}
	return input
}

func recoverExpiredItemEvents(tx *gorm.DB, input domain.ItemEventClaimInput) error {
	base := tx.Model(&itemEventModel{}).
		Where("status = ? AND lease_until IS NOT NULL AND lease_until <= ?", string(domain.ItemEventStatusProcessing), input.Now)
	requeued := base.Where("attempt_count < max_attempts").Updates(map[string]interface{}{
		"status":       string(domain.ItemEventStatusPending),
		"available_at": input.Now,
		"locked_by":    "",
		"locked_at":    nil,
		"lease_until":  nil,
		"last_error":   gorm.Expr("CASE WHEN COALESCE(last_error, '') = '' THEN ? ELSE last_error END", "worker lease expired"),
		"updated_at":   input.Now,
	})
	if requeued.Error != nil {
		return requeued.Error
	}
	failed := base.Where("attempt_count >= max_attempts").Updates(map[string]interface{}{
		"status":      string(domain.ItemEventStatusFailed),
		"locked_by":   "",
		"locked_at":   nil,
		"lease_until": nil,
		"last_error":  gorm.Expr("CASE WHEN COALESCE(last_error, '') = '' THEN ? ELSE last_error END", "worker lease expired"),
		"updated_at":  input.Now,
	})
	if failed.Error != nil {
		return failed.Error
	}
	recovered := requeued.RowsAffected + failed.RowsAffected
	if recovered > 0 {
		metrics.TaskQueueLeaseRecoveriesTotal.WithLabelValues(itemEventQueueName).Add(float64(recovered))
	}
	if failed.RowsAffected > 0 {
		metrics.TaskQueueDeadLettersTotal.WithLabelValues(itemEventQueueName).Add(float64(failed.RowsAffected))
	}
	return nil
}

func (r *ItemEventRepository) observeItemEventQueueState(ctx context.Context, now time.Time) {
	var depth int64
	if err := r.db.WithContext(ctx).Model(&itemEventModel{}).
		Where("status = ?", string(domain.ItemEventStatusPending)).Count(&depth).Error; err != nil {
		return
	}
	var oldest time.Time
	if err := r.db.WithContext(ctx).Model(&itemEventModel{}).
		Where("status = ?", string(domain.ItemEventStatusPending)).Select("MIN(available_at)").Scan(&oldest).Error; err != nil {
		return
	}
	var oldestPtr *time.Time
	if !oldest.IsZero() {
		oldestPtr = &oldest
	}
	metrics.ObserveTaskQueueState(itemEventQueueName, depth, oldestPtr, now)
}

func itemEventRetryDelay(attemptCount int) time.Duration {
	if attemptCount < 1 {
		attemptCount = 1
	}
	if attemptCount > 5 {
		attemptCount = 5
	}
	return time.Duration(attemptCount*attemptCount) * time.Minute
}

func itemEventModelFromDomain(event domain.ItemEvent) itemEventModel {
	if event.MaxAttempts < 1 {
		event.MaxAttempts = 3
	}
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
		MaxAttempts:  event.MaxAttempts,
		LockedBy:     event.LockedBy,
		LockedAt:     event.LockedAt,
		LeaseUntil:   event.LeaseUntil,
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
		MaxAttempts:  model.MaxAttempts,
		LockedBy:     model.LockedBy,
		LockedAt:     model.LockedAt,
		LeaseUntil:   model.LeaseUntil,
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
