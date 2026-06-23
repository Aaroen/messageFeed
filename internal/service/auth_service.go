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
	"golang.org/x/crypto/bcrypt"
)

const (
	authSessionTokenBytes = 32
	minPasswordLength     = 8
	maxUsernameLength     = 64
	maxDisplayNameLength  = 80
	defaultInviteMaxUses  = 1
	defaultInviteTTL      = 7 * 24 * time.Hour
)

type AuthRepository interface {
	EnsureOwner(ctx context.Context, username string) (domain.User, error)
	GetUserByID(ctx context.Context, userID int64) (domain.User, error)
	GetUserByUsername(ctx context.Context, username string) (domain.User, error)
	UpdateUserPassword(ctx context.Context, userID int64, passwordHash string, now time.Time) (domain.User, error)
	CreateSession(ctx context.Context, session domain.UserSession) (domain.UserSession, error)
	GetSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) (domain.UserSession, error)
	TouchSession(ctx context.Context, sessionID int64, now time.Time) error
	RevokeSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) error
	CreateOAuthState(ctx context.Context, state domain.AuthOAuthState) (domain.AuthOAuthState, error)
	ConsumeOAuthState(ctx context.Context, stateHash string, now time.Time) (domain.AuthOAuthState, error)
	BindExternalAccount(ctx context.Context, account domain.ExternalAccount) (domain.ExternalAccount, error)
	ListExternalAccounts(ctx context.Context, userID int64) ([]domain.ExternalAccount, error)
	DisableExternalAccount(ctx context.Context, userID int64, accountID int64, now time.Time) (domain.ExternalAccount, error)
	CreateInviteCode(ctx context.Context, invite domain.AuthInviteCode) (domain.AuthInviteCode, error)
	ListInviteCodes(ctx context.Context, createdByUserID int64) ([]domain.AuthInviteCode, error)
	RevokeInviteCode(ctx context.Context, createdByUserID int64, inviteID int64, now time.Time) (domain.AuthInviteCode, error)
	CreateUserWithInvite(ctx context.Context, codeHash string, user domain.User, redemption domain.AuthInviteRedemption, now time.Time) (domain.User, domain.AuthInviteCode, error)
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

type RegisterWithInviteInput struct {
	InviteCode    string
	Username      string
	Password      string
	DisplayName   string
	Email         string
	UserAgent     string
	RemoteAddress string
}

type ChangePasswordInput struct {
	UserID          int64
	CurrentPassword string
	NewPassword     string
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
	RegistrationEnabled    bool                  `json:"registration_enabled"`
	WeChatWorkOAuthEnabled bool                  `json:"wechat_work_oauth_enabled"`
	User                   *AuthUserResponse     `json:"user,omitempty"`
	Bindings               []AuthBindingResponse `json:"bindings"`
}

type AuthUserResponse struct {
	ID                 int64  `json:"id"`
	Username           string `json:"username"`
	DisplayName        string `json:"display_name"`
	Role               string `json:"role"`
	Status             string `json:"status"`
	PasswordConfigured bool   `json:"password_configured"`
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

type CreateInviteInput struct {
	Creator    domain.User
	Role       string
	MaxUses    int
	TTLSeconds int64
}

type InviteCodeResponse struct {
	ID        int64  `json:"id"`
	Role      string `json:"role"`
	MaxUses   int    `json:"max_uses"`
	UseCount  int    `json:"use_count"`
	Status    string `json:"status"`
	ExpiresAt string `json:"expires_at,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type CreateInviteResult struct {
	Invite InviteCodeResponse `json:"invite"`
	Code   string             `json:"code"`
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

	if user, err := s.repository.GetUserByUsername(ctx, username); err == nil && user.ID > 0 {
		if user.Status != domain.UserStatusActive {
			opErr = domain.NewAppError(domain.ErrorKindUnavailable, "auth_user_disabled", "user is disabled", "service.auth.login", false, nil)
			return AuthSessionResult{}, opErr
		}
		if strings.TrimSpace(user.PasswordHash) != "" {
			if err := verifyPassword(user.PasswordHash, password); err != nil {
				opErr = domain.NewAppError(domain.ErrorKindInvalidInput, "auth_invalid_credentials", "invalid username or password", "service.auth.login", false, nil)
				return AuthSessionResult{}, opErr
			}
			result, err := s.createSession(ctx, user, input.UserAgent, input.RemoteAddress)
			if err != nil {
				opErr = err
				return AuthSessionResult{}, err
			}
			span.SetAttributes(attribute.Int64("auth.user_id", user.ID))
			return result, nil
		}
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
	passwordHash, err := hashPassword(password)
	if err != nil {
		opErr = err
		return AuthSessionResult{}, err
	}
	user, err = s.repository.UpdateUserPassword(ctx, user.ID, passwordHash, s.now().UTC())
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

func (s *AuthService) RegisterWithInvite(ctx context.Context, input RegisterWithInviteInput) (AuthSessionResult, error) {
	if s == nil || s.repository == nil {
		return AuthSessionResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "auth_unavailable", "auth service is unavailable", "service.auth.register", false, nil)
	}
	ctx, span := observability.StartSpan(ctx, "service.auth.register")
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	inviteCode := strings.TrimSpace(input.InviteCode)
	if inviteCode == "" {
		opErr = fmt.Errorf("%w: invite code is required", domain.ErrInvalidInput)
		return AuthSessionResult{}, opErr
	}
	username, err := normalizeUsername(input.Username)
	if err != nil {
		opErr = err
		return AuthSessionResult{}, err
	}
	displayName := normalizeDisplayName(input.DisplayName, username)
	if err := validatePassword(input.Password); err != nil {
		opErr = err
		return AuthSessionResult{}, err
	}
	passwordHash, err := hashPassword(input.Password)
	if err != nil {
		opErr = err
		return AuthSessionResult{}, err
	}
	now := s.now().UTC()
	user, _, err := s.repository.CreateUserWithInvite(ctx, hashSecret(inviteCode), domain.User{
		Username:     username,
		Email:        strings.TrimSpace(input.Email),
		DisplayName:  displayName,
		PasswordHash: passwordHash,
		Status:       domain.UserStatusActive,
	}, domain.AuthInviteRedemption{
		IPAddress:     strings.TrimSpace(input.RemoteAddress),
		UserAgentHash: hashSecret(strings.TrimSpace(input.UserAgent)),
	}, now)
	if err != nil {
		if domain.ClassifyError(err) == domain.ErrorKindConflict {
			opErr = domain.NewAppError(domain.ErrorKindConflict, "auth_invite_unavailable", "invite code is invalid, expired or already used", "service.auth.register", false, err)
			return AuthSessionResult{}, opErr
		}
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

func (s *AuthService) ChangePassword(ctx context.Context, input ChangePasswordInput) (AuthUserResponse, error) {
	if s == nil || s.repository == nil {
		return AuthUserResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "auth_unavailable", "auth service is unavailable", "service.auth.change_password", false, nil)
	}
	if input.UserID < 1 {
		return AuthUserResponse{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	if err := validatePassword(input.NewPassword); err != nil {
		return AuthUserResponse{}, err
	}
	ctx, span := observability.StartSpan(ctx, "service.auth.change_password")
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	user, err := s.repository.GetUserByID(ctx, input.UserID)
	if err != nil {
		opErr = err
		return AuthUserResponse{}, err
	}
	if strings.TrimSpace(user.PasswordHash) != "" {
		if err := verifyPassword(user.PasswordHash, input.CurrentPassword); err != nil {
			opErr = domain.NewAppError(domain.ErrorKindInvalidInput, "auth_invalid_current_password", "current password is invalid", "service.auth.change_password", false, nil)
			return AuthUserResponse{}, opErr
		}
	} else if !s.ownerPasswordMatches(user.Username, input.CurrentPassword) {
		opErr = domain.NewAppError(domain.ErrorKindInvalidInput, "auth_invalid_current_password", "current password is invalid", "service.auth.change_password", false, nil)
		return AuthUserResponse{}, opErr
	}
	passwordHash, err := hashPassword(input.NewPassword)
	if err != nil {
		opErr = err
		return AuthUserResponse{}, err
	}
	user, err = s.repository.UpdateUserPassword(ctx, user.ID, passwordHash, s.now().UTC())
	if err != nil {
		opErr = err
		return AuthUserResponse{}, err
	}
	return *userResponse(user), nil
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
		RegistrationEnabled:    s != nil && s.repository != nil,
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

func (s *AuthService) CreateInvite(ctx context.Context, input CreateInviteInput) (CreateInviteResult, error) {
	if s == nil || s.repository == nil {
		return CreateInviteResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "auth_unavailable", "auth service is unavailable", "service.auth.create_invite", false, nil)
	}
	if input.Creator.ID < 1 || input.Creator.Role != domain.UserRoleOwner {
		return CreateInviteResult{}, domain.NewAppError(domain.ErrorKindInvalidInput, "auth_owner_required", "owner role is required", "service.auth.create_invite", false, nil)
	}
	role := domain.UserRole(strings.TrimSpace(input.Role))
	if !role.Valid() || role == domain.UserRoleOwner {
		role = domain.UserRoleUser
	}
	if input.MaxUses > defaultInviteMaxUses {
		return CreateInviteResult{}, fmt.Errorf("%w: invite code can only be used once", domain.ErrInvalidInput)
	}
	ttl := time.Duration(input.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = defaultInviteTTL
	}
	code, err := s.randomToken()
	if err != nil {
		return CreateInviteResult{}, err
	}
	now := s.now().UTC()
	expiresAt := now.Add(ttl)
	invite, err := s.repository.CreateInviteCode(ctx, domain.AuthInviteCode{
		CodeHash:    hashSecret(code),
		CreatedByID: input.Creator.ID,
		Role:        role,
		MaxUses:     defaultInviteMaxUses,
		Status:      domain.AuthInviteCodeStatusActive,
		ExpiresAt:   &expiresAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return CreateInviteResult{}, err
	}
	return CreateInviteResult{Invite: inviteResponse(invite), Code: code}, nil
}

func (s *AuthService) ListInvites(ctx context.Context, auth CurrentAuth) ([]InviteCodeResponse, error) {
	if s == nil || s.repository == nil {
		return nil, domain.NewAppError(domain.ErrorKindUnavailable, "auth_unavailable", "auth service is unavailable", "service.auth.list_invites", false, nil)
	}
	if !auth.Authenticated || auth.User.Role != domain.UserRoleOwner {
		return nil, domain.NewAppError(domain.ErrorKindInvalidInput, "auth_owner_required", "owner role is required", "service.auth.list_invites", false, nil)
	}
	invites, err := s.repository.ListInviteCodes(ctx, auth.User.ID)
	if err != nil {
		return nil, err
	}
	responses := make([]InviteCodeResponse, 0, len(invites))
	for _, invite := range invites {
		responses = append(responses, inviteResponse(invite))
	}
	return responses, nil
}

func (s *AuthService) RevokeInvite(ctx context.Context, auth CurrentAuth, inviteID int64) (InviteCodeResponse, error) {
	if s == nil || s.repository == nil {
		return InviteCodeResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "auth_unavailable", "auth service is unavailable", "service.auth.revoke_invite", false, nil)
	}
	if !auth.Authenticated || auth.User.Role != domain.UserRoleOwner {
		return InviteCodeResponse{}, domain.NewAppError(domain.ErrorKindInvalidInput, "auth_owner_required", "owner role is required", "service.auth.revoke_invite", false, nil)
	}
	if inviteID < 1 {
		return InviteCodeResponse{}, fmt.Errorf("%w: invite id is required", domain.ErrInvalidInput)
	}
	invite, err := s.repository.RevokeInviteCode(ctx, auth.User.ID, inviteID, s.now().UTC())
	if err != nil {
		return InviteCodeResponse{}, err
	}
	return inviteResponse(invite), nil
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
		ID:                 user.ID,
		Username:           user.Username,
		DisplayName:        user.DisplayName,
		Role:               string(user.Role),
		Status:             string(user.Status),
		PasswordConfigured: strings.TrimSpace(user.PasswordHash) != "",
	}
}

func inviteResponse(invite domain.AuthInviteCode) InviteCodeResponse {
	response := InviteCodeResponse{
		ID:        invite.ID,
		Role:      string(invite.Role),
		MaxUses:   invite.MaxUses,
		UseCount:  invite.UseCount,
		Status:    string(invite.Status),
		CreatedAt: invite.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: invite.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if invite.ExpiresAt != nil {
		response.ExpiresAt = invite.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return response
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

func normalizeUsername(username string) (string, error) {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(username) > maxUsernameLength {
		return "", fmt.Errorf("%w: username length must be between 3 and 64", domain.ErrInvalidInput)
	}
	for _, r := range username {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case '-', '_', '.':
			continue
		default:
			return "", fmt.Errorf("%w: username contains invalid characters", domain.ErrInvalidInput)
		}
	}
	return username, nil
}

func normalizeDisplayName(displayName string, username string) string {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return username
	}
	runes := []rune(displayName)
	if len(runes) > maxDisplayNameLength {
		return string(runes[:maxDisplayNameLength])
	}
	return displayName
}

func validatePassword(password string) error {
	if len([]rune(password)) < minPasswordLength {
		return fmt.Errorf("%w: password must be at least 8 characters", domain.ErrInvalidInput)
	}
	return nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func verifyPassword(passwordHash string, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(strings.TrimSpace(passwordHash)), []byte(password))
}

func (s *AuthService) ownerPasswordMatches(username string, password string) bool {
	return s != nil &&
		s.cfg.Auth.LocalLoginEnabled() &&
		subtle.ConstantTimeCompare([]byte(strings.TrimSpace(username)), []byte(s.cfg.Auth.OwnerUsername)) == 1 &&
		subtle.ConstantTimeCompare([]byte(password), []byte(s.cfg.Auth.OwnerPassword)) == 1
}
