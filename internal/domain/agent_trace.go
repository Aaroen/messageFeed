package domain

import "time"

type AgentTraceEventKind string

const (
	AgentTraceEventInbound           AgentTraceEventKind = "inbound"
	AgentTraceEventTranscript        AgentTraceEventKind = "transcript"
	AgentTraceEventContextProjection AgentTraceEventKind = "context_projection"
	AgentTraceEventPlanner           AgentTraceEventKind = "planner"
	AgentTraceEventSubagentDispatch  AgentTraceEventKind = "subagent_dispatch"
	AgentTraceEventToolExecution     AgentTraceEventKind = "tool_execution"
	AgentTraceEventApproval          AgentTraceEventKind = "approval"
	AgentTraceEventRecall            AgentTraceEventKind = "recall"
	AgentTraceEventMemory            AgentTraceEventKind = "memory"
	AgentTraceEventEmbedding         AgentTraceEventKind = "embedding"
	AgentTraceEventLLM               AgentTraceEventKind = "llm"
	AgentTraceEventNotification      AgentTraceEventKind = "notification"
	AgentTraceEventWorker            AgentTraceEventKind = "worker"
	AgentTraceEventRecovery          AgentTraceEventKind = "recovery"
)

func (k AgentTraceEventKind) Valid() bool {
	switch k {
	case AgentTraceEventInbound,
		AgentTraceEventTranscript,
		AgentTraceEventContextProjection,
		AgentTraceEventPlanner,
		AgentTraceEventSubagentDispatch,
		AgentTraceEventToolExecution,
		AgentTraceEventApproval,
		AgentTraceEventRecall,
		AgentTraceEventMemory,
		AgentTraceEventEmbedding,
		AgentTraceEventLLM,
		AgentTraceEventNotification,
		AgentTraceEventWorker,
		AgentTraceEventRecovery:
		return true
	default:
		return false
	}
}

type AgentTraceEventStatus string

const (
	AgentTraceEventStarted   AgentTraceEventStatus = "started"
	AgentTraceEventSucceeded AgentTraceEventStatus = "succeeded"
	AgentTraceEventFailed    AgentTraceEventStatus = "failed"
	AgentTraceEventSkipped   AgentTraceEventStatus = "skipped"
	AgentTraceEventDegraded  AgentTraceEventStatus = "degraded"
)

func (s AgentTraceEventStatus) Valid() bool {
	switch s {
	case AgentTraceEventStarted, AgentTraceEventSucceeded, AgentTraceEventFailed, AgentTraceEventSkipped, AgentTraceEventDegraded:
		return true
	default:
		return false
	}
}

type AgentTraceEvent struct {
	ID            int64
	RequestID     string
	TraceID       string
	SpanID        string
	UserID        int64
	SessionID     int64
	TurnID        int64
	PlanID        int64
	RunID         int64
	ParentRunID   int64
	StepID        int64
	EventKind     AgentTraceEventKind
	EventName     string
	Status        AgentTraceEventStatus
	StartedAt     time.Time
	FinishedAt    *time.Time
	DurationMS    int64
	ModelKey      string
	CapabilityKey string
	ToolName      string
	JobID         string
	ArtifactRefs  []string
	SourceRefs    []string
	InputSummary  string
	OutputSummary string
	ErrorCode     string
	ErrorMessage  string
	Metadata      AgentJSON
	CreatedAt     time.Time
}

type AgentTraceEventListOptions struct {
	UserID    int64
	RequestID string
	TraceID   string
	SessionID int64
	TurnID    int64
	PlanID    int64
	RunID     int64
	Limit     int
}
