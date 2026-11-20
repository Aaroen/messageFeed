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

type ItemRepository struct {
	db *gorm.DB
}

func NewItemRepository(db *gorm.DB) *ItemRepository {
	return &ItemRepository{db: db}
}

type itemModel struct {
	ID                  int64  `gorm:"primaryKey"`
	SourceID            int64  `gorm:"not null"`
	Title               string `gorm:"not null"`
	URL                 string `gorm:"column:url;not null"`
	NormalizedURL       string `gorm:"not null"`
	RawGUID             string
	ContentHash         string
	Summary             string
	ContentSnippet      string
	Author              string
	PublishedAt         *time.Time
	SourceUpdatedAt     *time.Time
	PublicationVerified bool
	FetchedAt           time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type itemViewModel struct {
	ID              int64
	SourceID        int64
	CatalogID       int64
	SourceName      string
	Title           string
	URL             string `gorm:"column:url"`
	NormalizedURL   string
	RawGUID         string
	ContentHash     string
	Summary         string
	ContentSnippet  string
	Author          string
	PublishedAt     *time.Time
	SourceUpdatedAt *time.Time
	FetchedAt       time.Time
	IsRead          bool
	ReadAt          *time.Time
	IsFavorite      bool
	FavoritedAt     *time.Time
	IsHidden        bool
	HiddenAt        *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

const activeSourceStatusFilter = "sources.status = ?"
const hiddenAgentOperationLogURLPattern = "messagefeed://ai/agent_operation_log/%"
const itemOrderTimeSQL = "COALESCE(items.published_at, items.source_updated_at, items.created_at)"

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
			upserted, created, err := upsertItem(ctx, tx, item)
			if err != nil {
				return err
			}
			appendItemUpsertResult(&result, upserted, created)
		}
		return nil
	})
	if err != nil {
		opErr = mapRepositoryError(err)
		return domain.ItemUpsertResult{}, opErr
	}
	return result, nil
}

func appendItemUpsertResult(result *domain.ItemUpsertResult, item domain.Item, created bool) {
	if result == nil {
		return
	}
	if created {
		result.CreatedCount++
		result.CreatedItems = append(result.CreatedItems, item)
		return
	}
	result.UpdatedCount++
	result.UpdatedItems = append(result.UpdatedItems, item)
}

func (r *ItemRepository) ListByUser(ctx context.Context, options domain.ItemListOptions) (domain.ItemListResult, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.item.list_by_user", "select", "items")
	var opErr error
	defer func() { finish(opErr) }()

	query := itemViewBaseQuery(r.db.WithContext(ctx), options.UserID)
	if options.CatalogOnly {
		query = query.Where("public_catalog.id IS NOT NULL")
	}
	query = applyItemSourceFilters(query, options)
	query = applyItemStateFilters(query, options)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.ItemListResult{}, opErr
	}

	var models []itemViewModel
	if err := query.
		Select(itemViewSelectColumns()).
		Scopes(itemCursorScope(options)).
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

func (r *ItemRepository) ListPublic(ctx context.Context, options domain.ItemListOptions) (domain.ItemListResult, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.item.list_public", "select", "items")
	var opErr error
	defer func() { finish(opErr) }()

	query := itemPublicBaseQuery(r.db.WithContext(ctx))
	query = applyItemSourceFilters(query, options)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.ItemListResult{}, opErr
	}

	var models []itemViewModel
	if err := query.
		Select(itemPublicViewSelectColumns()).
		Scopes(itemCursorScope(options)).
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

func (r *ItemRepository) GetByIDPublic(ctx context.Context, itemID int64) (domain.Item, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.item.get_by_id_public", "select", "items")
	var opErr error
	defer func() { finish(opErr) }()

	var model itemViewModel
	if err := itemPublicBaseQuery(r.db.WithContext(ctx)).
		Select(itemPublicViewSelectColumns()).
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
		Joins(itemCatalogJoin("LEFT JOIN")).
		Joins("LEFT JOIN user_item_states ON user_item_states.item_id = items.id AND user_item_states.user_id = ?", userID).
		Where("sources.user_id = ?", userID).
		Where("items.normalized_url NOT LIKE ?", hiddenAgentOperationLogURLPattern)
}

func itemPublicBaseQuery(db *gorm.DB) *gorm.DB {
	return db.Table("items").
		Joins("JOIN sources ON sources.id = items.source_id").
		Joins("JOIN users ON users.id = sources.user_id AND users.role = 'owner'").
		Joins(itemCatalogJoin("JOIN")).
		Where("items.normalized_url NOT LIKE ? AND sources.type <> ?", hiddenAgentOperationLogURLPattern, "internal")
}

func itemCatalogJoin(join string) string {
	return join + ` LATERAL (
		SELECT id, name FROM source_catalog_entries
		WHERE normalized_url = sources.normalized_url AND official AND license_status = 'allowed'
		ORDER BY id LIMIT 1
	) AS public_catalog ON true`
}

func itemViewSelectColumns() string {
	return `
		items.id,
		items.source_id,
		COALESCE(public_catalog.id, 0) AS catalog_id,
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
		items.source_updated_at,
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

func itemPublicViewSelectColumns() string {
	return `
		items.id,
		items.source_id,
		public_catalog.id AS catalog_id,
		public_catalog.name AS source_name,
		items.title,
		items.url,
		items.normalized_url,
		items.raw_guid,
		items.content_hash,
		items.summary,
		items.content_snippet,
		items.author,
		items.published_at,
		items.source_updated_at,
		items.fetched_at,
		false AS is_read,
		NULL::timestamp with time zone AS read_at,
		false AS is_favorite,
		NULL::timestamp with time zone AS favorited_at,
		false AS is_hidden,
		NULL::timestamp with time zone AS hidden_at,
		items.created_at,
		items.updated_at`
}

func itemListOrderClause(order domain.ItemSortOrder) string {
	if order == domain.ItemSortOrderAsc {
		return itemOrderTimeSQL + " ASC, items.id ASC"
	}
	return itemOrderTimeSQL + " DESC, items.id DESC"
}

func itemCursorScope(options domain.ItemListOptions) func(*gorm.DB) *gorm.DB {
	return func(query *gorm.DB) *gorm.DB {
		cursor := options.Cursor
		if cursor == nil {
			return query
		}
		query = query.Where("items.created_at <= ?", cursor.Snapshot)
		if cursor.ID == 0 {
			return query
		}
		operator := "<"
		if options.SortOrder == domain.ItemSortOrderAsc {
			operator = ">"
		}
		return query.Where("("+itemOrderTimeSQL+", items.id) "+operator+" (?, ?)", cursor.Time, cursor.ID)
	}
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

func applyItemSourceFilters(query *gorm.DB, options domain.ItemListOptions) *gorm.DB {
	if options.SourceID > 0 {
		return query.Where("items.source_id = ?", options.SourceID)
	}
	query = query.Where("items.normalized_url NOT LIKE ?", hiddenAgentOperationLogURLPattern)
	if !itemSourceFilterRequiresActive(options) {
		return query
	}
	return query.Where(activeSourceStatusFilter, string(domain.SourceStatusActive))
}

func itemSourceFilterRequiresActive(options domain.ItemListOptions) bool {
	return options.SourceID <= 0
}

func upsertItem(ctx context.Context, db *gorm.DB, item domain.Item) (domain.Item, bool, error) {
	item = sanitizeItemText(item)
	model := itemModelFromDomain(item)

	existing, found, err := findExistingItem(ctx, db, item)
	if err != nil {
		return domain.Item{}, false, err
	}
	if !found {
		result := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&model)
		if result.Error != nil {
			return domain.Item{}, false, result.Error
		}
		if result.RowsAffected == 1 {
			return itemModelToDomain(model), true, nil
		}
		existing, found, err = findExistingItem(ctx, db, item)
		if err != nil {
			return domain.Item{}, false, err
		}
	}
	if found {
		model.ID = existing.ID
		model.CreatedAt = existing.CreatedAt
		if existing.PublicationVerified && !model.PublicationVerified || model.PublishedAt == nil {
			model.PublishedAt = existing.PublishedAt
			model.PublicationVerified = existing.PublicationVerified
		}
		if err := db.WithContext(ctx).
			Model(&itemModel{}).
			Where("id = ?", existing.ID).
			Select("Title", "URL", "NormalizedURL", "RawGUID", "ContentHash", "Summary", "ContentSnippet", "Author", "PublishedAt", "SourceUpdatedAt", "PublicationVerified", "FetchedAt").
			Updates(&model).Error; err != nil {
			return domain.Item{}, false, err
		}
		return itemModelToDomain(model), false, nil
	}

	return domain.Item{}, false, domain.ErrConflict
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
	item = sanitizeItemText(item)
	return itemModel{
		ID:                  item.ID,
		SourceID:            item.SourceID,
		Title:               item.Title,
		URL:                 item.URL,
		NormalizedURL:       item.NormalizedURL,
		RawGUID:             item.RawGUID,
		ContentHash:         item.ContentHash,
		Summary:             item.Summary,
		ContentSnippet:      item.ContentSnippet,
		Author:              item.Author,
		PublishedAt:         item.PublishedAt,
		SourceUpdatedAt:     item.SourceUpdatedAt,
		PublicationVerified: item.PublicationVerified,
		FetchedAt:           item.FetchedAt,
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
	}
}

func sanitizeItemText(item domain.Item) domain.Item {
	item.Title = strings.ToValidUTF8(item.Title, "")
	item.URL = strings.ToValidUTF8(item.URL, "")
	item.NormalizedURL = strings.ToValidUTF8(item.NormalizedURL, "")
	item.RawGUID = strings.ToValidUTF8(item.RawGUID, "")
	item.ContentHash = strings.ToValidUTF8(item.ContentHash, "")
	item.Summary = strings.ToValidUTF8(item.Summary, "")
	item.ContentSnippet = strings.ToValidUTF8(item.ContentSnippet, "")
	item.Author = strings.ToValidUTF8(item.Author, "")
	return item
}

func itemModelToDomain(model itemModel) domain.Item {
	return domain.Item{
		ID:                  model.ID,
		SourceID:            model.SourceID,
		Title:               model.Title,
		URL:                 model.URL,
		NormalizedURL:       model.NormalizedURL,
		RawGUID:             model.RawGUID,
		ContentHash:         model.ContentHash,
		Summary:             model.Summary,
		ContentSnippet:      model.ContentSnippet,
		Author:              model.Author,
		PublishedAt:         model.PublishedAt,
		SourceUpdatedAt:     model.SourceUpdatedAt,
		PublicationVerified: model.PublicationVerified,
		FetchedAt:           model.FetchedAt,
		CreatedAt:           model.CreatedAt,
		UpdatedAt:           model.UpdatedAt,
	}
}

func itemViewModelToDomain(model itemViewModel) domain.Item {
	return domain.Item{
		ID:              model.ID,
		SourceID:        model.SourceID,
		SourceName:      model.SourceName,
		CatalogID:       model.CatalogID,
		SubscriptionID:  model.SourceID,
		SourceKind:      "subscriptions",
		Title:           model.Title,
		URL:             model.URL,
		NormalizedURL:   model.NormalizedURL,
		RawGUID:         model.RawGUID,
		ContentHash:     model.ContentHash,
		Summary:         model.Summary,
		ContentSnippet:  model.ContentSnippet,
		Author:          model.Author,
		PublishedAt:     model.PublishedAt,
		SourceUpdatedAt: model.SourceUpdatedAt,
		FetchedAt:       model.FetchedAt,
		IsRead:          model.IsRead,
		ReadAt:          model.ReadAt,
		IsFavorite:      model.IsFavorite,
		FavoritedAt:     model.FavoritedAt,
		IsHidden:        model.IsHidden,
		HiddenAt:        model.HiddenAt,
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
	}
}
