package repository

import (
	"context"
	"messagefeed/internal/domain"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultNotificationJobClaimLimit = 20
	maxNotificationJobClaimLimit     = 100
	defaultNotificationListLimit     = 20
	maxNotificationListLimit         = 100
)

type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

type notificationJobModel struct {
	ID               int64 `gorm:"primaryKey"`
	UserID           int64 `gorm:"not null"`
	AlertCandidateID int64
	AlertRuleID      int64
	AIAnalysisJobID  int64
	SourceID         int64
	ItemID           int64
	Status           string
	Channel          string
	PolicyDecision   domain.AlertPolicyDecision `gorm:"column:policy_decision_json;serializer:json;type:jsonb;not null"`
	Payload          domain.NotificationPayload `gorm:"column:payload_json;serializer:json;type:jsonb;not null"`
	RequestID        string
	TraceID          string
	DedupeKey        string
	ScheduledAt      time.Time
	StartedAt        *time.Time
	FinishedAt       *time.Time
	AttemptCount     int
	MaxAttempts      int
	LockedBy         string
	LockedAt         *time.Time
	LastError        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type notificationDeliveryModel struct {
	ID                int64 `gorm:"primaryKey"`
	NotificationJobID int64 `gorm:"not null"`
	UserID            int64 `gorm:"not null"`
	Channel           string
	Status            string
	RequestID         string
	TraceID           string
	ProviderMessageID string
	ResponseStatus    *int
	ResponseBody      string
	ErrorMessage      string
	SentAt            *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (notificationJobModel) TableName() string {
	return "notification_jobs"
}

func (notificationDeliveryModel) TableName() string {
	return "notification_deliveries"
}

func (r *NotificationRepository) CreateJob(ctx context.Context, job domain.NotificationJob) (domain.NotificationJob, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.notification_job.create", "create", "notification_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	model := notificationJobModelFromDomain(job)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.NotificationJob{}, opErr
	}
	return notificationJobModelToDomain(model), nil
}

func (r *NotificationRepository) ClaimDueJobs(ctx context.Context, input domain.NotificationJobClaimInput) ([]domain.NotificationJob, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.notification_job.claim_due", "update", "notification_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	input = normalizeNotificationJobClaimInput(input)
	var models []notificationJobModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ids []int64
		if err := tx.WithContext(ctx).
			Model(&notificationJobModel{}).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND scheduled_at <= ?", string(domain.NotificationJobStatusQueued), input.Now).
			Order("scheduled_at ASC, id ASC").
			Limit(input.Limit).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}

		updates := map[string]interface{}{
			"status":        string(domain.NotificationJobStatusRunning),
			"started_at":    input.Now,
			"locked_at":     input.Now,
			"locked_by":     input.WorkerID,
			"attempt_count": gorm.Expr("attempt_count + ?", 1),
			"updated_at":    input.Now,
		}
		if err := tx.WithContext(ctx).
			Model(&notificationJobModel{}).
			Where("id IN ?", ids).
			Updates(updates).Error; err != nil {
			return err
		}

		return tx.WithContext(ctx).
			Where("id IN ?", ids).
			Order("scheduled_at ASC, id ASC").
			Find(&models).Error
	})
	if err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}

	jobs := make([]domain.NotificationJob, 0, len(models))
	for _, model := range models {
		jobs = append(jobs, notificationJobModelToDomain(model))
	}
	return jobs, nil
}

func (r *NotificationRepository) UpdateJob(ctx context.Context, job domain.NotificationJob) (domain.NotificationJob, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.notification_job.update", "update", "notification_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	model := notificationJobModelFromDomain(job)
	result := r.db.WithContext(ctx).
		Model(&notificationJobModel{}).
		Where("user_id = ? AND id = ?", job.UserID, job.ID).
		Select("Status", "Channel", "PolicyDecision", "Payload", "RequestID", "TraceID", "ScheduledAt", "StartedAt", "FinishedAt", "AttemptCount", "MaxAttempts", "LockedBy", "LockedAt", "LastError").
		Updates(&model)
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.NotificationJob{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.NotificationJob{}, opErr
	}

	var updated notificationJobModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND id = ?", job.UserID, job.ID).
		First(&updated).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.NotificationJob{}, opErr
	}
	return notificationJobModelToDomain(updated), nil
}

func (r *NotificationRepository) ListJobsByUser(ctx context.Context, options domain.NotificationJobListOptions) (domain.NotificationJobListResult, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.notification_job.list_by_user", "select", "notification_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	options = normalizeNotificationJobListOptions(options)
	query := r.db.WithContext(ctx).Model(&notificationJobModel{}).Where("user_id = ?", options.UserID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.NotificationJobListResult{}, opErr
	}

	var models []notificationJobModel
	if err := query.
		Order("created_at DESC, id DESC").
		Limit(options.Limit).
		Offset(options.Offset).
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.NotificationJobListResult{}, opErr
	}

	jobs := make([]domain.NotificationJob, 0, len(models))
	for _, model := range models {
		jobs = append(jobs, notificationJobModelToDomain(model))
	}
	return domain.NotificationJobListResult{
		Jobs:   jobs,
		Total:  total,
		Limit:  options.Limit,
		Offset: options.Offset,
	}, nil
}

func (r *NotificationRepository) CreateDelivery(ctx context.Context, delivery domain.NotificationDelivery) (domain.NotificationDelivery, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.notification_delivery.create", "create", "notification_deliveries")
	var opErr error
	defer func() { finish(opErr) }()

	model := notificationDeliveryModelFromDomain(delivery)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.NotificationDelivery{}, opErr
	}
	return notificationDeliveryModelToDomain(model), nil
}

func (r *NotificationRepository) ListDeliveriesByJob(ctx context.Context, options domain.NotificationDeliveryListOptions) (domain.NotificationDeliveryListResult, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.notification_delivery.list_by_job", "select", "notification_deliveries")
	var opErr error
	defer func() { finish(opErr) }()

	options = normalizeNotificationDeliveryListOptions(options)
	query := r.db.WithContext(ctx).
		Model(&notificationDeliveryModel{}).
		Where("user_id = ? AND notification_job_id = ?", options.UserID, options.JobID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.NotificationDeliveryListResult{}, opErr
	}

	var models []notificationDeliveryModel
	if err := query.
		Order("created_at DESC, id DESC").
		Limit(options.Limit).
		Offset(options.Offset).
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.NotificationDeliveryListResult{}, opErr
	}

	deliveries := make([]domain.NotificationDelivery, 0, len(models))
	for _, model := range models {
		deliveries = append(deliveries, notificationDeliveryModelToDomain(model))
	}
	return domain.NotificationDeliveryListResult{
		Deliveries: deliveries,
		Total:      total,
		Limit:      options.Limit,
		Offset:     options.Offset,
	}, nil
}

func normalizeNotificationJobClaimInput(input domain.NotificationJobClaimInput) domain.NotificationJobClaimInput {
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	} else {
		input.Now = input.Now.UTC()
	}
	input.WorkerID = strings.TrimSpace(input.WorkerID)
	if input.WorkerID == "" {
		input.WorkerID = "unknown"
	}
	if input.Limit <= 0 {
		input.Limit = defaultNotificationJobClaimLimit
	}
	if input.Limit > maxNotificationJobClaimLimit {
		input.Limit = maxNotificationJobClaimLimit
	}
	return input
}

func normalizeNotificationJobListOptions(options domain.NotificationJobListOptions) domain.NotificationJobListOptions {
	if options.Limit <= 0 {
		options.Limit = defaultNotificationListLimit
	}
	if options.Limit > maxNotificationListLimit {
		options.Limit = maxNotificationListLimit
	}
	if options.Offset < 0 {
		options.Offset = 0
	}
	return options
}

func normalizeNotificationDeliveryListOptions(options domain.NotificationDeliveryListOptions) domain.NotificationDeliveryListOptions {
	if options.Limit <= 0 {
		options.Limit = defaultNotificationListLimit
	}
	if options.Limit > maxNotificationListLimit {
		options.Limit = maxNotificationListLimit
	}
	if options.Offset < 0 {
		options.Offset = 0
	}
	return options
}

func notificationJobModelFromDomain(job domain.NotificationJob) notificationJobModel {
	return notificationJobModel{
		ID:               job.ID,
		UserID:           job.UserID,
		AlertCandidateID: job.AlertCandidateID,
		AlertRuleID:      job.AlertRuleID,
		AIAnalysisJobID:  job.AIAnalysisJobID,
		SourceID:         job.SourceID,
		ItemID:           job.ItemID,
		Status:           string(job.Status),
		Channel:          string(job.Channel),
		PolicyDecision:   cloneAlertPolicyDecision(job.PolicyDecision),
		Payload:          cloneNotificationPayload(job.Payload),
		RequestID:        job.RequestID,
		TraceID:          job.TraceID,
		DedupeKey:        job.DedupeKey,
		ScheduledAt:      job.ScheduledAt,
		StartedAt:        job.StartedAt,
		FinishedAt:       job.FinishedAt,
		AttemptCount:     job.AttemptCount,
		MaxAttempts:      job.MaxAttempts,
		LockedBy:         job.LockedBy,
		LockedAt:         job.LockedAt,
		LastError:        job.LastError,
		CreatedAt:        job.CreatedAt,
		UpdatedAt:        job.UpdatedAt,
	}
}

func notificationJobModelToDomain(model notificationJobModel) domain.NotificationJob {
	return domain.NotificationJob{
		ID:               model.ID,
		UserID:           model.UserID,
		AlertCandidateID: model.AlertCandidateID,
		AlertRuleID:      model.AlertRuleID,
		AIAnalysisJobID:  model.AIAnalysisJobID,
		SourceID:         model.SourceID,
		ItemID:           model.ItemID,
		Status:           domain.NotificationJobStatus(model.Status),
		Channel:          domain.NotificationChannel(model.Channel),
		PolicyDecision:   cloneAlertPolicyDecision(model.PolicyDecision),
		Payload:          cloneNotificationPayload(model.Payload),
		RequestID:        model.RequestID,
		TraceID:          model.TraceID,
		DedupeKey:        model.DedupeKey,
		ScheduledAt:      model.ScheduledAt,
		StartedAt:        model.StartedAt,
		FinishedAt:       model.FinishedAt,
		AttemptCount:     model.AttemptCount,
		MaxAttempts:      model.MaxAttempts,
		LockedBy:         model.LockedBy,
		LockedAt:         model.LockedAt,
		LastError:        model.LastError,
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
	}
}

func notificationDeliveryModelFromDomain(delivery domain.NotificationDelivery) notificationDeliveryModel {
	return notificationDeliveryModel{
		ID:                delivery.ID,
		NotificationJobID: delivery.NotificationJobID,
		UserID:            delivery.UserID,
		Channel:           string(delivery.Channel),
		Status:            string(delivery.Status),
		RequestID:         delivery.RequestID,
		TraceID:           delivery.TraceID,
		ProviderMessageID: delivery.ProviderMessageID,
		ResponseStatus:    delivery.ResponseStatus,
		ResponseBody:      delivery.ResponseBody,
		ErrorMessage:      delivery.ErrorMessage,
		SentAt:            delivery.SentAt,
		CreatedAt:         delivery.CreatedAt,
		UpdatedAt:         delivery.UpdatedAt,
	}
}

func notificationDeliveryModelToDomain(model notificationDeliveryModel) domain.NotificationDelivery {
	return domain.NotificationDelivery{
		ID:                model.ID,
		NotificationJobID: model.NotificationJobID,
		UserID:            model.UserID,
		Channel:           domain.NotificationChannel(model.Channel),
		Status:            domain.NotificationDeliveryStatus(model.Status),
		RequestID:         model.RequestID,
		TraceID:           model.TraceID,
		ProviderMessageID: model.ProviderMessageID,
		ResponseStatus:    model.ResponseStatus,
		ResponseBody:      model.ResponseBody,
		ErrorMessage:      model.ErrorMessage,
		SentAt:            model.SentAt,
		CreatedAt:         model.CreatedAt,
		UpdatedAt:         model.UpdatedAt,
	}
}

func cloneNotificationPayload(payload domain.NotificationPayload) domain.NotificationPayload {
	if payload == nil {
		return nil
	}
	cloned := make(domain.NotificationPayload, len(payload))
	for key, value := range payload {
		cloned[key] = value
	}
	return cloned
}

func cloneAlertPolicyDecision(decision domain.AlertPolicyDecision) domain.AlertPolicyDecision {
	decision.Reasons = append([]string(nil), decision.Reasons...)
	return decision
}
