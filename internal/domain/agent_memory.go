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
	RelationRefs  []string
	After         *time.Time
	Before        *time.Time
	Limit         int
	Offset        int
	MinImportance int
	MaxRiskLevel  AgentMemoryRiskLevel
	UseFullText   bool
}

type AgentFactSource struct {
	CanonicalRef string
	FactType     AgentFactType
	FactID       int64
	UserID       int64
	SessionID    int64
	TurnID       int64
	Title        string
	Content      string
	Summary      string
	SourceRefs   []string
	Metadata     AgentJSON
	CreatedAt    time.Time
}

type AgentFactEmbeddingStatus string

const (
	AgentFactEmbeddingStatusReady    AgentFactEmbeddingStatus = "ready"
	AgentFactEmbeddingStatusPending  AgentFactEmbeddingStatus = "pending"
	AgentFactEmbeddingStatusFailed   AgentFactEmbeddingStatus = "failed"
	AgentFactEmbeddingStatusArchived AgentFactEmbeddingStatus = "archived"
)

func (s AgentFactEmbeddingStatus) Valid() bool {
	switch s {
	case AgentFactEmbeddingStatusReady, AgentFactEmbeddingStatusPending, AgentFactEmbeddingStatusFailed, AgentFactEmbeddingStatusArchived:
		return true
	default:
		return false
	}
}

type AgentFactEmbedding struct {
	ID                 int64
	CanonicalRef       string
	UserID             int64
	EmbeddingModel     string
	EmbeddingDimension int
	ContentHash        string
	Vector             []float32
	EmbeddingStatus    AgentFactEmbeddingStatus
	ErrorMessage       string
	Metadata           AgentJSON
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type AgentFactEmbeddingQueryOptions struct {
	UserID         int64
	EmbeddingModel string
	ContentHash    string
	Statuses       []AgentFactEmbeddingStatus
	Limit          int
}

type AgentFactRecallMode string

const (
	AgentFactRecallModeSearch    AgentFactRecallMode = "search"
	AgentFactRecallModeSemantic  AgentFactRecallMode = "semantic"
	AgentFactRecallModeHybrid    AgentFactRecallMode = "hybrid"
	AgentFactRecallModeTimeRange AgentFactRecallMode = "time_range"
	AgentFactRecallModeEarliest  AgentFactRecallMode = "earliest"
	AgentFactRecallModeLatest    AgentFactRecallMode = "latest"
)

func (m AgentFactRecallMode) Valid() bool {
	switch m {
	case AgentFactRecallModeSearch,
		AgentFactRecallModeSemantic,
		AgentFactRecallModeHybrid,
		AgentFactRecallModeTimeRange,
		AgentFactRecallModeEarliest,
		AgentFactRecallModeLatest:
		return true
	default:
		return false
	}
}

type AgentFactRecallPlan struct {
	Mode            AgentFactRecallMode
	Query           string
	UserID          int64
	SessionID       int64
	TurnID          int64
	After           *time.Time
	Before          *time.Time
	FactTypes       []AgentFactType
	MemoryKinds     []AgentMemoryKind
	Limit           int
	NeedsSourceFact bool
	MaxRiskLevel    AgentMemoryRiskLevel
	EmbeddingModel  string
}

type AgentFactRecallHit struct {
	Fact            AgentFactArchiveIndex
	CanonicalRef    string
	StructuredScore float64
	FullTextScore   float64
	VectorScore     float64
	ImportanceScore float64
	RecencyScore    float64
	RelationScore   float64
	FinalScore      float64
	HitSources      []string
	Reason          string
}

type AgentFactProjection struct {
	CanonicalRef  string
	IndexHit      AgentFactRecallHit
	SourceFact    AgentFactSource
	Text          string
	TokenEstimate int
	TrustLevel    string
	RiskLevel     AgentMemoryRiskLevel
}

type AgentFactRecallResult struct {
	Plan        AgentFactRecallPlan
	Hits        []AgentFactRecallHit
	Sources     []AgentFactSource
	Projections []AgentFactProjection
	GeneratedAt time.Time
}

type AgentFactIndexJobType string

const (
	AgentFactIndexJobBackfill AgentFactIndexJobType = "backfill_fact_index"
	AgentFactIndexJobEmbed    AgentFactIndexJobType = "embed_fact_index"
	AgentFactIndexJobRebuild  AgentFactIndexJobType = "rebuild_fact_index"
)

func (t AgentFactIndexJobType) Valid() bool {
	switch t {
	case AgentFactIndexJobBackfill, AgentFactIndexJobEmbed, AgentFactIndexJobRebuild:
		return true
	default:
		return false
	}
}

type AgentFactIndexJobStatus string

const (
	AgentFactIndexJobPending   AgentFactIndexJobStatus = "pending"
	AgentFactIndexJobRunning   AgentFactIndexJobStatus = "running"
	AgentFactIndexJobSucceeded AgentFactIndexJobStatus = "succeeded"
	AgentFactIndexJobFailed    AgentFactIndexJobStatus = "failed"
	AgentFactIndexJobCancelled AgentFactIndexJobStatus = "cancelled"
)

func (s AgentFactIndexJobStatus) Valid() bool {
	switch s {
	case AgentFactIndexJobPending, AgentFactIndexJobRunning, AgentFactIndexJobSucceeded, AgentFactIndexJobFailed, AgentFactIndexJobCancelled:
		return true
	default:
		return false
	}
}

type AgentFactIndexJob struct {
	ID             int64
	JobType        AgentFactIndexJobType
	Scope          AgentJSON
	Status         AgentFactIndexJobStatus
	Cursor         AgentJSON
	TotalCount     int
	ProcessedCount int
	FailedCount    int
	ErrorMessage   string
	StartedAt      *time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type AgentFactBackfillResult struct {
	ProcessedCount int
	FailedCount    int
}

type AgentFactIndexStats struct {
	UserID               int64
	FactIndexCount       int64
	ReadyCount           int64
	PendingCount         int64
	FailedCount          int64
	ArchivedCount        int64
	EmbeddingCount       int64
	ReadyEmbeddingCount  int64
	FailedEmbeddingCount int64
	LastIndexedAt        *time.Time
	LastEmbeddedAt       *time.Time
	ByFactType           map[string]int64
	ByMemoryKind         map[string]int64
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
