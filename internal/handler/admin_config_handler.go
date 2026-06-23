package handler

import (
	"context"
	"net/http"
	"time"

	"messagefeed/internal/service"

	"github.com/gin-gonic/gin"
)

const adminConfigTestTimeout = 45 * time.Second

type adminConfigService interface {
	Status(ctx context.Context) (service.AdminConfigStatus, error)
	TestLLM(ctx context.Context, input service.AdminLLMTestInput) (service.AdminLLMTestResult, error)
	TestWeChatWork(ctx context.Context, input service.AdminWeChatWorkTestInput) (service.AdminWeChatWorkTestResult, error)
}

type adminConfigHandler struct {
	service adminConfigService
}

func registerAdminConfigRoutes(router *gin.RouterGroup, adminConfigService adminConfigService) {
	handler := adminConfigHandler{service: adminConfigService}
	router.GET("/admin/config", handler.status)
	router.POST("/admin/config/tests/llm", handler.testLLM)
	router.POST("/admin/config/tests/wechat-work", handler.testWeChatWork)
}

func (h adminConfigHandler) status(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "admin config service unavailable")
		return
	}
	result, err := h.service.Status(c.Request.Context())
	if err != nil {
		RenderError(c, err, "admin config status failed")
		return
	}
	Success(c, result)
}

func (h adminConfigHandler) testLLM(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "admin config service unavailable")
		return
	}
	var input service.AdminLLMTestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), adminConfigTestTimeout)
	defer cancel()
	result, err := h.service.TestLLM(ctx, input)
	if err != nil {
		RenderError(c, err, "llm test failed")
		return
	}
	Success(c, result)
}

func (h adminConfigHandler) testWeChatWork(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "admin config service unavailable")
		return
	}
	var input service.AdminWeChatWorkTestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), adminConfigTestTimeout)
	defer cancel()
	result, err := h.service.TestWeChatWork(ctx, input)
	if err != nil {
		RenderError(c, err, "wechat work test failed")
		return
	}
	Success(c, result)
}
