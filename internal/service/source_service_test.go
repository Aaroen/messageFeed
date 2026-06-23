package service

import (
	"context"
	"errors"
	"messagefeed/internal/domain"
	"testing"
	"time"
	"unicode/utf8"
)

func TestCreateSourceNormalizesURLAndDefaults(t *testing.T) {
	repository := newFakeSourceRepository()
	service := NewSourceService(repository)

	source, err := service.CreateSource(context.Background(), CreateSourceInput{
		UserID: 1,
		URL:    "HTTPS://Example.COM:443/feed.xml#section",
		Tags:   []string{" go ", "go", "rss"},
	})
	if err != nil {
		t.Fatalf("CreateSource returned error: %v", err)
	}

	if source.Name != "example.com" {
		t.Fatalf("Name = %q, want %q", source.Name, "example.com")
	}
	if source.Type != domain.SourceTypeRSS {
		t.Fatalf("Type = %q, want %q", source.Type, domain.SourceTypeRSS)
	}
	if source.Status != domain.SourceStatusActive {
		t.Fatalf("Status = %q, want %q", source.Status, domain.SourceStatusActive)
	}
	if source.NormalizedURL != "https://example.com/feed.xml" {
		t.Fatalf("NormalizedURL = %q", source.NormalizedURL)
	}
	if source.FetchIntervalSeconds != DefaultSourceFetchIntervalSeconds {
		t.Fatalf("FetchIntervalSeconds = %d", source.FetchIntervalSeconds)
	}
	if got, want := len(source.Tags), 2; got != want {
		t.Fatalf("Tags length = %d, want %d", got, want)
	}
}

func TestCreateSourceRejectsInvalidURL(t *testing.T) {
	service := NewSourceService(newFakeSourceRepository())

	_, err := service.CreateSource(context.Background(), CreateSourceInput{
		UserID: 1,
		URL:    "ftp://example.com/feed.xml",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

func TestUpdateSource(t *testing.T) {
	repository := newFakeSourceRepository()
	service := NewSourceService(repository)

	source, err := service.CreateSource(context.Background(), CreateSourceInput{
		UserID: 1,
		Name:   "Old",
		URL:    "https://example.com/feed.xml",
	})
	if err != nil {
		t.Fatalf("CreateSource returned error: %v", err)
	}

	name := "New"
	status := domain.SourceStatusInactive
	interval := 7200
	updated, err := service.UpdateSource(context.Background(), UpdateSourceInput{
		UserID:               1,
		ID:                   source.ID,
		Name:                 &name,
		Status:               &status,
		FetchIntervalSeconds: &interval,
	})
	if err != nil {
		t.Fatalf("UpdateSource returned error: %v", err)
	}

	if updated.Name != name {
		t.Fatalf("Name = %q, want %q", updated.Name, name)
	}
	if updated.Status != status {
		t.Fatalf("Status = %q, want %q", updated.Status, status)
	}
	if updated.FetchIntervalSeconds != interval {
		t.Fatalf("FetchIntervalSeconds = %d, want %d", updated.FetchIntervalSeconds, interval)
	}
}

func TestTriggerFetchStoresItemsAndUpdatesSource(t *testing.T) {
	sourceRepository := newFakeSourceRepository()
	itemRepository := &fakeItemRepository{}
	feedFetcher := &fakeFeedFetcher{}
	service := NewSourceService(
		sourceRepository,
		WithItemRepository(itemRepository),
		WithFeedFetcher(feedFetcher),
		WithNow(func() time.Time {
			return time.Date(2026, 6, 17, 9, 0, 0, 0, time.UTC)
		}),
	)

	source, err := service.CreateSource(context.Background(), CreateSourceInput{
		UserID: 1,
		URL:    "https://example.com/feed.xml",
	})
	if err != nil {
		t.Fatalf("CreateSource returned error: %v", err)
	}

	result, err := service.TriggerFetch(context.Background(), FetchSourceInput{
		UserID: 1,
		ID:     source.ID,
	})
	if err != nil {
		t.Fatalf("TriggerFetch returned error: %v", err)
	}

	if result.ItemCount != 1 {
		t.Fatalf("ItemCount = %d, want 1", result.ItemCount)
	}
	if result.CreatedCount != 1 {
		t.Fatalf("CreatedCount = %d, want 1", result.CreatedCount)
	}
	if result.Source.LastFetchStatus != domain.SourceLastFetchStatusSuccess {
		t.Fatalf("LastFetchStatus = %q", result.Source.LastFetchStatus)
	}
	if result.Source.LastFetchItemCount == nil || *result.Source.LastFetchItemCount != 1 {
		t.Fatalf("LastFetchItemCount = %#v, want 1", result.Source.LastFetchItemCount)
	}
	if got, want := len(itemRepository.items), 1; got != want {
		t.Fatalf("stored items length = %d, want %d", got, want)
	}
}

func TestTriggerFetchAllowsInactiveSource(t *testing.T) {
	sourceRepository := newFakeSourceRepository()
	itemRepository := &fakeItemRepository{}
	service := NewSourceService(
		sourceRepository,
		WithItemRepository(itemRepository),
		WithFeedFetcher(&fakeFeedFetcher{}),
	)

	source, err := service.CreateSource(context.Background(), CreateSourceInput{
		UserID: 1,
		URL:    "https://example.com/feed.xml",
	})
	if err != nil {
		t.Fatalf("CreateSource returned error: %v", err)
	}

	status := domain.SourceStatusInactive
	source, err = service.UpdateSource(context.Background(), UpdateSourceInput{
		UserID: 1,
		ID:     source.ID,
		Status: &status,
	})
	if err != nil {
		t.Fatalf("UpdateSource returned error: %v", err)
	}

	result, err := service.TriggerFetch(context.Background(), FetchSourceInput{
		UserID: 1,
		ID:     source.ID,
	})
	if err != nil {
		t.Fatalf("TriggerFetch returned error: %v", err)
	}
	if result.ItemCount != 1 {
		t.Fatalf("ItemCount = %d, want 1", result.ItemCount)
	}
	if got, want := len(itemRepository.items), 1; got != want {
		t.Fatalf("stored items length = %d, want %d", got, want)
	}
}

func TestTriggerFetchMarksSourceFailed(t *testing.T) {
	sourceRepository := newFakeSourceRepository()
	service := NewSourceService(
		sourceRepository,
		WithItemRepository(&fakeItemRepository{}),
		WithFeedFetcher(&fakeFeedFetcher{err: errors.New("network failed")}),
		WithNow(func() time.Time {
			return time.Date(2026, 6, 17, 9, 0, 0, 0, time.UTC)
		}),
	)

	source, err := service.CreateSource(context.Background(), CreateSourceInput{
		UserID: 1,
		URL:    "https://example.com/feed.xml",
	})
	if err != nil {
		t.Fatalf("CreateSource returned error: %v", err)
	}

	_, err = service.TriggerFetch(context.Background(), FetchSourceInput{
		UserID: 1,
		ID:     source.ID,
	})
	if err == nil {
		t.Fatal("TriggerFetch returned nil error")
	}

	updated, err := sourceRepository.GetByID(context.Background(), 1, source.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if updated.LastFetchStatus != domain.SourceLastFetchStatusFailed {
		t.Fatalf("LastFetchStatus = %q", updated.LastFetchStatus)
	}
	if updated.LastFetchError == "" {
		t.Fatal("LastFetchError is empty")
	}
}

func TestTriggerFetchSanitizesInvalidUTF8FailureMessage(t *testing.T) {
	sourceRepository := newFakeSourceRepository()
	invalidSuffix := string([]byte{0xe6, 0xb5})
	service := NewSourceService(
		sourceRepository,
		WithItemRepository(&fakeItemRepository{}),
		WithFeedFetcher(&fakeFeedFetcher{err: errors.New("repository: " + invalidSuffix + " failed")}),
	)

	source, err := service.CreateSource(context.Background(), CreateSourceInput{
		UserID: 1,
		URL:    "https://example.com/feed.xml",
	})
	if err != nil {
		t.Fatalf("CreateSource returned error: %v", err)
	}

	_, err = service.TriggerFetch(context.Background(), FetchSourceInput{
		UserID: 1,
		ID:     source.ID,
	})
	if err == nil {
		t.Fatal("TriggerFetch returned nil error")
	}

	updated, err := sourceRepository.GetByID(context.Background(), 1, source.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if !utf8.ValidString(updated.LastFetchError) {
		t.Fatalf("LastFetchError contains invalid UTF-8: %q", updated.LastFetchError)
	}
	if updated.LastFetchError != "repository:  failed" {
		t.Fatalf("LastFetchError = %q, want sanitized message", updated.LastFetchError)
	}
}

func TestTruncateErrorPreservesUTF8Boundary(t *testing.T) {
	got := truncateError("刷新失败：消息流异常", len("刷新失败：消")-1)
	if !utf8.ValidString(got) {
		t.Fatalf("truncateError returned invalid UTF-8: %q", got)
	}
	if len(got) > len("刷新失败：消")-1 {
		t.Fatalf("truncateError length = %d, want <= %d", len(got), len("刷新失败：消")-1)
	}
}

type fakeSourceRepository struct {
	nextID  int64
	sources map[int64]domain.Source
}

func newFakeSourceRepository() *fakeSourceRepository {
	return &fakeSourceRepository{
		nextID:  1,
		sources: make(map[int64]domain.Source),
	}
}

func (r *fakeSourceRepository) Create(_ context.Context, source domain.Source) (domain.Source, error) {
	for _, existing := range r.sources {
		if existing.UserID == source.UserID && existing.NormalizedURL == source.NormalizedURL {
			return domain.Source{}, domain.ErrConflict
		}
	}
	now := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	source.ID = r.nextID
	source.CreatedAt = now
	source.UpdatedAt = now
	r.nextID++
	r.sources[source.ID] = source
	return source, nil
}

func (r *fakeSourceRepository) ListByUser(_ context.Context, userID int64) ([]domain.Source, error) {
	var sources []domain.Source
	for _, source := range r.sources {
		if source.UserID == userID {
			sources = append(sources, source)
		}
	}
	return sources, nil
}

func (r *fakeSourceRepository) GetByID(_ context.Context, userID int64, id int64) (domain.Source, error) {
	source, ok := r.sources[id]
	if !ok || source.UserID != userID {
		return domain.Source{}, domain.ErrNotFound
	}
	return source, nil
}

func (r *fakeSourceRepository) Update(_ context.Context, source domain.Source) (domain.Source, error) {
	if _, ok := r.sources[source.ID]; !ok {
		return domain.Source{}, domain.ErrNotFound
	}
	source.UpdatedAt = time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	r.sources[source.ID] = source
	return source, nil
}

func (r *fakeSourceRepository) UpdateFetchResult(_ context.Context, source domain.Source) (domain.Source, error) {
	existing, ok := r.sources[source.ID]
	if !ok || existing.UserID != source.UserID {
		return domain.Source{}, domain.ErrNotFound
	}
	existing.LastFetchedAt = source.LastFetchedAt
	existing.LastFetchStatus = source.LastFetchStatus
	existing.LastFetchError = source.LastFetchError
	existing.LastFetchDurationMS = source.LastFetchDurationMS
	existing.LastFetchItemCount = source.LastFetchItemCount
	existing.UpdatedAt = time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	r.sources[source.ID] = existing
	return existing, nil
}

type fakeItemRepository struct {
	items []domain.Item
}

func (r *fakeItemRepository) UpsertMany(_ context.Context, items []domain.Item) (domain.ItemUpsertResult, error) {
	r.items = append(r.items, items...)
	return domain.ItemUpsertResult{
		CreatedCount: len(items),
		TotalCount:   len(items),
	}, nil
}

type fakeFeedFetcher struct {
	err error
}

func (f *fakeFeedFetcher) Fetch(_ context.Context, source domain.Source) (domain.FeedFetchResult, error) {
	if f.err != nil {
		return domain.FeedFetchResult{}, f.err
	}
	return domain.FeedFetchResult{
		Items: []domain.Item{
			{
				SourceID:      source.ID,
				Title:         "Fetched item",
				URL:           "https://example.com/item",
				NormalizedURL: "https://example.com/item",
				RawGUID:       "item-guid",
				FetchedAt:     time.Date(2026, 6, 17, 9, 0, 0, 0, time.UTC),
			},
		},
	}, nil
}
