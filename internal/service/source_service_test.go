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

func TestImportURLSourcesRecordsImportJob(t *testing.T) {
	importJobRepository := &fakeSourceImportJobRepository{}
	service := NewSourceService(
		newFakeSourceRepository(),
		WithSourceImportJobRepository(importJobRepository),
	)

	result, err := service.ImportURLSources(context.Background(), ImportURLSourcesInput{
		UserID: 1,
		URLs: []string{
			"https://example.com/feed.xml",
			"ftp://invalid.example/feed.xml",
		},
	})
	if err != nil {
		t.Fatalf("ImportURLSources returned error: %v", err)
	}

	if result.RequestedCount != 2 {
		t.Fatalf("RequestedCount = %d, want 2", result.RequestedCount)
	}
	if result.SuccessCount != 1 {
		t.Fatalf("SuccessCount = %d, want 1", result.SuccessCount)
	}
	if result.FailureCount != 1 {
		t.Fatalf("FailureCount = %d, want 1", result.FailureCount)
	}
	if result.ImportJob == nil {
		t.Fatal("ImportJob is nil")
	}
	if result.ImportJob.Status != domain.SourceImportStatusPartial {
		t.Fatalf("ImportJob.Status = %q, want %q", result.ImportJob.Status, domain.SourceImportStatusPartial)
	}
	if got, want := len(importJobRepository.jobs), 1; got != want {
		t.Fatalf("recorded jobs length = %d, want %d", got, want)
	}
	job := importJobRepository.jobs[0]
	if job.ImportType != domain.SourceImportTypeURLs {
		t.Fatalf("ImportType = %q, want %q", job.ImportType, domain.SourceImportTypeURLs)
	}
	if got, want := len(job.ErrorDetails), 1; got != want {
		t.Fatalf("ErrorDetails length = %d, want %d", got, want)
	}
	if job.ErrorDetails[0].Reference != "ftp://invalid.example/feed.xml" {
		t.Fatalf("ErrorDetails[0].Reference = %q", job.ErrorDetails[0].Reference)
	}
}

func TestImportURLSourcesRejectsUnsupportedTypeBeforeCreatingSources(t *testing.T) {
	sourceRepository := newFakeSourceRepository()
	importJobRepository := &fakeSourceImportJobRepository{}
	service := NewSourceService(
		sourceRepository,
		WithSourceImportJobRepository(importJobRepository),
	)

	_, err := service.ImportURLSources(context.Background(), ImportURLSourcesInput{
		UserID:     1,
		URLs:       []string{"https://example.com/feed.xml"},
		ImportType: domain.SourceImportType("invalid"),
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}

	sources, err := sourceRepository.ListByUser(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListByUser returned error: %v", err)
	}
	if got, want := len(sources), 0; got != want {
		t.Fatalf("sources length = %d, want %d", got, want)
	}
	if got, want := len(importJobRepository.jobs), 0; got != want {
		t.Fatalf("recorded jobs length = %d, want %d", got, want)
	}
}

func TestImportCatalogSourcesRecordsImportJob(t *testing.T) {
	importJobRepository := &fakeSourceImportJobRepository{}
	service := NewSourceService(
		newFakeSourceRepository(),
		WithSourceCatalogRepository(&fakeSourceCatalogRepository{
			entries: []domain.SourceCatalogEntry{
				{
					ID:      1,
					Name:    "Example",
					Type:    domain.SourceTypeRSS,
					FeedURL: "https://example.com/feed.xml",
					Tags:    []string{"example"},
				},
			},
		}),
		WithSourceImportJobRepository(importJobRepository),
	)

	result, err := service.ImportCatalogSources(context.Background(), ImportCatalogSourcesInput{
		UserID:     1,
		CatalogIDs: []int64{1, 2},
	})
	if err != nil {
		t.Fatalf("ImportCatalogSources returned error: %v", err)
	}

	if result.RequestedCount != 2 {
		t.Fatalf("RequestedCount = %d, want 2", result.RequestedCount)
	}
	if result.SuccessCount != 1 {
		t.Fatalf("SuccessCount = %d, want 1", result.SuccessCount)
	}
	if result.FailureCount != 1 {
		t.Fatalf("FailureCount = %d, want 1", result.FailureCount)
	}
	if result.ImportJob == nil {
		t.Fatal("ImportJob is nil")
	}
	if result.ImportJob.ImportType != domain.SourceImportTypeCatalog {
		t.Fatalf("ImportType = %q, want %q", result.ImportJob.ImportType, domain.SourceImportTypeCatalog)
	}
	if result.ImportJob.Status != domain.SourceImportStatusPartial {
		t.Fatalf("Status = %q, want %q", result.ImportJob.Status, domain.SourceImportStatusPartial)
	}
	if got, want := len(importJobRepository.jobs), 1; got != want {
		t.Fatalf("recorded jobs length = %d, want %d", got, want)
	}
	if importJobRepository.jobs[0].ErrorDetails[0].Reference != "2" {
		t.Fatalf("ErrorDetails[0].Reference = %q, want 2", importJobRepository.jobs[0].ErrorDetails[0].Reference)
	}
}

func TestListSourceImportJobsNormalizesPagination(t *testing.T) {
	importJobRepository := &fakeSourceImportJobRepository{
		jobs: []domain.SourceImportJob{
			{
				ID:         1,
				UserID:     1,
				ImportType: domain.SourceImportTypeURLs,
				Status:     domain.SourceImportStatusCompleted,
			},
		},
	}
	service := NewSourceService(
		newFakeSourceRepository(),
		WithSourceImportJobRepository(importJobRepository),
	)

	result, err := service.ListSourceImportJobs(context.Background(), ListSourceImportJobsInput{
		UserID: 1,
		Limit:  domain.MaxSourceImportJobListLimit + 1,
		Offset: 3,
	})
	if err != nil {
		t.Fatalf("ListSourceImportJobs returned error: %v", err)
	}

	if importJobRepository.listOptions.UserID != 1 {
		t.Fatalf("UserID = %d, want 1", importJobRepository.listOptions.UserID)
	}
	if importJobRepository.listOptions.Limit != domain.MaxSourceImportJobListLimit {
		t.Fatalf("Limit = %d, want %d", importJobRepository.listOptions.Limit, domain.MaxSourceImportJobListLimit)
	}
	if importJobRepository.listOptions.Offset != 3 {
		t.Fatalf("Offset = %d, want 3", importJobRepository.listOptions.Offset)
	}
	if result.Total != 1 {
		t.Fatalf("Total = %d, want 1", result.Total)
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

type fakeSourceImportJobRepository struct {
	nextID      int64
	jobs        []domain.SourceImportJob
	listOptions domain.SourceImportJobListOptions
}

func (r *fakeSourceImportJobRepository) Create(_ context.Context, job domain.SourceImportJob) (domain.SourceImportJob, error) {
	if r.nextID == 0 {
		r.nextID = int64(len(r.jobs) + 1)
	}
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	job.ID = r.nextID
	job.CreatedAt = now
	job.UpdatedAt = now
	r.nextID++
	r.jobs = append(r.jobs, job)
	return job, nil
}

func (r *fakeSourceImportJobRepository) ListByUser(_ context.Context, options domain.SourceImportJobListOptions) (domain.SourceImportJobListResult, error) {
	r.listOptions = options
	jobs := make([]domain.SourceImportJob, 0, len(r.jobs))
	for _, job := range r.jobs {
		if job.UserID == options.UserID {
			jobs = append(jobs, job)
		}
	}
	return domain.SourceImportJobListResult{
		Jobs:   jobs,
		Total:  int64(len(jobs)),
		Limit:  options.Limit,
		Offset: options.Offset,
	}, nil
}

type fakeSourceCatalogRepository struct {
	entries []domain.SourceCatalogEntry
}

func (r *fakeSourceCatalogRepository) List(_ context.Context, options domain.SourceCatalogListOptions) (domain.SourceCatalogListResult, error) {
	return domain.SourceCatalogListResult{
		Entries: append([]domain.SourceCatalogEntry(nil), r.entries...),
		Total:   int64(len(r.entries)),
		Limit:   options.Limit,
		Offset:  options.Offset,
	}, nil
}

func (r *fakeSourceCatalogRepository) GetByIDs(_ context.Context, ids []int64) ([]domain.SourceCatalogEntry, error) {
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	entries := make([]domain.SourceCatalogEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		if _, ok := seen[entry.ID]; ok {
			entries = append(entries, entry)
		}
	}
	return entries, nil
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
