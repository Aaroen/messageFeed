package handler

import (
	"context"
	"net/http"

	"messagefeed/internal/service"

	"github.com/gin-gonic/gin"
)

type agentApprovalService interface {
	Get(ctx context.Context, userID int64, token string) (service.AgentApprovalDetail, error)
	Decide(ctx context.Context, userID int64, token string, input service.AgentApprovalDecisionInput) (service.AgentApprovalDetail, error)
}

type agentApprovalHandler struct {
	service agentApprovalService
}

func registerAgentApprovalRoutes(router *gin.RouterGroup, approvalService agentApprovalService) {
	handler := agentApprovalHandler{service: approvalService}
	router.GET("/agent/approvals/:token", handler.get)
	router.POST("/agent/approvals/:token/approve", handler.approve)
	router.POST("/agent/approvals/:token/reject", handler.reject)
}

func (h agentApprovalHandler) get(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent approval service unavailable")
		return
	}
	result, err := h.service.Get(c.Request.Context(), currentUserID(c), c.Param("token"))
	if err != nil {
		RenderError(c, err, "load approval failed")
		return
	}
	Success(c, result)
}

func (h agentApprovalHandler) approve(c *gin.Context) {
	h.decide(c, "approve")
}

func (h agentApprovalHandler) reject(c *gin.Context) {
	h.decide(c, "reject")
}

func (h agentApprovalHandler) decide(c *gin.Context, decision string) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent approval service unavailable")
		return
	}
	result, err := h.service.Decide(c.Request.Context(), currentUserID(c), c.Param("token"), service.AgentApprovalDecisionInput{Decision: decision})
	if err != nil {
		RenderError(c, err, "update approval failed")
		return
	}
	Success(c, result)
}
