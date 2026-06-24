package domain

import "time"

type AgentRunRole string

const (
	AgentRunRoleController AgentRunRole = "controller"
	AgentRunRoleExecutor   AgentRunRole = "executor"
)

func (r AgentRunRole) Valid() bool {
	switch r {
	case AgentRunRoleController, AgentRunRoleExecutor:
		return true
	default:
		return false
	}
}

type AgentRunStatus string

const (
	AgentRunStatusRunning       AgentRunStatus = "running"
	AgentRunStatusSucceeded     AgentRunStatus = "succeeded"
	AgentRunStatusFailed        AgentRunStatus = "failed"
	AgentRunStatusInputRequired AgentRunStatus = "input_required"
	AgentRunStatusCanceled      AgentRunStatus = "canceled"
)

func (s AgentRunStatus) Valid() bool {
	switch s {
	case AgentRunStatusRunning, AgentRunStatusSucceeded, AgentRunStatusFailed, AgentRunStatusInputRequired, AgentRunStatusCanceled:
		return true
	default:
		return false
	}
}

type AgentRun struct {
	ID              int64
	ParentRunID     int64
	SessionID       int64
	TurnID          int64
	Role            AgentRunRole
	Status          AgentRunStatus
	TaskPacket      AgentJSON
	CapabilityScope []string
	ModelKey        string
	ContextBudget   AgentJSON
	ContextTraceRef string
	ResultRef       string
	ErrorMessage    string
	TraceID         string
	StartedAt       time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ContextTraces   []AgentRunContextTrace
	Observations    []AgentObservation
	Artifacts       []AgentArtifact
	ChildRuns       []AgentRun
}

type AgentRunContextTrace struct {
	ID              int64
	RunID           int64
	TraceKind       string
	PromptVersion   string
	ModelKey        string
	Content         AgentJSON
	ContentHash     string
	RedactionStatus string
	TokenEstimate   int
	CreatedAt       time.Time
}

type AgentObservation struct {
	ID            int64
	RunID         int64
	CapabilityKey string
	InputSummary  string
	OutputSummary string
	Status        string
	Error         string
	ArtifactRefs  []string
	CreatedAt     time.Time
}

type AgentArtifact struct {
	ID           int64
	RunID        int64
	ArtifactType string
	ContentRef   string
	Summary      string
	SourceRefs   []string
	ContentHash  string
	CreatedAt    time.Time
}
