package repository

import (
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestSourceFetchJobModelRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 23, 9, 0, 0, 0, time.UTC)
	finishedAt := now.Add(3 * time.Second)
	job := domain.SourceFetchJob{
		ID:           9,
		UserID:       1,
		SourceID:     2,
		Status:       domain.SourceFetchJobStatusSucceeded,
		Trigger:      domain.SourceFetchTriggerScheduled,
		ScheduledAt:  now,
		StartedAt:    &now,
		FinishedAt:   &finishedAt,
		AttemptCount: 1,
		MaxAttempts:  3,
		Priority:     5,
		LockedBy:     "worker-a",
		LockedAt:     &now,
		LastError:    "",
		CreatedAt:    now,
		UpdatedAt:    finishedAt,
	}

	model := sourceFetchJobModelFromDomain(job)
	converted := sourceFetchJobModelToDomain(model)

	if converted.ID != job.ID {
		t.Fatalf("ID = %d, want %d", converted.ID, job.ID)
	}
	if converted.Status != job.Status {
		t.Fatalf("Status = %q, want %q", converted.Status, job.Status)
	}
	if converted.Trigger != job.Trigger {
		t.Fatalf("Trigger = %q, want %q", converted.Trigger, job.Trigger)
	}
	if converted.Priority != job.Priority {
		t.Fatalf("Priority = %d, want %d", converted.Priority, job.Priority)
	}
	if converted.FinishedAt == nil || !converted.FinishedAt.Equal(finishedAt) {
		t.Fatalf("FinishedAt = %#v, want %s", converted.FinishedAt, finishedAt)
	}
}

func TestSourceFetchAttemptModelRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	finishedAt := now.Add(1200 * time.Millisecond)
	durationMS := 1200
	httpStatus := 200
	attempt := domain.SourceFetchAttempt{
		ID:            4,
		JobID:         9,
		SourceID:      2,
		AttemptNumber: 1,
		Status:        domain.SourceFetchAttemptStatusSucceeded,
		StartedAt:     now,
		FinishedAt:    &finishedAt,
		DurationMS:    &durationMS,
		HTTPStatus:    &httpStatus,
		ItemCount:     3,
		CreatedCount:  2,
		UpdatedCount:  1,
		CreatedAt:     now,
		UpdatedAt:     finishedAt,
	}

	model := sourceFetchAttemptModelFromDomain(attempt)
	converted := sourceFetchAttemptModelToDomain(model)

	if converted.JobID != attempt.JobID {
		t.Fatalf("JobID = %d, want %d", converted.JobID, attempt.JobID)
	}
	if converted.Status != attempt.Status {
		t.Fatalf("Status = %q, want %q", converted.Status, attempt.Status)
	}
	if converted.DurationMS == nil || *converted.DurationMS != durationMS {
		t.Fatalf("DurationMS = %#v, want %d", converted.DurationMS, durationMS)
	}
	if converted.HTTPStatus == nil || *converted.HTTPStatus != httpStatus {
		t.Fatalf("HTTPStatus = %#v, want %d", converted.HTTPStatus, httpStatus)
	}
	if converted.CreatedCount != 2 || converted.UpdatedCount != 1 {
		t.Fatalf("counts = created:%d updated:%d", converted.CreatedCount, converted.UpdatedCount)
	}
}

func TestNormalizeSourceFetchJobClaimInput(t *testing.T) {
	now := time.Date(2026, 6, 23, 11, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	input := normalizeSourceFetchJobClaimInput(domain.SourceFetchJobClaimInput{
		Now:      now,
		WorkerID: " worker-a ",
		Limit:    maxSourceFetchJobClaimLimit + 1,
	})

	if input.WorkerID != "worker-a" {
		t.Fatalf("WorkerID = %q, want worker-a", input.WorkerID)
	}
	if input.Limit != maxSourceFetchJobClaimLimit {
		t.Fatalf("Limit = %d, want %d", input.Limit, maxSourceFetchJobClaimLimit)
	}
	if input.Now.Location() != time.UTC {
		t.Fatalf("Now location = %s, want UTC", input.Now.Location())
	}

	defaulted := normalizeSourceFetchJobClaimInput(domain.SourceFetchJobClaimInput{})
	if defaulted.WorkerID != "unknown" {
		t.Fatalf("default WorkerID = %q, want unknown", defaulted.WorkerID)
	}
	if defaulted.Limit != defaultSourceFetchJobClaimLimit {
		t.Fatalf("default Limit = %d, want %d", defaulted.Limit, defaultSourceFetchJobClaimLimit)
	}
}
