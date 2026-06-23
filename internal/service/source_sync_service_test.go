package service

import (
	"context"
	"errors"
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestSourceSyncServiceExecuteFetchJobCreatesItemEvents(t *testing.T) {
	sourceRepository := newFakeSourceRepository()
	source, err := sourceRepository.Create(context.Background(), domain.Source{
		UserID:        1,
		Name:          "Example",
		Type:          domain.SourceTypeRSS,
		URL:           "https://example.com/feed.xml",
		NormalizedURL: "https://example.com/feed.xml",
		Status:        domain.SourceStatusActive,
	})
	if err != nil {
		t.Fatalf("Create source returned error: %v", err)
	}

	itemRepository := &fakeSourceSyncItemRepository{}
	fetchJobStore := &fakeSourceFetchJobStore{}
	itemEventStore := &fakeItemEventStore{}
	service := NewSourceSyncService(
		sourceRepository,
		itemRepository,
		&fakeFeedFetcher{},
		fetchJobStore,
		itemEventStore,
		WithSourceSyncNow(func() time.Time {
			return time.Date(2026, 6, 23, 14, 0, 0, 0, time.UTC)
		}),
	)

	result, err := service.ExecuteFetchJob(context.Background(), ExecuteSourceFetchJobInput{
		Job: domain.SourceFetchJob{
			ID:           7,
			UserID:       1,
			SourceID:     source.ID,
			Status:       domain.SourceFetchJobStatusRunning,
			Trigger:      domain.SourceFetchTriggerScheduled,
			AttemptCount: 1,
			MaxAttempts:  3,
		},
	})
	if err != nil {
		t.Fatalf("ExecuteFetchJob returned error: %v", err)
	}

	if result.Job.Status != domain.SourceFetchJobStatusSucceeded {
		t.Fatalf("Job.Status = %q, want %q", result.Job.Status, domain.SourceFetchJobStatusSucceeded)
	}
	if result.Attempt.Status != domain.SourceFetchAttemptStatusSucceeded {
		t.Fatalf("Attempt.Status = %q, want %q", result.Attempt.Status, domain.SourceFetchAttemptStatusSucceeded)
	}
	if result.EventCount != 1 {
		t.Fatalf("EventCount = %d, want 1", result.EventCount)
	}
	if got, want := len(itemEventStore.events), 1; got != want {
		t.Fatalf("events length = %d, want %d", got, want)
	}
	if itemEventStore.events[0].DedupeKey != "item.created:100" {
		t.Fatalf("DedupeKey = %q, want item.created:100", itemEventStore.events[0].DedupeKey)
	}
	if itemEventStore.events[0].Status != domain.ItemEventStatusPending {
		t.Fatalf("Event status = %q, want %q", itemEventStore.events[0].Status, domain.ItemEventStatusPending)
	}
	if result.Source.LastFetchStatus != domain.SourceLastFetchStatusSuccess {
		t.Fatalf("LastFetchStatus = %q, want success", result.Source.LastFetchStatus)
	}
	if got, want := len(fetchJobStore.attempts), 1; got != want {
		t.Fatalf("attempts length = %d, want %d", got, want)
	}
}

func TestSourceSyncServiceExecuteFetchJobRecordsFailure(t *testing.T) {
	sourceRepository := newFakeSourceRepository()
	source, err := sourceRepository.Create(context.Background(), domain.Source{
		UserID:        1,
		Name:          "Example",
		Type:          domain.SourceTypeRSS,
		URL:           "https://example.com/feed.xml",
		NormalizedURL: "https://example.com/feed.xml",
		Status:        domain.SourceStatusActive,
	})
	if err != nil {
		t.Fatalf("Create source returned error: %v", err)
	}

	fetchJobStore := &fakeSourceFetchJobStore{}
	itemEventStore := &fakeItemEventStore{}
	service := NewSourceSyncService(
		sourceRepository,
		&fakeSourceSyncItemRepository{},
		&fakeFeedFetcher{err: errors.New("network failed")},
		fetchJobStore,
		itemEventStore,
		WithSourceSyncNow(func() time.Time {
			return time.Date(2026, 6, 23, 15, 0, 0, 0, time.UTC)
		}),
	)

	result, err := service.ExecuteFetchJob(context.Background(), ExecuteSourceFetchJobInput{
		Job: domain.SourceFetchJob{
			ID:           8,
			UserID:       1,
			SourceID:     source.ID,
			Status:       domain.SourceFetchJobStatusRunning,
			Trigger:      domain.SourceFetchTriggerScheduled,
			AttemptCount: 1,
			MaxAttempts:  3,
		},
	})
	if err == nil {
		t.Fatal("ExecuteFetchJob returned nil error")
	}

	if result.Job.Status != domain.SourceFetchJobStatusFailed {
		t.Fatalf("Job.Status = %q, want %q", result.Job.Status, domain.SourceFetchJobStatusFailed)
	}
	if result.Attempt.Status != domain.SourceFetchAttemptStatusFailed {
		t.Fatalf("Attempt.Status = %q, want %q", result.Attempt.Status, domain.SourceFetchAttemptStatusFailed)
	}
	if result.Source.LastFetchStatus != domain.SourceLastFetchStatusFailed {
		t.Fatalf("LastFetchStatus = %q, want failed", result.Source.LastFetchStatus)
	}
	if result.Source.LastFetchItemCount == nil || *result.Source.LastFetchItemCount != 0 {
		t.Fatalf("LastFetchItemCount = %#v, want 0", result.Source.LastFetchItemCount)
	}
	if got, want := len(itemEventStore.events), 0; got != want {
		t.Fatalf("events length = %d, want %d", got, want)
	}
}

func TestSourceSyncServiceEnqueueDueSourcesCreatesJobs(t *testing.T) {
	now := time.Date(2026, 6, 23, 16, 0, 0, 0, time.UTC)
	sourceRepository := newFakeSourceRepository()
	dueAt := now.Add(-time.Minute)
	futureAt := now.Add(time.Hour)
	dueSource, err := sourceRepository.Create(context.Background(), domain.Source{
		UserID:        1,
		Name:          "Due",
		Type:          domain.SourceTypeRSS,
		URL:           "https://due.example/feed.xml",
		NormalizedURL: "https://due.example/feed.xml",
		Status:        domain.SourceStatusActive,
		NextFetchAt:   &dueAt,
		FetchPriority: 4,
	})
	if err != nil {
		t.Fatalf("Create due source returned error: %v", err)
	}
	_, err = sourceRepository.Create(context.Background(), domain.Source{
		UserID:        1,
		Name:          "Future",
		Type:          domain.SourceTypeRSS,
		URL:           "https://future.example/feed.xml",
		NormalizedURL: "https://future.example/feed.xml",
		Status:        domain.SourceStatusActive,
		NextFetchAt:   &futureAt,
	})
	if err != nil {
		t.Fatalf("Create future source returned error: %v", err)
	}

	fetchJobStore := &fakeSourceFetchJobStore{}
	service := NewSourceSyncService(
		sourceRepository,
		&fakeSourceSyncItemRepository{},
		&fakeFeedFetcher{},
		fetchJobStore,
		&fakeItemEventStore{},
		WithSourceSyncNow(func() time.Time { return now }),
	)

	result, err := service.EnqueueDueSources(context.Background(), EnqueueDueSourcesInput{
		Now:         now,
		Limit:       10,
		MaxAttempts: 5,
	})
	if err != nil {
		t.Fatalf("EnqueueDueSources returned error: %v", err)
	}

	if result.RequestedCount != 1 {
		t.Fatalf("RequestedCount = %d, want 1", result.RequestedCount)
	}
	if result.CreatedCount != 1 {
		t.Fatalf("CreatedCount = %d, want 1", result.CreatedCount)
	}
	if got, want := len(fetchJobStore.jobs), 1; got != want {
		t.Fatalf("jobs length = %d, want %d", got, want)
	}
	job := fetchJobStore.jobs[0]
	if job.SourceID != dueSource.ID {
		t.Fatalf("SourceID = %d, want %d", job.SourceID, dueSource.ID)
	}
	if job.Status != domain.SourceFetchJobStatusQueued {
		t.Fatalf("Status = %q, want %q", job.Status, domain.SourceFetchJobStatusQueued)
	}
	if job.MaxAttempts != 5 {
		t.Fatalf("MaxAttempts = %d, want 5", job.MaxAttempts)
	}
	if job.Priority != dueSource.FetchPriority {
		t.Fatalf("Priority = %d, want %d", job.Priority, dueSource.FetchPriority)
	}
}

type fakeSourceSyncItemRepository struct {
	nextID int64
	items  []domain.Item
}

func (r *fakeSourceSyncItemRepository) UpsertMany(_ context.Context, items []domain.Item) (domain.ItemUpsertResult, error) {
	if r.nextID == 0 {
		r.nextID = 100
	}
	result := domain.ItemUpsertResult{TotalCount: len(items)}
	for _, item := range items {
		item.ID = r.nextID
		r.nextID++
		r.items = append(r.items, item)
		result.CreatedCount++
		result.CreatedItems = append(result.CreatedItems, item)
	}
	return result, nil
}

type fakeSourceFetchJobStore struct {
	nextID   int64
	jobs     []domain.SourceFetchJob
	attempts []domain.SourceFetchAttempt
}

func (s *fakeSourceFetchJobStore) CreateJob(_ context.Context, job domain.SourceFetchJob) (domain.SourceFetchJob, error) {
	if s.nextID == 0 {
		s.nextID = int64(len(s.jobs) + 1)
	}
	job.ID = s.nextID
	s.nextID++
	s.jobs = append(s.jobs, job)
	return job, nil
}

func (s *fakeSourceFetchJobStore) UpdateJob(_ context.Context, job domain.SourceFetchJob) (domain.SourceFetchJob, error) {
	s.jobs = append(s.jobs, job)
	return job, nil
}

func (s *fakeSourceFetchJobStore) CreateAttempt(_ context.Context, attempt domain.SourceFetchAttempt) (domain.SourceFetchAttempt, error) {
	if attempt.ID == 0 {
		attempt.ID = int64(len(s.attempts) + 1)
	}
	s.attempts = append(s.attempts, attempt)
	return attempt, nil
}

type fakeItemEventStore struct {
	events []domain.ItemEvent
}

func (s *fakeItemEventStore) Create(_ context.Context, event domain.ItemEvent) (domain.ItemEvent, error) {
	if event.ID == 0 {
		event.ID = int64(len(s.events) + 1)
	}
	s.events = append(s.events, event)
	return event, nil
}

func (r *fakeSourceRepository) ListDueForFetch(_ context.Context, options domain.SourceDueFetchOptions) ([]domain.Source, error) {
	if options.Now.IsZero() {
		options.Now = time.Now().UTC()
	}
	var sources []domain.Source
	for _, source := range r.sources {
		if source.Status != domain.SourceStatusActive || source.NextFetchAt == nil {
			continue
		}
		if source.NextFetchAt.After(options.Now) {
			continue
		}
		sources = append(sources, source)
	}
	return sources, nil
}
