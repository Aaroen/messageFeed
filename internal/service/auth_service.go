package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"messagefeed/internal/config"
	"messagefeed/internal/domain"
	"messagefeed/internal/observability"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const authSessionTokenBytes = 32

type AuthRepository interface {
	EnsureOwner(ctx context.Context, username string) (domain.User, error)
	GetUserByID(ctx context.Context, userID int64) (domain.User, error)
	CreateSession(ctx context.Context, session domain.UserSession) (domain.UserSession, error)
	GetSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) (domain.UserSession, error)
	TouchSession(ctx context.Context, sessionID int64, now time.Time) error
	RevokeSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) error
	CreateOAuthState(ctx context.Context, state domain.AuthOAuthState) (domain.AuthOAuthState, error)
	ConsumeOAuthState(ctx context.Context, stateHash string, now time.Time) (domain.AuthOAuthState, error)
	BindExternalAccount(ctx context.Context, account domain.ExternalAccount) (domain.ExternalAccount, error)
	ListExternalAccounts(ctx context.Context, userID int64) ([]domain.ExternalAccount, error)
	DisableExternalAccount(ctx context.Context, userID int64, accountID int64, now time.Time) (domain.ExternalAccount, error)
}

type WeChatWorkOAuthExchanger interface {
	ExchangeCode(ctx context.Context, code string) (WeChatWorkOAuthUser, error)
}

type AuthService struct {
	repository  AuthRepository
	cfg         config.Config
	wechatOAuth WeChatWorkOAuthExchanger
	now         func() time.Time
	randomToken func() (string, error)
}

type AuthServiceOption func(*AuthService)

func WithAuthWeChatWorkOAuth(exchanger WeChatWorkOAuthExchanger) AuthServiceOption {
	return func(service *AuthService) {
		service.wechatOAuth = exchanger
	}
}

func WithAuthNow(now func() time.Time) AuthServiceOption {
	return func(service *AuthService) {
		if now != nil {
			service.now = now
		}
	}
}

func WithAuthRandomToken(randomToken func() (string, error)) AuthServiceOption {
	return func(service *AuthService) {
		if randomToken != nil {
			service.randomToken = randomToken
		}
	}
}

func NewAuthService(repository AuthRepository, cfg config.Config, options ...AuthServiceOption) *AuthService {
	service := &AuthService{
		repository:  repository,
		cfg:         cfg,
		now:         time.Now,
		randomToken: newURLToken,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

type LocalLoginInput struct {
	Username      string
	Password      string
	UserAgent     string
	RemoteAddress string
}

type AuthSessionResult struct {
	User      domain.User
	Session   domain.UserSession
	Token     string
	ExpiresAt time.Time
}

type CurrentAuth struct {
	Authenticated bool
	User          domain.User
	Session       domain.UserSession
}

type AuthMeResult struct {
	Authenticated          bool                  `json:"authenticated"`
	LoginEnabled           bool                  `json:"login_enabled"`
	WeChatWorkOAuthEnabled bool                  `json:"wechat_work_oauth_enabled"`
	User                   *AuthUserResponse     `json:"user,omitempty"`
	Bindings               []AuthBindingResponse `json:"bindings"`
}

type AuthUserResponse struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
}

type AuthBindingResponse struct {
	ID             int64  `json:"id"`
	Provider       string `json:"provider"`
	CorpIDMasked   string `json:"corp_id_masked"`
	AgentID        string `json:"agent_id"`
	ExternalUserID string `json:"external_user_id"`
	DisplayName    string `json:"display_name"`
	BindingStatus  string `json:"binding_status"`
	VerifiedAt     string `json:"verified_at,omitempty"`
	LastSeenAt     string `json:"last_seen_at,omitempty"`
}

type WeChatWorkOAuthURLInput struct {
	UserID       int64
	Purpose      string
	RedirectPath string
}

type WeChatWorkOAuthURLResult struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

type WeChatWorkOAuthCallbackInput struct {
	Code          string
	State         string
	UserAgent     string
	RemoteAddress string
}

type WeChatWorkOAuthCallbackResult struct {
	AuthSessionResult
	Binding      domain.ExternalAccount
	RedirectPath string
}

func (s *AuthService) LocalLogin(ctx context.Context, input LocalLoginInput) (AuthSessionResult, error) {
	if s == nil || s.repository == nil {
		return AuthSessionResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "auth_unavailable", "auth service is unavailable", "service.auth.login", false, nil)
	}
	ctx, span := observability.StartSpan(ctx, "service.auth.login")
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if !s.cfg.Auth.LocalLoginEnabled() {
		opErr = domain.NewAppError(domain.ErrorKindUnavailable, "auth_local_login_disabled", "local login is not configured", "service.auth.login", false, nil)
		return AuthSessionResult{}, opErr
	}
	username := strings.TrimSpace(input.Username)
	password := strings.TrimSpace(input.Password)
	if username == "" || password == "" {
		opErr = fmt.Errorf("%w: username and password are required", domain.ErrInvalidInput)
		return AuthSessionResult{}, opErr
	}
	if subtle.ConstantTimeCompare([]byte(username), []byte(s.cfg.Auth.OwnerUsername)) != 1 ||
		subtle.ConstantTimeCompare([]byte(password), []byte(s.cfg.Auth.OwnerPassword)) != 1 {
		opErr = domain.NewAppError(domain.ErrorKindInvalidInput, "auth_invalid_credentials", "invalid username or password", "service.auth.login", false, nil)
		return AuthSessionResult{}, opErr
	}

	user, err := s.repository.EnsureOwner(ctx, s.cfg.Auth.OwnerUsername)
	if err != nil {
		opErr = err
		return AuthSessionResult{}, err
	}
	result, err := s.createSession(ctx, user, input.UserAgent, input.RemoteAddress)
	if err != nil {
		opErr = err
		return AuthSessionResult{}, err
	}
	span.SetAttributes(attribute.Int64("auth.user_id", user.ID))
	return result, nil
}

func (s *AuthService) AuthenticateSession(ctx context.Context, rawToken string) (CurrentAuth, error) {
	if s == nil || s.repository == nil {
		return CurrentAuth{}, nil
	}
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return CurrentAuth{}, nil
	}
	ctx, span := observability.StartSpan(ctx, "service.auth.session.authenticate")
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	now := s.now().UTC()
	session, err := s.repository.GetSessionByTokenHash(ctx, hashSecret(rawToken), now)
	if err != nil {
		if domain.ClassifyError(err) == domain.ErrorKindNotFound {
			return CurrentAuth{}, nil
		}
		opErr = err
		return CurrentAuth{}, err
	}
	user, err := s.repository.GetUserByID(ctx, session.UserID)
	if err != nil {
		opErr = err
		return CurrentAuth{}, err
	}
	if user.Status != domain.UserStatusActive {
		return CurrentAuth{}, nil
	}
	_ = s.repository.TouchSession(ctx, session.ID, now)
	span.SetAttributes(attribute.Int64("auth.user_id", user.ID), attribute.Int64("auth.session_id", session.ID))
	return CurrentAuth{Authenticated: true, User: user, Session: session}, nil
}

func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	if s == nil || s.repository == nil {
		return nil
	}
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil
	}
	return s.repository.RevokeSessionByTokenHash(ctx, hashSecret(rawToken), s.now().UTC())
}

func (s *AuthService) Me(ctx context.Context, auth CurrentAuth) (AuthMeResult, error) {
	result := AuthMeResult{
		Authenticated:          auth.Authenticated,
		LoginEnabled:           s != nil && s.cfg.Auth.LocalLoginEnabled(),
		WeChatWorkOAuthEnabled: s != nil && s.cfg.WeChatWork.Enabled() && s.wechatOAuth != nil,
		Bindings:               []AuthBindingResponse{},
	}
	if s == nil || s.repository == nil || !auth.Authenticated {
		return result, nil
	}
	result.User = userResponse(auth.User)
	accounts, err := s.repository.ListExternalAccounts(ctx, auth.User.ID)
	if err != nil {
		return AuthMeResult{}, err
	}
	result.Bindings = bindingResponses(accounts)
	return result, nil
}

func (s *AuthService) BuildWeChatWorkOAuthURL(ctx context.Context, input WeChatWorkOAuthURLInput) (WeChatWorkOAuthURLResult, error) {
	if s == nil || s.repository == nil || s.wechatOAuth == nil || !s.cfg.WeChatWork.Enabled() {
		return WeChatWorkOAuthURLResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "auth_wechat_work_oauth_unavailable", "wechat work oauth is not configured", "service.auth.wechat_work.oauth_url", false, nil)
	}
	if input.UserID < 1 {
		return WeChatWorkOAuthURLResult{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	ctx, span := observability.StartSpan(ctx, "service.auth.wechat_work.oauth_url")
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	purpose := domain.AuthOAuthPurpose(strings.TrimSpace(input.Purpose))
	if !purpose.Valid() {
		purpose = domain.AuthOAuthPurposeBind
	}
	redirectPath := sanitizeRedirectPath(input.RedirectPath)
	state, err := s.randomToken()
	if err != nil {
		opErr = err
		return WeChatWorkOAuthURLResult{}, err
	}
	now := s.now().UTC()
	expiresAt := now.Add(s.cfg.Auth.OAuthStateTTL)
	if _, err := s.repository.CreateOAuthState(ctx, domain.AuthOAuthState{
		StateHash:    hashSecret(state),
		UserID:       input.UserID,
		Provider:     domain.AgentProviderWeChatWorkApp,
		Purpose:      purpose,
		RedirectPath: redirectPath,
		CorpID:       s.cfg.WeChatWork.CorpID,
		AgentID:      s.cfg.WeChatWork.AgentID,
		ExpiresAt:    expiresAt,
		Metadata:     domain.AgentJSON{},
		CreatedAt:    now,
	}); err != nil {
		opErr = err
		return WeChatWorkOAuthURLResult{}, err
	}

	oauthURL, err := s.buildWeChatWorkOAuthAuthorizeURL(state)
	if err != nil {
		opErr = err
		return WeChatWorkOAuthURLResult{}, err
	}
	span.SetAttributes(attribute.String("auth.oauth.purpose", string(purpose)))
	return WeChatWorkOAuthURLResult{URL: oauthURL, ExpiresAt: expiresAt}, nil
}

func (s *AuthService) HandleWeChatWorkOAuthCallback(ctx context.Context, input WeChatWorkOAuthCallbackInput) (WeChatWorkOAuthCallbackResult, error) {
	if s == nil || s.repository == nil || s.wechatOAuth == nil || !s.cfg.WeChatWork.Enabled() {
		return WeChatWorkOAuthCallbackResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "auth_wechat_work_oauth_unavailable", "wechat work oauth is not configured", "service.auth.wechat_work.callback", false, nil)
	}
	code := strings.TrimSpace(input.Code)
	stateToken := strings.TrimSpace(input.State)
	if code == "" || stateToken == "" {
		return WeChatWorkOAuthCallbackResult{}, fmt.Errorf("%w: code and state are required", domain.ErrInvalidInput)
	}
	ctx, span := observability.StartSpan(ctx, "service.auth.wechat_work.callback")
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	now := s.now().UTC()
	state, err := s.repository.ConsumeOAuthState(ctx, hashSecret(stateToken), now)
	if err != nil {
		opErr = err
		return WeChatWorkOAuthCallbackResult{}, domain.NewAppError(domain.ErrorKindInvalidInput, "auth_oauth_state_invalid", "oauth state is invalid or expired", "service.auth.wechat_work.callback", false, err)
	}
	user, err := s.repository.GetUserByID(ctx, state.UserID)
	if err != nil {
		opErr = err
		return WeChatWorkOAuthCallbackResult{}, err
	}
	if user.Status != domain.UserStatusActive {
		opErr = domain.NewAppError(domain.ErrorKindUnavailable, "auth_user_disabled", "user is disabled", "service.auth.wechat_work.callback", false, nil)
		return WeChatWorkOAuthCallbackResult{}, opErr
	}
	wechatUser, err := s.wechatOAuth.ExchangeCode(ctx, code)
	if err != nil {
		opErr = err
		return WeChatWorkOAuthCallbackResult{}, err
	}
	if strings.TrimSpace(wechatUser.UserID) == "" {
		opErr = domain.NewAppError(domain.ErrorKindUnavailable, "auth_wechat_work_user_missing", "wechat work oauth did not return userid", "service.auth.wechat_work.callback", true, nil)
		return WeChatWorkOAuthCallbackResult{}, opErr
	}
	verifiedAt := now
	account, err := s.repository.BindExternalAccount(ctx, domain.ExternalAccount{
		UserID:         user.ID,
		Provider:       domain.AgentProviderWeChatWorkApp,
		CorpID:         s.cfg.WeChatWork.CorpID,
		AgentID:        s.cfg.WeChatWork.AgentID,
		ExternalUserID: wechatUser.UserID,
		DisplayName:    wechatUser.Name,
		BindingStatus:  domain.ExternalAccountBindingStatusActive,
		VerifiedAt:     &verifiedAt,
		LastSeenAt:     &verifiedAt,
	})
	if err != nil {
		opErr = err
		return WeChatWorkOAuthCallbackResult{}, err
	}
	session, err := s.createSession(ctx, user, input.UserAgent, input.RemoteAddress)
	if err != nil {
		opErr = err
		return WeChatWorkOAuthCallbackResult{}, err
	}
	span.SetAttributes(attribute.Int64("auth.user_id", user.ID), attribute.Int64("auth.external_account_id", account.ID))
	return WeChatWorkOAuthCallbackResult{
		AuthSessionResult: session,
		Binding:           account,
		RedirectPath:      state.RedirectPath,
	}, nil
}

func (s *AuthService) ListBindings(ctx context.Context, userID int64) ([]AuthBindingResponse, error) {
	if s == nil || s.repository == nil {
		return nil, domain.NewAppError(domain.ErrorKindUnavailable, "auth_unavailable", "auth service is unavailable", "service.auth.bindings", false, nil)
	}
	accounts, err := s.repository.ListExternalAccounts(ctx, userID)
	if err != nil {
		return nil, err
	}
	return bindingResponses(accounts), nil
}

func (s *AuthService) DisableBinding(ctx context.Context, userID int64, accountID int64) (AuthBindingResponse, error) {
	if s == nil || s.repository == nil {
		return AuthBindingResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "auth_unavailable", "auth service is unavailable", "service.auth.disable_binding", false, nil)
	}
	if userID < 1 || accountID < 1 {
		return AuthBindingResponse{}, fmt.Errorf("%w: user id and account id are required", domain.ErrInvalidInput)
	}
	account, err := s.repository.DisableExternalAccount(ctx, userID, accountID, s.now().UTC())
	if err != nil {
		return AuthBindingResponse{}, err
	}
	return bindingResponse(account), nil
}

func (s *AuthService) CookieName() string {
	if s == nil || strings.TrimSpace(s.cfg.Auth.SessionCookie) == "" {
		return config.DefaultAuthSessionCookie
	}
	return s.cfg.Auth.SessionCookie
}

func (s *AuthService) CookieMaxAge() int {
	if s == nil || s.cfg.Auth.SessionTTL <= 0 {
		return int(config.DefaultAuthSessionTTL.Seconds())
	}
	return int(s.cfg.Auth.SessionTTL.Seconds())
}

func (s *AuthService) CookieSecure() bool {
	return s != nil && s.cfg.Auth.SessionSecure
}

func (s *AuthService) createSession(ctx context.Context, user domain.User, userAgent string, remoteAddress string) (AuthSessionResult, error) {
	token, err := s.randomToken()
	if err != nil {
		return AuthSessionResult{}, err
	}
	now := s.now().UTC()
	expiresAt := now.Add(s.cfg.Auth.SessionTTL)
	session, err := s.repository.CreateSession(ctx, domain.UserSession{
		UserID:           user.ID,
		SessionTokenHash: hashSecret(token),
		ExpiresAt:        expiresAt,
		UserAgentHash:    hashSecret(strings.TrimSpace(userAgent)),
		IPAddress:        strings.TrimSpace(remoteAddress),
		LastSeenAt:       now,
		CreatedAt:        now,
		UpdatedAt:        now,
	})
	if err != nil {
		return AuthSessionResult{}, err
	}
	return AuthSessionResult{User: user, Session: session, Token: token, ExpiresAt: expiresAt}, nil
}

func (s *AuthService) buildWeChatWorkOAuthAuthorizeURL(state string) (string, error) {
	callbackURL := joinPublicURL(strings.TrimRight(s.cfg.Runtime.PublicBaseURL, "/"), "/api/v1/auth/wechat-work/callback")
	parsedCallback, err := url.Parse(callbackURL)
	if err != nil || parsedCallback.Scheme == "" || parsedCallback.Host == "" {
		if err == nil {
			err = fmt.Errorf("scheme and host are required")
		}
		return "", domain.NewAppError(domain.ErrorKindInvalidInput, "auth_invalid_oauth_callback", "oauth callback url is invalid", "service.auth.wechat_work.oauth_url", false, err)
	}

	values := url.Values{}
	values.Set("appid", s.cfg.WeChatWork.CorpID)
	values.Set("redirect_uri", callbackURL)
	values.Set("response_type", "code")
	values.Set("scope", "snsapi_base")
	values.Set("state", state)
	return "https://open.weixin.qq.com/connect/oauth2/authorize?" + values.Encode() + "#wechat_redirect", nil
}

func sanitizeRedirectPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/settings"
	}
	parsed, err := url.Parse(path)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return "/settings"
	}
	return path
}

func userResponse(user domain.User) *AuthUserResponse {
	return &AuthUserResponse{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        string(user.Role),
		Status:      string(user.Status),
	}
}

func bindingResponses(accounts []domain.ExternalAccount) []AuthBindingResponse {
	responses := make([]AuthBindingResponse, 0, len(accounts))
	for _, account := range accounts {
		responses = append(responses, bindingResponse(account))
	}
	return responses
}

func bindingResponse(account domain.ExternalAccount) AuthBindingResponse {
	response := AuthBindingResponse{
		ID:             account.ID,
		Provider:       account.Provider,
		CorpIDMasked:   maskConfigValue(account.CorpID),
		AgentID:        account.AgentID,
		ExternalUserID: account.ExternalUserID,
		DisplayName:    account.DisplayName,
		BindingStatus:  string(account.BindingStatus),
	}
	if account.VerifiedAt != nil {
		response.VerifiedAt = account.VerifiedAt.UTC().Format(time.RFC3339)
	}
	if account.LastSeenAt != nil {
		response.LastSeenAt = account.LastSeenAt.UTC().Format(time.RFC3339)
	}
	return response
}

func hashSecret(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func newURLToken() (string, error) {
	var b [authSessionTokenBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
