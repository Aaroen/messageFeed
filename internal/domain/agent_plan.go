package domain

import "time"

type AgentPlanStatus string

const (
	AgentPlanStatusDraft            AgentPlanStatus = "draft"
	AgentPlanStatusAwaitingApproval AgentPlanStatus = "awaiting_approval"
	AgentPlanStatusApproved         AgentPlanStatus = "approved"
	AgentPlanStatusRejected         AgentPlanStatus = "rejected"
	AgentPlanStatusExpired          AgentPlanStatus = "expired"
	AgentPlanStatusExecuting        AgentPlanStatus = "executing"
	AgentPlanStatusCompleted        AgentPlanStatus = "completed"
	AgentPlanStatusFailed           AgentPlanStatus = "failed"
)

func (s AgentPlanStatus) Valid() bool {
	switch s {
	case AgentPlanStatusDraft,
		AgentPlanStatusAwaitingApproval,
		AgentPlanStatusApproved,
		AgentPlanStatusRejected,
		AgentPlanStatusExpired,
		AgentPlanStatusExecuting,
		AgentPlanStatusCompleted,
		AgentPlanStatusFailed:
		return true
	default:
		return false
	}
}

type AgentPlanStepStatus string

const (
	AgentPlanStepStatusPending   AgentPlanStepStatus = "pending"
	AgentPlanStepStatusApproved  AgentPlanStepStatus = "approved"
	AgentPlanStepStatusExecuting AgentPlanStepStatus = "executing"
	AgentPlanStepStatusCompleted AgentPlanStepStatus = "completed"
	AgentPlanStepStatusFailed    AgentPlanStepStatus = "failed"
	AgentPlanStepStatusSkipped   AgentPlanStepStatus = "skipped"
)

func (s AgentPlanStepStatus) Valid() bool {
	switch s {
	case AgentPlanStepStatusPending,
		AgentPlanStepStatusApproved,
		AgentPlanStepStatusExecuting,
		AgentPlanStepStatusCompleted,
		AgentPlanStepStatusFailed,
		AgentPlanStepStatusSkipped:
		return true
	default:
		return false
	}
}

type AgentPlan struct {
	ID                 int64
	UserID             int64
	SessionID          int64
	TurnID             int64
	ControllerRunID    int64
	Status             AgentPlanStatus
	Goal               string
	Summary            string
	ImpactSummary      string
	RiskLevel          string
	ConfirmationPolicy string
	AllowedScopes      []string
	DedupeKey          string
	PolicyDecision     string
	PolicyReason       string
	ExpiresAt          *time.Time
	ApprovedAt         *time.Time
	RejectedAt         *time.Time
	CompletedAt        *time.Time
	FailedAt           *time.Time
	ErrorMessage       string
	Metadata           AgentJSON
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Steps              []AgentPlanStep
	Approvals          []AgentApproval
}

type AgentPlanStep struct {
	ID              int64
	PlanID          int64
	StepOrder       int
	Status          AgentPlanStepStatus
	CapabilityKey   string
	CapabilityScope []string
	Title           string
	InputSummary    string
	OutputSummary   string
	ExpectedOutput  string
	FailureStrategy string
	ExecutorRunID   int64
	ObservationRef  string
	ArtifactRefs    []string
	ErrorMessage    string
	RetryCount      int
	MaxRetries      int
	LastRetryAt     *time.Time
	RetryReason     string
	RetryMetadata   AgentJSON
	StartedAt       *time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type AgentCapabilityAuditLog struct {
	ID            int64
	UserID        int64
	SessionID     int64
	TurnID        int64
	RunID         int64
	PlanID        int64
	PlanStepID    int64
	CapabilityKey string
	Decision      string
	Reason        string
	InputSummary  string
	OutputSummary string
	Status        string
	ErrorMessage  string
	SourceRefs    []string
	Metadata      AgentJSON
	CreatedAt     time.Time
}
