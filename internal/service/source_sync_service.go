package service

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/observability"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const sourceSyncLastFetchErrorMaxLength = 2000

type SourceFetchJobStore interface {
	CreateJob(ctx context.Context, job domain.SourceFetchJob) (domain.SourceFetchJob, error)
	UpdateJob(ctx context.Context, job domain.SourceFetchJob) (domain.SourceFetchJob, error)
	CreateAttempt(ctx context.Context, attempt domain.SourceFetchAttempt) (domain.SourceFetchAttempt, error)
}

type DueSourceRepository interface {
	SourceRepository
	ListDueForFetch(ctx context.Context, options domain.SourceDueFetchOptions) ([]domain.Source, error)
}

type ItemEventStore interface {
	Create(ctx context.Context, event domain.ItemEvent) (domain.ItemEvent, error)
}

type SourceSyncService struct {
	sourceRepository   DueSourceRepository
	itemRepository     ItemRepository
	feedFetcher        FeedFetcher
	fetchJobRepository SourceFetchJobStore
	itemEventStore     ItemEventStore
	now                func() time.Time
}

type SourceSyncServiceOption func(*SourceSyncService)

func WithSourceSyncNow(now func() time.Time) SourceSyncServiceOption {
	return func(service *SourceSyncService) {
		if now != nil {
			service.now = now
		}
	}
}

func NewSourceSyncService(
	sourceRepository DueSourceRepository,
	itemRepository ItemRepository,
	feedFetcher FeedFetcher,
	fetchJobRepository SourceFetchJobStore,
	itemEventStore ItemEventStore,
	options ...SourceSyncServiceOption,
) *SourceSyncService {
	service := &SourceSyncService{
		sourceRepository:   sourceRepository,
		itemRepository:     itemRepository,
		feedFetcher:        feedFetcher,
		fetchJobRepository: fetchJobRepository,
		itemEventStore:     itemEventStore,
		now:                time.Now,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

type ExecuteSourceFetchJobInput struct {
	Job domain.SourceFetchJob
}

type EnqueueDueSourcesInput struct {
	Now         time.Time
	Limit       int
	MaxAttempts int
}

type EnqueueDueSourcesResult struct {
	RequestedCount int
	CreatedCount   int
	Jobs           []domain.SourceFetchJob
}

type ExecuteSourceFetchJobResult struct {
	Job          domain.SourceFetchJob
	Attempt      domain.SourceFetchAttempt
	Source       domain.Source
	ItemCount    int
	CreatedCount int
	UpdatedCount int
	EventCount   int
}

func (s *SourceSyncService) EnqueueDueSources(ctx context.Context, input EnqueueDueSourcesInput) (EnqueueDueSourcesResult, error) {
	ctx, span := observability.StartSpan(ctx, "service.source_sync.enqueue_due_sources",
		attribute.Int("limit", input.Limit),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.sourceRepository == nil || s.fetchJobRepository == nil {
		opErr = fmt.Errorf("source sync service is not configured")
		return EnqueueDueSourcesResult{}, opErr
	}
	now := input.Now
	if now.IsZero() {
		now = s.now().UTC()
	} else {
		now = now.UTC()
	}
	maxAttempts := input.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	sources, err := s.sourceRepository.ListDueForFetch(ctx, domain.SourceDueFetchOptions{
		Now:   now,
		Limit: input.Limit,
	})
	if err != nil {
		opErr = err
		return EnqueueDueSourcesResult{}, opErr
	}

	result := EnqueueDueSourcesResult{
		RequestedCount: len(sources),
		Jobs:           make([]domain.SourceFetchJob, 0, len(sources)),
	}
	for _, source := range sources {
		job, err := s.fetchJobRepository.CreateJob(ctx, domain.SourceFetchJob{
			UserID:      source.UserID,
			SourceID:    source.ID,
			Status:      domain.SourceFetchJobStatusQueued,
			Trigger:     domain.SourceFetchTriggerScheduled,
			ScheduledAt: now,
			MaxAttempts: maxAttempts,
			Priority:    source.FetchPriority,
		})
		if err != nil {
			opErr = err
			return EnqueueDueSourcesResult{}, opErr
		}
		result.CreatedCount++
		result.Jobs = append(result.Jobs, job)
	}
	span.SetAttributes(
		attribute.Int("source.due_count", result.RequestedCount),
		attribute.Int("source_fetch_job.created", result.CreatedCount),
	)
	return result, nil
}

func (s *SourceSyncService) ExecuteFetchJob(ctx context.Context, input ExecuteSourceFetchJobInput) (ExecuteSourceFetchJobResult, error) {
	ctx, span := observability.StartSpan(ctx, "service.source_sync.execute_fetch_job",
		attribute.Int64("user.id", input.Job.UserID),
		attribute.Int64("source.id", input.Job.SourceID),
		attribute.Int64("source_fetch_job.id", input.Job.ID),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.sourceRepository == nil || s.itemRepository == nil || s.feedFetcher == nil || s.fetchJobRepository == nil || s.itemEventStore == nil {
		opErr = fmt.Errorf("source sync service is not configured")
		return ExecuteSourceFetchJobResult{}, opErr
	}
	if input.Job.ID < 1 {
		opErr = fmt.Errorf("%w: source fetch job id must be positive", domain.ErrInvalidInput)
		return ExecuteSourceFetchJobResult{}, opErr
	}
	if input.Job.UserID < 1 {
		opErr = fmt.Errorf("%w: user id must be positive", domain.ErrInvalidInput)
		return ExecuteSourceFetchJobResult{}, opErr
	}
	if input.Job.SourceID < 1 {
		opErr = fmt.Errorf("%w: source id must be positive", domain.ErrInvalidInput)
		return ExecuteSourceFetchJobResult{}, opErr
	}

	source, err := s.sourceRepository.GetByID(ctx, input.Job.UserID, input.Job.SourceID)
	if err != nil {
		opErr = err
		return ExecuteSourceFetchJobResult{}, opErr
	}

	startedAt := s.now().UTC()
	fetchResult, err := s.feedFetcher.Fetch(ctx, source)
	durationMS := int(s.now().UTC().Sub(startedAt).Milliseconds())
	if err != nil {
		result, recordErr := s.recordFetchJobFailure(ctx, input.Job, source, startedAt, durationMS, err)
		if recordErr != nil {
			opErr = recordErr
			return ExecuteSourceFetchJobResult{}, opErr
		}
		opErr = err
		return result, opErr
	}

	upsertResult, err := s.itemRepository.UpsertMany(ctx, fetchResult.Items)
	if err != nil {
		result, recordErr := s.recordFetchJobFailure(ctx, input.Job, source, startedAt, durationMS, err)
		if recordErr != nil {
			opErr = recordErr
			return ExecuteSourceFetchJobResult{}, opErr
		}
		opErr = err
		return result, opErr
	}

	itemCount := len(fetchResult.Items)
	updatedSource, err := s.recordSourceFetchSuccess(ctx, source, durationMS, itemCount)
	if err != nil {
		opErr = err
		return ExecuteSourceFetchJobResult{}, opErr
	}

	eventCount, err := s.createItemCreatedEvents(ctx, source, upsertResult.CreatedItems)
	if err != nil {
		opErr = err
		return ExecuteSourceFetchJobResult{}, opErr
	}

	attempt, err := s.fetchJobRepository.CreateAttempt(ctx, domain.SourceFetchAttempt{
		JobID:         input.Job.ID,
		SourceID:      source.ID,
		AttemptNumber: sourceFetchJobAttemptNumber(input.Job),
		Status:        domain.SourceFetchAttemptStatusSucceeded,
		StartedAt:     startedAt,
		FinishedAt:    timePtr(s.now().UTC()),
		DurationMS:    &durationMS,
		ItemCount:     itemCount,
		CreatedCount:  upsertResult.CreatedCount,
		UpdatedCount:  upsertResult.UpdatedCount,
	})
	if err != nil {
		opErr = err
		return ExecuteSourceFetchJobResult{}, opErr
	}

	job := input.Job
	job.Status = domain.SourceFetchJobStatusSucceeded
	job.FinishedAt = timePtr(s.now().UTC())
	job.LastError = ""
	updatedJob, err := s.fetchJobRepository.UpdateJob(ctx, job)
	if err != nil {
		opErr = err
		return ExecuteSourceFetchJobResult{}, opErr
	}

	span.SetAttributes(
		attribute.Int("feed.items", itemCount),
		attribute.Int("feed.items_created", upsertResult.CreatedCount),
		attribute.Int("feed.items_updated", upsertResult.UpdatedCount),
		attribute.Int("item_events.created", eventCount),
	)
	return ExecuteSourceFetchJobResult{
		Job:          updatedJob,
		Attempt:      attempt,
		Source:       updatedSource,
		ItemCount:    itemCount,
		CreatedCount: upsertResult.CreatedCount,
		UpdatedCount: upsertResult.UpdatedCount,
		EventCount:   eventCount,
	}, nil
}

func (s *SourceSyncService) recordFetchJobFailure(
	ctx context.Context,
	job domain.SourceFetchJob,
	source domain.Source,
	startedAt time.Time,
	durationMS int,
	err error,
) (ExecuteSourceFetchJobResult, error) {
	message := truncateError(err.Error(), sourceSyncLastFetchErrorMaxLength)
	updatedSource := source
	fetchedAt := s.now().UTC()
	itemCount := 0
	updatedSource.LastFetchedAt = &fetchedAt
	updatedSource.LastFetchStatus = domain.SourceLastFetchStatusFailed
	updatedSource.LastFetchError = message
	updatedSource.LastFetchDurationMS = &durationMS
	updatedSource.LastFetchItemCount = &itemCount
	updatedSource, updateErr := s.sourceRepository.UpdateFetchResult(ctx, updatedSource)
	if updateErr != nil {
		return ExecuteSourceFetchJobResult{}, updateErr
	}

	attempt, attemptErr := s.fetchJobRepository.CreateAttempt(ctx, domain.SourceFetchAttempt{
		JobID:         job.ID,
		SourceID:      source.ID,
		AttemptNumber: sourceFetchJobAttemptNumber(job),
		Status:        domain.SourceFetchAttemptStatusFailed,
		StartedAt:     startedAt,
		FinishedAt:    timePtr(s.now().UTC()),
		DurationMS:    &durationMS,
		ErrorMessage:  message,
	})
	if attemptErr != nil {
		return ExecuteSourceFetchJobResult{}, attemptErr
	}

	job.Status = domain.SourceFetchJobStatusFailed
	job.FinishedAt = timePtr(s.now().UTC())
	job.LastError = message
	updatedJob, jobErr := s.fetchJobRepository.UpdateJob(ctx, job)
	if jobErr != nil {
		return ExecuteSourceFetchJobResult{}, jobErr
	}

	return ExecuteSourceFetchJobResult{
		Job:     updatedJob,
		Attempt: attempt,
		Source:  updatedSource,
	}, nil
}

func (s *SourceSyncService) recordSourceFetchSuccess(ctx context.Context, source domain.Source, durationMS int, itemCount int) (domain.Source, error) {
	fetchedAt := s.now().UTC()
	source.LastFetchedAt = &fetchedAt
	source.LastFetchStatus = domain.SourceLastFetchStatusSuccess
	source.LastFetchError = ""
	source.LastFetchDurationMS = &durationMS
	source.LastFetchItemCount = &itemCount
	return s.sourceRepository.UpdateFetchResult(ctx, source)
}

func (s *SourceSyncService) createItemCreatedEvents(ctx context.Context, source domain.Source, items []domain.Item) (int, error) {
	count := 0
	for _, item := range items {
		if item.ID < 1 {
			continue
		}
		_, err := s.itemEventStore.Create(ctx, domain.ItemEvent{
			UserID:      source.UserID,
			SourceID:    source.ID,
			ItemID:      item.ID,
			EventType:   domain.ItemEventTypeItemCreated,
			Status:      domain.ItemEventStatusPending,
			Payload:     itemCreatedEventPayload(item),
			DedupeKey:   itemCreatedEventDedupeKey(item.ID),
			AvailableAt: s.now().UTC(),
		})
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func itemCreatedEventPayload(item domain.Item) domain.ItemEventPayload {
	return domain.ItemEventPayload{
		"title":          item.Title,
		"url":            item.URL,
		"normalized_url": item.NormalizedURL,
	}
}

func itemCreatedEventDedupeKey(itemID int64) string {
	return "item.created:" + strconv.FormatInt(itemID, 10)
}

func sourceFetchJobAttemptNumber(job domain.SourceFetchJob) int {
	if job.AttemptCount > 0 {
		return job.AttemptCount
	}
	return 1
}

func timePtr(value time.Time) *time.Time {
	return &value
}
