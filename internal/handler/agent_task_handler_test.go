package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"messagefeed/internal/domain"
	"messagefeed/internal/service"
)

func TestAgentTaskRouteRequiresMessage(t *testing.T) {
	router := newTestRouter(t, RouterOptions{
		AuthService:      fakeAuthEndpointService{auth: service.CurrentAuth{Authenticated: true, User: domain.User{ID: 1}}},
		AgentTaskService: &fakeAgentTaskService{},
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/tasks", bytes.NewBufferString(`{"message":"   "}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "messagefeed_session", Value: "token"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestAgentTaskRouteCreatesWebTask(t *testing.T) {
	fakeService := &fakeAgentTaskService{
		result: service.ReceiveWebAgentTaskResult{
			Session:     service.AgentSessionResponse{ID: 3, Provider: domain.AgentProviderWeb},
			Turn:        service.AgentTurnResponse{ID: 4, Status: string(domain.AgentTurnStatusSucceeded)},
			Plan:        service.AgentPlanResponse{ID: 5, Status: string(domain.AgentPlanStatusCompleted)},
			ProgressURL: "/agent/plans/5",
			Reply:       "done",
		},
	}
	router := newTestRouter(t, RouterOptions{
		AuthService: fakeAuthEndpointService{auth: service.CurrentAuth{
			Authenticated: true,
			User:          domain.User{ID: 9},
		}},
		AgentTaskService: fakeService,
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/tasks", bytes.NewBufferString(`{"message":"执行任务","session_id":3,"channel":"web"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "request-1")
	request.AddCookie(&http.Cookie{Name: "messagefeed_session", Value: "token"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status code = %d, want %d, body = %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if fakeService.auth.User.ID != 9 {
		t.Fatalf("auth user id = %d, want 9", fakeService.auth.User.ID)
	}
	if fakeService.input.Message != "执行任务" || fakeService.input.SessionID != 3 || fakeService.input.Channel != "web" {
		t.Fatalf("input = %#v", fakeService.input)
	}
	if fakeService.input.RequestID != "request-1" {
		t.Fatalf("request id = %q, want request-1", fakeService.input.RequestID)
	}
	var response struct {
		Data service.ReceiveWebAgentTaskResult `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Plan.ID != 5 || response.Data.ProgressURL != "/agent/plans/5" {
		t.Fatalf("response data = %#v", response.Data)
	}
}

func TestAgentTaskRouteStopsPlan(t *testing.T) {
	fakeService := &fakeAgentTaskService{
		stopResult: service.StopAgentPlanResult{
			Plan: service.AgentPlanResponse{ID: 8, Status: string(domain.AgentPlanStatusFailed)},
			Runtime: service.AgentPlanStopRuntimeInfo{
				PlanID:        8,
				TurnID:        62,
				StopConfirmed: true,
				Confirmation:  "process_exited_after_cancel",
			},
		},
	}
	router := newTestRouter(t, RouterOptions{
		AuthService: fakeAuthEndpointService{auth: service.CurrentAuth{
			Authenticated: true,
			User:          domain.User{ID: 9},
		}},
		AgentTaskService: fakeService,
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/plans/8/stop", bytes.NewBufferString(`{"reason":"用户停止执行"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "messagefeed_session", Value: "token"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if fakeService.stopPlanID != 8 || fakeService.stopInput.Reason != "用户停止执行" {
		t.Fatalf("stop input = plan %d %#v", fakeService.stopPlanID, fakeService.stopInput)
	}
	var response struct {
		Data service.StopAgentPlanResult `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Plan.ID != 8 || !response.Data.Runtime.StopConfirmed {
		t.Fatalf("response data = %#v", response.Data)
	}
}

type fakeAgentTaskService struct {
	auth       service.CurrentAuth
	input      service.ReceiveWebAgentTaskInput
	result     service.ReceiveWebAgentTaskResult
	err        error
	stopPlanID int64
	stopInput  service.StopAgentPlanInput
	stopResult service.StopAgentPlanResult
	stopErr    error
}

func (f *fakeAgentTaskService) ReceiveWebAgentTask(_ context.Context, auth service.CurrentAuth, input service.ReceiveWebAgentTaskInput) (service.ReceiveWebAgentTaskResult, error) {
	f.auth = auth
	f.input = input
	if f.err != nil {
		return service.ReceiveWebAgentTaskResult{}, f.err
	}
	return f.result, nil
}

func (f *fakeAgentTaskService) StopAgentPlan(_ context.Context, auth service.CurrentAuth, planID int64, input service.StopAgentPlanInput) (service.StopAgentPlanResult, error) {
	f.auth = auth
	f.stopPlanID = planID
	f.stopInput = input
	if f.stopErr != nil {
		return service.StopAgentPlanResult{}, f.stopErr
	}
	return f.stopResult, nil
}
