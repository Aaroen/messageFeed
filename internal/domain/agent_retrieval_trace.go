package domain

import "time"

type AgentRecallTraceStatus string

const (
	AgentRecallTraceSucceeded AgentRecallTraceStatus = "succeeded"
	AgentRecallTraceFailed    AgentRecallTraceStatus = "failed"
	AgentRecallTraceDegraded  AgentRecallTraceStatus = "degraded"
)

func (s AgentRecallTraceStatus) Valid() bool {
	switch s {
	case AgentRecallTraceSucceeded, AgentRecallTraceFailed, AgentRecallTraceDegraded:
		return true
	default:
		return false
	}
}

type AgentRecallTrace struct {
	ID                   int64
	RequestID            string
	TraceID              string
	UserID               int64
	SessionID            int64
	TurnID               int64
	RunID                int64
	PlanID               int64
	Mode                 AgentFactRecallMode
	QueryText            string
	NeedsHistoryRecall   bool
	HistoryQueryPlan     AgentJSON
	FullTextAttempted    bool
	FullTextCount        int
	FullTextMS           int64
	EmbeddingAttempted   bool
	EmbeddingModel       string
	EmbeddingDimension   int
	EmbeddingMS          int64
	EmbeddingStatus      string
	EmbeddingError       string
	VectorAttempted      bool
	VectorCandidateCount int
	VectorMS             int64
	RelationAttempted    bool
	RelationCount        int
	RelationMS           int64
	FinalHitCount        int
	FinalSources         AgentJSON
	FallbackReason       string
	TotalMS              int64
	Status               AgentRecallTraceStatus
	ErrorMessage         string
	CreatedAt            time.Time
}

type AgentEmbeddingTraceStatus string

const (
	AgentEmbeddingTraceSucceeded AgentEmbeddingTraceStatus = "succeeded"
	AgentEmbeddingTraceFailed    AgentEmbeddingTraceStatus = "failed"
	AgentEmbeddingTraceSkipped   AgentEmbeddingTraceStatus = "skipped"
)

func (s AgentEmbeddingTraceStatus) Valid() bool {
	switch s {
	case AgentEmbeddingTraceSucceeded, AgentEmbeddingTraceFailed, AgentEmbeddingTraceSkipped:
		return true
	default:
		return false
	}
}

type AgentEmbeddingTrace struct {
	ID                 int64
	JobID              string
	RequestID          string
	TraceID            string
	UserID             int64
	CanonicalRef       string
	EmbeddingModel     string
	EmbeddingDimension int
	InputChars         int
	ContentHash        string
	Status             AgentEmbeddingTraceStatus
	DurationMS         int64
	ErrorMessage       string
	RetryCount         int
	Metadata           AgentJSON
	CreatedAt          time.Time
}
