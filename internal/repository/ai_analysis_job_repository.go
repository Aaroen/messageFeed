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
	defaultAIAnalysisJobClaimLimit = 20
	maxAIAnalysisJobClaimLimit     = 100
	defaultAIAnalysisJobListLimit  = 20
	maxAIAnalysisJobListLimit      = 100
	defaultAIAnalysisJobLease      = 2 * time.Minute
	aiAnalysisJobQueueName         = "ai_analysis"
)

type AIAnalysisJobRepository struct {
	db *gorm.DB
}

func NewAIAnalysisJobRepository(db *gorm.DB) *AIAnalysisJobRepository {
	return &AIAnalysisJobRepository{db: db}
}

type aiAnalysisJobModel struct {
	ID               int64 `gorm:"primaryKey"`
	UserID           int64 `gorm:"not null"`
	AlertCandidateID int64 `gorm:"not null"`
	SourceID         int64 `gorm:"not null"`
	ItemID           int64
	Status           string
	Input            domain.AIAnalysisJobInput `gorm:"column:input_json;serializer:json;type:jsonb;not null"`
	Result           domain.AIAnalysisResult   `gorm:"column:result_json;serializer:json;type:jsonb;not null"`
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

func (aiAnalysisJobModel) TableName() string {
	return "ai_analysis_jobs"
}

func (r *AIAnalysisJobRepository) Create(ctx context.Context, job domain.AIAnalysisJob) (domain.AIAnalysisJob, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.ai_analysis_job.create", "create", "ai_analysis_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	model := aiAnalysisJobModelFromDomain(job)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AIAnalysisJob{}, opErr
	}
	return aiAnalysisJobModelToDomain(model), nil
}

func (r *AIAnalysisJobRepository) ClaimDue(ctx context.Context, input domain.AIAnalysisJobClaimInput) ([]domain.AIAnalysisJob, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.ai_analysis_job.claim_due", "update", "ai_analysis_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	input = normalizeAIAnalysisJobClaimInput(input)
	claimStarted := time.Now()
	var models []aiAnalysisJobModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := recoverExpiredAIAnalysisJobs(tx, input); err != nil {
			return err
		}
		var ids []int64
		if err := tx.WithContext(ctx).
			Model(&aiAnalysisJobModel{}).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND scheduled_at <= ?", string(domain.AIAnalysisJobStatusQueued), input.Now).
			Order("scheduled_at ASC, id ASC").
			Limit(input.Limit).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}

		updates := map[string]interface{}{
			"status":        string(domain.AIAnalysisJobStatusRunning),
			"started_at":    input.Now,
			"locked_at":     input.Now,
			"locked_by":     input.WorkerID,
			"lease_until":   input.Now.Add(input.LeaseDuration),
			"attempt_count": gorm.Expr("attempt_count + ?", 1),
			"updated_at":    input.Now,
		}
		if err := tx.WithContext(ctx).
			Model(&aiAnalysisJobModel{}).
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
		metrics.TaskQueueClaimDuration.WithLabelValues(aiAnalysisJobQueueName).Observe(time.Since(claimStarted).Seconds())
		return nil, opErr
	}
	metrics.TaskQueueClaimDuration.WithLabelValues(aiAnalysisJobQueueName).Observe(time.Since(claimStarted).Seconds())
	r.observeAIAnalysisQueueState(ctx, input.Now)

	jobs := make([]domain.AIAnalysisJob, 0, len(models))
	for _, model := range models {
		jobs = append(jobs, aiAnalysisJobModelToDomain(model))
	}
	return jobs, nil
}

func (r *AIAnalysisJobRepository) Update(ctx context.Context, job domain.AIAnalysisJob) (domain.AIAnalysisJob, error) {
	return r.update(ctx, job, "")
}

func (r *AIAnalysisJobRepository) UpdateIfOwned(ctx context.Context, job domain.AIAnalysisJob, workerID string) (domain.AIAnalysisJob, error) {
	return r.update(ctx, job, strings.TrimSpace(workerID))
}

func (r *AIAnalysisJobRepository) update(ctx context.Context, job domain.AIAnalysisJob, workerID string) (domain.AIAnalysisJob, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.ai_analysis_job.update", "update", "ai_analysis_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	model := aiAnalysisJobModelFromDomain(job)
	result := r.db.WithContext(ctx).
		Model(&aiAnalysisJobModel{}).
		Where("user_id = ? AND id = ?", job.UserID, job.ID)
	if workerID != "" {
		result = result.Where("locked_by = ? AND status = ?", workerID, string(domain.AIAnalysisJobStatusRunning))
	}
	result = result.
		Select("Status", "Input", "Result", "ScheduledAt", "StartedAt", "FinishedAt", "AttemptCount", "MaxAttempts", "LockedBy", "LockedAt", "LeaseUntil", "LastError").
		Updates(&model)
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.AIAnalysisJob{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.AIAnalysisJob{}, opErr
	}

	var updated aiAnalysisJobModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND id = ?", job.UserID, job.ID).
		First(&updated).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AIAnalysisJob{}, opErr
	}
	return aiAnalysisJobModelToDomain(updated), nil
}

func (r *AIAnalysisJobRepository) ListByUser(ctx context.Context, options domain.AIAnalysisJobListOptions) (domain.AIAnalysisJobListResult, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.ai_analysis_job.list_by_user", "select", "ai_analysis_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	options = normalizeAIAnalysisJobListOptions(options)
	query := r.db.WithContext(ctx).Model(&aiAnalysisJobModel{}).Where("user_id = ?", options.UserID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AIAnalysisJobListResult{}, opErr
	}

	var models []aiAnalysisJobModel
	if err := query.
		Order("created_at DESC, id DESC").
		Limit(options.Limit).
		Offset(options.Offset).
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AIAnalysisJobListResult{}, opErr
	}

	jobs := make([]domain.AIAnalysisJob, 0, len(models))
	for _, model := range models {
		jobs = append(jobs, aiAnalysisJobModelToDomain(model))
	}
	return domain.AIAnalysisJobListResult{
		Jobs:   jobs,
		Total:  total,
		Limit:  options.Limit,
		Offset: options.Offset,
	}, nil
}

func normalizeAIAnalysisJobClaimInput(input domain.AIAnalysisJobClaimInput) domain.AIAnalysisJobClaimInput {
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
		input.Limit = defaultAIAnalysisJobClaimLimit
	}
	if input.Limit > maxAIAnalysisJobClaimLimit {
		input.Limit = maxAIAnalysisJobClaimLimit
	}
	if input.LeaseDuration <= 0 {
		input.LeaseDuration = defaultAIAnalysisJobLease
	}
	return input
}

func recoverExpiredAIAnalysisJobs(tx *gorm.DB, input domain.AIAnalysisJobClaimInput) error {
	base := tx.Model(&aiAnalysisJobModel{}).
		Where("status = ? AND lease_until IS NOT NULL AND lease_until <= ?", string(domain.AIAnalysisJobStatusRunning), input.Now)
	requeued := base.Where("attempt_count < max_attempts").Updates(map[string]interface{}{
		"status":       string(domain.AIAnalysisJobStatusQueued),
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
		"status":      string(domain.AIAnalysisJobStatusFailed),
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
		metrics.TaskQueueLeaseRecoveriesTotal.WithLabelValues(aiAnalysisJobQueueName).Add(float64(recovered))
	}
	if failed.RowsAffected > 0 {
		metrics.TaskQueueDeadLettersTotal.WithLabelValues(aiAnalysisJobQueueName).Add(float64(failed.RowsAffected))
	}
	return nil
}

func (r *AIAnalysisJobRepository) observeAIAnalysisQueueState(ctx context.Context, now time.Time) {
	var depth int64
	if err := r.db.WithContext(ctx).Model(&aiAnalysisJobModel{}).
		Where("status = ?", string(domain.AIAnalysisJobStatusQueued)).Count(&depth).Error; err != nil {
		return
	}
	var oldest time.Time
	if err := r.db.WithContext(ctx).Model(&aiAnalysisJobModel{}).
		Where("status = ?", string(domain.AIAnalysisJobStatusQueued)).Select("MIN(scheduled_at)").Scan(&oldest).Error; err != nil {
		return
	}
	var oldestPtr *time.Time
	if !oldest.IsZero() {
		oldestPtr = &oldest
	}
	metrics.ObserveTaskQueueState(aiAnalysisJobQueueName, depth, oldestPtr, now)
}

func normalizeAIAnalysisJobListOptions(options domain.AIAnalysisJobListOptions) domain.AIAnalysisJobListOptions {
	if options.Limit <= 0 {
		options.Limit = defaultAIAnalysisJobListLimit
	}
	if options.Limit > maxAIAnalysisJobListLimit {
		options.Limit = maxAIAnalysisJobListLimit
	}
	if options.Offset < 0 {
		options.Offset = 0
	}
	return options
}

func aiAnalysisJobModelFromDomain(job domain.AIAnalysisJob) aiAnalysisJobModel {
	return aiAnalysisJobModel{
		ID:               job.ID,
		UserID:           job.UserID,
		AlertCandidateID: job.AlertCandidateID,
		SourceID:         job.SourceID,
		ItemID:           job.ItemID,
		Status:           string(job.Status),
		Input:            cloneAIAnalysisJobInput(job.Input),
		Result:           cloneAIAnalysisResult(job.Result),
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

func aiAnalysisJobModelToDomain(model aiAnalysisJobModel) domain.AIAnalysisJob {
	return domain.AIAnalysisJob{
		ID:               model.ID,
		UserID:           model.UserID,
		AlertCandidateID: model.AlertCandidateID,
		SourceID:         model.SourceID,
		ItemID:           model.ItemID,
		Status:           domain.AIAnalysisJobStatus(model.Status),
		Input:            cloneAIAnalysisJobInput(model.Input),
		Result:           cloneAIAnalysisResult(model.Result),
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

func cloneAIAnalysisJobInput(input domain.AIAnalysisJobInput) domain.AIAnalysisJobInput {
	if input == nil {
		return nil
	}
	cloned := make(domain.AIAnalysisJobInput, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func cloneAIAnalysisResult(result domain.AIAnalysisResult) domain.AIAnalysisResult {
	result.MatchedReasons = append([]string(nil), result.MatchedReasons...)
	return result
}
