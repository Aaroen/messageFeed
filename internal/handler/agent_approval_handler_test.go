package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"messagefeed/internal/service"

	"github.com/gin-gonic/gin"
)

func TestAgentApprovalRouteDecidesByID(t *testing.T) {
	fakeService := &fakeAgentApprovalDecisionService{
		result: service.AgentApprovalDetail{ID: 7, Status: "approved", Channel: "web"},
	}
	router := gin.New()
	api := router.Group("/api/v1")
	registerAgentApprovalRoutes(api, fakeService)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/approval-records/7/approve", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	if fakeService.approvalID != 7 || fakeService.decision != "approve" {
		t.Fatalf("decision = id %d / %q", fakeService.approvalID, fakeService.decision)
	}
}

type fakeAgentApprovalDecisionService struct {
	approvalID int64
	decision   string
	result     service.AgentApprovalDetail
}

func (f *fakeAgentApprovalDecisionService) Get(context.Context, int64, string) (service.AgentApprovalDetail, error) {
	return f.result, nil
}

func (f *fakeAgentApprovalDecisionService) Decide(_ context.Context, _ int64, _ string, input service.AgentApprovalDecisionInput) (service.AgentApprovalDetail, error) {
	f.decision = input.Decision
	return f.result, nil
}

func (f *fakeAgentApprovalDecisionService) DecideByID(_ context.Context, _ int64, approvalID int64, input service.AgentApprovalDecisionInput) (service.AgentApprovalDetail, error) {
	f.approvalID = approvalID
	f.decision = input.Decision
	return f.result, nil
}
