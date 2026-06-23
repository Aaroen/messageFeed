package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"messagefeed/internal/domain"
	"messagefeed/internal/service"
)

func TestAdminConfigStatusRoute(t *testing.T) {
	fakeService := &fakeAdminConfigService{
		status: service.AdminConfigStatus{
			UpdatedAt: time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC),
			WeChatWork: service.AdminWeChatWorkConfigStatus{
				Enabled:            true,
				CallbackConfigured: true,
				SenderConfigured:   true,
				AgentID:            "1000002",
				CallbackURL:        "https://example.test/api/v1/channels/wechat-work/app/callback",
			},
			LLM: service.AdminLLMConfigStatus{
				Enabled:     true,
				ClientReady: true,
				Provider:    "hyb",
				Model:       "model-a",
			},
		},
	}
	router := newTestRouter(t, RouterOptions{AdminConfigService: fakeService})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/config", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response struct {
		Code int                       `json:"code"`
		Data service.AdminConfigStatus `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Data.WeChatWork.CallbackConfigured {
		t.Fatalf("wechat work status = %#v", response.Data.WeChatWork)
	}
	if response.Data.LLM.Provider != "hyb" {
		t.Fatalf("llm provider = %q", response.Data.LLM.Provider)
	}
}

func TestAdminConfigTestRoutes(t *testing.T) {
	fakeService := &fakeAdminConfigService{
		llmResult: service.AdminLLMTestResult{
			Status:       "succeeded",
			Provider:     "hyb",
			Model:        "model-a",
			ResponseText: "OK",
		},
		weChatWorkResult: service.AdminWeChatWorkTestResult{
			Status:    "succeeded",
			MessageID: "wx-msg-1",
		},
	}
	router := newTestRouter(t, RouterOptions{AdminConfigService: fakeService})

	llmRequest := httptest.NewRequest(http.MethodPost, "/api/v1/admin/config/tests/llm", strings.NewReader(`{"message":"ping"}`))
	llmRequest.Header.Set("Content-Type", "application/json")
	llmRecorder := httptest.NewRecorder()
	router.ServeHTTP(llmRecorder, llmRequest)

	if llmRecorder.Code != http.StatusOK {
		t.Fatalf("llm status code = %d, want %d", llmRecorder.Code, http.StatusOK)
	}
	if fakeService.llmInput.Message != "ping" {
		t.Fatalf("llm input message = %q", fakeService.llmInput.Message)
	}

	wechatRequest := httptest.NewRequest(http.MethodPost, "/api/v1/admin/config/tests/wechat-work", strings.NewReader(`{"to_user":"zhangsan"}`))
	wechatRequest.Header.Set("Content-Type", "application/json")
	wechatRecorder := httptest.NewRecorder()
	router.ServeHTTP(wechatRecorder, wechatRequest)

	if wechatRecorder.Code != http.StatusOK {
		t.Fatalf("wechat status code = %d, want %d", wechatRecorder.Code, http.StatusOK)
	}
	if fakeService.weChatWorkInput.ToUser != "zhangsan" {
		t.Fatalf("wechat input to_user = %q", fakeService.weChatWorkInput.ToUser)
	}
}

func TestAdminConfigRoutesRequireConfiguredService(t *testing.T) {
	router := newTestRouter(t, RouterOptions{})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/config", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestAdminConfigRoutesRequireOwner(t *testing.T) {
	router := newTestRouter(t, RouterOptions{
		AdminConfigService: &fakeAdminConfigService{},
		AuthService: fakeAuthEndpointService{auth: service.CurrentAuth{
			Authenticated: true,
			User: domain.User{
				ID:     2,
				Role:   domain.UserRoleUser,
				Status: domain.UserStatusActive,
			},
		}},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/config", nil)
	request.AddCookie(&http.Cookie{Name: "messagefeed_session", Value: "session-token"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

type fakeAdminConfigService struct {
	status           service.AdminConfigStatus
	statusErr        error
	llmInput         service.AdminLLMTestInput
	llmResult        service.AdminLLMTestResult
	llmErr           error
	weChatWorkInput  service.AdminWeChatWorkTestInput
	weChatWorkResult service.AdminWeChatWorkTestResult
	weChatWorkErr    error
}

func (f *fakeAdminConfigService) Status(_ context.Context) (service.AdminConfigStatus, error) {
	if f.statusErr != nil {
		return service.AdminConfigStatus{}, f.statusErr
	}
	return f.status, nil
}

func (f *fakeAdminConfigService) TestLLM(_ context.Context, input service.AdminLLMTestInput) (service.AdminLLMTestResult, error) {
	f.llmInput = input
	if f.llmErr != nil {
		return service.AdminLLMTestResult{}, f.llmErr
	}
	return f.llmResult, nil
}

func (f *fakeAdminConfigService) TestWeChatWork(_ context.Context, input service.AdminWeChatWorkTestInput) (service.AdminWeChatWorkTestResult, error) {
	f.weChatWorkInput = input
	if f.weChatWorkErr != nil {
		return service.AdminWeChatWorkTestResult{}, f.weChatWorkErr
	}
	return f.weChatWorkResult, nil
}
