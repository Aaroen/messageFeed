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
	defaultAIAnalysisJobClaimLimit = 20
	maxAIAnalysisJobClaimLimit     = 100
	defaultAIAnalysisJobListLimit  = 20
	maxAIAnalysisJobListLimit      = 100
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
	var models []aiAnalysisJobModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
		return nil, opErr
	}

	jobs := make([]domain.AIAnalysisJob, 0, len(models))
	for _, model := range models {
		jobs = append(jobs, aiAnalysisJobModelToDomain(model))
	}
	return jobs, nil
}

func (r *AIAnalysisJobRepository) Update(ctx context.Context, job domain.AIAnalysisJob) (domain.AIAnalysisJob, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.ai_analysis_job.update", "update", "ai_analysis_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	model := aiAnalysisJobModelFromDomain(job)
	result := r.db.WithContext(ctx).
		Model(&aiAnalysisJobModel{}).
		Where("user_id = ? AND id = ?", job.UserID, job.ID).
		Select("Status", "Input", "Result", "ScheduledAt", "StartedAt", "FinishedAt", "AttemptCount", "MaxAttempts", "LockedBy", "LockedAt", "LastError").
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
	return input
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
