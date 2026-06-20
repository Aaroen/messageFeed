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
