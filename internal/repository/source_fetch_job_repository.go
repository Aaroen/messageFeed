package repository

import (
	"context"
	"errors"
	"messagefeed/internal/domain"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultSourceFetchJobClaimLimit = 20
	maxSourceFetchJobClaimLimit     = 100
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
	var models []sourceFetchJobModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
		return nil, opErr
	}

	jobs := make([]domain.SourceFetchJob, 0, len(models))
	for _, model := range models {
		jobs = append(jobs, sourceFetchJobModelToDomain(model))
	}
	return jobs, nil
}

func (r *SourceFetchJobRepository) UpdateJob(ctx context.Context, job domain.SourceFetchJob) (domain.SourceFetchJob, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.source_fetch_job.update", "update", "source_fetch_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	model := sourceFetchJobModelFromDomain(job)
	result := r.db.WithContext(ctx).
		Model(&sourceFetchJobModel{}).
		Where("user_id = ? AND id = ?", job.UserID, job.ID).
		Select("Status", "TriggerType", "ScheduledAt", "StartedAt", "FinishedAt", "AttemptCount", "MaxAttempts", "Priority", "LockedBy", "LockedAt", "LastError").
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
	return input
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
