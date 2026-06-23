package repository

import (
	"context"
	"errors"
	"fmt"
	"messagefeed/internal/domain"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type SourceRepository struct {
	db *gorm.DB
}

func NewSourceRepository(db *gorm.DB) *SourceRepository {
	return &SourceRepository{db: db}
}

type sourceModel struct {
	ID                   int64    `gorm:"primaryKey"`
	UserID               int64    `gorm:"not null"`
	Name                 string   `gorm:"not null"`
	Type                 string   `gorm:"not null"`
	URL                  string   `gorm:"column:url;not null"`
	NormalizedURL        string   `gorm:"not null"`
	Status               string   `gorm:"not null"`
	FetchIntervalSeconds int      `gorm:"not null"`
	Tags                 []string `gorm:"serializer:json;type:jsonb;not null"`
	Weight               int
	LastFetchedAt        *time.Time
	LastFetchStatus      string
	LastFetchError       string
	LastFetchDurationMS  *int
	LastFetchItemCount   *int
	NextFetchAt          *time.Time
	ETag                 string
	LastModified         string
	FetchPriority        int
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (sourceModel) TableName() string {
	return "sources"
}

func (r *SourceRepository) Create(ctx context.Context, source domain.Source) (domain.Source, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.source.create", "create", "sources")
	var opErr error
	defer func() { finish(opErr) }()

	model := sourceModelFromDomain(source)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.Source{}, opErr
	}
	return sourceModelToDomain(model), nil
}

func (r *SourceRepository) ListByUser(ctx context.Context, userID int64) ([]domain.Source, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.source.list_by_user", "select", "sources")
	var opErr error
	defer func() { finish(opErr) }()

	var models []sourceModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("id ASC").
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}

	sources := make([]domain.Source, 0, len(models))
	for _, model := range models {
		sources = append(sources, sourceModelToDomain(model))
	}
	return sources, nil
}

func (r *SourceRepository) GetByID(ctx context.Context, userID int64, id int64) (domain.Source, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.source.get_by_id", "select", "sources")
	var opErr error
	defer func() { finish(opErr) }()

	var model sourceModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND id = ?", userID, id).
		First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.Source{}, opErr
	}
	return sourceModelToDomain(model), nil
}

func (r *SourceRepository) Update(ctx context.Context, source domain.Source) (domain.Source, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.source.update", "update", "sources")
	var opErr error
	defer func() { finish(opErr) }()

	model := sourceModelFromDomain(source)
	result := r.db.WithContext(ctx).
		Model(&model).
		Select("Name", "Type", "URL", "NormalizedURL", "Status", "FetchIntervalSeconds", "Tags", "Weight", "NextFetchAt", "ETag", "LastModified", "FetchPriority").
		Where("user_id = ? AND id = ?", source.UserID, source.ID).
		Updates(&model)
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.Source{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.Source{}, opErr
	}
	updated, err := r.GetByID(ctx, source.UserID, source.ID)
	if err != nil {
		opErr = err
		return domain.Source{}, opErr
	}
	return updated, nil
}

func (r *SourceRepository) UpdateFetchResult(ctx context.Context, source domain.Source) (domain.Source, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.source.update_fetch_result", "update", "sources")
	var opErr error
	defer func() { finish(opErr) }()

	model := sourceModelFromDomain(source)
	result := r.db.WithContext(ctx).
		Model(&model).
		Select("LastFetchedAt", "LastFetchStatus", "LastFetchError", "LastFetchDurationMS", "LastFetchItemCount").
		Where("user_id = ? AND id = ?", source.UserID, source.ID).
		Updates(&model)
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.Source{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.Source{}, opErr
	}
	updated, err := r.GetByID(ctx, source.UserID, source.ID)
	if err != nil {
		opErr = err
		return domain.Source{}, opErr
	}
	return updated, nil
}

func sourceModelFromDomain(source domain.Source) sourceModel {
	return sourceModel{
		ID:                   source.ID,
		UserID:               source.UserID,
		Name:                 source.Name,
		Type:                 string(source.Type),
		URL:                  source.URL,
		NormalizedURL:        source.NormalizedURL,
		Status:               string(source.Status),
		FetchIntervalSeconds: source.FetchIntervalSeconds,
		Tags:                 append([]string(nil), source.Tags...),
		Weight:               source.Weight,
		LastFetchedAt:        source.LastFetchedAt,
		LastFetchStatus:      source.LastFetchStatus,
		LastFetchError:       source.LastFetchError,
		LastFetchDurationMS:  source.LastFetchDurationMS,
		LastFetchItemCount:   source.LastFetchItemCount,
		NextFetchAt:          source.NextFetchAt,
		ETag:                 source.ETag,
		LastModified:         source.LastModified,
		FetchPriority:        source.FetchPriority,
		CreatedAt:            source.CreatedAt,
		UpdatedAt:            source.UpdatedAt,
	}
}

func sourceModelToDomain(model sourceModel) domain.Source {
	return domain.Source{
		ID:                   model.ID,
		UserID:               model.UserID,
		Name:                 model.Name,
		Type:                 domain.SourceType(model.Type),
		URL:                  model.URL,
		NormalizedURL:        model.NormalizedURL,
		Status:               domain.SourceStatus(model.Status),
		FetchIntervalSeconds: model.FetchIntervalSeconds,
		Tags:                 append([]string(nil), model.Tags...),
		Weight:               model.Weight,
		LastFetchedAt:        model.LastFetchedAt,
		LastFetchStatus:      model.LastFetchStatus,
		LastFetchError:       model.LastFetchError,
		LastFetchDurationMS:  model.LastFetchDurationMS,
		LastFetchItemCount:   model.LastFetchItemCount,
		NextFetchAt:          model.NextFetchAt,
		ETag:                 model.ETag,
		LastModified:         model.LastModified,
		FetchPriority:        model.FetchPriority,
		CreatedAt:            model.CreatedAt,
		UpdatedAt:            model.UpdatedAt,
	}
}

func mapRepositoryError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrConflict
	}

	return fmt.Errorf("repository: %w", err)
}
