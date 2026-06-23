package handler

import (
	"context"
	"messagefeed/internal/service"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProtectedRoutesRequireAuthWhenAuthServiceConfigured(t *testing.T) {
	router := newTestRouter(t, RouterOptions{AuthService: fakeAuthEndpointService{}})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

type fakeAuthEndpointService struct{}

func (fakeAuthEndpointService) AuthenticateSession(ctx context.Context, rawToken string) (service.CurrentAuth, error) {
	return service.CurrentAuth{}, nil
}

func (fakeAuthEndpointService) CookieName() string {
	return "messagefeed_session"
}

func (fakeAuthEndpointService) LocalLogin(ctx context.Context, input service.LocalLoginInput) (service.AuthSessionResult, error) {
	return service.AuthSessionResult{}, nil
}

func (fakeAuthEndpointService) Logout(ctx context.Context, rawToken string) error {
	return nil
}

func (fakeAuthEndpointService) Me(ctx context.Context, auth service.CurrentAuth) (service.AuthMeResult, error) {
	return service.AuthMeResult{Bindings: []service.AuthBindingResponse{}}, nil
}

func (fakeAuthEndpointService) BuildWeChatWorkOAuthURL(ctx context.Context, input service.WeChatWorkOAuthURLInput) (service.WeChatWorkOAuthURLResult, error) {
	return service.WeChatWorkOAuthURLResult{}, nil
}

func (fakeAuthEndpointService) HandleWeChatWorkOAuthCallback(ctx context.Context, input service.WeChatWorkOAuthCallbackInput) (service.WeChatWorkOAuthCallbackResult, error) {
	return service.WeChatWorkOAuthCallbackResult{}, nil
}

func (fakeAuthEndpointService) ListBindings(ctx context.Context, userID int64) ([]service.AuthBindingResponse, error) {
	return []service.AuthBindingResponse{}, nil
}

func (fakeAuthEndpointService) DisableBinding(ctx context.Context, userID int64, accountID int64) (service.AuthBindingResponse, error) {
	return service.AuthBindingResponse{}, nil
}

func (fakeAuthEndpointService) CookieMaxAge() int {
	return int(time.Hour.Seconds())
}

func (fakeAuthEndpointService) CookieSecure() bool {
	return false
}
