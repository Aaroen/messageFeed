package service

import (
	"context"
	"errors"
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestItemEventWorkerServiceRunOnceProcessesClaimedEvents(t *testing.T) {
	now := time.Date(2026, 6, 23, 22, 0, 0, 0, time.UTC)
	queueStore := &fakeItemEventWorkerQueueStore{
		claimEvents: []domain.ItemEvent{
			{
				ID:        30,
				UserID:    1,
				SourceID:  40,
				ItemID:    50,
				EventType: domain.ItemEventTypeItemCreated,
			},
		},
	}
	processor := &fakeItemEventProcessor{
		result: ProcessItemEventResult{CandidateCount: 2},
	}
	service := NewItemEventWorkerService(
		queueStore,
		processor,
		WithItemEventWorkerNow(func() time.Time { return now }),
	)

	result, err := service.RunOnce(context.Background(), RunItemEventWorkerOnceInput{
		Now:   now,
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if queueStore.claimInput.Limit != 5 {
		t.Fatalf("claim limit = %d, want 5", queueStore.claimInput.Limit)
	}
	if !queueStore.claimInput.Now.Equal(now) {
		t.Fatalf("claim now = %s, want %s", queueStore.claimInput.Now, now)
	}
	if result.ClaimedCount != 1 || result.ProcessedCount != 1 || result.CandidateCount != 2 {
		t.Fatalf("result = %#v, want one processed event and two candidates", result)
	}
	if got, want := len(queueStore.processedIDs), 1; got != want {
		t.Fatalf("processed IDs length = %d, want %d", got, want)
	}
	if queueStore.processedIDs[0] != 30 {
		t.Fatalf("processed ID = %d, want 30", queueStore.processedIDs[0])
	}
	if got, want := len(processor.events), 1; got != want {
		t.Fatalf("processor events length = %d, want %d", got, want)
	}
}

func TestItemEventWorkerServiceRunOnceMarksFailedEvent(t *testing.T) {
	now := time.Date(2026, 6, 23, 22, 30, 0, 0, time.UTC)
	queueStore := &fakeItemEventWorkerQueueStore{
		claimEvents: []domain.ItemEvent{
			{
				ID:        31,
				UserID:    2,
				SourceID:  41,
				ItemID:    51,
				EventType: domain.ItemEventTypeItemCreated,
			},
		},
	}
	processor := &fakeItemEventProcessor{err: errors.New("rule store unavailable")}
	service := NewItemEventWorkerService(
		queueStore,
		processor,
		WithItemEventWorkerNow(func() time.Time { return now }),
	)

	result, err := service.RunOnce(context.Background(), RunItemEventWorkerOnceInput{
		Now:   now,
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if result.ClaimedCount != 1 || result.FailureCount != 1 || result.ProcessedCount != 0 {
		t.Fatalf("result = %#v, want one failed event", result)
	}
	if got, want := len(queueStore.failed), 1; got != want {
		t.Fatalf("failed length = %d, want %d", got, want)
	}
	if queueStore.failed[0].eventID != 31 || queueStore.failed[0].userID != 2 {
		t.Fatalf("failed record = %#v, want event 31 user 2", queueStore.failed[0])
	}
	if queueStore.failed[0].message != "rule store unavailable" {
		t.Fatalf("failed message = %q, want rule store unavailable", queueStore.failed[0].message)
	}
	if result.Errors[0].Message != "rule store unavailable" {
		t.Fatalf("result error message = %q, want rule store unavailable", result.Errors[0].Message)
	}
}

func TestItemEventWorkerServiceRunOnceUsesTaskLock(t *testing.T) {
	now := time.Date(2026, 6, 23, 23, 0, 0, 0, time.UTC)
	locker := &fakeItemEventWorkerTaskLocker{}
	service := NewItemEventWorkerService(
		&fakeItemEventWorkerQueueStore{},
		&fakeItemEventProcessor{},
		WithItemEventWorkerNow(func() time.Time { return now }),
		WithItemEventWorkerTaskLocker(locker),
	)

	_, err := service.RunOnce(context.Background(), RunItemEventWorkerOnceInput{
		Now:      now,
		Limit:    1,
		LockName: "item-event-worker-test",
	})
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if !locker.called {
		t.Fatal("task locker was not called")
	}
	if locker.name != "item-event-worker-test" {
		t.Fatalf("lock name = %q, want item-event-worker-test", locker.name)
	}
}

type fakeItemEventWorkerQueueStore struct {
	claimEvents  []domain.ItemEvent
	claimInput   domain.ItemEventClaimInput
	processedIDs []int64
	failed       []fakeItemEventWorkerFailure
}

type fakeItemEventWorkerFailure struct {
	userID  int64
	eventID int64
	message string
}

func (s *fakeItemEventWorkerQueueStore) ClaimPending(_ context.Context, input domain.ItemEventClaimInput) ([]domain.ItemEvent, error) {
	s.claimInput = input
	return append([]domain.ItemEvent(nil), s.claimEvents...), nil
}

func (s *fakeItemEventWorkerQueueStore) MarkProcessed(_ context.Context, _ int64, id int64, _ time.Time) (domain.ItemEvent, error) {
	s.processedIDs = append(s.processedIDs, id)
	return domain.ItemEvent{ID: id, Status: domain.ItemEventStatusProcessed}, nil
}

func (s *fakeItemEventWorkerQueueStore) MarkFailed(_ context.Context, userID int64, id int64, message string, _ time.Time) (domain.ItemEvent, error) {
	s.failed = append(s.failed, fakeItemEventWorkerFailure{
		userID:  userID,
		eventID: id,
		message: message,
	})
	return domain.ItemEvent{ID: id, UserID: userID, Status: domain.ItemEventStatusFailed, LastError: message}, nil
}

type fakeItemEventProcessor struct {
	result ProcessItemEventResult
	err    error
	events []domain.ItemEvent
}

func (p *fakeItemEventProcessor) ProcessItemEvent(_ context.Context, input ProcessItemEventInput) (ProcessItemEventResult, error) {
	p.events = append(p.events, input.Event)
	if p.err != nil {
		return ProcessItemEventResult{}, p.err
	}
	return p.result, nil
}

type fakeItemEventWorkerTaskLocker struct {
	called bool
	name   string
}

func (l *fakeItemEventWorkerTaskLocker) WithLock(ctx context.Context, name string, _ time.Duration, run func(context.Context) error) error {
	l.called = true
	l.name = name
	return run(ctx)
}
