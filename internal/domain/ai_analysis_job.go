package domain

import "time"

type AIAnalysisJobStatus string

const (
	AIAnalysisJobStatusQueued    AIAnalysisJobStatus = "queued"
	AIAnalysisJobStatusRunning   AIAnalysisJobStatus = "running"
	AIAnalysisJobStatusSucceeded AIAnalysisJobStatus = "succeeded"
	AIAnalysisJobStatusFailed    AIAnalysisJobStatus = "failed"
	AIAnalysisJobStatusSkipped   AIAnalysisJobStatus = "skipped"
	AIAnalysisJobStatusCanceled  AIAnalysisJobStatus = "canceled"
)

func (s AIAnalysisJobStatus) Valid() bool {
	switch s {
	case AIAnalysisJobStatusQueued, AIAnalysisJobStatusRunning, AIAnalysisJobStatusSucceeded, AIAnalysisJobStatusFailed, AIAnalysisJobStatusSkipped, AIAnalysisJobStatusCanceled:
		return true
	default:
		return false
	}
}

type AIAnalysisRiskLevel string

const (
	AIAnalysisRiskLevelUnknown AIAnalysisRiskLevel = "unknown"
	AIAnalysisRiskLevelLow     AIAnalysisRiskLevel = "low"
	AIAnalysisRiskLevelMedium  AIAnalysisRiskLevel = "medium"
	AIAnalysisRiskLevelHigh    AIAnalysisRiskLevel = "high"
)

func (l AIAnalysisRiskLevel) Valid() bool {
	switch l {
	case AIAnalysisRiskLevelUnknown, AIAnalysisRiskLevelLow, AIAnalysisRiskLevelMedium, AIAnalysisRiskLevelHigh:
		return true
	default:
		return false
	}
}

type AIAnalysisJobInput map[string]any

type AIAnalysisResult struct {
	ShouldNotify   bool                `json:"should_notify"`
	Importance     float64             `json:"importance"`
	MatchedReasons []string            `json:"matched_reasons"`
	Summary        string              `json:"summary"`
	RiskLevel      AIAnalysisRiskLevel `json:"risk_level"`
	Confidence     float64             `json:"confidence"`
}

type AIAnalysisJob struct {
	ID               int64
	UserID           int64
	AlertCandidateID int64
	SourceID         int64
	ItemID           int64
	Status           AIAnalysisJobStatus
	Input            AIAnalysisJobInput
	Result           AIAnalysisResult
	ScheduledAt      time.Time
	StartedAt        *time.Time
	FinishedAt       *time.Time
	AttemptCount     int
	MaxAttempts      int
	LockedBy         string
	LockedAt         *time.Time
	LastError        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type AIAnalysisJobClaimInput struct {
	Now      time.Time
	WorkerID string
	Limit    int
}

type AIAnalysisJobListOptions struct {
	UserID int64
	Limit  int
	Offset int
}

type AIAnalysisJobListResult struct {
	Jobs   []AIAnalysisJob
	Total  int64
	Limit  int
	Offset int
}
