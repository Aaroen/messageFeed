package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"messagefeed/internal/service"

	"github.com/gin-gonic/gin"
)

const agentLLMConfigTestTimeout = 2 * time.Minute

type agentLLMConfigService interface {
	ListConfigs(ctx context.Context, auth service.CurrentAuth) (service.AgentLLMProviderConfigListResult, error)
	CreateConfig(ctx context.Context, auth service.CurrentAuth, input service.CreateAgentLLMProviderConfigInput) (service.AgentLLMProviderConfigResponse, error)
	UpdateConfig(ctx context.Context, auth service.CurrentAuth, id int64, input service.UpdateAgentLLMProviderConfigInput) (service.AgentLLMProviderConfigResponse, error)
	SetDefaultConfig(ctx context.Context, auth service.CurrentAuth, id int64) (service.AgentLLMProviderConfigResponse, error)
	TestConfig(ctx context.Context, auth service.CurrentAuth, id int64, input service.TestAgentLLMProviderConfigInput) (service.TestAgentLLMProviderConfigResult, error)
}

type agentLLMConfigHandler struct {
	service agentLLMConfigService
}

func registerAgentLLMConfigRoutes(router *gin.RouterGroup, configService agentLLMConfigService) {
	handler := agentLLMConfigHandler{service: configService}
	agent := router.Group("/agent/llm-provider-configs")
	agent.GET("", handler.list)
	agent.POST("", handler.create)
	agent.PATCH("/:id", handler.update)
	agent.POST("/:id/default", handler.setDefault)
	agent.POST("/:id/test", handler.test)
}

func (h agentLLMConfigHandler) list(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent llm config service unavailable")
		return
	}
	result, err := h.service.ListConfigs(c.Request.Context(), currentAuth(c))
	if err != nil {
		RenderError(c, err, "list agent llm configs failed")
		return
	}
	Success(c, result)
}

func (h agentLLMConfigHandler) create(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent llm config service unavailable")
		return
	}
	var input service.CreateAgentLLMProviderConfigInput
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.CreateConfig(c.Request.Context(), currentAuth(c), input)
	if err != nil {
		RenderError(c, err, "create agent llm config failed")
		return
	}
	Created(c, result)
}

func (h agentLLMConfigHandler) update(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent llm config service unavailable")
		return
	}
	id, ok := parseAgentLLMConfigID(c)
	if !ok {
		return
	}
	var input service.UpdateAgentLLMProviderConfigInput
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.UpdateConfig(c.Request.Context(), currentAuth(c), id, input)
	if err != nil {
		RenderError(c, err, "update agent llm config failed")
		return
	}
	Success(c, result)
}

func (h agentLLMConfigHandler) setDefault(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent llm config service unavailable")
		return
	}
	id, ok := parseAgentLLMConfigID(c)
	if !ok {
		return
	}
	result, err := h.service.SetDefaultConfig(c.Request.Context(), currentAuth(c), id)
	if err != nil {
		RenderError(c, err, "set default agent llm config failed")
		return
	}
	Success(c, result)
}

func (h agentLLMConfigHandler) test(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent llm config service unavailable")
		return
	}
	id, ok := parseAgentLLMConfigID(c)
	if !ok {
		return
	}
	var input service.TestAgentLLMProviderConfigInput
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), agentLLMConfigTestTimeout)
	defer cancel()
	result, err := h.service.TestConfig(ctx, currentAuth(c), id, input)
	if err != nil {
		RenderError(c, err, "test agent llm config failed")
		return
	}
	Success(c, result)
}

func parseAgentLLMConfigID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid agent llm config id")
		return 0, false
	}
	return id, true
}
