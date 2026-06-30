package service

import (
	"context"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"messagefeed/internal/notifier"
	"time"
)

// AgentConversationRepository 聚合主闭环所需的持久化能力，具体能力按职责拆分。
type AgentConversationRepository interface {
	AgentConversationAccountStore
	AgentConversationMessageStore
	AgentConversationSessionStore
	AgentConversationTurnStore
	AgentConversationMemoryStore
	AgentConversationAuditStore
	AgentConversationRunStore
	AgentConversationPlanStore
	AgentConversationApprovalStore
	AgentConversationPreferenceStore
}

type AgentConversationAccountStore interface {
	EnsureExternalAccount(ctx context.Context, account domain.ExternalAccount) (domain.ExternalAccount, error)
	ListExternalAccounts(ctx context.Context, userID int64) ([]domain.ExternalAccount, error)
}

type AgentConversationMessageStore interface {
	CreateInboundMessage(ctx context.Context, message domain.AgentInboundMessage) (domain.AgentInboundMessage, bool, error)
	UpdateInboundMessageStatus(ctx context.Context, userID int64, id int64, status domain.AgentInboundMessageStatus, now time.Time) (domain.AgentInboundMessage, error)
}

type AgentConversationSessionStore interface {
	GetOrCreateSession(ctx context.Context, session domain.AgentSession) (domain.AgentSession, error)
	GetAgentSession(ctx context.Context, userID int64, sessionID int64) (domain.AgentSession, error)
	TouchAgentSession(ctx context.Context, userID int64, sessionID int64, now time.Time) error
}

type AgentConversationTurnStore interface {
	CreateTurn(ctx context.Context, turn domain.AgentTurn) (domain.AgentTurn, error)
	UpdateTurn(ctx context.Context, turn domain.AgentTurn) (domain.AgentTurn, error)
	AppendTranscriptEntry(ctx context.Context, entry domain.AgentTranscriptEntry) (domain.AgentTranscriptEntry, error)
}

type AgentConversationMemoryStore interface {
	ListRecentTranscriptEntries(ctx context.Context, options domain.AgentTranscriptListOptions) ([]domain.AgentTranscriptEntry, error)
	QueryTranscriptEntries(ctx context.Context, options domain.AgentTranscriptQueryOptions) ([]domain.AgentTranscriptEntry, error)
	CreateRecallEvent(ctx context.Context, event domain.AgentRecallEvent) (domain.AgentRecallEvent, error)
	UpsertAgentFactArchiveIndex(ctx context.Context, fact domain.AgentFactArchiveIndex) (domain.AgentFactArchiveIndex, error)
	QueryAgentFactArchiveIndex(ctx context.Context, options domain.AgentFactArchiveQueryOptions) ([]domain.AgentFactArchiveIndex, error)
	ResolveAgentFactSources(ctx context.Context, userID int64, facts []domain.AgentFactArchiveIndex) ([]domain.AgentFactSource, error)
	CreateAgentMemoryCandidate(ctx context.Context, candidate domain.AgentMemoryCandidate) (domain.AgentMemoryCandidate, error)
	ListAgentMemoryCandidates(ctx context.Context, userID int64, status domain.AgentMemoryCandidateStatus, limit int) ([]domain.AgentMemoryCandidate, error)
	UpdateAgentMemoryCandidateStatus(ctx context.Context, userID int64, candidateID int64, status domain.AgentMemoryCandidateStatus, reason string, now time.Time) (domain.AgentMemoryCandidate, error)
	ApplyAgentMemoryCandidate(ctx context.Context, userID int64, candidateID int64, now time.Time) (domain.AgentMemoryBlock, error)
	ListAgentMemoryBlocks(ctx context.Context, options domain.AgentMemoryBlockQueryOptions) ([]domain.AgentMemoryBlock, error)
	CreateAgentMemoryEvent(ctx context.Context, event domain.AgentMemoryEvent) (domain.AgentMemoryEvent, error)
}

type AgentConversationAuditStore interface {
	CreateAuditLog(ctx context.Context, log domain.AgentAuditLog) (domain.AgentAuditLog, error)
	CreateAgentCapabilityAuditLog(ctx context.Context, log domain.AgentCapabilityAuditLog) (domain.AgentCapabilityAuditLog, error)
}

type AgentConversationRunStore interface {
	CreateAgentRun(ctx context.Context, run domain.AgentRun) (domain.AgentRun, error)
	UpdateAgentRun(ctx context.Context, run domain.AgentRun) (domain.AgentRun, error)
	ListAgentRunsByTurn(ctx context.Context, userID int64, turnID int64) ([]domain.AgentRun, error)
	CreateAgentRunContextTrace(ctx context.Context, trace domain.AgentRunContextTrace) (domain.AgentRunContextTrace, error)
	CreateAgentObservation(ctx context.Context, observation domain.AgentObservation) (domain.AgentObservation, error)
	CreateAgentArtifact(ctx context.Context, artifact domain.AgentArtifact) (domain.AgentArtifact, error)
}

type AgentConversationPlanStore interface {
	CreateAgentPlan(ctx context.Context, plan domain.AgentPlan, steps []domain.AgentPlanStep) (domain.AgentPlan, error)
	ListAgentPlans(ctx context.Context, userID int64, sessionID int64, turnID int64, limit int) ([]domain.AgentPlan, error)
	GetAgentPlan(ctx context.Context, userID int64, planID int64) (domain.AgentPlan, error)
	ListAgentScheduledTasks(ctx context.Context, options domain.AgentScheduledTaskListOptions) ([]domain.AgentScheduledTask, error)
	UpdateAgentScheduledTask(ctx context.Context, task domain.AgentScheduledTask) (domain.AgentScheduledTask, error)
	UpdateAgentPlanStatus(ctx context.Context, userID int64, planID int64, status domain.AgentPlanStatus, now time.Time, errorMessage string) (domain.AgentPlan, error)
	UpdateAgentPlanMetadata(ctx context.Context, userID int64, planID int64, metadata domain.AgentJSON, now time.Time) (domain.AgentPlan, error)
	UpdateAgentPlanStepStatus(ctx context.Context, userID int64, step domain.AgentPlanStep) (domain.AgentPlanStep, error)
}

type AgentConversationApprovalStore interface {
	CreateAgentApproval(ctx context.Context, approval domain.AgentApproval) (domain.AgentApproval, error)
}

type AgentConversationPreferenceStore interface {
	GetAgentNotificationPreference(ctx context.Context, userID int64) (domain.AgentNotificationPreference, error)
}

type AgentExternalAccountResolver interface {
	ResolveExternalAccount(ctx context.Context, provider string, corpID string, agentID string, externalUserID string) (domain.ExternalAccount, error)
}

type AgentUserContextProvider interface {
	BuildAgentUserContext(ctx context.Context, userID int64) (UserContextResult, error)
}

type AgentConversationLLM interface {
	Chat(ctx context.Context, request llm.ChatRequest) (llm.ChatResponse, error)
}

type AgentConversationSender interface {
	SendText(ctx context.Context, message notifier.WeChatWorkTextMessage) (notifier.WeChatWorkSendResult, error)
}

type AgentRecentItemsProvider interface {
	ListItems(ctx context.Context, input ListItemsInput) (ListItemsResult, error)
}

type AgentSourceProvider interface {
	ListSources(ctx context.Context, userID int64) ([]domain.Source, error)
}

type AgentNotificationJobStore interface {
	CreateJob(ctx context.Context, job domain.NotificationJob) (domain.NotificationJob, error)
}

type ReceiveWeChatWorkAppMessageInput struct {
	Provider          string
	ProviderMessageID string
	CorpID            string
	AgentID           string
	ExternalUserID    string
	ChatID            string
	ChatType          string
	MsgType           string
	TextContent       string
	EventType         string
	EventKey          string
	RawXML            string
	RequestID         string
	TraceID           string
}

type ReceiveWeChatWorkAppMessageResult struct {
	ExternalAccount domain.ExternalAccount
	InboundMessage  domain.AgentInboundMessage
	Session         domain.AgentSession
	Turn            domain.AgentTurn
	Plan            domain.AgentPlan
	Reply           string
	SendResult      notifier.WeChatWorkSendResult
	Duplicate       bool
	BindingRequired bool
	ProcessingAsync bool
}

type ReceiveWebAgentTaskInput struct {
	Message   string
	SessionID int64
	Channel   string
	RequestID string
	TraceID   string
}

type AgentTurnResponse struct {
	ID               int64  `json:"id"`
	SessionID        int64  `json:"session_id"`
	InboundMessageID int64  `json:"inbound_message_id"`
	Status           string `json:"status"`
	InputText        string `json:"input_text"`
	OutputText       string `json:"output_text"`
	ErrorMessage     string `json:"error_message"`
	StartedAt        string `json:"started_at"`
	FinishedAt       string `json:"finished_at,omitempty"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

type ReceiveWebAgentTaskResult struct {
	Session     AgentSessionResponse `json:"session"`
	Turn        AgentTurnResponse    `json:"turn"`
	Plan        AgentPlanResponse    `json:"plan"`
	Reply       string               `json:"reply"`
	ProgressURL string               `json:"progress_url"`
	Duplicate   bool                 `json:"duplicate"`
}
