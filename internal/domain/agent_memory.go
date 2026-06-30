package domain

import "time"

type AgentFactType string

const (
	AgentFactTypeTranscript  AgentFactType = "transcript"
	AgentFactTypeObservation AgentFactType = "observation"
	AgentFactTypeArtifact    AgentFactType = "artifact"
	AgentFactTypePlan        AgentFactType = "plan"
	AgentFactTypePlanStep    AgentFactType = "plan_step"
	AgentFactTypeRunTrace    AgentFactType = "run_trace"
	AgentFactTypeItem        AgentFactType = "item"
	AgentFactTypeWebSnapshot AgentFactType = "web_snapshot"
)

func (t AgentFactType) Valid() bool {
	switch t {
	case AgentFactTypeTranscript,
		AgentFactTypeObservation,
		AgentFactTypeArtifact,
		AgentFactTypePlan,
		AgentFactTypePlanStep,
		AgentFactTypeRunTrace,
		AgentFactTypeItem,
		AgentFactTypeWebSnapshot:
		return true
	default:
		return false
	}
}

type AgentFactIndexStatus string

const (
	AgentFactIndexStatusReady    AgentFactIndexStatus = "ready"
	AgentFactIndexStatusPending  AgentFactIndexStatus = "pending"
	AgentFactIndexStatusFailed   AgentFactIndexStatus = "failed"
	AgentFactIndexStatusArchived AgentFactIndexStatus = "archived"
)

func (s AgentFactIndexStatus) Valid() bool {
	switch s {
	case AgentFactIndexStatusReady, AgentFactIndexStatusPending, AgentFactIndexStatusFailed, AgentFactIndexStatusArchived:
		return true
	default:
		return false
	}
}

type AgentMemoryRiskLevel string

const (
	AgentMemoryRiskLow    AgentMemoryRiskLevel = "low"
	AgentMemoryRiskMedium AgentMemoryRiskLevel = "medium"
	AgentMemoryRiskHigh   AgentMemoryRiskLevel = "high"
)

func (r AgentMemoryRiskLevel) Valid() bool {
	switch r {
	case AgentMemoryRiskLow, AgentMemoryRiskMedium, AgentMemoryRiskHigh:
		return true
	default:
		return false
	}
}

type AgentFactArchiveIndex struct {
	ID              int64
	CanonicalRef    string
	FactType        AgentFactType
	FactID          int64
	UserID          int64
	SessionID       int64
	TurnID          int64
	MemoryKind      AgentMemoryKind
	Topics          []string
	Keywords        []string
	Entities        []string
	SummaryForIndex string
	ContextualText  string
	Embedding       AgentJSON
	Importance      int
	Confidence      float64
	SourceRefs      []string
	RelationRefs    []string
	IndexStatus     AgentFactIndexStatus
	RiskLevel       AgentMemoryRiskLevel
	AccessCount     int
	LastAccessedAt  *time.Time
	Metadata        AgentJSON
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type AgentFactArchiveQueryOptions struct {
	UserID        int64
	SessionID     int64
	TurnID        int64
	FactTypes     []AgentFactType
	MemoryKinds   []AgentMemoryKind
	Query         string
	Keywords      []string
	CanonicalRefs []string
	After         *time.Time
	Before        *time.Time
	Limit         int
	Offset        int
	MinImportance int
}

type AgentMemoryCandidateStatus string

const (
	AgentMemoryCandidatePending              AgentMemoryCandidateStatus = "pending"
	AgentMemoryCandidateApplied              AgentMemoryCandidateStatus = "applied"
	AgentMemoryCandidateRequiresConfirmation AgentMemoryCandidateStatus = "requires_confirmation"
	AgentMemoryCandidateRejected             AgentMemoryCandidateStatus = "rejected"
	AgentMemoryCandidateRevoked              AgentMemoryCandidateStatus = "revoked"
	AgentMemoryCandidateExpired              AgentMemoryCandidateStatus = "expired"
)

func (s AgentMemoryCandidateStatus) Valid() bool {
	switch s {
	case AgentMemoryCandidatePending,
		AgentMemoryCandidateApplied,
		AgentMemoryCandidateRequiresConfirmation,
		AgentMemoryCandidateRejected,
		AgentMemoryCandidateRevoked,
		AgentMemoryCandidateExpired:
		return true
	default:
		return false
	}
}

type AgentMemoryCandidate struct {
	ID             int64
	UserID         int64
	SessionID      int64
	TurnID         int64
	MemoryKind     AgentMemoryKind
	CandidateText  string
	Summary        string
	EvidenceRefs   []string
	SourceRefs     []string
	Confidence     float64
	Importance     int
	RiskLevel      AgentMemoryRiskLevel
	Status         AgentMemoryCandidateStatus
	ProposedBy     string
	ExpiresAt      *time.Time
	ReviewedAt     *time.Time
	ReviewerUserID int64
	MemoryBlockID  int64
	Metadata       AgentJSON
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type AgentMemoryBlockStatus string

const (
	AgentMemoryBlockActive     AgentMemoryBlockStatus = "active"
	AgentMemoryBlockSuperseded AgentMemoryBlockStatus = "superseded"
	AgentMemoryBlockRevoked    AgentMemoryBlockStatus = "revoked"
	AgentMemoryBlockArchived   AgentMemoryBlockStatus = "archived"
)

func (s AgentMemoryBlockStatus) Valid() bool {
	switch s {
	case AgentMemoryBlockActive, AgentMemoryBlockSuperseded, AgentMemoryBlockRevoked, AgentMemoryBlockArchived:
		return true
	default:
		return false
	}
}

type AgentMemoryBlock struct {
	ID                int64
	UserID            int64
	MemoryKind        AgentMemoryKind
	Title             string
	Content           string
	Summary           string
	EvidenceRefs      []string
	SourceCandidateID int64
	Confidence        float64
	Importance        int
	Status            AgentMemoryBlockStatus
	Version           int
	LastUsedAt        *time.Time
	UseCount          int
	Metadata          AgentJSON
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type AgentMemoryBlockQueryOptions struct {
	UserID      int64
	Statuses    []AgentMemoryBlockStatus
	MemoryKinds []AgentMemoryKind
	Query       string
	Limit       int
	Offset      int
}

type AgentMemoryEventType string

const (
	AgentMemoryEventCandidateGenerated            AgentMemoryEventType = "candidate_generated"
	AgentMemoryEventCandidateApplied              AgentMemoryEventType = "candidate_applied"
	AgentMemoryEventCandidateRequiresConfirmation AgentMemoryEventType = "candidate_requires_confirmation"
	AgentMemoryEventCandidateRejected             AgentMemoryEventType = "candidate_rejected"
	AgentMemoryEventCandidateRevoked              AgentMemoryEventType = "candidate_revoked"
	AgentMemoryEventMemoryCreated                 AgentMemoryEventType = "memory_created"
	AgentMemoryEventMemoryUpdated                 AgentMemoryEventType = "memory_updated"
	AgentMemoryEventMemoryRevoked                 AgentMemoryEventType = "memory_revoked"
	AgentMemoryEventMemoryUsed                    AgentMemoryEventType = "memory_used"
)

func (t AgentMemoryEventType) Valid() bool {
	switch t {
	case AgentMemoryEventCandidateGenerated,
		AgentMemoryEventCandidateApplied,
		AgentMemoryEventCandidateRequiresConfirmation,
		AgentMemoryEventCandidateRejected,
		AgentMemoryEventCandidateRevoked,
		AgentMemoryEventMemoryCreated,
		AgentMemoryEventMemoryUpdated,
		AgentMemoryEventMemoryRevoked,
		AgentMemoryEventMemoryUsed:
		return true
	default:
		return false
	}
}

type AgentMemoryActorType string

const (
	AgentMemoryActorSystem AgentMemoryActorType = "system"
	AgentMemoryActorModel  AgentMemoryActorType = "model"
	AgentMemoryActorUser   AgentMemoryActorType = "user"
	AgentMemoryActorAdmin  AgentMemoryActorType = "admin"
)

func (t AgentMemoryActorType) Valid() bool {
	switch t {
	case AgentMemoryActorSystem, AgentMemoryActorModel, AgentMemoryActorUser, AgentMemoryActorAdmin:
		return true
	default:
		return false
	}
}

type AgentMemoryEvent struct {
	ID            int64
	UserID        int64
	SessionID     int64
	TurnID        int64
	CandidateID   int64
	MemoryBlockID int64
	EventType     AgentMemoryEventType
	ActorType     AgentMemoryActorType
	ActorUserID   int64
	Reason        string
	Payload       AgentJSON
	CreatedAt     time.Time
}
