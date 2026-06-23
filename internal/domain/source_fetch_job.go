package domain

import "time"

type SourceFetchJobStatus string

const (
	SourceFetchJobStatusQueued    SourceFetchJobStatus = "queued"
	SourceFetchJobStatusRunning   SourceFetchJobStatus = "running"
	SourceFetchJobStatusSucceeded SourceFetchJobStatus = "succeeded"
	SourceFetchJobStatusFailed    SourceFetchJobStatus = "failed"
	SourceFetchJobStatusCanceled  SourceFetchJobStatus = "canceled"
)

func (s SourceFetchJobStatus) Valid() bool {
	switch s {
	case SourceFetchJobStatusQueued, SourceFetchJobStatusRunning, SourceFetchJobStatusSucceeded, SourceFetchJobStatusFailed, SourceFetchJobStatusCanceled:
		return true
	default:
		return false
	}
}

type SourceFetchTrigger string

const (
	SourceFetchTriggerScheduled SourceFetchTrigger = "scheduled"
	SourceFetchTriggerManual    SourceFetchTrigger = "manual"
	SourceFetchTriggerRetry     SourceFetchTrigger = "retry"
)

func (t SourceFetchTrigger) Valid() bool {
	switch t {
	case SourceFetchTriggerScheduled, SourceFetchTriggerManual, SourceFetchTriggerRetry:
		return true
	default:
		return false
	}
}

type SourceFetchAttemptStatus string

const (
	SourceFetchAttemptStatusRunning   SourceFetchAttemptStatus = "running"
	SourceFetchAttemptStatusSucceeded SourceFetchAttemptStatus = "succeeded"
	SourceFetchAttemptStatusFailed    SourceFetchAttemptStatus = "failed"
)

func (s SourceFetchAttemptStatus) Valid() bool {
	switch s {
	case SourceFetchAttemptStatusRunning, SourceFetchAttemptStatusSucceeded, SourceFetchAttemptStatusFailed:
		return true
	default:
		return false
	}
}

type SourceFetchJob struct {
	ID           int64
	UserID       int64
	SourceID     int64
	Status       SourceFetchJobStatus
	Trigger      SourceFetchTrigger
	ScheduledAt  time.Time
	StartedAt    *time.Time
	FinishedAt   *time.Time
	AttemptCount int
	MaxAttempts  int
	Priority     int
	LockedBy     string
	LockedAt     *time.Time
	LastError    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type SourceFetchAttempt struct {
	ID            int64
	JobID         int64
	SourceID      int64
	AttemptNumber int
	Status        SourceFetchAttemptStatus
	StartedAt     time.Time
	FinishedAt    *time.Time
	DurationMS    *int
	HTTPStatus    *int
	ErrorMessage  string
	ItemCount     int
	CreatedCount  int
	UpdatedCount  int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type SourceFetchJobClaimInput struct {
	Now      time.Time
	WorkerID string
	Limit    int
}

type SourceFetchJobListOptions struct {
	UserID int64
	Limit  int
	Offset int
}

type SourceFetchJobListByIDsOptions struct {
	UserID int64
	IDs    []int64
}

type SourceFetchJobListResult struct {
	Jobs   []SourceFetchJob
	Total  int64
	Limit  int
	Offset int
}

type SourceFetchAttemptListOptions struct {
	UserID int64
	JobID  int64
	Limit  int
	Offset int
}

type SourceFetchAttemptListResult struct {
	Attempts []SourceFetchAttempt
	Total    int64
	Limit    int
	Offset   int
}
