package service

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/observability"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const itemEventLastErrorMaxLength = 2000

type ItemEventQueueStore interface {
	ClaimPending(ctx context.Context, input domain.ItemEventClaimInput) ([]domain.ItemEvent, error)
	MarkProcessed(ctx context.Context, userID int64, id int64, now time.Time) (domain.ItemEvent, error)
	MarkFailed(ctx context.Context, userID int64, id int64, message string, now time.Time) (domain.ItemEvent, error)
}

type ItemEventProcessor interface {
	ProcessItemEvent(ctx context.Context, input ProcessItemEventInput) (ProcessItemEventResult, error)
}

type ItemEventWorkerService struct {
	eventStore ItemEventQueueStore
	processor  ItemEventProcessor
	taskLocker ItemEventWorkerTaskLocker
	now        func() time.Time
}

type ItemEventWorkerTaskLocker interface {
	WithLock(ctx context.Context, name string, ttl time.Duration, run func(context.Context) error) error
}

type ItemEventWorkerServiceOption func(*ItemEventWorkerService)

func WithItemEventWorkerNow(now func() time.Time) ItemEventWorkerServiceOption {
	return func(service *ItemEventWorkerService) {
		if now != nil {
			service.now = now
		}
	}
}

func WithItemEventWorkerTaskLocker(locker ItemEventWorkerTaskLocker) ItemEventWorkerServiceOption {
	return func(service *ItemEventWorkerService) {
		service.taskLocker = locker
	}
}

func NewItemEventWorkerService(
	eventStore ItemEventQueueStore,
	processor ItemEventProcessor,
	options ...ItemEventWorkerServiceOption,
) *ItemEventWorkerService {
	service := &ItemEventWorkerService{
		eventStore: eventStore,
		processor:  processor,
		now:        time.Now,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

type RunItemEventWorkerOnceInput struct {
	Now      time.Time
	Limit    int
	LockName string
	LockTTL  time.Duration
}

type RunItemEventWorkerOnceResult struct {
	ClaimedCount   int
	ProcessedCount int
	FailureCount   int
	CandidateCount int
	Errors         []ItemEventProcessingError
}

type ItemEventProcessingError struct {
	EventID int64
	UserID  int64
	Message string
}

func (s *ItemEventWorkerService) RunOnce(ctx context.Context, input RunItemEventWorkerOnceInput) (RunItemEventWorkerOnceResult, error) {
	if input.LockName == "" {
		input.LockName = "item-event-prefilter"
	}
	if input.LockTTL <= 0 {
		input.LockTTL = time.Minute
	}

	var result RunItemEventWorkerOnceResult
	run := func(runCtx context.Context) error {
		var err error
		result, err = s.runOnceUnlocked(runCtx, input)
		return err
	}
	if s != nil && s.taskLocker != nil {
		if err := s.taskLocker.WithLock(ctx, input.LockName, input.LockTTL, run); err != nil {
			return RunItemEventWorkerOnceResult{}, err
		}
		return result, nil
	}
	if err := run(ctx); err != nil {
		return RunItemEventWorkerOnceResult{}, err
	}
	return result, nil
}

func (s *ItemEventWorkerService) runOnceUnlocked(ctx context.Context, input RunItemEventWorkerOnceInput) (RunItemEventWorkerOnceResult, error) {
	ctx, span := observability.StartSpan(ctx, "service.item_event_worker.run_once",
		attribute.Int("claim.limit", input.Limit),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.eventStore == nil || s.processor == nil {
		opErr = fmt.Errorf("item event worker service is not configured")
		return RunItemEventWorkerOnceResult{}, opErr
	}
	now := input.Now
	if now.IsZero() {
		now = s.now().UTC()
	} else {
		now = now.UTC()
	}

	events, err := s.eventStore.ClaimPending(ctx, domain.ItemEventClaimInput{
		Now:   now,
		Limit: input.Limit,
	})
	if err != nil {
		opErr = err
		return RunItemEventWorkerOnceResult{}, opErr
	}

	result := RunItemEventWorkerOnceResult{
		ClaimedCount: len(events),
		Errors:       make([]ItemEventProcessingError, 0),
	}
	for _, event := range events {
		processing, err := s.processor.ProcessItemEvent(ctx, ProcessItemEventInput{Event: event})
		if err != nil {
			message := truncateError(err.Error(), itemEventLastErrorMaxLength)
			if _, markErr := s.eventStore.MarkFailed(ctx, event.UserID, event.ID, message, now); markErr != nil {
				opErr = markErr
				return result, opErr
			}
			result.FailureCount++
			result.Errors = append(result.Errors, ItemEventProcessingError{
				EventID: event.ID,
				UserID:  event.UserID,
				Message: message,
			})
			continue
		}
		if _, err := s.eventStore.MarkProcessed(ctx, event.UserID, event.ID, now); err != nil {
			opErr = err
			return result, opErr
		}
		result.ProcessedCount++
		result.CandidateCount += processing.CandidateCount
	}
	span.SetAttributes(
		attribute.Int("item_event.claimed", result.ClaimedCount),
		attribute.Int("item_event.processed", result.ProcessedCount),
		attribute.Int("item_event.failed", result.FailureCount),
		attribute.Int("alert_candidate.created", result.CandidateCount),
	)
	return result, nil
}
