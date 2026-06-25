package handler

import (
	"context"
	"net/http"
	"strconv"

	"messagefeed/internal/service"

	"github.com/gin-gonic/gin"
)

type agentApprovalService interface {
	Get(ctx context.Context, userID int64, token string) (service.AgentApprovalDetail, error)
	Decide(ctx context.Context, userID int64, token string, input service.AgentApprovalDecisionInput) (service.AgentApprovalDetail, error)
	DecideByID(ctx context.Context, userID int64, approvalID int64, input service.AgentApprovalDecisionInput) (service.AgentApprovalDetail, error)
}

type agentApprovalHandler struct {
	service agentApprovalService
}

func registerAgentApprovalRoutes(router *gin.RouterGroup, approvalService agentApprovalService) {
	handler := agentApprovalHandler{service: approvalService}
	router.GET("/agent/approvals/:token", handler.get)
	router.POST("/agent/approvals/:token/approve", handler.approve)
	router.POST("/agent/approvals/:token/reject", handler.reject)
	router.POST("/agent/approval-records/:id/approve", handler.approveByID)
	router.POST("/agent/approval-records/:id/reject", handler.rejectByID)
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

func (h agentApprovalHandler) approveByID(c *gin.Context) {
	h.decideByID(c, "approve")
}

func (h agentApprovalHandler) rejectByID(c *gin.Context) {
	h.decideByID(c, "reject")
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

func (h agentApprovalHandler) decideByID(c *gin.Context, decision string) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent approval service unavailable")
		return
	}
	approvalID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || approvalID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid approval id")
		return
	}
	result, err := h.service.DecideByID(c.Request.Context(), currentUserID(c), approvalID, service.AgentApprovalDecisionInput{Decision: decision})
	if err != nil {
		RenderError(c, err, "update approval failed")
		return
	}
	Success(c, result)
}
