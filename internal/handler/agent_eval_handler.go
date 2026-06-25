package handler

import (
	"context"
	"net/http"
	"strconv"

	"messagefeed/internal/domain"
	"messagefeed/internal/service"

	"github.com/gin-gonic/gin"
)

type agentEvalService interface {
	RunBuiltinEval(ctx context.Context, auth service.CurrentAuth, input service.RunBuiltinAgentEvalInput) (service.AgentEvalRunDetailResult, error)
	ListEvalRuns(ctx context.Context, auth service.CurrentAuth, limit int) (service.AgentEvalRunListResult, error)
	GetEvalRunDetail(ctx context.Context, auth service.CurrentAuth, runID int64) (service.AgentEvalRunDetailResult, error)
	RetryPlanStep(ctx context.Context, auth service.CurrentAuth, input service.RetryAgentPlanStepInput) (service.RetryAgentPlanStepResult, error)
	RetryPlan(ctx context.Context, auth service.CurrentAuth, input service.RetryAgentPlanInput) (service.RetryAgentPlanResult, error)
	RecoverPlan(ctx context.Context, auth service.CurrentAuth, input service.RecoverAgentPlanInput) (service.RecoverAgentPlanResult, error)
	RecoverScheduledTask(ctx context.Context, auth service.CurrentAuth, input service.RecoverAgentScheduledTaskInput) (service.RecoverAgentScheduledTaskResult, error)
	GetNotificationPreference(ctx context.Context, auth service.CurrentAuth) (service.AgentNotificationPreferenceResponse, error)
	UpdateNotificationPreference(ctx context.Context, auth service.CurrentAuth, input service.UpdateAgentNotificationPreferenceInput) (service.AgentNotificationPreferenceResponse, error)
}

type agentEvalHandler struct {
	service agentEvalService
}

type createAgentEvalRunRequest struct {
	Trigger  string `json:"trigger"`
	ModelKey string `json:"model_key"`
}

type retryAgentPlanStepRequest struct {
	Reason string `json:"reason"`
}

type agentControlReasonRequest struct {
	Reason string `json:"reason"`
}

type updateAgentNotificationPreferenceRequest struct {
	ProcessNotificationsEnabled  *bool            `json:"process_notifications_enabled"`
	FinalReportsEnabled          *bool            `json:"final_reports_enabled"`
	FailureNotificationsEnabled  *bool            `json:"failure_notifications_enabled"`
	RecoveryNotificationsEnabled *bool            `json:"recovery_notifications_enabled"`
	MaxConcurrentTasks           *int             `json:"max_concurrent_tasks"`
	MaxQueuedTasks               *int             `json:"max_queued_tasks"`
	AutoRecoveryEnabled          *bool            `json:"auto_recovery_enabled"`
	QualityHandoffThreshold      *float64         `json:"quality_handoff_threshold"`
	HandoffOnFailure             *bool            `json:"handoff_on_failure"`
	HandoffOnPermission          *bool            `json:"handoff_on_permission"`
	HandoffOnBudget              *bool            `json:"handoff_on_budget"`
	CapabilityPolicy             domain.AgentJSON `json:"capability_policy"`
	DailyTaskQuota               *int             `json:"daily_task_quota"`
	DailyExternalCallQuota       *int             `json:"daily_external_call_quota"`
	DailyCapabilityCallQuota     *int             `json:"daily_capability_call_quota"`
}

func registerAgentEvalRoutes(router *gin.RouterGroup, evalService agentEvalService) {
	handler := agentEvalHandler{service: evalService}
	agent := router.Group("/agent")
	agent.GET("/eval-runs", handler.listEvalRuns)
	agent.POST("/eval-runs", handler.runBuiltinEval)
	agent.GET("/eval-runs/:id", handler.evalRunDetail)
	agent.GET("/notification-preferences", handler.notificationPreference)
	agent.PATCH("/notification-preferences", handler.updateNotificationPreference)
	agent.POST("/plans/:plan_id/retry", handler.retryPlan)
	agent.POST("/plans/:plan_id/recover", handler.recoverPlan)
	agent.POST("/plans/:plan_id/steps/:step_id/retry", handler.retryPlanStep)
	agent.POST("/scheduled-tasks/:id/recover", handler.recoverScheduledTask)
}

func (h agentEvalHandler) notificationPreference(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent eval service unavailable")
		return
	}
	result, err := h.service.GetNotificationPreference(c.Request.Context(), currentAuth(c))
	if err != nil {
		RenderError(c, err, "load agent notification preference failed")
		return
	}
	Success(c, result)
}

func (h agentEvalHandler) updateNotificationPreference(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent eval service unavailable")
		return
	}
	var request updateAgentNotificationPreferenceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.UpdateNotificationPreference(c.Request.Context(), currentAuth(c), service.UpdateAgentNotificationPreferenceInput{
		ProcessNotificationsEnabled:  request.ProcessNotificationsEnabled,
		FinalReportsEnabled:          request.FinalReportsEnabled,
		FailureNotificationsEnabled:  request.FailureNotificationsEnabled,
		RecoveryNotificationsEnabled: request.RecoveryNotificationsEnabled,
		MaxConcurrentTasks:           request.MaxConcurrentTasks,
		MaxQueuedTasks:               request.MaxQueuedTasks,
		AutoRecoveryEnabled:          request.AutoRecoveryEnabled,
		QualityHandoffThreshold:      request.QualityHandoffThreshold,
		HandoffOnFailure:             request.HandoffOnFailure,
		HandoffOnPermission:          request.HandoffOnPermission,
		HandoffOnBudget:              request.HandoffOnBudget,
		CapabilityPolicy:             request.CapabilityPolicy,
		DailyTaskQuota:               request.DailyTaskQuota,
		DailyExternalCallQuota:       request.DailyExternalCallQuota,
		DailyCapabilityCallQuota:     request.DailyCapabilityCallQuota,
	})
	if err != nil {
		RenderError(c, err, "update agent notification preference failed")
		return
	}
	Success(c, result)
}

func (h agentEvalHandler) listEvalRuns(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent eval service unavailable")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	result, err := h.service.ListEvalRuns(c.Request.Context(), currentAuth(c), limit)
	if err != nil {
		RenderError(c, err, "load agent eval runs failed")
		return
	}
	Success(c, result)
}

func (h agentEvalHandler) runBuiltinEval(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent eval service unavailable")
		return
	}
	var request createAgentEvalRunRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.RunBuiltinEval(c.Request.Context(), currentAuth(c), service.RunBuiltinAgentEvalInput{
		Trigger:  request.Trigger,
		ModelKey: request.ModelKey,
	})
	if err != nil {
		RenderError(c, err, "run agent eval failed")
		return
	}
	Created(c, result)
}

func (h agentEvalHandler) evalRunDetail(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent eval service unavailable")
		return
	}
	runID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || runID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid eval run id")
		return
	}
	result, err := h.service.GetEvalRunDetail(c.Request.Context(), currentAuth(c), runID)
	if err != nil {
		RenderError(c, err, "load agent eval run failed")
		return
	}
	Success(c, result)
}

func (h agentEvalHandler) retryPlanStep(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent eval service unavailable")
		return
	}
	planID, err := strconv.ParseInt(c.Param("plan_id"), 10, 64)
	if err != nil || planID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid agent plan id")
		return
	}
	stepID, err := strconv.ParseInt(c.Param("step_id"), 10, 64)
	if err != nil || stepID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid agent plan step id")
		return
	}
	var request retryAgentPlanStepRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.RetryPlanStep(c.Request.Context(), currentAuth(c), service.RetryAgentPlanStepInput{
		PlanID: planID,
		StepID: stepID,
		Reason: request.Reason,
	})
	if err != nil {
		RenderError(c, err, "retry agent plan step failed")
		return
	}
	Success(c, result)
}

func (h agentEvalHandler) retryPlan(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent eval service unavailable")
		return
	}
	planID, err := strconv.ParseInt(c.Param("plan_id"), 10, 64)
	if err != nil || planID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid agent plan id")
		return
	}
	var request agentControlReasonRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.RetryPlan(c.Request.Context(), currentAuth(c), service.RetryAgentPlanInput{
		PlanID: planID,
		Reason: request.Reason,
	})
	if err != nil {
		RenderError(c, err, "retry agent plan failed")
		return
	}
	Success(c, result)
}

func (h agentEvalHandler) recoverPlan(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent eval service unavailable")
		return
	}
	planID, err := strconv.ParseInt(c.Param("plan_id"), 10, 64)
	if err != nil || planID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid agent plan id")
		return
	}
	var request agentControlReasonRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.RecoverPlan(c.Request.Context(), currentAuth(c), service.RecoverAgentPlanInput{
		PlanID: planID,
		Reason: request.Reason,
	})
	if err != nil {
		RenderError(c, err, "recover agent plan failed")
		return
	}
	Success(c, result)
}

func (h agentEvalHandler) recoverScheduledTask(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent eval service unavailable")
		return
	}
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || taskID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid scheduled task id")
		return
	}
	var request agentControlReasonRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.RecoverScheduledTask(c.Request.Context(), currentAuth(c), service.RecoverAgentScheduledTaskInput{
		TaskID: taskID,
		Reason: request.Reason,
	})
	if err != nil {
		RenderError(c, err, "recover scheduled task failed")
		return
	}
	Success(c, result)
}
