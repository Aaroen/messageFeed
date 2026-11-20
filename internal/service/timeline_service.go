package service

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/observability"

	"go.opentelemetry.io/otel/attribute"
)

const (
	DefaultItemListLimit = 20
	MaxItemListLimit     = 500
)

type TimelineRepository interface {
	ListByUser(ctx context.Context, options domain.ItemListOptions) (domain.ItemListResult, error)
	GetByIDForUser(ctx context.Context, userID int64, itemID int64) (domain.Item, error)
	ListPublic(ctx context.Context, options domain.ItemListOptions) (domain.ItemListResult, error)
	GetByIDPublic(ctx context.Context, itemID int64) (domain.Item, error)
}

type TimelineService struct {
	repository TimelineRepository
}

func NewTimelineService(repository TimelineRepository) *TimelineService {
	return &TimelineService{repository: repository}
}

type ListItemsInput struct {
	UserID        int64
	SourceID      int64
	IsRead        *bool
	IsFavorite    *bool
	IsHidden      *bool
	IncludeHidden bool
	Limit         int
	Offset        int
	Order         string
	Cursor        *domain.ItemCursor
}

type ListItemsResult struct {
	Items         []domain.Item
	Total         int64
	Limit         int
	Offset        int
	NextCursor    *domain.ItemCursor
	Generation    string
	RefreshStatus string
}

func (s *TimelineService) ListItems(ctx context.Context, input ListItemsInput) (ListItemsResult, error) {
	ctx, span := observability.StartSpan(ctx, "service.timeline.list_items",
		attribute.Int64("user.id", input.UserID),
		attribute.Int("limit", input.Limit),
		attribute.Int("offset", input.Offset),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.repository == nil {
		opErr = fmt.Errorf("timeline service is not configured")
		return ListItemsResult{}, opErr
	}
	options, err := normalizeItemListOptions(input)
	if err != nil {
		opErr = err
		return ListItemsResult{}, opErr
	}
	if options.Cursor != nil && input.Offset != 0 {
		return ListItemsResult{}, fmt.Errorf("%w: cursor and offset cannot be combined", domain.ErrInvalidInput)
	}
	pageLimit := options.Limit
	if options.Cursor != nil {
		options.Limit++
	}

	var result domain.ItemListResult
	if input.UserID < 1 {
		result, err = s.repository.ListPublic(ctx, options)
	} else {
		result, err = s.repository.ListByUser(ctx, options)
	}
	if err != nil {
		opErr = err
		return ListItemsResult{}, opErr
	}
	span.SetAttributes(
		attribute.Int("items.count", len(result.Items)),
		attribute.Int64("items.total", result.Total),
		attribute.Int("limit.normalized", result.Limit),
		attribute.Int("offset.normalized", result.Offset),
	)
	var nextCursor *domain.ItemCursor
	if options.Cursor != nil && len(result.Items) > pageLimit {
		result.Items = result.Items[:pageLimit]
		last := result.Items[len(result.Items)-1]
		nextCursor = &domain.ItemCursor{Snapshot: options.Cursor.Snapshot, Time: last.FeedOrderTime(), ID: last.ID}
	}
	return ListItemsResult{
		Items:      result.Items,
		Total:      result.Total,
		Limit:      pageLimit,
		Offset:     result.Offset,
		NextCursor: nextCursor,
	}, nil
}

func (s *TimelineService) GetItem(ctx context.Context, input GetItemInput) (domain.Item, error) {
	ctx, span := observability.StartSpan(ctx, "service.timeline.get_item",
		attribute.Int64("user.id", input.UserID),
		attribute.Int64("item.id", input.ItemID),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.repository == nil {
		opErr = fmt.Errorf("timeline service is not configured")
		return domain.Item{}, opErr
	}
	if input.ItemID < 1 {
		opErr = fmt.Errorf("%w: item id must be positive", domain.ErrInvalidInput)
		return domain.Item{}, opErr
	}
	var item domain.Item
	var err error
	if input.UserID < 1 {
		item, err = s.repository.GetByIDPublic(ctx, input.ItemID)
	} else {
		item, err = s.repository.GetByIDForUser(ctx, input.UserID, input.ItemID)
	}
	if err != nil {
		opErr = err
		return domain.Item{}, opErr
	}
	return item, nil
}

type GetItemInput struct {
	UserID int64
	ItemID int64
}

func normalizeItemListOptions(input ListItemsInput) (domain.ItemListOptions, error) {
	if input.SourceID < 0 {
		return domain.ItemListOptions{}, fmt.Errorf("%w: source_id must be non-negative", domain.ErrInvalidInput)
	}
	if input.Offset < 0 {
		return domain.ItemListOptions{}, fmt.Errorf("%w: offset must be non-negative", domain.ErrInvalidInput)
	}

	limit := input.Limit
	if limit == 0 {
		limit = DefaultItemListLimit
	}
	if limit < 0 {
		return domain.ItemListOptions{}, fmt.Errorf("%w: limit must be non-negative", domain.ErrInvalidInput)
	}
	if limit > MaxItemListLimit {
		limit = MaxItemListLimit
	}

	return domain.ItemListOptions{
		UserID:        input.UserID,
		SourceID:      input.SourceID,
		IsRead:        input.IsRead,
		IsFavorite:    input.IsFavorite,
		IsHidden:      input.IsHidden,
		IncludeHidden: input.IncludeHidden,
		Limit:         limit,
		Offset:        input.Offset,
		SortOrder:     normalizeItemSortOrder(input.Order),
		Cursor:        input.Cursor,
	}, nil
}

func normalizeItemSortOrder(order string) domain.ItemSortOrder {
	if order == string(domain.ItemSortOrderAsc) {
		return domain.ItemSortOrderAsc
	}
	return domain.ItemSortOrderDesc
}
