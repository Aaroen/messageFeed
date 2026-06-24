package domain

import "time"

const (
	AgentProviderWeChatWorkApp = "wechat_work_app"
)

type ExternalAccountBindingStatus string

const (
	ExternalAccountBindingStatusActive   ExternalAccountBindingStatus = "active"
	ExternalAccountBindingStatusDisabled ExternalAccountBindingStatus = "disabled"
)

func (s ExternalAccountBindingStatus) Valid() bool {
	switch s {
	case ExternalAccountBindingStatusActive, ExternalAccountBindingStatusDisabled:
		return true
	default:
		return false
	}
}

type AgentInboundMessageStatus string

const (
	AgentInboundMessageStatusReceived  AgentInboundMessageStatus = "received"
	AgentInboundMessageStatusSucceeded AgentInboundMessageStatus = "succeeded"
	AgentInboundMessageStatusFailed    AgentInboundMessageStatus = "failed"
)

func (s AgentInboundMessageStatus) Valid() bool {
	switch s {
	case AgentInboundMessageStatusReceived, AgentInboundMessageStatusSucceeded, AgentInboundMessageStatusFailed:
		return true
	default:
		return false
	}
}

type AgentSessionStatus string

const (
	AgentSessionStatusActive AgentSessionStatus = "active"
	AgentSessionStatusClosed AgentSessionStatus = "closed"
)

func (s AgentSessionStatus) Valid() bool {
	switch s {
	case AgentSessionStatusActive, AgentSessionStatusClosed:
		return true
	default:
		return false
	}
}

type AgentTurnStatus string

const (
	AgentTurnStatusRunning   AgentTurnStatus = "running"
	AgentTurnStatusSucceeded AgentTurnStatus = "succeeded"
	AgentTurnStatusFailed    AgentTurnStatus = "failed"
)

func (s AgentTurnStatus) Valid() bool {
	switch s {
	case AgentTurnStatusRunning, AgentTurnStatusSucceeded, AgentTurnStatusFailed:
		return true
	default:
		return false
	}
}

type AgentTranscriptRole string

const (
	AgentTranscriptRoleUser      AgentTranscriptRole = "user"
	AgentTranscriptRoleAssistant AgentTranscriptRole = "assistant"
	AgentTranscriptRoleSystem    AgentTranscriptRole = "system"
	AgentTranscriptRoleTool      AgentTranscriptRole = "tool"
)

func (r AgentTranscriptRole) Valid() bool {
	switch r {
	case AgentTranscriptRoleUser, AgentTranscriptRoleAssistant, AgentTranscriptRoleSystem, AgentTranscriptRoleTool:
		return true
	default:
		return false
	}
}

type AgentJSON map[string]any

type ExternalAccount struct {
	ID                   int64
	UserID               int64
	Provider             string
	CorpID               string
	AgentID              string
	ExternalUserID       string
	DisplayName          string
	BindingStatus        ExternalAccountBindingStatus
	ActiveAgentSessionID int64
	VerifiedAt           *time.Time
	LastSeenAt           *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type AgentInboundMessage struct {
	ID                int64
	UserID            int64
	ExternalAccountID int64
	Provider          string
	ProviderMessageID string
	CorpID            string
	AgentID           string
	ExternalUserID    string
	ChatID            string
	ChatType          string
	MsgType           string
	TextContent       string
	Payload           AgentJSON
	RequestID         string
	TraceID           string
	Status            AgentInboundMessageStatus
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type AgentSession struct {
	ID                       int64
	UserID                   int64
	ExternalAccountID        int64
	Provider                 string
	ChannelSessionKey        string
	Status                   AgentSessionStatus
	Title                    string
	StartedAt                time.Time
	LastActiveAt             time.Time
	ContextInitializedAt     *time.Time
	ContextRebuildStartedAt  *time.Time
	ContextRebuildFinishedAt *time.Time
	ContextVersion           int64
	TranscriptCountIndexed   int64
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type AgentSessionStats struct {
	SessionID         int64
	TranscriptCount   int64
	ArchiveIndexCount int64
	RecallCount       int64
	FirstTranscriptAt *time.Time
	LastTranscriptAt  *time.Time
}

type AgentTurn struct {
	ID               int64
	SessionID        int64
	InboundMessageID int64
	UserID           int64
	Status           AgentTurnStatus
	InputText        string
	OutputText       string
	ModelProvider    string
	Model            string
	ErrorMessage     string
	StartedAt        time.Time
	FinishedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type AgentTranscriptEntry struct {
	ID        int64
	SessionID int64
	TurnID    int64
	UserID    int64
	Role      AgentTranscriptRole
	Content   string
	Metadata  AgentJSON
	CreatedAt time.Time
}

type AgentTranscriptArchiveStatus string

const (
	AgentTranscriptArchiveStatusHot  AgentTranscriptArchiveStatus = "hot"
	AgentTranscriptArchiveStatusWarm AgentTranscriptArchiveStatus = "warm"
	AgentTranscriptArchiveStatusCold AgentTranscriptArchiveStatus = "cold"
)

func (s AgentTranscriptArchiveStatus) Valid() bool {
	switch s {
	case AgentTranscriptArchiveStatusHot, AgentTranscriptArchiveStatusWarm, AgentTranscriptArchiveStatusCold:
		return true
	default:
		return false
	}
}

type AgentMemoryKind string

const (
	AgentMemoryKindPreference AgentMemoryKind = "preference"
	AgentMemoryKindTask       AgentMemoryKind = "task"
	AgentMemoryKindFact       AgentMemoryKind = "fact"
	AgentMemoryKindDecision   AgentMemoryKind = "decision"
	AgentMemoryKindCasual     AgentMemoryKind = "casual"
	AgentMemoryKindUnknown    AgentMemoryKind = "unknown"
)

func (k AgentMemoryKind) Valid() bool {
	switch k {
	case AgentMemoryKindPreference, AgentMemoryKindTask, AgentMemoryKindFact, AgentMemoryKindDecision, AgentMemoryKindCasual, AgentMemoryKindUnknown:
		return true
	default:
		return false
	}
}

type AgentTranscriptArchiveIndex struct {
	ID                int64
	TranscriptEntryID int64
	SessionID         int64
	UserID            int64
	ArchiveStatus     AgentTranscriptArchiveStatus
	MemoryKind        AgentMemoryKind
	Importance        int
	Keywords          []string
	LastAccessedAt    *time.Time
	AccessCount       int
	Metadata          AgentJSON
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type AgentRecallEvent struct {
	ID           int64
	SessionID    int64
	TurnID       int64
	UserID       int64
	Query        string
	QueryParams  AgentJSON
	RecalledRefs AgentJSON
	Reason       string
	BudgetChars  int
	CreatedAt    time.Time
}

type AgentTranscriptListOptions struct {
	SessionID     int64
	UserID        int64
	BeforeTurnID  int64
	BeforeEntryID int64
	Roles         []AgentTranscriptRole
	Limit         int
}

type AgentTranscriptQueryOptions struct {
	SessionID     int64
	UserID        int64
	Mode          string
	Keyword       string
	Roles         []AgentTranscriptRole
	ArchiveStatus AgentTranscriptArchiveStatus
	MemoryKind    AgentMemoryKind
	BeforeEntryID int64
	AfterEntryID  int64
	BeforeTurnID  int64
	After         *time.Time
	Before        *time.Time
	Order         string
	Offset        int
	Limit         int
}

type AgentAuditLog struct {
	ID        int64
	SessionID int64
	TurnID    int64
	UserID    int64
	EventType string
	Status    string
	Message   string
	Metadata  AgentJSON
	RequestID string
	TraceID   string
	CreatedAt time.Time
}
