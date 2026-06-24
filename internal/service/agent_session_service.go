package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"messagefeed/internal/domain"
	"strings"
	"time"
)

type AgentSessionRepository interface {
	ListAgentSessions(ctx context.Context, userID int64) ([]domain.AgentSession, error)
	GetAgentSession(ctx context.Context, userID int64, sessionID int64) (domain.AgentSession, error)
	CreateAgentSession(ctx context.Context, session domain.AgentSession) (domain.AgentSession, error)
	ListExternalAccounts(ctx context.Context, userID int64) ([]domain.ExternalAccount, error)
	GetExternalAccount(ctx context.Context, userID int64, accountID int64) (domain.ExternalAccount, error)
	SetExternalAccountActiveSession(ctx context.Context, userID int64, externalAccountID int64, sessionID int64) (domain.ExternalAccount, error)
	GetAgentSessionStats(ctx context.Context, userID int64, sessionID int64) (domain.AgentSessionStats, error)
	ClearAgentSessionContext(ctx context.Context, userID int64, sessionID int64, now time.Time) (domain.AgentSessionStats, error)
	RebuildAgentSessionContext(ctx context.Context, userID int64, sessionID int64, now time.Time) (domain.AgentSessionStats, error)
	DeleteAgentSession(ctx context.Context, userID int64, sessionID int64) error
	ListRecentTranscriptEntries(ctx context.Context, options domain.AgentTranscriptListOptions) ([]domain.AgentTranscriptEntry, error)
	ListAgentRunsByTurn(ctx context.Context, userID int64, turnID int64) ([]domain.AgentRun, error)
	GetAgentRunDetail(ctx context.Context, userID int64, runID int64) (domain.AgentRun, error)
}

type AgentSessionService struct {
	repository AgentSessionRepository
	now        func() time.Time
}

type AgentSessionServiceOption func(*AgentSessionService)

func WithAgentSessionNow(now func() time.Time) AgentSessionServiceOption {
	return func(service *AgentSessionService) {
		if now != nil {
			service.now = now
		}
	}
}

func NewAgentSessionService(repository AgentSessionRepository, options ...AgentSessionServiceOption) *AgentSessionService {
	service := &AgentSessionService{repository: repository, now: time.Now}
	for _, option := range options {
		option(service)
	}
	return service
}

type AgentSessionListResult struct {
	Accounts []AgentExternalAccountResponse `json:"accounts"`
	Sessions []AgentSessionResponse         `json:"sessions"`
}

type AgentExternalAccountResponse struct {
	ID                   int64  `json:"id"`
	Provider             string `json:"provider"`
	CorpID               string `json:"corp_id"`
	AgentID              string `json:"agent_id"`
	ExternalUserID       string `json:"external_user_id"`
	DisplayName          string `json:"display_name"`
	BindingStatus        string `json:"binding_status"`
	ActiveAgentSessionID int64  `json:"active_agent_session_id"`
	UpdatedAt            string `json:"updated_at"`
}

type AgentSessionResponse struct {
	ID                       int64             `json:"id"`
	ExternalAccountID        int64             `json:"external_account_id"`
	Provider                 string            `json:"provider"`
	ChannelSessionKey        string            `json:"channel_session_key"`
	Status                   string            `json:"status"`
	Title                    string            `json:"title"`
	ActiveForAccount         bool              `json:"active_for_account"`
	ContextInitializedAt     string            `json:"context_initialized_at,omitempty"`
	ContextRebuildStartedAt  string            `json:"context_rebuild_started_at,omitempty"`
	ContextRebuildFinishedAt string            `json:"context_rebuild_finished_at,omitempty"`
	ContextVersion           int64             `json:"context_version"`
	TranscriptCountIndexed   int64             `json:"transcript_count_indexed"`
	Stats                    AgentSessionStats `json:"stats"`
	StartedAt                string            `json:"started_at"`
	LastActiveAt             string            `json:"last_active_at"`
	CreatedAt                string            `json:"created_at"`
	UpdatedAt                string            `json:"updated_at"`
}

type AgentSessionStats struct {
	TranscriptCount   int64  `json:"transcript_count"`
	ArchiveIndexCount int64  `json:"archive_index_count"`
	RecallCount       int64  `json:"recall_count"`
	FirstTranscriptAt string `json:"first_transcript_at,omitempty"`
	LastTranscriptAt  string `json:"last_transcript_at,omitempty"`
}

type AgentTranscriptListResult struct {
	Entries []AgentTranscriptEntryResponse `json:"entries"`
}

type AgentTranscriptEntryResponse struct {
	ID        int64  `json:"id"`
	TurnID    int64  `json:"turn_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type AgentRunListResult struct {
	Runs []AgentRunResponse `json:"runs"`
}

type AgentRunDetailResult struct {
	Run AgentRunResponse `json:"run"`
}

type AgentRunResponse struct {
	ID              int64                          `json:"id"`
	ParentRunID     int64                          `json:"parent_run_id"`
	SessionID       int64                          `json:"session_id"`
	TurnID          int64                          `json:"turn_id"`
	Role            string                         `json:"role"`
	Status          string                         `json:"status"`
	TaskPacket      domain.AgentJSON               `json:"task_packet"`
	CapabilityScope []string                       `json:"capability_scope"`
	ModelKey        string                         `json:"model_key"`
	ContextBudget   domain.AgentJSON               `json:"context_budget"`
	ContextTraceRef string                         `json:"context_trace_ref"`
	ResultRef       string                         `json:"result_ref"`
	ErrorMessage    string                         `json:"error_message"`
	TraceID         string                         `json:"trace_id"`
	StartedAt       string                         `json:"started_at"`
	CompletedAt     string                         `json:"completed_at,omitempty"`
	CreatedAt       string                         `json:"created_at"`
	UpdatedAt       string                         `json:"updated_at"`
	ContextTraces   []AgentRunContextTraceResponse `json:"context_traces,omitempty"`
	Observations    []AgentObservationResponse     `json:"observations,omitempty"`
	Artifacts       []AgentArtifactResponse        `json:"artifacts,omitempty"`
	ChildRuns       []AgentRunResponse             `json:"child_runs,omitempty"`
}

type AgentRunContextTraceResponse struct {
	ID              int64            `json:"id"`
	RunID           int64            `json:"run_id"`
	TraceKind       string           `json:"trace_kind"`
	PromptVersion   string           `json:"prompt_version"`
	ModelKey        string           `json:"model_key"`
	Content         domain.AgentJSON `json:"content"`
	ContentHash     string           `json:"content_hash"`
	RedactionStatus string           `json:"redaction_status"`
	TokenEstimate   int              `json:"token_estimate"`
	CreatedAt       string           `json:"created_at"`
}

type AgentObservationResponse struct {
	ID            int64    `json:"id"`
	RunID         int64    `json:"run_id"`
	CapabilityKey string   `json:"capability_key"`
	InputSummary  string   `json:"input_summary"`
	OutputSummary string   `json:"output_summary"`
	Status        string   `json:"status"`
	Error         string   `json:"error"`
	ArtifactRefs  []string `json:"artifact_refs"`
	CreatedAt     string   `json:"created_at"`
}

type AgentArtifactResponse struct {
	ID           int64    `json:"id"`
	RunID        int64    `json:"run_id"`
	ArtifactType string   `json:"artifact_type"`
	ContentRef   string   `json:"content_ref"`
	Summary      string   `json:"summary"`
	SourceRefs   []string `json:"source_refs"`
	ContentHash  string   `json:"content_hash"`
	CreatedAt    string   `json:"created_at"`
}

func (s *AgentSessionService) ListSessions(ctx context.Context, auth CurrentAuth) (AgentSessionListResult, error) {
	if s == nil || s.repository == nil {
		return AgentSessionListResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_sessions_unavailable", "agent session service is unavailable", "service.agent_session.list", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentSessionListResult{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	accounts, err := s.repository.ListExternalAccounts(ctx, auth.User.ID)
	if err != nil {
		return AgentSessionListResult{}, err
	}
	sessions, err := s.repository.ListAgentSessions(ctx, auth.User.ID)
	if err != nil {
		return AgentSessionListResult{}, err
	}
	activeByAccount := map[int64]int64{}
	accountResponses := make([]AgentExternalAccountResponse, 0, len(accounts))
	for _, account := range accounts {
		activeByAccount[account.ID] = account.ActiveAgentSessionID
		accountResponses = append(accountResponses, agentExternalAccountResponse(account))
	}
	sessionResponses := make([]AgentSessionResponse, 0, len(sessions))
	for _, session := range sessions {
		stats, err := s.repository.GetAgentSessionStats(ctx, auth.User.ID, session.ID)
		if err != nil {
			return AgentSessionListResult{}, err
		}
		sessionResponses = append(sessionResponses, agentSessionResponse(session, stats, activeByAccount[session.ExternalAccountID] == session.ID))
	}
	return AgentSessionListResult{Accounts: accountResponses, Sessions: sessionResponses}, nil
}

func (s *AgentSessionService) CreateSession(ctx context.Context, auth CurrentAuth, externalAccountID int64, title string) (AgentSessionResponse, error) {
	if s == nil || s.repository == nil {
		return AgentSessionResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_sessions_unavailable", "agent session service is unavailable", "service.agent_session.create", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentSessionResponse{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	account, err := s.repository.GetExternalAccount(ctx, auth.User.ID, externalAccountID)
	if err != nil {
		return AgentSessionResponse{}, err
	}
	now := s.now().UTC()
	title = strings.TrimSpace(title)
	if title == "" {
		title = "企业微信对话"
	}
	session, err := s.repository.CreateAgentSession(ctx, domain.AgentSession{
		UserID:            auth.User.ID,
		ExternalAccountID: account.ID,
		Provider:          account.Provider,
		ChannelSessionKey: manualAgentSessionKey(account),
		Status:            domain.AgentSessionStatusActive,
		Title:             title,
		StartedAt:         now,
		LastActiveAt:      now,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		return AgentSessionResponse{}, err
	}
	stats, err := s.repository.GetAgentSessionStats(ctx, auth.User.ID, session.ID)
	if err != nil {
		return AgentSessionResponse{}, err
	}
	return agentSessionResponse(session, stats, false), nil
}

func (s *AgentSessionService) SelectSession(ctx context.Context, auth CurrentAuth, sessionID int64) (AgentExternalAccountResponse, error) {
	if s == nil || s.repository == nil {
		return AgentExternalAccountResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_sessions_unavailable", "agent session service is unavailable", "service.agent_session.select", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentExternalAccountResponse{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	session, err := s.repository.GetAgentSession(ctx, auth.User.ID, sessionID)
	if err != nil {
		return AgentExternalAccountResponse{}, err
	}
	account, err := s.repository.SetExternalAccountActiveSession(ctx, auth.User.ID, session.ExternalAccountID, session.ID)
	if err != nil {
		return AgentExternalAccountResponse{}, err
	}
	return agentExternalAccountResponse(account), nil
}

func (s *AgentSessionService) RebuildContext(ctx context.Context, auth CurrentAuth, sessionID int64) (AgentSessionStats, error) {
	if s == nil || s.repository == nil {
		return AgentSessionStats{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_sessions_unavailable", "agent session service is unavailable", "service.agent_session.rebuild", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentSessionStats{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	stats, err := s.repository.RebuildAgentSessionContext(ctx, auth.User.ID, sessionID, s.now().UTC())
	if err != nil {
		return AgentSessionStats{}, err
	}
	return agentSessionStats(stats), nil
}

func (s *AgentSessionService) ClearContext(ctx context.Context, auth CurrentAuth, sessionID int64) (AgentSessionStats, error) {
	if s == nil || s.repository == nil {
		return AgentSessionStats{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_sessions_unavailable", "agent session service is unavailable", "service.agent_session.clear", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentSessionStats{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	stats, err := s.repository.ClearAgentSessionContext(ctx, auth.User.ID, sessionID, s.now().UTC())
	if err != nil {
		return AgentSessionStats{}, err
	}
	return agentSessionStats(stats), nil
}

func (s *AgentSessionService) DeleteSession(ctx context.Context, auth CurrentAuth, sessionID int64) error {
	if s == nil || s.repository == nil {
		return domain.NewAppError(domain.ErrorKindUnavailable, "agent_sessions_unavailable", "agent session service is unavailable", "service.agent_session.delete", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	if sessionID < 1 {
		return fmt.Errorf("%w: session id is required", domain.ErrInvalidInput)
	}
	return s.repository.DeleteAgentSession(ctx, auth.User.ID, sessionID)
}

func (s *AgentSessionService) ListTranscripts(ctx context.Context, auth CurrentAuth, sessionID int64, beforeEntryID int64, limit int) (AgentTranscriptListResult, error) {
	if s == nil || s.repository == nil {
		return AgentTranscriptListResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_sessions_unavailable", "agent session service is unavailable", "service.agent_session.transcripts", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentTranscriptListResult{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	if _, err := s.repository.GetAgentSession(ctx, auth.User.ID, sessionID); err != nil {
		return AgentTranscriptListResult{}, err
	}
	entries, err := s.repository.ListRecentTranscriptEntries(ctx, domain.AgentTranscriptListOptions{
		SessionID:     sessionID,
		UserID:        auth.User.ID,
		BeforeEntryID: beforeEntryID,
		Roles:         []domain.AgentTranscriptRole{domain.AgentTranscriptRoleUser, domain.AgentTranscriptRoleAssistant, domain.AgentTranscriptRoleTool, domain.AgentTranscriptRoleSystem},
		Limit:         limit,
	})
	if err != nil {
		return AgentTranscriptListResult{}, err
	}
	responses := make([]AgentTranscriptEntryResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, AgentTranscriptEntryResponse{
			ID:        entry.ID,
			TurnID:    entry.TurnID,
			Role:      string(entry.Role),
			Content:   entry.Content,
			CreatedAt: formatOptionalTime(&entry.CreatedAt),
		})
	}
	return AgentTranscriptListResult{Entries: responses}, nil
}

func (s *AgentSessionService) ListRunsByTurn(ctx context.Context, auth CurrentAuth, turnID int64) (AgentRunListResult, error) {
	if s == nil || s.repository == nil {
		return AgentRunListResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_runs_unavailable", "agent run service is unavailable", "service.agent_session.runs", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentRunListResult{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	if turnID < 1 {
		return AgentRunListResult{}, fmt.Errorf("%w: turn id is required", domain.ErrInvalidInput)
	}
	runs, err := s.repository.ListAgentRunsByTurn(ctx, auth.User.ID, turnID)
	if err != nil {
		return AgentRunListResult{}, err
	}
	responses := make([]AgentRunResponse, 0, len(runs))
	for _, run := range runs {
		responses = append(responses, agentRunResponse(run, false))
	}
	return AgentRunListResult{Runs: responses}, nil
}

func (s *AgentSessionService) GetRunDetail(ctx context.Context, auth CurrentAuth, runID int64) (AgentRunDetailResult, error) {
	if s == nil || s.repository == nil {
		return AgentRunDetailResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_runs_unavailable", "agent run service is unavailable", "service.agent_session.run_detail", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentRunDetailResult{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	if runID < 1 {
		return AgentRunDetailResult{}, fmt.Errorf("%w: run id is required", domain.ErrInvalidInput)
	}
	run, err := s.repository.GetAgentRunDetail(ctx, auth.User.ID, runID)
	if err != nil {
		return AgentRunDetailResult{}, err
	}
	return AgentRunDetailResult{Run: agentRunResponse(run, true)}, nil
}

func manualAgentSessionKey(account domain.ExternalAccount) string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("%s:%s:%s:manual:%d", account.CorpID, account.AgentID, account.ExternalUserID, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s:%s:%s:manual:%s", account.CorpID, account.AgentID, account.ExternalUserID, hex.EncodeToString(random[:]))
}

func agentExternalAccountResponse(account domain.ExternalAccount) AgentExternalAccountResponse {
	return AgentExternalAccountResponse{
		ID:                   account.ID,
		Provider:             account.Provider,
		CorpID:               account.CorpID,
		AgentID:              account.AgentID,
		ExternalUserID:       account.ExternalUserID,
		DisplayName:          account.DisplayName,
		BindingStatus:        string(account.BindingStatus),
		ActiveAgentSessionID: account.ActiveAgentSessionID,
		UpdatedAt:            formatOptionalTime(&account.UpdatedAt),
	}
}

func agentSessionResponse(session domain.AgentSession, stats domain.AgentSessionStats, active bool) AgentSessionResponse {
	return AgentSessionResponse{
		ID:                       session.ID,
		ExternalAccountID:        session.ExternalAccountID,
		Provider:                 session.Provider,
		ChannelSessionKey:        session.ChannelSessionKey,
		Status:                   string(session.Status),
		Title:                    session.Title,
		ActiveForAccount:         active,
		ContextInitializedAt:     formatOptionalTime(session.ContextInitializedAt),
		ContextRebuildStartedAt:  formatOptionalTime(session.ContextRebuildStartedAt),
		ContextRebuildFinishedAt: formatOptionalTime(session.ContextRebuildFinishedAt),
		ContextVersion:           session.ContextVersion,
		TranscriptCountIndexed:   session.TranscriptCountIndexed,
		Stats:                    agentSessionStats(stats),
		StartedAt:                formatOptionalTime(&session.StartedAt),
		LastActiveAt:             formatOptionalTime(&session.LastActiveAt),
		CreatedAt:                formatOptionalTime(&session.CreatedAt),
		UpdatedAt:                formatOptionalTime(&session.UpdatedAt),
	}
}

func agentSessionStats(stats domain.AgentSessionStats) AgentSessionStats {
	return AgentSessionStats{
		TranscriptCount:   stats.TranscriptCount,
		ArchiveIndexCount: stats.ArchiveIndexCount,
		RecallCount:       stats.RecallCount,
		FirstTranscriptAt: formatOptionalTime(stats.FirstTranscriptAt),
		LastTranscriptAt:  formatOptionalTime(stats.LastTranscriptAt),
	}
}

func agentRunResponse(run domain.AgentRun, includeDetail bool) AgentRunResponse {
	response := AgentRunResponse{
		ID:              run.ID,
		ParentRunID:     run.ParentRunID,
		SessionID:       run.SessionID,
		TurnID:          run.TurnID,
		Role:            string(run.Role),
		Status:          string(run.Status),
		TaskPacket:      run.TaskPacket,
		CapabilityScope: append([]string(nil), run.CapabilityScope...),
		ModelKey:        run.ModelKey,
		ContextBudget:   run.ContextBudget,
		ContextTraceRef: run.ContextTraceRef,
		ResultRef:       run.ResultRef,
		ErrorMessage:    run.ErrorMessage,
		TraceID:         run.TraceID,
		StartedAt:       formatOptionalTime(&run.StartedAt),
		CompletedAt:     formatOptionalTime(run.CompletedAt),
		CreatedAt:       formatOptionalTime(&run.CreatedAt),
		UpdatedAt:       formatOptionalTime(&run.UpdatedAt),
	}
	if !includeDetail {
		return response
	}
	for _, trace := range run.ContextTraces {
		response.ContextTraces = append(response.ContextTraces, AgentRunContextTraceResponse{
			ID:              trace.ID,
			RunID:           trace.RunID,
			TraceKind:       trace.TraceKind,
			PromptVersion:   trace.PromptVersion,
			ModelKey:        trace.ModelKey,
			Content:         trace.Content,
			ContentHash:     trace.ContentHash,
			RedactionStatus: trace.RedactionStatus,
			TokenEstimate:   trace.TokenEstimate,
			CreatedAt:       formatOptionalTime(&trace.CreatedAt),
		})
	}
	for _, observation := range run.Observations {
		response.Observations = append(response.Observations, AgentObservationResponse{
			ID:            observation.ID,
			RunID:         observation.RunID,
			CapabilityKey: observation.CapabilityKey,
			InputSummary:  observation.InputSummary,
			OutputSummary: observation.OutputSummary,
			Status:        observation.Status,
			Error:         observation.Error,
			ArtifactRefs:  append([]string(nil), observation.ArtifactRefs...),
			CreatedAt:     formatOptionalTime(&observation.CreatedAt),
		})
	}
	for _, artifact := range run.Artifacts {
		response.Artifacts = append(response.Artifacts, AgentArtifactResponse{
			ID:           artifact.ID,
			RunID:        artifact.RunID,
			ArtifactType: artifact.ArtifactType,
			ContentRef:   artifact.ContentRef,
			Summary:      artifact.Summary,
			SourceRefs:   append([]string(nil), artifact.SourceRefs...),
			ContentHash:  artifact.ContentHash,
			CreatedAt:    formatOptionalTime(&artifact.CreatedAt),
		})
	}
	for _, child := range run.ChildRuns {
		response.ChildRuns = append(response.ChildRuns, agentRunResponse(child, false))
	}
	return response
}

func formatOptionalTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
