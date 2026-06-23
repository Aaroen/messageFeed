package service

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestListRecommendationsRefreshBypassesCache(t *testing.T) {
	now := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)
	catalogRepository := &fakeRecommendationCatalogRepository{
		entries: []domain.SourceCatalogEntry{
			{
				ID:            1,
				Name:          "Example",
				FeedURL:       "https://example.com/feed.xml",
				NormalizedURL: "https://example.com/feed.xml",
				Type:          domain.SourceTypeRSS,
				Official:      true,
				HealthStatus:  domain.SourceCatalogHealthHealthy,
			},
		},
	}
	feedFetcher := &sequencedRecommendationFetcher{}
	service := NewRecommendationService(
		catalogRepository,
		feedFetcher,
		WithNow(func() time.Time {
			return now
		}),
	)

	first, err := service.ListRecommendations(context.Background(), ListRecommendationsInput{
		UserID: 1,
		Limit:  10,
		Order:  string(domain.ItemSortOrderAsc),
	})
	if err != nil {
		t.Fatalf("ListRecommendations returned error: %v", err)
	}
	if got, want := first.Items[0].Title, "Fetched item 1"; got != want {
		t.Fatalf("first title = %q, want %q", got, want)
	}

	second, err := service.ListRecommendations(context.Background(), ListRecommendationsInput{
		UserID:  1,
		Limit:   10,
		Order:   string(domain.ItemSortOrderAsc),
		Refresh: true,
	})
	if err != nil {
		t.Fatalf("refresh ListRecommendations returned error: %v", err)
	}
	if got, want := second.Items[0].Title, "Fetched item 2"; got != want {
		t.Fatalf("refresh title = %q, want %q", got, want)
	}

	third, err := service.ListRecommendations(context.Background(), ListRecommendationsInput{
		UserID: 1,
		Limit:  10,
		Order:  string(domain.ItemSortOrderAsc),
	})
	if err != nil {
		t.Fatalf("cached ListRecommendations returned error: %v", err)
	}
	if got, want := third.Items[0].Title, "Fetched item 2"; got != want {
		t.Fatalf("cached title = %q, want %q", got, want)
	}
	if got, want := feedFetcher.calls, 2; got != want {
		t.Fatalf("fetch calls = %d, want %d", got, want)
	}
}

func TestRecommendationRefreshShuffleSeedUsesInstant(t *testing.T) {
	first := time.Date(2026, 6, 20, 9, 0, 0, 1, time.UTC)
	second := time.Date(2026, 6, 20, 9, 0, 0, 2, time.UTC)
	if recommendationShuffleSeed(1, first) != recommendationShuffleSeed(1, second) {
		t.Fatal("daily recommendation seed changed within the same day")
	}
	if recommendationRefreshShuffleSeed(1, 0, first) == recommendationRefreshShuffleSeed(1, 0, second) {
		t.Fatal("refresh recommendation seed did not change across instants")
	}
}

func TestListRecommendationsPaginatesCachedItems(t *testing.T) {
	now := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)
	catalogRepository := &fakeRecommendationCatalogRepository{
		entries: []domain.SourceCatalogEntry{
			recommendationCatalogEntry(1),
			recommendationCatalogEntry(2),
			recommendationCatalogEntry(3),
			recommendationCatalogEntry(4),
			recommendationCatalogEntry(5),
		},
	}
	feedFetcher := &multiItemRecommendationFetcher{itemsPerSource: 3}
	service := NewRecommendationService(
		catalogRepository,
		feedFetcher,
		WithNow(func() time.Time {
			return now
		}),
	)

	first, err := service.ListRecommendations(context.Background(), ListRecommendationsInput{
		UserID: 1,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("ListRecommendations returned error: %v", err)
	}
	if got, want := len(first.Items), 10; got != want {
		t.Fatalf("first page count = %d, want %d", got, want)
	}
	if got, want := first.Total, int64(15); got != want {
		t.Fatalf("first page total = %d, want %d", got, want)
	}

	second, err := service.ListRecommendations(context.Background(), ListRecommendationsInput{
		UserID: 1,
		Limit:  10,
		Offset: 10,
	})
	if err != nil {
		t.Fatalf("second page ListRecommendations returned error: %v", err)
	}
	if got, want := len(second.Items), 5; got != want {
		t.Fatalf("second page count = %d, want %d", got, want)
	}
	if got, want := second.Total, int64(15); got != want {
		t.Fatalf("second page total = %d, want %d", got, want)
	}
	if got, want := feedFetcher.calls, 5; got != want {
		t.Fatalf("fetch calls = %d, want %d", got, want)
	}

	firstIDs := make(map[int64]struct{}, len(first.Items))
	for _, item := range first.Items {
		firstIDs[item.ID] = struct{}{}
	}
	for _, item := range second.Items {
		if _, exists := firstIDs[item.ID]; exists {
			t.Fatalf("second page item %d also appeared on first page", item.ID)
		}
	}
}

func TestListRecommendationsPaginatesSourceItemsBeyondRecommendationCap(t *testing.T) {
	now := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)
	catalogRepository := &fakeRecommendationCatalogRepository{
		entries: []domain.SourceCatalogEntry{
			recommendationCatalogEntry(1),
		},
	}
	feedFetcher := &multiItemRecommendationFetcher{itemsPerSource: 12}
	service := NewRecommendationService(
		catalogRepository,
		feedFetcher,
		WithNow(func() time.Time {
			return now
		}),
	)

	first, err := service.ListRecommendations(context.Background(), ListRecommendationsInput{
		UserID:   1,
		SourceID: 1,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("ListRecommendations returned error: %v", err)
	}
	if got, want := len(first.Items), 10; got != want {
		t.Fatalf("first page count = %d, want %d", got, want)
	}
	if got, want := first.Total, int64(12); got != want {
		t.Fatalf("first page total = %d, want %d", got, want)
	}

	second, err := service.ListRecommendations(context.Background(), ListRecommendationsInput{
		UserID:   1,
		SourceID: 1,
		Limit:    10,
		Offset:   10,
	})
	if err != nil {
		t.Fatalf("second page ListRecommendations returned error: %v", err)
	}
	if got, want := len(second.Items), 2; got != want {
		t.Fatalf("second page count = %d, want %d", got, want)
	}
	if got, want := second.Total, int64(12); got != want {
		t.Fatalf("second page total = %d, want %d", got, want)
	}
	if got, want := feedFetcher.calls, 1; got != want {
		t.Fatalf("fetch calls = %d, want %d", got, want)
	}
}

func TestListRecommendationsSourceUsesSubscribedHistoryWhenAvailable(t *testing.T) {
	now := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)
	catalogRepository := &fakeRecommendationCatalogRepository{
		entries: []domain.SourceCatalogEntry{
			{
				ID:            10,
				Name:          "Catalog source",
				FeedURL:       "https://example.com/feed.xml",
				NormalizedURL: "https://example.com/feed.xml",
				Type:          domain.SourceTypeRSS,
				Official:      true,
				HealthStatus:  domain.SourceCatalogHealthHealthy,
			},
		},
	}
	feedFetcher := &multiItemRecommendationFetcher{itemsPerSource: 12}
	service := NewRecommendationService(
		catalogRepository,
		feedFetcher,
		WithNow(func() time.Time {
			return now
		}),
	)
	sourceRepository := &fakeRecommendationSourceRepository{
		sources: []domain.Source{
			{
				ID:            200,
				UserID:        1,
				Name:          "Subscribed source",
				Type:          domain.SourceTypeRSS,
				URL:           "https://example.com/feed.xml",
				NormalizedURL: "https://example.com/feed.xml",
				Status:        domain.SourceStatusActive,
			},
		},
	}
	itemRepository := &fakeRecommendationItemRepository{
		items: recommendationHistoryItems(200, 12),
	}
	service.SetLocalHistoryRepositories(sourceRepository, itemRepository)

	result, err := service.ListRecommendations(context.Background(), ListRecommendationsInput{
		UserID:   1,
		SourceID: 10,
		Limit:    10,
		Offset:   10,
	})
	if err != nil {
		t.Fatalf("ListRecommendations returned error: %v", err)
	}

	if got, want := len(result.Items), 2; got != want {
		t.Fatalf("items length = %d, want %d", got, want)
	}
	if got, want := result.Total, int64(12); got != want {
		t.Fatalf("total = %d, want %d", got, want)
	}
	if got, want := result.Items[0].SourceID, int64(200); got != want {
		t.Fatalf("first item SourceID = %d, want %d", got, want)
	}
	if got, want := itemRepository.options.SourceID, int64(200); got != want {
		t.Fatalf("history SourceID option = %d, want %d", got, want)
	}
	if got, want := itemRepository.options.Offset, 10; got != want {
		t.Fatalf("history Offset option = %d, want %d", got, want)
	}
	if got, want := feedFetcher.calls, 0; got != want {
		t.Fatalf("fetch calls = %d, want %d", got, want)
	}
}

type fakeRecommendationCatalogRepository struct {
	entries []domain.SourceCatalogEntry
}

func (r *fakeRecommendationCatalogRepository) List(_ context.Context, options domain.SourceCatalogListOptions) (domain.SourceCatalogListResult, error) {
	entries := append([]domain.SourceCatalogEntry(nil), r.entries...)
	if options.Offset > len(entries) {
		entries = nil
	} else {
		entries = entries[options.Offset:]
	}
	if options.Limit > 0 && len(entries) > options.Limit {
		entries = entries[:options.Limit]
	}
	return domain.SourceCatalogListResult{
		Entries: entries,
		Total:   int64(len(r.entries)),
		Limit:   options.Limit,
		Offset:  options.Offset,
	}, nil
}

func (r *fakeRecommendationCatalogRepository) GetByIDs(_ context.Context, ids []int64) ([]domain.SourceCatalogEntry, error) {
	idSet := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	entries := make([]domain.SourceCatalogEntry, 0, len(ids))
	for _, entry := range r.entries {
		if _, ok := idSet[entry.ID]; ok {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

type sequencedRecommendationFetcher struct {
	calls int
}

func (f *sequencedRecommendationFetcher) Fetch(_ context.Context, source domain.Source) (domain.FeedFetchResult, error) {
	f.calls++
	url := fmt.Sprintf("https://example.com/items/%d", f.calls)
	return domain.FeedFetchResult{
		Items: []domain.Item{
			{
				SourceID:      source.ID,
				Title:         fmt.Sprintf("Fetched item %d", f.calls),
				URL:           url,
				NormalizedURL: url,
				RawGUID:       fmt.Sprintf("item-%d", f.calls),
				FetchedAt:     time.Date(2026, 6, 20, 9, f.calls, 0, 0, time.UTC),
			},
		},
	}, nil
}

func recommendationCatalogEntry(id int64) domain.SourceCatalogEntry {
	return domain.SourceCatalogEntry{
		ID:            id,
		Name:          fmt.Sprintf("Example %d", id),
		FeedURL:       fmt.Sprintf("https://example.com/%d/feed.xml", id),
		NormalizedURL: fmt.Sprintf("https://example.com/%d/feed.xml", id),
		Type:          domain.SourceTypeRSS,
		Official:      true,
		HealthStatus:  domain.SourceCatalogHealthHealthy,
	}
}

type multiItemRecommendationFetcher struct {
	calls          int
	itemsPerSource int
}

func (f *multiItemRecommendationFetcher) Fetch(_ context.Context, source domain.Source) (domain.FeedFetchResult, error) {
	f.calls++
	items := make([]domain.Item, 0, f.itemsPerSource)
	for index := 0; index < f.itemsPerSource; index++ {
		url := fmt.Sprintf("https://example.com/%d/items/%d", source.ID, index)
		items = append(items, domain.Item{
			SourceID:      source.ID,
			Title:         fmt.Sprintf("Source %d item %d", source.ID, index),
			URL:           url,
			NormalizedURL: url,
			RawGUID:       fmt.Sprintf("source-%d-item-%d", source.ID, index),
			FetchedAt:     time.Date(2026, 6, 20, 9, int(source.ID), index, 0, time.UTC),
		})
	}
	return domain.FeedFetchResult{Items: items}, nil
}

type fakeRecommendationSourceRepository struct {
	sources []domain.Source
}

func (r *fakeRecommendationSourceRepository) ListByUser(_ context.Context, userID int64) ([]domain.Source, error) {
	sources := make([]domain.Source, 0, len(r.sources))
	for _, source := range r.sources {
		if source.UserID == userID {
			sources = append(sources, source)
		}
	}
	return sources, nil
}

type fakeRecommendationItemRepository struct {
	items   []domain.Item
	options domain.ItemListOptions
}

func (r *fakeRecommendationItemRepository) ListByUser(_ context.Context, options domain.ItemListOptions) (domain.ItemListResult, error) {
	r.options = options
	items := make([]domain.Item, 0, len(r.items))
	for _, item := range r.items {
		if item.SourceID == options.SourceID {
			items = append(items, item)
		}
	}
	total := int64(len(items))
	if options.Offset > len(items) {
		items = nil
	} else {
		items = items[options.Offset:]
	}
	if options.Limit > 0 && len(items) > options.Limit {
		items = items[:options.Limit]
	}
	return domain.ItemListResult{
		Items:  items,
		Total:  total,
		Limit:  options.Limit,
		Offset: options.Offset,
	}, nil
}

func recommendationHistoryItems(sourceID int64, count int) []domain.Item {
	items := make([]domain.Item, 0, count)
	for index := 0; index < count; index++ {
		url := fmt.Sprintf("https://example.com/history/%d", index)
		items = append(items, domain.Item{
			ID:            int64(index + 1),
			SourceID:      sourceID,
			SourceName:    "Subscribed source",
			Title:         fmt.Sprintf("History item %d", index),
			URL:           url,
			NormalizedURL: url,
			FetchedAt:     time.Date(2026, 6, 20, 8, index, 0, 0, time.UTC),
			CreatedAt:     time.Date(2026, 6, 20, 8, index, 0, 0, time.UTC),
			UpdatedAt:     time.Date(2026, 6, 20, 8, index, 0, 0, time.UTC),
		})
	}
	return items
}
