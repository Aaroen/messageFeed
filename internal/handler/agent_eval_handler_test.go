package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"messagefeed/internal/service"

	"github.com/gin-gonic/gin"
)

func TestAgentEvalRunRouteCreatesBuiltinRun(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fakeService := &fakeAgentEvalService{
		runDetail: service.AgentEvalRunDetailResult{
			Run: service.AgentEvalRunResponse{ID: 3, Status: "completed", CaseCount: 10, PassedCount: 10},
		},
	}
	router := gin.New()
	api := router.Group("/api/v1")
	registerAgentEvalRoutes(api, fakeService)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/eval-runs", strings.NewReader(`{"trigger":"test"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
	if fakeService.runInput.Trigger != "test" {
		t.Fatalf("input = %#v", fakeService.runInput)
	}
	var response struct {
		Data service.AgentEvalRunDetailResult `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Run.ID != 3 || response.Data.Run.PassedCount != 10 {
		t.Fatalf("run = %#v", response.Data.Run)
	}
}

func TestAgentPlanStepRetryRoute(t *testing.T) {
	fakeService := &fakeAgentEvalService{
		retryResult: service.RetryAgentPlanStepResult{
			PlanID: 5,
			Step:   service.AgentPlanStepResponse{ID: 7, Status: "approved", RetryCount: 1},
		},
	}
	router := gin.New()
	api := router.Group("/api/v1")
	registerAgentEvalRoutes(api, fakeService)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/plans/5/steps/7/retry", strings.NewReader(`{"reason":"temporary"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if fakeService.retryInput.PlanID != 5 || fakeService.retryInput.StepID != 7 || fakeService.retryInput.Reason != "temporary" {
		t.Fatalf("retry input = %#v", fakeService.retryInput)
	}
}

func TestAgentPlanRetryRoute(t *testing.T) {
	fakeService := &fakeAgentEvalService{
		planRetryResult: service.RetryAgentPlanResult{PlanID: 5, Queued: 2},
	}
	router := gin.New()
	api := router.Group("/api/v1")
	registerAgentEvalRoutes(api, fakeService)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/plans/5/retry", strings.NewReader(`{"reason":"batch"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if fakeService.planRetryInput.PlanID != 5 || fakeService.planRetryInput.Reason != "batch" {
		t.Fatalf("retry input = %#v", fakeService.planRetryInput)
	}
}

func TestAgentPlanRecoverRoute(t *testing.T) {
	fakeService := &fakeAgentEvalService{
		planRecoverResult: service.RecoverAgentPlanResult{Plan: service.AgentPlanResponse{ID: 5, Status: "executing"}},
	}
	router := gin.New()
	api := router.Group("/api/v1")
	registerAgentEvalRoutes(api, fakeService)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/plans/5/recover", strings.NewReader(`{"reason":"worker restart"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if fakeService.planRecoverInput.PlanID != 5 || fakeService.planRecoverInput.Reason != "worker restart" {
		t.Fatalf("recover input = %#v", fakeService.planRecoverInput)
	}
}

func TestAgentScheduledTaskRecoverRoute(t *testing.T) {
	fakeService := &fakeAgentEvalService{
		taskRecoverResult: service.RecoverAgentScheduledTaskResult{Task: service.AgentScheduledTaskResponse{ID: 9, Status: "queued"}},
	}
	router := gin.New()
	api := router.Group("/api/v1")
	registerAgentEvalRoutes(api, fakeService)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/scheduled-tasks/9/recover", strings.NewReader(`{"reason":"lock expired"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if fakeService.taskRecoverInput.TaskID != 9 || fakeService.taskRecoverInput.Reason != "lock expired" {
		t.Fatalf("recover input = %#v", fakeService.taskRecoverInput)
	}
}

func TestAgentNotificationPreferenceRoutes(t *testing.T) {
	enabled := true
	disabled := false
	fakeService := &fakeAgentEvalService{
		preferenceResult: service.AgentNotificationPreferenceResponse{
			ProcessNotificationsEnabled: true,
			FinalReportsEnabled:         false,
		},
	}
	router := gin.New()
	api := router.Group("/api/v1")
	registerAgentEvalRoutes(api, fakeService)

	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/agent/notification-preferences", nil)
	getRecorder := httptest.NewRecorder()
	router.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", getRecorder.Code, http.StatusOK)
	}

	patchRequest := httptest.NewRequest(http.MethodPatch, "/api/v1/agent/notification-preferences", strings.NewReader(`{"process_notifications_enabled":true,"final_reports_enabled":false}`))
	patchRequest.Header.Set("Content-Type", "application/json")
	patchRecorder := httptest.NewRecorder()
	router.ServeHTTP(patchRecorder, patchRequest)
	if patchRecorder.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want %d", patchRecorder.Code, http.StatusOK)
	}
	if fakeService.preferenceInput.ProcessNotificationsEnabled == nil || *fakeService.preferenceInput.ProcessNotificationsEnabled != enabled {
		t.Fatalf("process preference = %#v", fakeService.preferenceInput.ProcessNotificationsEnabled)
	}
	if fakeService.preferenceInput.FinalReportsEnabled == nil || *fakeService.preferenceInput.FinalReportsEnabled != disabled {
		t.Fatalf("final preference = %#v", fakeService.preferenceInput.FinalReportsEnabled)
	}
}

type fakeAgentEvalService struct {
	runInput          service.RunBuiltinAgentEvalInput
	runDetail         service.AgentEvalRunDetailResult
	listResult        service.AgentEvalRunListResult
	detailID          int64
	retryInput        service.RetryAgentPlanStepInput
	retryResult       service.RetryAgentPlanStepResult
	planRetryInput    service.RetryAgentPlanInput
	planRetryResult   service.RetryAgentPlanResult
	planRecoverInput  service.RecoverAgentPlanInput
	planRecoverResult service.RecoverAgentPlanResult
	taskRecoverInput  service.RecoverAgentScheduledTaskInput
	taskRecoverResult service.RecoverAgentScheduledTaskResult
	preferenceInput   service.UpdateAgentNotificationPreferenceInput
	preferenceResult  service.AgentNotificationPreferenceResponse
}

func (f *fakeAgentEvalService) RunBuiltinEval(_ context.Context, _ service.CurrentAuth, input service.RunBuiltinAgentEvalInput) (service.AgentEvalRunDetailResult, error) {
	f.runInput = input
	return f.runDetail, nil
}

func (f *fakeAgentEvalService) ListEvalRuns(context.Context, service.CurrentAuth, int) (service.AgentEvalRunListResult, error) {
	return f.listResult, nil
}

func (f *fakeAgentEvalService) GetEvalRunDetail(_ context.Context, _ service.CurrentAuth, runID int64) (service.AgentEvalRunDetailResult, error) {
	f.detailID = runID
	return f.runDetail, nil
}

func (f *fakeAgentEvalService) RetryPlanStep(_ context.Context, _ service.CurrentAuth, input service.RetryAgentPlanStepInput) (service.RetryAgentPlanStepResult, error) {
	f.retryInput = input
	return f.retryResult, nil
}

func (f *fakeAgentEvalService) RetryPlan(_ context.Context, _ service.CurrentAuth, input service.RetryAgentPlanInput) (service.RetryAgentPlanResult, error) {
	f.planRetryInput = input
	return f.planRetryResult, nil
}

func (f *fakeAgentEvalService) RecoverPlan(_ context.Context, _ service.CurrentAuth, input service.RecoverAgentPlanInput) (service.RecoverAgentPlanResult, error) {
	f.planRecoverInput = input
	return f.planRecoverResult, nil
}

func (f *fakeAgentEvalService) RecoverScheduledTask(_ context.Context, _ service.CurrentAuth, input service.RecoverAgentScheduledTaskInput) (service.RecoverAgentScheduledTaskResult, error) {
	f.taskRecoverInput = input
	return f.taskRecoverResult, nil
}

func (f *fakeAgentEvalService) GetNotificationPreference(context.Context, service.CurrentAuth) (service.AgentNotificationPreferenceResponse, error) {
	return f.preferenceResult, nil
}

func (f *fakeAgentEvalService) UpdateNotificationPreference(_ context.Context, _ service.CurrentAuth, input service.UpdateAgentNotificationPreferenceInput) (service.AgentNotificationPreferenceResponse, error) {
	f.preferenceInput = input
	return f.preferenceResult, nil
}
