package handler

import (
	"context"
	"encoding/json"
	"messagefeed/internal/service"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAgentProgressRouteRequiresQueryID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	registerAgentSessionRoutes(api, &fakeAgentProgressService{}, nil)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/agent/progress", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestAgentProgressRouteReturnsSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fakeService := &fakeAgentProgressService{
		progress: service.AgentProgressResult{
			Progress: service.AgentProgressSnapshot{
				SubjectType: "plan",
				SubjectID:   9,
				Status:      "executing",
				Summary:     "执行计划",
				Phases: []service.AgentProgressPhaseResponse{
					{Key: "plan", Title: "结构化计划", Status: "executing"},
				},
			},
		},
	}
	router := gin.New()
	api := router.Group("/api/v1")
	registerAgentSessionRoutes(api, fakeService, nil)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/agent/progress?plan_id=9", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if fakeService.query.PlanID != 9 {
		t.Fatalf("query = %#v", fakeService.query)
	}
	var response struct {
		Data service.AgentProgressResult `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Progress.SubjectID != 9 || len(response.Data.Progress.Phases) != 1 {
		t.Fatalf("progress = %#v", response.Data.Progress)
	}
}

func TestAgentProgressStreamRouteReturnsSSESnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fakeService := &fakeAgentProgressService{
		progress: service.AgentProgressResult{
			Progress: service.AgentProgressSnapshot{
				SubjectType: "plan",
				SubjectID:   9,
				Status:      "executing",
				EventCursor: "cursor-1",
			},
		},
	}
	router := gin.New()
	api := router.Group("/api/v1")
	registerAgentSessionRoutes(api, fakeService, nil)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/agent/progress/stream?plan_id=9&once=true", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "text/event-stream" {
		t.Fatalf("content type = %q, want text/event-stream", contentType)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "event: progress") || !strings.Contains(body, `"event_cursor":"cursor-1"`) {
		t.Fatalf("sse body = %q", body)
	}
	if fakeService.query.PlanID != 9 {
		t.Fatalf("query = %#v", fakeService.query)
	}
}

func TestWriteAgentProgressSSESplitsEventFields(t *testing.T) {
	var builder strings.Builder
	if err := writeAgentProgressSSE(&builder, nil, "progress\nbad", "cursor\n1", gin.H{"ok": true}); err != nil {
		t.Fatalf("write sse: %v", err)
	}
	body := builder.String()
	if strings.Contains(body, "progress\nbad") || strings.Contains(body, "cursor\n1") {
		t.Fatalf("sse body contains raw newline: %q", body)
	}
	if !strings.Contains(body, "event: progress bad") || !strings.Contains(body, "id: cursor 1") {
		t.Fatalf("sse body = %q", body)
	}
}

func TestAgentTasksRouteReturnsRecentTasks(t *testing.T) {
	fakeService := &fakeAgentProgressService{
		tasks: service.AgentTaskListResult{Tasks: []service.AgentTaskSummaryResponse{
			{ID: "plan:5", Kind: "plan", PlanID: 5, Status: "completed", ProgressURL: "/agent/plans/5"},
		}},
	}
	router := gin.New()
	api := router.Group("/api/v1")
	registerAgentSessionRoutes(api, fakeService, nil)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/agent/tasks?limit=10", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response struct {
		Data service.AgentTaskListResult `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data.Tasks) != 1 || response.Data.Tasks[0].ProgressURL != "/agent/plans/5" {
		t.Fatalf("tasks = %#v", response.Data.Tasks)
	}
}

func TestAgentScheduledTaskCancelRoute(t *testing.T) {
	fakeService := &fakeAgentProgressService{
		cancelResult: service.CancelAgentScheduledTaskResult{
			Task: service.AgentScheduledTaskResponse{ID: 9, Status: "canceled"},
		},
	}
	router := gin.New()
	api := router.Group("/api/v1")
	registerAgentSessionRoutes(api, fakeService, nil)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/scheduled-tasks/9/cancel", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	if fakeService.cancelTaskID != 9 {
		t.Fatalf("cancel task id = %d, want 9", fakeService.cancelTaskID)
	}
}

func TestAgentCallbackReplayExecuteRoute(t *testing.T) {
	fakeService := &fakeAgentProgressService{
		callbackReplayResult: service.AgentCallbackReplayAPIResult{
			ReplayExecution: service.AgentCallbackReplayExecutionResponse{
				Status:         "ready",
				ApprovalStatus: "approved",
				ExecutionGate:  "approved_and_idempotency_verified",
			},
			AuditEvent: "agent.callback_replay_execute_requested",
		},
	}
	router := gin.New()
	api := router.Group("/api/v1")
	registerAgentSessionRoutes(api, fakeService, nil)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/callback-replay/execute", strings.NewReader(`{"plan_id":9,"callback_key":"approval","replay_entry":"web.agent.callback.replay.approval","approved":true}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if fakeService.callbackReplayInput.PlanID != 9 || fakeService.callbackReplayInput.CallbackKey != "approval" || !fakeService.callbackReplayInput.Approved {
		t.Fatalf("callback replay input = %#v", fakeService.callbackReplayInput)
	}
}

type fakeAgentProgressService struct {
	query                service.AgentProgressQuery
	progress             service.AgentProgressResult
	tasks                service.AgentTaskListResult
	cancelTaskID         int64
	cancelResult         service.CancelAgentScheduledTaskResult
	callbackReplayInput  service.AgentCallbackReplayInput
	callbackReplayResult service.AgentCallbackReplayAPIResult
}

func (f *fakeAgentProgressService) ListSessions(context.Context, service.CurrentAuth) (service.AgentSessionListResult, error) {
	return service.AgentSessionListResult{}, nil
}

func (f *fakeAgentProgressService) CreateSession(context.Context, service.CurrentAuth, int64, string) (service.AgentSessionResponse, error) {
	return service.AgentSessionResponse{}, nil
}

func (f *fakeAgentProgressService) SelectSession(context.Context, service.CurrentAuth, int64) (service.AgentExternalAccountResponse, error) {
	return service.AgentExternalAccountResponse{}, nil
}

func (f *fakeAgentProgressService) RebuildContext(context.Context, service.CurrentAuth, int64) (service.AgentSessionStats, error) {
	return service.AgentSessionStats{}, nil
}

func (f *fakeAgentProgressService) ClearContext(context.Context, service.CurrentAuth, int64) (service.AgentSessionStats, error) {
	return service.AgentSessionStats{}, nil
}

func (f *fakeAgentProgressService) DeleteSession(context.Context, service.CurrentAuth, int64) error {
	return nil
}

func (f *fakeAgentProgressService) ListTranscripts(context.Context, service.CurrentAuth, int64, int64, int) (service.AgentTranscriptListResult, error) {
	return service.AgentTranscriptListResult{}, nil
}

func (f *fakeAgentProgressService) ListRunsByTurn(context.Context, service.CurrentAuth, int64) (service.AgentRunListResult, error) {
	return service.AgentRunListResult{}, nil
}

func (f *fakeAgentProgressService) GetRunDetail(context.Context, service.CurrentAuth, int64) (service.AgentRunDetailResult, error) {
	return service.AgentRunDetailResult{}, nil
}

func (f *fakeAgentProgressService) ListPlans(context.Context, service.CurrentAuth, int64, int64, int) (service.AgentPlanListResult, error) {
	return service.AgentPlanListResult{}, nil
}

func (f *fakeAgentProgressService) GetPlanDetail(context.Context, service.CurrentAuth, int64) (service.AgentPlanDetailResult, error) {
	return service.AgentPlanDetailResult{}, nil
}

func (f *fakeAgentProgressService) ListTasks(context.Context, service.CurrentAuth, int) (service.AgentTaskListResult, error) {
	return f.tasks, nil
}

func (f *fakeAgentProgressService) CancelScheduledTask(_ context.Context, _ service.CurrentAuth, taskID int64) (service.CancelAgentScheduledTaskResult, error) {
	f.cancelTaskID = taskID
	return f.cancelResult, nil
}

func (f *fakeAgentProgressService) GetProgress(_ context.Context, _ service.CurrentAuth, query service.AgentProgressQuery) (service.AgentProgressResult, error) {
	f.query = query
	return f.progress, nil
}

func (f *fakeAgentProgressService) RequestCallbackReplayApproval(_ context.Context, _ service.CurrentAuth, input service.AgentCallbackReplayInput) (service.AgentCallbackReplayAPIResult, error) {
	f.callbackReplayInput = input
	return f.callbackReplayResult, nil
}

func (f *fakeAgentProgressService) ExecuteCallbackReplay(_ context.Context, _ service.CurrentAuth, input service.AgentCallbackReplayInput) (service.AgentCallbackReplayAPIResult, error) {
	f.callbackReplayInput = input
	return f.callbackReplayResult, nil
}
