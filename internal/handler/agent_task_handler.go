package handler

import (
	"context"
	"net/http"
	"strings"

	"messagefeed/internal/observability"
	"messagefeed/internal/service"

	"github.com/gin-gonic/gin"
)

type agentTaskService interface {
	ReceiveWebAgentTask(ctx context.Context, auth service.CurrentAuth, input service.ReceiveWebAgentTaskInput) (service.ReceiveWebAgentTaskResult, error)
}

type agentTaskHandler struct {
	service agentTaskService
}

type createAgentTaskRequest struct {
	Message   string `json:"message"`
	SessionID int64  `json:"session_id"`
	Channel   string `json:"channel"`
}

func registerAgentTaskRoutes(router *gin.RouterGroup, taskService agentTaskService) {
	handler := agentTaskHandler{service: taskService}
	agent := router.Group("/agent")
	agent.POST("/tasks", handler.create)
}

func (h agentTaskHandler) create(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent task service unavailable")
		return
	}
	var request createAgentTaskRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(request.Message) == "" {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "message is required")
		return
	}
	result, err := h.service.ReceiveWebAgentTask(c.Request.Context(), currentAuth(c), service.ReceiveWebAgentTaskInput{
		Message:   request.Message,
		SessionID: request.SessionID,
		Channel:   request.Channel,
		RequestID: requestID(c),
		TraceID:   observability.TraceID(c.Request.Context()),
	})
	if err != nil {
		RenderError(c, err, "create agent task failed")
		return
	}
	Created(c, result)
}
