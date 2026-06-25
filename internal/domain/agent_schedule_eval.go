package domain

import "time"

type AgentScheduledTaskStatus string

const (
	AgentScheduledTaskStatusQueued        AgentScheduledTaskStatus = "queued"
	AgentScheduledTaskStatusRunning       AgentScheduledTaskStatus = "running"
	AgentScheduledTaskStatusInputRequired AgentScheduledTaskStatus = "input_required"
	AgentScheduledTaskStatusSucceeded     AgentScheduledTaskStatus = "succeeded"
	AgentScheduledTaskStatusFailed        AgentScheduledTaskStatus = "failed"
	AgentScheduledTaskStatusCanceled      AgentScheduledTaskStatus = "canceled"
	AgentScheduledTaskStatusExpired       AgentScheduledTaskStatus = "expired"
)

func (s AgentScheduledTaskStatus) Valid() bool {
	switch s {
	case AgentScheduledTaskStatusQueued,
		AgentScheduledTaskStatusRunning,
		AgentScheduledTaskStatusInputRequired,
		AgentScheduledTaskStatusSucceeded,
		AgentScheduledTaskStatusFailed,
		AgentScheduledTaskStatusCanceled,
		AgentScheduledTaskStatusExpired:
		return true
	default:
		return false
	}
}

type AgentScheduledTask struct {
	ID                   int64
	UserID               int64
	SessionID            int64
	TurnID               int64
	PlanID               int64
	SourceRunID          int64
	Status               AgentScheduledTaskStatus
	TaskType             string
	Goal                 string
	TargetChannel        string
	TargetRef            string
	ExecutionWindowStart *time.Time
	ExecutionWindowEnd   *time.Time
	ScheduledAt          time.Time
	DeliverAt            *time.Time
	FreshnessPolicy      string
	AllowedCapabilities  []string
	ModelPolicy          AgentJSON
	FailurePolicy        AgentJSON
	Payload              AgentJSON
	AttemptCount         int
	MaxAttempts          int
	LockedBy             string
	LockedAt             *time.Time
	LastError            string
	NextRunAt            *time.Time
	CompletedAt          *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type AgentScheduledTaskClaimInput struct {
	Now      time.Time
	WorkerID string
	Limit    int
}

type AgentScheduledTaskListOptions struct {
	UserID int64
	Status AgentScheduledTaskStatus
	Limit  int
}

type AgentEvalCaseListOptions struct {
	EnabledOnly bool
	Category    string
	Limit       int
}

type AgentEvalRunListOptions struct {
	UserID int64
	Limit  int
}

type AgentEvalRunStatus string

const (
	AgentEvalRunStatusQueued    AgentEvalRunStatus = "queued"
	AgentEvalRunStatusRunning   AgentEvalRunStatus = "running"
	AgentEvalRunStatusCompleted AgentEvalRunStatus = "completed"
	AgentEvalRunStatusFailed    AgentEvalRunStatus = "failed"
	AgentEvalRunStatusCanceled  AgentEvalRunStatus = "canceled"
)

func (s AgentEvalRunStatus) Valid() bool {
	switch s {
	case AgentEvalRunStatusQueued,
		AgentEvalRunStatusRunning,
		AgentEvalRunStatusCompleted,
		AgentEvalRunStatusFailed,
		AgentEvalRunStatusCanceled:
		return true
	default:
		return false
	}
}

type AgentEvalResultStatus string

const (
	AgentEvalResultStatusPassed  AgentEvalResultStatus = "passed"
	AgentEvalResultStatusFailed  AgentEvalResultStatus = "failed"
	AgentEvalResultStatusSkipped AgentEvalResultStatus = "skipped"
	AgentEvalResultStatusError   AgentEvalResultStatus = "error"
)

func (s AgentEvalResultStatus) Valid() bool {
	switch s {
	case AgentEvalResultStatusPassed,
		AgentEvalResultStatusFailed,
		AgentEvalResultStatusSkipped,
		AgentEvalResultStatusError:
		return true
	default:
		return false
	}
}

type AgentEvalCase struct {
	ID               int64
	CaseKey          string
	Name             string
	Category         string
	Description      string
	Input            AgentJSON
	ExpectedBehavior string
	SafetyTags       []string
	Enabled          bool
	Metadata         AgentJSON
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type AgentEvalRun struct {
	ID           int64
	UserID       int64
	Trigger      string
	Status       AgentEvalRunStatus
	ModelKey     string
	CaseCount    int
	PassedCount  int
	FailedCount  int
	Metrics      AgentJSON
	StartedAt    *time.Time
	CompletedAt  *time.Time
	ErrorMessage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Results      []AgentEvalResult
}

type AgentEvalResult struct {
	ID            int64
	RunID         int64
	CaseID        int64
	Status        AgentEvalResultStatus
	Score         float64
	Input         AgentJSON
	Expected      string
	Actual        string
	FailureReason string
	Metrics       AgentJSON
	EvidenceRefs  []string
	CreatedAt     time.Time
}
