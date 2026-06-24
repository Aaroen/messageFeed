package handler

import (
	"context"
	"net/http"
	"strconv"

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

func registerAgentSessionRoutes(router *gin.RouterGroup, sessionService agentSessionService, authVerifier authPasswordVerifier) {
	handler := agentSessionHandler{service: sessionService, authVerifier: authVerifier}
	agent := router.Group("/agent")
	agent.GET("/sessions", handler.list)
	agent.POST("/sessions", handler.create)
	agent.GET("/sessions/:id/transcripts", handler.transcripts)
	agent.GET("/turns/:turn_id/runs", handler.turnRuns)
	agent.GET("/runs/:run_id", handler.runDetail)
	agent.POST("/sessions/:id/select", handler.selectSession)
	agent.POST("/sessions/:id/rebuild-context", handler.rebuildContext)
	agent.DELETE("/sessions/:id/context", handler.clearContext)
	agent.DELETE("/sessions/:id", handler.deleteSession)
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

func parseAgentSessionID(c *gin.Context) (int64, bool) {
	sessionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || sessionID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid agent session id")
		return 0, false
	}
	return sessionID, true
}
