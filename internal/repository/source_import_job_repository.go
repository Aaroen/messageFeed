package repository

import (
	"context"
	"messagefeed/internal/domain"
	"time"

	"gorm.io/gorm"
)

type SourceImportJobRepository struct {
	db *gorm.DB
}

func NewSourceImportJobRepository(db *gorm.DB) *SourceImportJobRepository {
	return &SourceImportJobRepository{db: db}
}

type sourceImportJobModel struct {
	ID             int64                         `gorm:"primaryKey"`
	UserID         int64                         `gorm:"not null"`
	ImportType     string                        `gorm:"not null"`
	Status         string                        `gorm:"not null"`
	RequestedCount int                           `gorm:"not null"`
	SuccessCount   int                           `gorm:"not null"`
	FailureCount   int                           `gorm:"not null"`
	ErrorDetails   []domain.SourceImportJobError `gorm:"serializer:json;type:jsonb;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (sourceImportJobModel) TableName() string {
	return "source_import_jobs"
}

func (r *SourceImportJobRepository) Create(ctx context.Context, job domain.SourceImportJob) (domain.SourceImportJob, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.source_import_job.create", "create", "source_import_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	model := sourceImportJobModelFromDomain(job)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.SourceImportJob{}, opErr
	}
	return sourceImportJobModelToDomain(model), nil
}

func (r *SourceImportJobRepository) ListByUser(ctx context.Context, options domain.SourceImportJobListOptions) (domain.SourceImportJobListResult, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.source_import_job.list_by_user", "select", "source_import_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	options = normalizeSourceImportJobListOptions(options)
	query := r.db.WithContext(ctx).Model(&sourceImportJobModel{}).Where("user_id = ?", options.UserID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.SourceImportJobListResult{}, opErr
	}

	var models []sourceImportJobModel
	if err := query.
		Order("created_at DESC, id DESC").
		Limit(options.Limit).
		Offset(options.Offset).
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.SourceImportJobListResult{}, opErr
	}

	jobs := make([]domain.SourceImportJob, 0, len(models))
	for _, model := range models {
		jobs = append(jobs, sourceImportJobModelToDomain(model))
	}
	return domain.SourceImportJobListResult{
		Jobs:   jobs,
		Total:  total,
		Limit:  options.Limit,
		Offset: options.Offset,
	}, nil
}

func normalizeSourceImportJobListOptions(options domain.SourceImportJobListOptions) domain.SourceImportJobListOptions {
	if options.Limit <= 0 {
		options.Limit = domain.DefaultSourceImportJobListLimit
	}
	if options.Limit > domain.MaxSourceImportJobListLimit {
		options.Limit = domain.MaxSourceImportJobListLimit
	}
	if options.Offset < 0 {
		options.Offset = 0
	}
	return options
}

func sourceImportJobModelFromDomain(job domain.SourceImportJob) sourceImportJobModel {
	return sourceImportJobModel{
		ID:             job.ID,
		UserID:         job.UserID,
		ImportType:     string(job.ImportType),
		Status:         string(job.Status),
		RequestedCount: job.RequestedCount,
		SuccessCount:   job.SuccessCount,
		FailureCount:   job.FailureCount,
		ErrorDetails:   append([]domain.SourceImportJobError(nil), job.ErrorDetails...),
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      job.UpdatedAt,
	}
}

func sourceImportJobModelToDomain(model sourceImportJobModel) domain.SourceImportJob {
	return domain.SourceImportJob{
		ID:             model.ID,
		UserID:         model.UserID,
		ImportType:     domain.SourceImportType(model.ImportType),
		Status:         domain.SourceImportStatus(model.Status),
		RequestedCount: model.RequestedCount,
		SuccessCount:   model.SuccessCount,
		FailureCount:   model.FailureCount,
		ErrorDetails:   append([]domain.SourceImportJobError(nil), model.ErrorDetails...),
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}
}
