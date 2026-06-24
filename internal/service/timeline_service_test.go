package service

import (
	"context"
	"errors"
	"messagefeed/internal/domain"
	"testing"
)

func TestListItemsNormalizesPagination(t *testing.T) {
	repository := &fakeTimelineRepository{}
	service := NewTimelineService(repository)
	isFavorite := true

	result, err := service.ListItems(context.Background(), ListItemsInput{
		UserID:        1,
		IsFavorite:    &isFavorite,
		IncludeHidden: true,
		Limit:         999,
	})
	if err != nil {
		t.Fatalf("ListItems returned error: %v", err)
	}

	if result.Limit != MaxItemListLimit {
		t.Fatalf("Limit = %d, want %d", result.Limit, MaxItemListLimit)
	}
	if repository.options.Limit != MaxItemListLimit {
		t.Fatalf("repository Limit = %d, want %d", repository.options.Limit, MaxItemListLimit)
	}
	if repository.options.IsFavorite == nil || !*repository.options.IsFavorite {
		t.Fatalf("repository IsFavorite = %#v, want true", repository.options.IsFavorite)
	}
	if !repository.options.IncludeHidden {
		t.Fatal("repository IncludeHidden = false, want true")
	}
}

func TestListItemsRejectsInvalidOffset(t *testing.T) {
	service := NewTimelineService(&fakeTimelineRepository{})

	_, err := service.ListItems(context.Background(), ListItemsInput{
		UserID: 1,
		Offset: -1,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

func TestListItemsAllowsPublicRead(t *testing.T) {
	repository := &fakeTimelineRepository{}
	service := NewTimelineService(repository)

	result, err := service.ListItems(context.Background(), ListItemsInput{Limit: 5})
	if err != nil {
		t.Fatalf("ListItems returned error: %v", err)
	}
	if !repository.publicList {
		t.Fatal("repository public list was not used")
	}
	if result.Limit != 5 {
		t.Fatalf("Limit = %d, want 5", result.Limit)
	}
}

func TestGetItemAllowsPublicRead(t *testing.T) {
	repository := &fakeTimelineRepository{}
	service := NewTimelineService(repository)

	item, err := service.GetItem(context.Background(), GetItemInput{ItemID: 7})
	if err != nil {
		t.Fatalf("GetItem returned error: %v", err)
	}
	if !repository.publicGet {
		t.Fatal("repository public get was not used")
	}
	if item.ID != 7 {
		t.Fatalf("item ID = %d, want 7", item.ID)
	}
}

func TestGetItemRejectsInvalidItemID(t *testing.T) {
	service := NewTimelineService(&fakeTimelineRepository{})

	_, err := service.GetItem(context.Background(), GetItemInput{
		UserID: 1,
		ItemID: 0,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

type fakeTimelineRepository struct {
	options    domain.ItemListOptions
	publicList bool
	publicGet  bool
}

func (r *fakeTimelineRepository) ListByUser(_ context.Context, options domain.ItemListOptions) (domain.ItemListResult, error) {
	r.options = options
	return domain.ItemListResult{
		Items: []domain.Item{
			{
				ID:            1,
				SourceID:      1,
				Title:         "Item",
				URL:           "https://example.com/item",
				NormalizedURL: "https://example.com/item",
			},
		},
		Total:  1,
		Limit:  options.Limit,
		Offset: options.Offset,
	}, nil
}

func (r *fakeTimelineRepository) GetByIDForUser(_ context.Context, userID int64, itemID int64) (domain.Item, error) {
	return domain.Item{
		ID:            itemID,
		SourceID:      1,
		Title:         "Item",
		URL:           "https://example.com/item",
		NormalizedURL: "https://example.com/item",
	}, nil
}

func (r *fakeTimelineRepository) ListPublic(_ context.Context, options domain.ItemListOptions) (domain.ItemListResult, error) {
	r.options = options
	r.publicList = true
	return domain.ItemListResult{
		Items: []domain.Item{
			{
				ID:            1,
				SourceID:      1,
				Title:         "Public item",
				URL:           "https://example.com/public",
				NormalizedURL: "https://example.com/public",
			},
		},
		Total:  1,
		Limit:  options.Limit,
		Offset: options.Offset,
	}, nil
}

func (r *fakeTimelineRepository) GetByIDPublic(_ context.Context, itemID int64) (domain.Item, error) {
	r.publicGet = true
	return domain.Item{
		ID:            itemID,
		SourceID:      1,
		Title:         "Public item",
		URL:           "https://example.com/public",
		NormalizedURL: "https://example.com/public",
	}, nil
}
