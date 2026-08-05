package repository

import (
	"context"
	"errors"
	"messagefeed/internal/domain"
	"messagefeed/internal/metrics"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultSourceFetchJobClaimLimit = 20
	maxSourceFetchJobClaimLimit     = 100
	defaultSourceFetchJobLease      = 2 * time.Minute
	sourceFetchJobQueueName         = "source_fetch"
	defaultSourceFetchJobListLimit  = 20
	maxSourceFetchJobListLimit      = 100
)

type SourceFetchJobRepository struct {
	db *gorm.DB
}

func NewSourceFetchJobRepository(db *gorm.DB) *SourceFetchJobRepository {
	return &SourceFetchJobRepository{db: db}
}

type sourceFetchJobModel struct {
	ID           int64 `gorm:"primaryKey"`
	UserID       int64 `gorm:"not null"`
	SourceID     int64 `gorm:"not null"`
	Status       string
	TriggerType  string
	ScheduledAt  time.Time
	StartedAt    *time.Time
	FinishedAt   *time.Time
	AttemptCount int
	MaxAttempts  int
	Priority     int
	LockedBy     string
	LockedAt     *time.Time
	LeaseUntil   *time.Time
	LastError    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type sourceFetchAttemptModel struct {
	ID            int64 `gorm:"primaryKey"`
	JobID         int64 `gorm:"not null"`
	SourceID      int64 `gorm:"not null"`
	AttemptNumber int
	Status        string
	StartedAt     time.Time
	FinishedAt    *time.Time
	DurationMS    *int
	HTTPStatus    *int
	ErrorMessage  string
	ItemCount     int
	CreatedCount  int
	UpdatedCount  int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (sourceFetchJobModel) TableName() string {
	return "source_fetch_jobs"
}

func (sourceFetchAttemptModel) TableName() string {
	return "source_fetch_attempts"
}

func (r *SourceFetchJobRepository) CreateJob(ctx context.Context, job domain.SourceFetchJob) (domain.SourceFetchJob, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.source_fetch_job.create", "create", "source_fetch_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	var existing sourceFetchJobModel
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND source_id = ? AND status IN ?", job.UserID, job.SourceID, []string{
			string(domain.SourceFetchJobStatusQueued),
			string(domain.SourceFetchJobStatusRunning),
		}).
		Order("created_at DESC, id DESC").
		First(&existing).Error
	if err == nil {
		return sourceFetchJobModelToDomain(existing), nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		opErr = mapRepositoryError(err)
		return domain.SourceFetchJob{}, opErr
	}

	model := sourceFetchJobModelFromDomain(job)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.SourceFetchJob{}, opErr
	}
	return sourceFetchJobModelToDomain(model), nil
}

func (r *SourceFetchJobRepository) ClaimDueJobs(ctx context.Context, input domain.SourceFetchJobClaimInput) ([]domain.SourceFetchJob, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.source_fetch_job.claim_due", "update", "source_fetch_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	input = normalizeSourceFetchJobClaimInput(input)
	claimStarted := time.Now()
	var models []sourceFetchJobModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := recoverExpiredSourceFetchJobs(tx, input); err != nil {
			return err
		}
		var ids []int64
		if err := tx.WithContext(ctx).
			Model(&sourceFetchJobModel{}).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND scheduled_at <= ?", string(domain.SourceFetchJobStatusQueued), input.Now).
			Order("priority DESC, scheduled_at ASC, id ASC").
			Limit(input.Limit).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}

		updates := map[string]interface{}{
			"status":        string(domain.SourceFetchJobStatusRunning),
			"started_at":    input.Now,
			"locked_at":     input.Now,
			"locked_by":     input.WorkerID,
			"lease_until":   input.Now.Add(input.LeaseDuration),
			"attempt_count": gorm.Expr("attempt_count + ?", 1),
			"updated_at":    input.Now,
		}
		if err := tx.WithContext(ctx).
			Model(&sourceFetchJobModel{}).
			Where("id IN ?", ids).
			Updates(updates).Error; err != nil {
			return err
		}

		return tx.WithContext(ctx).
			Where("id IN ?", ids).
			Order("priority DESC, scheduled_at ASC, id ASC").
			Find(&models).Error
	})
	if err != nil {
		opErr = mapRepositoryError(err)
		metrics.TaskQueueClaimDuration.WithLabelValues(sourceFetchJobQueueName).Observe(time.Since(claimStarted).Seconds())
		return nil, opErr
	}
	metrics.TaskQueueClaimDuration.WithLabelValues(sourceFetchJobQueueName).Observe(time.Since(claimStarted).Seconds())
	r.observeSourceFetchQueueState(ctx, input.Now)

	jobs := make([]domain.SourceFetchJob, 0, len(models))
	for _, model := range models {
		jobs = append(jobs, sourceFetchJobModelToDomain(model))
	}
	return jobs, nil
}

func (r *SourceFetchJobRepository) UpdateJob(ctx context.Context, job domain.SourceFetchJob) (domain.SourceFetchJob, error) {
	return r.updateJob(ctx, job, "")
}

func (r *SourceFetchJobRepository) UpdateJobIfOwned(ctx context.Context, job domain.SourceFetchJob, workerID string) (domain.SourceFetchJob, error) {
	return r.updateJob(ctx, job, strings.TrimSpace(workerID))
}

func (r *SourceFetchJobRepository) updateJob(ctx context.Context, job domain.SourceFetchJob, workerID string) (domain.SourceFetchJob, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.source_fetch_job.update", "update", "source_fetch_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	model := sourceFetchJobModelFromDomain(job)
	result := r.db.WithContext(ctx).
		Model(&sourceFetchJobModel{}).
		Where("user_id = ? AND id = ?", job.UserID, job.ID).
		Select("Status", "TriggerType", "ScheduledAt", "StartedAt", "FinishedAt", "AttemptCount", "MaxAttempts", "Priority", "LockedBy", "LockedAt", "LeaseUntil", "LastError")
	if workerID != "" {
		result = result.Where("locked_by = ? AND status = ?", workerID, string(domain.SourceFetchJobStatusRunning))
	}
	result = result.
		Updates(&model)
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.SourceFetchJob{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.SourceFetchJob{}, opErr
	}

	var updated sourceFetchJobModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND id = ?", job.UserID, job.ID).
		First(&updated).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.SourceFetchJob{}, opErr
	}
	return sourceFetchJobModelToDomain(updated), nil
}

func (r *SourceFetchJobRepository) ListJobsByUser(ctx context.Context, options domain.SourceFetchJobListOptions) (domain.SourceFetchJobListResult, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.source_fetch_job.list_by_user", "select", "source_fetch_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	options = normalizeSourceFetchJobListOptions(options)
	query := r.db.WithContext(ctx).Model(&sourceFetchJobModel{}).Where("user_id = ?", options.UserID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.SourceFetchJobListResult{}, opErr
	}

	var models []sourceFetchJobModel
	if err := query.
		Order("created_at DESC, id DESC").
		Limit(options.Limit).
		Offset(options.Offset).
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.SourceFetchJobListResult{}, opErr
	}

	jobs := make([]domain.SourceFetchJob, 0, len(models))
	for _, model := range models {
		jobs = append(jobs, sourceFetchJobModelToDomain(model))
	}
	return domain.SourceFetchJobListResult{
		Jobs:   jobs,
		Total:  total,
		Limit:  options.Limit,
		Offset: options.Offset,
	}, nil
}

func (r *SourceFetchJobRepository) ListJobsByIDs(ctx context.Context, options domain.SourceFetchJobListByIDsOptions) ([]domain.SourceFetchJob, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.source_fetch_job.list_by_ids", "select", "source_fetch_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	ids := uniquePositiveInt64s(options.IDs)
	if len(ids) == 0 {
		return nil, nil
	}

	var models []sourceFetchJobModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND id IN ?", options.UserID, ids).
		Order("id ASC").
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}

	jobs := make([]domain.SourceFetchJob, 0, len(models))
	for _, model := range models {
		jobs = append(jobs, sourceFetchJobModelToDomain(model))
	}
	return jobs, nil
}

func (r *SourceFetchJobRepository) ListAttemptsByJob(ctx context.Context, options domain.SourceFetchAttemptListOptions) (domain.SourceFetchAttemptListResult, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.source_fetch_attempt.list_by_job", "select", "source_fetch_attempts")
	var opErr error
	defer func() { finish(opErr) }()

	options = normalizeSourceFetchAttemptListOptions(options)
	query := r.db.WithContext(ctx).
		Model(&sourceFetchAttemptModel{}).
		Joins("JOIN source_fetch_jobs ON source_fetch_jobs.id = source_fetch_attempts.job_id").
		Where("source_fetch_jobs.user_id = ? AND source_fetch_attempts.job_id = ?", options.UserID, options.JobID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.SourceFetchAttemptListResult{}, opErr
	}

	var models []sourceFetchAttemptModel
	if err := query.
		Order("source_fetch_attempts.attempt_number ASC, source_fetch_attempts.id ASC").
		Limit(options.Limit).
		Offset(options.Offset).
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.SourceFetchAttemptListResult{}, opErr
	}

	attempts := make([]domain.SourceFetchAttempt, 0, len(models))
	for _, model := range models {
		attempts = append(attempts, sourceFetchAttemptModelToDomain(model))
	}
	return domain.SourceFetchAttemptListResult{
		Attempts: attempts,
		Total:    total,
		Limit:    options.Limit,
		Offset:   options.Offset,
	}, nil
}

func uniquePositiveInt64s(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value < 1 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (r *SourceFetchJobRepository) CreateAttempt(ctx context.Context, attempt domain.SourceFetchAttempt) (domain.SourceFetchAttempt, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.source_fetch_attempt.create", "create", "source_fetch_attempts")
	var opErr error
	defer func() { finish(opErr) }()

	model := sourceFetchAttemptModelFromDomain(attempt)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.SourceFetchAttempt{}, opErr
	}
	return sourceFetchAttemptModelToDomain(model), nil
}

func normalizeSourceFetchJobClaimInput(input domain.SourceFetchJobClaimInput) domain.SourceFetchJobClaimInput {
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
		input.Limit = defaultSourceFetchJobClaimLimit
	}
	if input.Limit > maxSourceFetchJobClaimLimit {
		input.Limit = maxSourceFetchJobClaimLimit
	}
	if input.LeaseDuration <= 0 {
		input.LeaseDuration = defaultSourceFetchJobLease
	}
	return input
}

func recoverExpiredSourceFetchJobs(tx *gorm.DB, input domain.SourceFetchJobClaimInput) error {
	base := tx.Model(&sourceFetchJobModel{}).
		Where("status = ? AND lease_until IS NOT NULL AND lease_until <= ?", string(domain.SourceFetchJobStatusRunning), input.Now)
	requeued := base.Where("attempt_count < max_attempts").Updates(map[string]interface{}{
		"status":      string(domain.SourceFetchJobStatusQueued),
		"scheduled_at": input.Now,
		"started_at":  nil,
		"finished_at": nil,
		"locked_by":   "",
		"locked_at":   nil,
		"lease_until": nil,
		"last_error":  gorm.Expr("CASE WHEN COALESCE(last_error, '') = '' THEN ? ELSE last_error END", "worker lease expired"),
		"updated_at":  input.Now,
	})
	if requeued.Error != nil {
		return requeued.Error
	}
	failed := base.Where("attempt_count >= max_attempts").Updates(map[string]interface{}{
		"status":      string(domain.SourceFetchJobStatusFailed),
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
		metrics.TaskQueueLeaseRecoveriesTotal.WithLabelValues(sourceFetchJobQueueName).Add(float64(recovered))
	}
	if failed.RowsAffected > 0 {
		metrics.TaskQueueDeadLettersTotal.WithLabelValues(sourceFetchJobQueueName).Add(float64(failed.RowsAffected))
	}
	return nil
}

func (r *SourceFetchJobRepository) observeSourceFetchQueueState(ctx context.Context, now time.Time) {
	var depth int64
	if err := r.db.WithContext(ctx).Model(&sourceFetchJobModel{}).
		Where("status = ?", string(domain.SourceFetchJobStatusQueued)).Count(&depth).Error; err != nil {
		return
	}
	var oldest time.Time
	if err := r.db.WithContext(ctx).Model(&sourceFetchJobModel{}).
		Where("status = ?", string(domain.SourceFetchJobStatusQueued)).Select("MIN(scheduled_at)").Scan(&oldest).Error; err != nil {
		return
	}
	var oldestPtr *time.Time
	if !oldest.IsZero() {
		oldestPtr = &oldest
	}
	metrics.ObserveTaskQueueState(sourceFetchJobQueueName, depth, oldestPtr, now)
}

func normalizeSourceFetchJobListOptions(options domain.SourceFetchJobListOptions) domain.SourceFetchJobListOptions {
	if options.Limit <= 0 {
		options.Limit = defaultSourceFetchJobListLimit
	}
	if options.Limit > maxSourceFetchJobListLimit {
		options.Limit = maxSourceFetchJobListLimit
	}
	if options.Offset < 0 {
		options.Offset = 0
	}
	return options
}

func normalizeSourceFetchAttemptListOptions(options domain.SourceFetchAttemptListOptions) domain.SourceFetchAttemptListOptions {
	if options.Limit <= 0 {
		options.Limit = defaultSourceFetchJobListLimit
	}
	if options.Limit > maxSourceFetchJobListLimit {
		options.Limit = maxSourceFetchJobListLimit
	}
	if options.Offset < 0 {
		options.Offset = 0
	}
	return options
}

func sourceFetchJobModelFromDomain(job domain.SourceFetchJob) sourceFetchJobModel {
	return sourceFetchJobModel{
		ID:           job.ID,
		UserID:       job.UserID,
		SourceID:     job.SourceID,
		Status:       string(job.Status),
		TriggerType:  string(job.Trigger),
		ScheduledAt:  job.ScheduledAt,
		StartedAt:    job.StartedAt,
		FinishedAt:   job.FinishedAt,
		AttemptCount: job.AttemptCount,
		MaxAttempts:  job.MaxAttempts,
		Priority:     job.Priority,
		LockedBy:     job.LockedBy,
		LockedAt:     job.LockedAt,
		LeaseUntil:   job.LeaseUntil,
		LastError:    job.LastError,
		CreatedAt:    job.CreatedAt,
		UpdatedAt:    job.UpdatedAt,
	}
}

func sourceFetchJobModelToDomain(model sourceFetchJobModel) domain.SourceFetchJob {
	return domain.SourceFetchJob{
		ID:           model.ID,
		UserID:       model.UserID,
		SourceID:     model.SourceID,
		Status:       domain.SourceFetchJobStatus(model.Status),
		Trigger:      domain.SourceFetchTrigger(model.TriggerType),
		ScheduledAt:  model.ScheduledAt,
		StartedAt:    model.StartedAt,
		FinishedAt:   model.FinishedAt,
		AttemptCount: model.AttemptCount,
		MaxAttempts:  model.MaxAttempts,
		Priority:     model.Priority,
		LockedBy:     model.LockedBy,
		LockedAt:     model.LockedAt,
		LeaseUntil:   model.LeaseUntil,
		LastError:    model.LastError,
		CreatedAt:    model.CreatedAt,
		UpdatedAt:    model.UpdatedAt,
	}
}

func sourceFetchAttemptModelFromDomain(attempt domain.SourceFetchAttempt) sourceFetchAttemptModel {
	return sourceFetchAttemptModel{
		ID:            attempt.ID,
		JobID:         attempt.JobID,
		SourceID:      attempt.SourceID,
		AttemptNumber: attempt.AttemptNumber,
		Status:        string(attempt.Status),
		StartedAt:     attempt.StartedAt,
		FinishedAt:    attempt.FinishedAt,
		DurationMS:    attempt.DurationMS,
		HTTPStatus:    attempt.HTTPStatus,
		ErrorMessage:  attempt.ErrorMessage,
		ItemCount:     attempt.ItemCount,
		CreatedCount:  attempt.CreatedCount,
		UpdatedCount:  attempt.UpdatedCount,
		CreatedAt:     attempt.CreatedAt,
		UpdatedAt:     attempt.UpdatedAt,
	}
}

func sourceFetchAttemptModelToDomain(model sourceFetchAttemptModel) domain.SourceFetchAttempt {
	return domain.SourceFetchAttempt{
		ID:            model.ID,
		JobID:         model.JobID,
		SourceID:      model.SourceID,
		AttemptNumber: model.AttemptNumber,
		Status:        domain.SourceFetchAttemptStatus(model.Status),
		StartedAt:     model.StartedAt,
		FinishedAt:    model.FinishedAt,
		DurationMS:    model.DurationMS,
		HTTPStatus:    model.HTTPStatus,
		ErrorMessage:  model.ErrorMessage,
		ItemCount:     model.ItemCount,
		CreatedCount:  model.CreatedCount,
		UpdatedCount:  model.UpdatedCount,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}
}
