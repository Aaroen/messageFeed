package repository

import (
	"context"
	"database/sql"
	"messagefeed/internal/domain"
	"messagefeed/internal/metrics"
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
	defaultNotificationJobLease      = 2 * time.Minute
	notificationJobQueueName         = "notification"
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
	AlertCandidateID *int64
	AlertRuleID      *int64
	AIAnalysisJobID  *int64
	SourceID         *int64
	ItemID           *int64
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
	LeaseUntil       *time.Time
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
	claimStarted := time.Now()
	var models []notificationJobModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := recoverExpiredNotificationJobs(tx, input); err != nil {
			return err
		}
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
			"lease_until":   input.Now.Add(input.LeaseDuration),
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
		metrics.TaskQueueClaimDuration.WithLabelValues(notificationJobQueueName).Observe(time.Since(claimStarted).Seconds())
		return nil, opErr
	}
	metrics.TaskQueueClaimDuration.WithLabelValues(notificationJobQueueName).Observe(time.Since(claimStarted).Seconds())
	r.observeNotificationQueueState(ctx, input.Now)

	jobs := make([]domain.NotificationJob, 0, len(models))
	for _, model := range models {
		jobs = append(jobs, notificationJobModelToDomain(model))
	}
	return jobs, nil
}

func (r *NotificationRepository) UpdateJob(ctx context.Context, job domain.NotificationJob) (domain.NotificationJob, error) {
	return r.updateJob(ctx, job, "")
}

func (r *NotificationRepository) UpdateJobIfOwned(ctx context.Context, job domain.NotificationJob, workerID string) (domain.NotificationJob, error) {
	return r.updateJob(ctx, job, strings.TrimSpace(workerID))
}

func (r *NotificationRepository) updateJob(ctx context.Context, job domain.NotificationJob, workerID string) (domain.NotificationJob, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.notification_job.update", "update", "notification_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	model := notificationJobModelFromDomain(job)
	result := r.db.WithContext(ctx).
		Model(&notificationJobModel{}).
		Where("user_id = ? AND id = ?", job.UserID, job.ID)
	if workerID != "" {
		result = result.Where("locked_by = ? AND status = ?", workerID, string(domain.NotificationJobStatusRunning))
	}
	result = result.
		Select("Status", "Channel", "PolicyDecision", "Payload", "RequestID", "TraceID", "ScheduledAt", "StartedAt", "FinishedAt", "AttemptCount", "MaxAttempts", "LockedBy", "LockedAt", "LeaseUntil", "LastError").
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
	if input.LeaseDuration <= 0 {
		input.LeaseDuration = defaultNotificationJobLease
	}
	return input
}

func recoverExpiredNotificationJobs(tx *gorm.DB, input domain.NotificationJobClaimInput) error {
	base := tx.Model(&notificationJobModel{}).
		Where("status = ? AND lease_until IS NOT NULL AND lease_until <= ?", string(domain.NotificationJobStatusRunning), input.Now)
	requeued := base.Where("attempt_count < max_attempts").Updates(map[string]interface{}{
		"status":       string(domain.NotificationJobStatusQueued),
		"scheduled_at": input.Now,
		"started_at":   nil,
		"finished_at":  nil,
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
		"status":      string(domain.NotificationJobStatusFailed),
		"finished_at": input.Now,
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
		metrics.TaskQueueLeaseRecoveriesTotal.WithLabelValues(notificationJobQueueName).Add(float64(recovered))
	}
	if failed.RowsAffected > 0 {
		metrics.TaskQueueDeadLettersTotal.WithLabelValues(notificationJobQueueName).Add(float64(failed.RowsAffected))
	}
	return nil
}

func (r *NotificationRepository) observeNotificationQueueState(ctx context.Context, now time.Time) {
	var depth int64
	if err := r.db.WithContext(ctx).Model(&notificationJobModel{}).
		Where("status = ?", string(domain.NotificationJobStatusQueued)).Count(&depth).Error; err != nil {
		return
	}
	var oldest sql.NullTime
	if err := r.db.WithContext(ctx).Model(&notificationJobModel{}).
		Where("status = ?", string(domain.NotificationJobStatusQueued)).Select("MIN(scheduled_at)").Scan(&oldest).Error; err != nil {
		return
	}
	var oldestPtr *time.Time
	if oldest.Valid {
		oldestPtr = &oldest.Time
	}
	metrics.ObserveTaskQueueState(notificationJobQueueName, depth, oldestPtr, now)
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
		AlertCandidateID: int64PtrOrNil(job.AlertCandidateID),
		AlertRuleID:      int64PtrOrNil(job.AlertRuleID),
		AIAnalysisJobID:  int64PtrOrNil(job.AIAnalysisJobID),
		SourceID:         int64PtrOrNil(job.SourceID),
		ItemID:           int64PtrOrNil(job.ItemID),
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
		LeaseUntil:       job.LeaseUntil,
		LastError:        job.LastError,
		CreatedAt:        job.CreatedAt,
		UpdatedAt:        job.UpdatedAt,
	}
}

func notificationJobModelToDomain(model notificationJobModel) domain.NotificationJob {
	return domain.NotificationJob{
		ID:               model.ID,
		UserID:           model.UserID,
		AlertCandidateID: int64ValueOrZero(model.AlertCandidateID),
		AlertRuleID:      int64ValueOrZero(model.AlertRuleID),
		AIAnalysisJobID:  int64ValueOrZero(model.AIAnalysisJobID),
		SourceID:         int64ValueOrZero(model.SourceID),
		ItemID:           int64ValueOrZero(model.ItemID),
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
		LeaseUntil:       model.LeaseUntil,
		LastError:        model.LastError,
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
	}
}

func int64PtrOrNil(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func int64ValueOrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
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
