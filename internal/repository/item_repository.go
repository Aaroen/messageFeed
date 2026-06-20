package repository

import (
	"context"
	"errors"
	"messagefeed/internal/domain"
	"time"

	"gorm.io/gorm"
)

type ItemRepository struct {
	db *gorm.DB
}

func NewItemRepository(db *gorm.DB) *ItemRepository {
	return &ItemRepository{db: db}
}

type itemModel struct {
	ID             int64  `gorm:"primaryKey"`
	SourceID       int64  `gorm:"not null"`
	Title          string `gorm:"not null"`
	URL            string `gorm:"column:url;not null"`
	NormalizedURL  string `gorm:"not null"`
	RawGUID        string
	ContentHash    string
	Summary        string
	ContentSnippet string
	Author         string
	PublishedAt    *time.Time
	FetchedAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type itemViewModel struct {
	ID             int64
	SourceID       int64
	SourceName     string
	Title          string
	URL            string `gorm:"column:url"`
	NormalizedURL  string
	RawGUID        string
	ContentHash    string
	Summary        string
	ContentSnippet string
	Author         string
	PublishedAt    *time.Time
	FetchedAt      time.Time
	IsRead         bool
	ReadAt         *time.Time
	IsFavorite     bool
	FavoritedAt    *time.Time
	IsHidden       bool
	HiddenAt       *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

const activeSourceStatusFilter = "sources.status = ?"

func (itemModel) TableName() string {
	return "items"
}

func (r *ItemRepository) UpsertMany(ctx context.Context, items []domain.Item) (domain.ItemUpsertResult, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.item.upsert_many", "upsert", "items")
	var opErr error
	defer func() { finish(opErr) }()

	result := domain.ItemUpsertResult{TotalCount: len(items)}
	if len(items) == 0 {
		return result, nil
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			created, err := upsertItem(ctx, tx, item)
			if err != nil {
				return err
			}
			if created {
				result.CreatedCount++
			} else {
				result.UpdatedCount++
			}
		}
		return nil
	})
	if err != nil {
		opErr = mapRepositoryError(err)
		return domain.ItemUpsertResult{}, opErr
	}
	return result, nil
}

func (r *ItemRepository) ListByUser(ctx context.Context, options domain.ItemListOptions) (domain.ItemListResult, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.item.list_by_user", "select", "items")
	var opErr error
	defer func() { finish(opErr) }()

	query := itemViewBaseQuery(r.db.WithContext(ctx), options.UserID)
	if options.SourceID > 0 {
		query = query.Where("items.source_id = ?", options.SourceID)
	}
	query = applyItemStateFilters(query, options)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.ItemListResult{}, opErr
	}

	var models []itemViewModel
	if err := query.
		Select(itemViewSelectColumns()).
		Order(itemListOrderClause(options.SortOrder)).
		Limit(options.Limit).
		Offset(options.Offset).
		Scan(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.ItemListResult{}, opErr
	}

	items := make([]domain.Item, 0, len(models))
	for _, model := range models {
		items = append(items, itemViewModelToDomain(model))
	}
	return domain.ItemListResult{
		Items:  items,
		Total:  total,
		Limit:  options.Limit,
		Offset: options.Offset,
	}, nil
}

func (r *ItemRepository) GetByIDForUser(ctx context.Context, userID int64, itemID int64) (domain.Item, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.item.get_by_id_for_user", "select", "items")
	var opErr error
	defer func() { finish(opErr) }()

	var model itemViewModel
	if err := itemViewBaseQuery(r.db.WithContext(ctx), userID).
		Select(itemViewSelectColumns()).
		Where("items.id = ?", itemID).
		Limit(1).
		Scan(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.Item{}, opErr
	}
	if model.ID == 0 {
		opErr = domain.ErrNotFound
		return domain.Item{}, opErr
	}
	return itemViewModelToDomain(model), nil
}

func itemViewBaseQuery(db *gorm.DB, userID int64) *gorm.DB {
	return db.Table("items").
		Joins("JOIN sources ON sources.id = items.source_id").
		Joins("LEFT JOIN user_item_states ON user_item_states.item_id = items.id AND user_item_states.user_id = ?", userID).
		Where("sources.user_id = ?", userID).
		Where(activeSourceStatusFilter, string(domain.SourceStatusActive))
}

func itemViewSelectColumns() string {
	return `
		items.id,
		items.source_id,
		sources.name AS source_name,
		items.title,
		items.url,
		items.normalized_url,
		items.raw_guid,
		items.content_hash,
		items.summary,
		items.content_snippet,
		items.author,
		items.published_at,
		items.fetched_at,
		COALESCE(user_item_states.is_read, false) AS is_read,
		user_item_states.read_at,
		COALESCE(user_item_states.is_favorite, false) AS is_favorite,
		user_item_states.favorited_at,
		COALESCE(user_item_states.is_hidden, false) AS is_hidden,
		user_item_states.hidden_at,
		items.created_at,
		items.updated_at`
}

func itemListOrderClause(order domain.ItemSortOrder) string {
	if order == domain.ItemSortOrderAsc {
		return "items.published_at ASC NULLS LAST, items.fetched_at ASC, items.id ASC"
	}
	return "items.published_at DESC NULLS LAST, items.fetched_at DESC, items.id DESC"
}

func applyItemStateFilters(query *gorm.DB, options domain.ItemListOptions) *gorm.DB {
	if options.IsRead != nil {
		query = query.Where("COALESCE(user_item_states.is_read, false) = ?", *options.IsRead)
	}
	if options.IsFavorite != nil {
		query = query.Where("COALESCE(user_item_states.is_favorite, false) = ?", *options.IsFavorite)
	}
	if options.IsHidden != nil {
		query = query.Where("COALESCE(user_item_states.is_hidden, false) = ?", *options.IsHidden)
	} else if !options.IncludeHidden {
		query = query.Where("COALESCE(user_item_states.is_hidden, false) = false")
	}
	return query
}

func upsertItem(ctx context.Context, db *gorm.DB, item domain.Item) (bool, error) {
	model := itemModelFromDomain(item)

	existing, found, err := findExistingItem(ctx, db, item)
	if err != nil {
		return false, err
	}
	if found {
		model.ID = existing.ID
		model.CreatedAt = existing.CreatedAt
		if err := db.WithContext(ctx).
			Model(&itemModel{}).
			Where("id = ?", existing.ID).
			Select("Title", "URL", "NormalizedURL", "RawGUID", "ContentHash", "Summary", "ContentSnippet", "Author", "PublishedAt", "FetchedAt").
			Updates(&model).Error; err != nil {
			return false, err
		}
		return false, nil
	}

	if err := db.WithContext(ctx).Create(&model).Error; err != nil {
		return false, err
	}
	return true, nil
}

func findExistingItem(ctx context.Context, db *gorm.DB, item domain.Item) (itemModel, bool, error) {
	var model itemModel
	if item.RawGUID != "" {
		err := db.WithContext(ctx).
			Where("source_id = ? AND raw_guid = ?", item.SourceID, item.RawGUID).
			First(&model).Error
		if err == nil {
			return model, true, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return itemModel{}, false, err
		}
	}

	err := db.WithContext(ctx).
		Where("source_id = ? AND normalized_url = ?", item.SourceID, item.NormalizedURL).
		First(&model).Error
	if err == nil {
		return model, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return itemModel{}, false, nil
	}
	return itemModel{}, false, err
}

func itemModelFromDomain(item domain.Item) itemModel {
	return itemModel{
		ID:             item.ID,
		SourceID:       item.SourceID,
		Title:          item.Title,
		URL:            item.URL,
		NormalizedURL:  item.NormalizedURL,
		RawGUID:        item.RawGUID,
		ContentHash:    item.ContentHash,
		Summary:        item.Summary,
		ContentSnippet: item.ContentSnippet,
		Author:         item.Author,
		PublishedAt:    item.PublishedAt,
		FetchedAt:      item.FetchedAt,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

func itemModelToDomain(model itemModel) domain.Item {
	return domain.Item{
		ID:             model.ID,
		SourceID:       model.SourceID,
		Title:          model.Title,
		URL:            model.URL,
		NormalizedURL:  model.NormalizedURL,
		RawGUID:        model.RawGUID,
		ContentHash:    model.ContentHash,
		Summary:        model.Summary,
		ContentSnippet: model.ContentSnippet,
		Author:         model.Author,
		PublishedAt:    model.PublishedAt,
		FetchedAt:      model.FetchedAt,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}
}

func itemViewModelToDomain(model itemViewModel) domain.Item {
	return domain.Item{
		ID:             model.ID,
		SourceID:       model.SourceID,
		SourceName:     model.SourceName,
		Title:          model.Title,
		URL:            model.URL,
		NormalizedURL:  model.NormalizedURL,
		RawGUID:        model.RawGUID,
		ContentHash:    model.ContentHash,
		Summary:        model.Summary,
		ContentSnippet: model.ContentSnippet,
		Author:         model.Author,
		PublishedAt:    model.PublishedAt,
		FetchedAt:      model.FetchedAt,
		IsRead:         model.IsRead,
		ReadAt:         model.ReadAt,
		IsFavorite:     model.IsFavorite,
		FavoritedAt:    model.FavoritedAt,
		IsHidden:       model.IsHidden,
		HiddenAt:       model.HiddenAt,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}
}
