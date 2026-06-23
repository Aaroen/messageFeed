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

type fakeAuthEndpointService struct {
	auth service.CurrentAuth
}

func (f fakeAuthEndpointService) AuthenticateSession(ctx context.Context, rawToken string) (service.CurrentAuth, error) {
	return f.auth, nil
}

func (fakeAuthEndpointService) CookieName() string {
	return "messagefeed_session"
}

func (fakeAuthEndpointService) LocalLogin(ctx context.Context, input service.LocalLoginInput) (service.AuthSessionResult, error) {
	return service.AuthSessionResult{}, nil
}

func (fakeAuthEndpointService) RegisterWithInvite(ctx context.Context, input service.RegisterWithInviteInput) (service.AuthSessionResult, error) {
	return service.AuthSessionResult{}, nil
}

func (fakeAuthEndpointService) ChangePassword(ctx context.Context, input service.ChangePasswordInput) (service.AuthUserResponse, error) {
	return service.AuthUserResponse{}, nil
}

func (fakeAuthEndpointService) UpdateProfile(ctx context.Context, input service.UpdateProfileInput) (service.UserProfileResponse, error) {
	return service.UserProfileResponse{}, nil
}

func (fakeAuthEndpointService) ListSessions(ctx context.Context, auth service.CurrentAuth) ([]service.UserSessionResponse, error) {
	return []service.UserSessionResponse{}, nil
}

func (fakeAuthEndpointService) RevokeSession(ctx context.Context, auth service.CurrentAuth, sessionID int64) error {
	return nil
}

func (fakeAuthEndpointService) DeactivateAccount(ctx context.Context, input service.DeactivateAccountInput) error {
	return nil
}

func (fakeAuthEndpointService) Logout(ctx context.Context, rawToken string) error {
	return nil
}

func (fakeAuthEndpointService) Me(ctx context.Context, auth service.CurrentAuth) (service.AuthMeResult, error) {
	return service.AuthMeResult{Bindings: []service.AuthBindingResponse{}}, nil
}

func (fakeAuthEndpointService) GetUserContext(ctx context.Context, auth service.CurrentAuth) (service.UserContextResult, error) {
	return service.UserContextResult{}, nil
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

func (fakeAuthEndpointService) CreateInvite(ctx context.Context, input service.CreateInviteInput) (service.CreateInviteResult, error) {
	return service.CreateInviteResult{}, nil
}

func (fakeAuthEndpointService) ListInvites(ctx context.Context, auth service.CurrentAuth) ([]service.InviteCodeResponse, error) {
	return []service.InviteCodeResponse{}, nil
}

func (fakeAuthEndpointService) RevokeInvite(ctx context.Context, auth service.CurrentAuth, inviteID int64) (service.InviteCodeResponse, error) {
	return service.InviteCodeResponse{}, nil
}

func (fakeAuthEndpointService) ListUsers(ctx context.Context, auth service.CurrentAuth) ([]service.AdminUserResponse, error) {
	return []service.AdminUserResponse{}, nil
}

func (fakeAuthEndpointService) DeactivateUser(ctx context.Context, auth service.CurrentAuth, userID int64) (service.AdminUserResponse, error) {
	return service.AdminUserResponse{}, nil
}

func (fakeAuthEndpointService) CookieMaxAge() int {
	return int(time.Hour.Seconds())
}

func (fakeAuthEndpointService) CookieSecure() bool {
	return false
}
