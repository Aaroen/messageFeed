package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"messagefeed/internal/service"

	"github.com/gin-gonic/gin"
)

type agentSessionService interface {
	ListSessions(ctx context.Context, auth service.CurrentAuth) (service.AgentSessionListResult, error)
	CreateSession(ctx context.Context, auth service.CurrentAuth, externalAccountID int64, title string) (service.AgentSessionResponse, error)
	SelectSession(ctx context.Context, auth service.CurrentAuth, sessionID int64) (service.AgentExternalAccountResponse, error)
	RebuildContext(ctx context.Context, auth service.CurrentAuth, sessionID int64) (service.AgentSessionStats, error)
	ClearContext(ctx context.Context, auth service.CurrentAuth, sessionID int64) (service.AgentSessionStats, error)
	DeleteSession(ctx context.Context, auth service.CurrentAuth, sessionID int64) error
	ListTranscripts(ctx context.Context, auth service.CurrentAuth, sessionID int64, beforeEntryID int64, limit int) (service.AgentTranscriptListResult, error)
	ListRunsByTurn(ctx context.Context, auth service.CurrentAuth, turnID int64) (service.AgentRunListResult, error)
	GetRunDetail(ctx context.Context, auth service.CurrentAuth, runID int64) (service.AgentRunDetailResult, error)
	ListPlans(ctx context.Context, auth service.CurrentAuth, sessionID int64, turnID int64, limit int) (service.AgentPlanListResult, error)
	GetPlanDetail(ctx context.Context, auth service.CurrentAuth, planID int64) (service.AgentPlanDetailResult, error)
	ListTasks(ctx context.Context, auth service.CurrentAuth, limit int) (service.AgentTaskListResult, error)
	CancelScheduledTask(ctx context.Context, auth service.CurrentAuth, taskID int64) (service.CancelAgentScheduledTaskResult, error)
	GetProgress(ctx context.Context, auth service.CurrentAuth, query service.AgentProgressQuery) (service.AgentProgressResult, error)
	RequestCallbackReplayApproval(ctx context.Context, auth service.CurrentAuth, input service.AgentCallbackReplayInput) (service.AgentCallbackReplayAPIResult, error)
	ExecuteCallbackReplay(ctx context.Context, auth service.CurrentAuth, input service.AgentCallbackReplayInput) (service.AgentCallbackReplayAPIResult, error)
	ListMemoryCandidates(ctx context.Context, auth service.CurrentAuth, status string, limit int) (service.AgentMemoryCandidateListResult, error)
	ApplyMemoryCandidate(ctx context.Context, auth service.CurrentAuth, candidateID int64) (service.AgentMemoryCandidateDecisionResult, error)
	RejectMemoryCandidate(ctx context.Context, auth service.CurrentAuth, candidateID int64, input service.AgentMemoryCandidateDecisionInput) (service.AgentMemoryCandidateDecisionResult, error)
	RevokeMemoryCandidate(ctx context.Context, auth service.CurrentAuth, candidateID int64, input service.AgentMemoryCandidateDecisionInput) (service.AgentMemoryCandidateDecisionResult, error)
	RunFactIndexBackfill(ctx context.Context, auth service.CurrentAuth, input service.AgentFactIndexBackfillInput) (service.AgentFactIndexBackfillResult, error)
	GetFactIndexStats(ctx context.Context, auth service.CurrentAuth) (service.AgentFactIndexStatsResult, error)
	PreviewFactRecall(ctx context.Context, auth service.CurrentAuth, input service.AgentFactRecallPreviewInput) (service.AgentFactRecallPreviewResult, error)
}

type authPasswordVerifier interface {
	VerifyCurrentPassword(ctx context.Context, auth service.CurrentAuth, currentPassword string) error
}

type agentSessionHandler struct {
	service      agentSessionService
	authVerifier authPasswordVerifier
}

type createAgentSessionRequest struct {
	ExternalAccountID int64  `json:"external_account_id"`
	Title             string `json:"title"`
}

type deleteAgentSessionRequest struct {
	CurrentPassword string `json:"current_password"`
}

type agentCallbackReplayRequest struct {
	PlanID      int64  `json:"plan_id"`
	CallbackKey string `json:"callback_key"`
	ReplayEntry string `json:"replay_entry"`
	Reason      string `json:"reason"`
	Approved    bool   `json:"approved"`
}

type agentMemoryCandidateDecisionRequest struct {
	Reason string `json:"reason"`
}

type agentFactIndexBackfillRequest struct {
	Limit int `json:"limit"`
}

type agentFactRecallPreviewRequest struct {
	Mode         string   `json:"mode"`
	Query        string   `json:"query"`
	SessionID    int64    `json:"session_id"`
	TurnID       int64    `json:"turn_id"`
	FactTypes    []string `json:"fact_types"`
	MemoryKinds  []string `json:"memory_kinds"`
	Limit        int      `json:"limit"`
	MaxRiskLevel string   `json:"max_risk_level"`
}

func registerAgentSessionRoutes(router *gin.RouterGroup, sessionService agentSessionService, authVerifier authPasswordVerifier) {
	handler := agentSessionHandler{service: sessionService, authVerifier: authVerifier}
	agent := router.Group("/agent")
	agent.GET("/sessions", handler.list)
	agent.POST("/sessions", handler.create)
	agent.GET("/sessions/:id/transcripts", handler.transcripts)
	agent.GET("/turns/:turn_id/runs", handler.turnRuns)
	agent.GET("/runs/:run_id", handler.runDetail)
	agent.GET("/tasks", handler.tasks)
	agent.POST("/scheduled-tasks/:id/cancel", handler.cancelScheduledTask)
	agent.GET("/plans", handler.plans)
	agent.GET("/plans/:plan_id", handler.planDetail)
	agent.GET("/progress", handler.progress)
	agent.GET("/progress/stream", handler.progressStream)
	agent.GET("/memory-candidates", handler.memoryCandidates)
	agent.POST("/memory-candidates/:candidate_id/apply", handler.applyMemoryCandidate)
	agent.POST("/memory-candidates/:candidate_id/reject", handler.rejectMemoryCandidate)
	agent.POST("/memory-candidates/:candidate_id/revoke", handler.revokeMemoryCandidate)
	agent.GET("/fact-index/stats", handler.factIndexStats)
	agent.POST("/fact-index/backfill", handler.factIndexBackfill)
	agent.POST("/fact-recall/preview", handler.factRecallPreview)
	agent.POST("/callback-replay/requests", handler.requestCallbackReplay)
	agent.POST("/callback-replay/execute", handler.executeCallbackReplay)
	agent.POST("/sessions/:id/select", handler.selectSession)
	agent.POST("/sessions/:id/rebuild-context", handler.rebuildContext)
	agent.DELETE("/sessions/:id/context", handler.clearContext)
	agent.DELETE("/sessions/:id", handler.deleteSession)
}

func (h agentSessionHandler) factIndexStats(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	result, err := h.service.GetFactIndexStats(c.Request.Context(), currentAuth(c))
	if err != nil {
		RenderError(c, err, "load fact index stats failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) factIndexBackfill(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	var request agentFactIndexBackfillRequest
	if err := c.ShouldBindJSON(&request); err != nil && err != io.EOF {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.RunFactIndexBackfill(c.Request.Context(), currentAuth(c), service.AgentFactIndexBackfillInput{
		Limit: request.Limit,
	})
	if err != nil {
		RenderError(c, err, "run fact index backfill failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) factRecallPreview(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	var request agentFactRecallPreviewRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.PreviewFactRecall(c.Request.Context(), currentAuth(c), service.AgentFactRecallPreviewInput{
		Mode:         request.Mode,
		Query:        request.Query,
		SessionID:    request.SessionID,
		TurnID:       request.TurnID,
		FactTypes:    request.FactTypes,
		MemoryKinds:  request.MemoryKinds,
		Limit:        request.Limit,
		MaxRiskLevel: request.MaxRiskLevel,
	})
	if err != nil {
		RenderError(c, err, "preview fact recall failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) requestCallbackReplay(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	var request agentCallbackReplayRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.RequestCallbackReplayApproval(c.Request.Context(), currentAuth(c), service.AgentCallbackReplayInput{
		PlanID:      request.PlanID,
		CallbackKey: request.CallbackKey,
		ReplayEntry: request.ReplayEntry,
		Reason:      request.Reason,
	})
	if err != nil {
		RenderError(c, err, "request callback replay approval failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) executeCallbackReplay(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	var request agentCallbackReplayRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.ExecuteCallbackReplay(c.Request.Context(), currentAuth(c), service.AgentCallbackReplayInput{
		PlanID:      request.PlanID,
		CallbackKey: request.CallbackKey,
		ReplayEntry: request.ReplayEntry,
		Reason:      request.Reason,
		Approved:    request.Approved,
	})
	if err != nil {
		RenderError(c, err, "execute callback replay failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) tasks(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	result, err := h.service.ListTasks(c.Request.Context(), currentAuth(c), limit)
	if err != nil {
		RenderError(c, err, "load agent tasks failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) cancelScheduledTask(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || taskID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid scheduled task id")
		return
	}
	result, err := h.service.CancelScheduledTask(c.Request.Context(), currentAuth(c), taskID)
	if err != nil {
		RenderError(c, err, "cancel scheduled task failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) progress(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	query := parseAgentProgressQuery(c)
	if query.PlanID < 1 && query.TurnID < 1 && query.RunID < 1 && query.ScheduledTaskID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "one progress query id is required")
		return
	}
	result, err := h.service.GetProgress(c.Request.Context(), currentAuth(c), query)
	if err != nil {
		RenderError(c, err, "load agent progress failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) progressStream(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	query := parseAgentProgressQuery(c)
	if query.PlanID < 1 && query.TurnID < 1 && query.RunID < 1 && query.ScheduledTaskID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "one progress query id is required")
		return
	}
	initial, err := h.service.GetProgress(c.Request.Context(), currentAuth(c), query)
	if err != nil {
		RenderError(c, err, "load agent progress failed")
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher, _ := c.Writer.(http.Flusher)
	lastCursor := initial.Progress.EventCursor
	_ = writeAgentProgressSSE(c.Writer, flusher, "progress", lastCursor, initial)
	if strings.EqualFold(c.Query("once"), "true") {
		return
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			result, err := h.service.GetProgress(c.Request.Context(), currentAuth(c), query)
			if err != nil {
				_ = writeAgentProgressSSE(c.Writer, flusher, "error", "", gin.H{"message": err.Error()})
				continue
			}
			if result.Progress.EventCursor == lastCursor {
				_ = writeAgentProgressSSE(c.Writer, flusher, "heartbeat", result.Progress.EventCursor, gin.H{"event_cursor": result.Progress.EventCursor})
				continue
			}
			lastCursor = result.Progress.EventCursor
			_ = writeAgentProgressSSE(c.Writer, flusher, "progress", lastCursor, result)
		}
	}
}

func (h agentSessionHandler) plans(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	sessionID, _ := strconv.ParseInt(c.DefaultQuery("session_id", "0"), 10, 64)
	turnID, _ := strconv.ParseInt(c.DefaultQuery("turn_id", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	result, err := h.service.ListPlans(c.Request.Context(), currentAuth(c), sessionID, turnID, limit)
	if err != nil {
		RenderError(c, err, "load agent plans failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) planDetail(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	planID, err := strconv.ParseInt(c.Param("plan_id"), 10, 64)
	if err != nil || planID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid agent plan id")
		return
	}
	result, err := h.service.GetPlanDetail(c.Request.Context(), currentAuth(c), planID)
	if err != nil {
		RenderError(c, err, "load agent plan failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) turnRuns(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	turnID, err := strconv.ParseInt(c.Param("turn_id"), 10, 64)
	if err != nil || turnID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid agent turn id")
		return
	}
	result, err := h.service.ListRunsByTurn(c.Request.Context(), currentAuth(c), turnID)
	if err != nil {
		RenderError(c, err, "load agent runs failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) runDetail(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	runID, err := strconv.ParseInt(c.Param("run_id"), 10, 64)
	if err != nil || runID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid agent run id")
		return
	}
	result, err := h.service.GetRunDetail(c.Request.Context(), currentAuth(c), runID)
	if err != nil {
		RenderError(c, err, "load agent run failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) list(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	result, err := h.service.ListSessions(c.Request.Context(), currentAuth(c))
	if err != nil {
		RenderError(c, err, "load agent sessions failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) create(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	var request createAgentSessionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.CreateSession(c.Request.Context(), currentAuth(c), request.ExternalAccountID, request.Title)
	if err != nil {
		RenderError(c, err, "create agent session failed")
		return
	}
	Created(c, result)
}

func (h agentSessionHandler) selectSession(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	sessionID, ok := parseAgentSessionID(c)
	if !ok {
		return
	}
	result, err := h.service.SelectSession(c.Request.Context(), currentAuth(c), sessionID)
	if err != nil {
		RenderError(c, err, "select agent session failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) rebuildContext(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	sessionID, ok := parseAgentSessionID(c)
	if !ok {
		return
	}
	result, err := h.service.RebuildContext(c.Request.Context(), currentAuth(c), sessionID)
	if err != nil {
		RenderError(c, err, "rebuild agent session context failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) clearContext(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	sessionID, ok := parseAgentSessionID(c)
	if !ok {
		return
	}
	result, err := h.service.ClearContext(c.Request.Context(), currentAuth(c), sessionID)
	if err != nil {
		RenderError(c, err, "clear agent session context failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) deleteSession(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	if h.authVerifier == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	sessionID, ok := parseAgentSessionID(c)
	if !ok {
		return
	}
	var request deleteAgentSessionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.authVerifier.VerifyCurrentPassword(c.Request.Context(), currentAuth(c), request.CurrentPassword); err != nil {
		RenderError(c, err, "verify current password failed")
		return
	}
	if err := h.service.DeleteSession(c.Request.Context(), currentAuth(c), sessionID); err != nil {
		RenderError(c, err, "delete agent session failed")
		return
	}
	Success(c, gin.H{"deleted": true})
}

func (h agentSessionHandler) transcripts(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	sessionID, ok := parseAgentSessionID(c)
	if !ok {
		return
	}
	beforeEntryID, _ := strconv.ParseInt(c.DefaultQuery("before_entry_id", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	result, err := h.service.ListTranscripts(c.Request.Context(), currentAuth(c), sessionID, beforeEntryID, limit)
	if err != nil {
		RenderError(c, err, "load agent transcripts failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) memoryCandidates(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	result, err := h.service.ListMemoryCandidates(c.Request.Context(), currentAuth(c), c.Query("status"), limit)
	if err != nil {
		RenderError(c, err, "load memory candidates failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) applyMemoryCandidate(c *gin.Context) {
	candidateID, ok := parseAgentMemoryCandidateID(c)
	if !ok {
		return
	}
	result, err := h.service.ApplyMemoryCandidate(c.Request.Context(), currentAuth(c), candidateID)
	if err != nil {
		RenderError(c, err, "apply memory candidate failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) rejectMemoryCandidate(c *gin.Context) {
	h.decideMemoryCandidate(c, "reject")
}

func (h agentSessionHandler) revokeMemoryCandidate(c *gin.Context) {
	h.decideMemoryCandidate(c, "revoke")
}

func (h agentSessionHandler) decideMemoryCandidate(c *gin.Context, decision string) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent session service unavailable")
		return
	}
	candidateID, ok := parseAgentMemoryCandidateID(c)
	if !ok {
		return
	}
	var request agentMemoryCandidateDecisionRequest
	if c.Request.Body != nil {
		_ = c.ShouldBindJSON(&request)
	}
	input := service.AgentMemoryCandidateDecisionInput{Reason: request.Reason}
	var (
		result service.AgentMemoryCandidateDecisionResult
		err    error
	)
	if decision == "revoke" {
		result, err = h.service.RevokeMemoryCandidate(c.Request.Context(), currentAuth(c), candidateID, input)
	} else {
		result, err = h.service.RejectMemoryCandidate(c.Request.Context(), currentAuth(c), candidateID, input)
	}
	if err != nil {
		RenderError(c, err, "update memory candidate failed")
		return
	}
	Success(c, result)
}

func parseInt64Query(c *gin.Context, key string) int64 {
	value := c.Query(key)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 1 {
		return 0
	}
	return parsed
}

func parseAgentProgressQuery(c *gin.Context) service.AgentProgressQuery {
	return service.AgentProgressQuery{
		PlanID:          parseInt64Query(c, "plan_id"),
		TurnID:          parseInt64Query(c, "turn_id"),
		RunID:           parseInt64Query(c, "run_id"),
		ScheduledTaskID: parseInt64Query(c, "scheduled_task_id"),
	}
}

func writeAgentProgressSSE(writer io.Writer, flusher http.Flusher, event string, id string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	event = strings.TrimSpace(event)
	if event == "" {
		event = "message"
	}
	if id != "" {
		if _, err := fmt.Fprintf(writer, "id: %s\n", sanitizeSSELine(id)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(writer, "event: %s\n", sanitizeSSELine(event)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "data: %s\n\n", payload); err != nil {
		return err
	}
	if flusher != nil {
		flusher.Flush()
	}
	return nil
}

func sanitizeSSELine(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

func parseAgentSessionID(c *gin.Context) (int64, bool) {
	sessionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || sessionID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid agent session id")
		return 0, false
	}
	return sessionID, true
}

func parseAgentMemoryCandidateID(c *gin.Context) (int64, bool) {
	candidateID, err := strconv.ParseInt(c.Param("candidate_id"), 10, 64)
	if err != nil || candidateID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid memory candidate id")
		return 0, false
	}
	return candidateID, true
}
