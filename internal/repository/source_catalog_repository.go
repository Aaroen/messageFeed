package repository

import (
	"context"
	"messagefeed/internal/domain"
	"strings"
	"time"

	"gorm.io/gorm"
)

type SourceCatalogRepository struct {
	db *gorm.DB
}

func NewSourceCatalogRepository(db *gorm.DB) *SourceCatalogRepository {
	return &SourceCatalogRepository{db: db}
}

type sourceCatalogEntryModel struct {
	ID             int64  `gorm:"primaryKey"`
	SourceKey      string `gorm:"not null"`
	Name           string `gorm:"not null"`
	SiteURL        string
	FeedURL        string   `gorm:"not null"`
	NormalizedURL  string   `gorm:"not null"`
	Type           string   `gorm:"not null"`
	Category       string   `gorm:"not null"`
	Tags           []string `gorm:"serializer:json;type:jsonb;not null"`
	Language       string   `gorm:"not null"`
	Country        string
	Official       bool
	SourceOrigin   string `gorm:"not null"`
	HealthStatus   string `gorm:"not null"`
	LastCheckedAt  *time.Time
	LastCheckError string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type sourceCatalogEntryViewModel struct {
	ID             int64  `gorm:"primaryKey"`
	SourceKey      string `gorm:"not null"`
	Name           string `gorm:"not null"`
	SiteURL        string
	FeedURL        string   `gorm:"not null"`
	NormalizedURL  string   `gorm:"not null"`
	Type           string   `gorm:"not null"`
	Category       string   `gorm:"not null"`
	Tags           []string `gorm:"serializer:json;type:jsonb;not null"`
	Language       string   `gorm:"not null"`
	Country        string
	Official       bool
	SourceOrigin   string `gorm:"not null"`
	HealthStatus   string `gorm:"not null"`
	LastCheckedAt  *time.Time
	LastCheckError string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Subscribed     bool
	SourceID       int64
	SourceStatus   string
}

func (sourceCatalogEntryModel) TableName() string {
	return "source_catalog_entries"
}

func (r *SourceCatalogRepository) List(ctx context.Context, options domain.SourceCatalogListOptions) (domain.SourceCatalogListResult, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.source_catalog.list", "select", "source_catalog_entries")
	var opErr error
	defer func() { finish(opErr) }()

	limit := options.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	offset := options.Offset
	if offset < 0 {
		offset = 0
	}

	query := r.db.WithContext(ctx).Table("source_catalog_entries").
		Joins("LEFT JOIN sources ON sources.user_id = ? AND sources.normalized_url = source_catalog_entries.normalized_url", options.UserID)
	query = applySourceCatalogFilters(query, options)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.SourceCatalogListResult{}, opErr
	}

	var models []sourceCatalogEntryViewModel
	if err := query.
		Select(`
			source_catalog_entries.*,
			(sources.id IS NOT NULL) AS subscribed,
			COALESCE(sources.id, 0) AS source_id,
			COALESCE(sources.status, '') AS source_status`).
		Order("source_catalog_entries.category ASC, source_catalog_entries.name ASC, source_catalog_entries.id ASC").
		Limit(limit).
		Offset(offset).
		Scan(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.SourceCatalogListResult{}, opErr
	}

	entries := make([]domain.SourceCatalogEntry, 0, len(models))
	for _, model := range models {
		entries = append(entries, sourceCatalogEntryViewModelToDomain(model))
	}
	return domain.SourceCatalogListResult{
		Entries: entries,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	}, nil
}

func (r *SourceCatalogRepository) GetByIDs(ctx context.Context, ids []int64) ([]domain.SourceCatalogEntry, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.source_catalog.get_by_ids", "select", "source_catalog_entries")
	var opErr error
	defer func() { finish(opErr) }()

	if len(ids) == 0 {
		return nil, nil
	}

	var models []sourceCatalogEntryModel
	if err := r.db.WithContext(ctx).
		Where("id IN ?", ids).
		Order("id ASC").
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}

	entries := make([]domain.SourceCatalogEntry, 0, len(models))
	for _, model := range models {
		entries = append(entries, sourceCatalogEntryModelToDomain(model))
	}
	return entries, nil
}

func applySourceCatalogFilters(query *gorm.DB, options domain.SourceCatalogListOptions) *gorm.DB {
	if category := strings.TrimSpace(options.Category); category != "" {
		query = query.Where("source_catalog_entries.category = ?", category)
	}
	if q := strings.TrimSpace(options.Query); q != "" {
		pattern := "%" + strings.ToLower(q) + "%"
		query = query.Where(`
			LOWER(source_catalog_entries.name) LIKE ?
			OR LOWER(source_catalog_entries.category) LIKE ?
			OR LOWER(source_catalog_entries.feed_url) LIKE ?
			OR LOWER(source_catalog_entries.tags::text) LIKE ?`,
			pattern, pattern, pattern, pattern)
	}
	return query
}

func sourceCatalogEntryViewModelToDomain(model sourceCatalogEntryViewModel) domain.SourceCatalogEntry {
	entry := sourceCatalogEntryModelToDomain(sourceCatalogEntryModel{
		ID:             model.ID,
		SourceKey:      model.SourceKey,
		Name:           model.Name,
		SiteURL:        model.SiteURL,
		FeedURL:        model.FeedURL,
		NormalizedURL:  model.NormalizedURL,
		Type:           model.Type,
		Category:       model.Category,
		Tags:           model.Tags,
		Language:       model.Language,
		Country:        model.Country,
		Official:       model.Official,
		SourceOrigin:   model.SourceOrigin,
		HealthStatus:   model.HealthStatus,
		LastCheckedAt:  model.LastCheckedAt,
		LastCheckError: model.LastCheckError,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	})
	entry.Subscribed = model.Subscribed
	entry.SourceID = model.SourceID
	entry.SourceStatus = domain.SourceStatus(model.SourceStatus)
	return entry
}

func sourceCatalogEntryModelToDomain(model sourceCatalogEntryModel) domain.SourceCatalogEntry {
	return domain.SourceCatalogEntry{
		ID:             model.ID,
		SourceKey:      model.SourceKey,
		Name:           model.Name,
		SiteURL:        model.SiteURL,
		FeedURL:        model.FeedURL,
		NormalizedURL:  model.NormalizedURL,
		Type:           domain.SourceType(model.Type),
		Category:       model.Category,
		Tags:           append([]string(nil), model.Tags...),
		Language:       model.Language,
		Country:        model.Country,
		Official:       model.Official,
		SourceOrigin:   model.SourceOrigin,
		HealthStatus:   domain.SourceCatalogHealthStatus(model.HealthStatus),
		LastCheckedAt:  model.LastCheckedAt,
		LastCheckError: model.LastCheckError,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}
}
