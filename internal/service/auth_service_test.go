package service

import (
	"context"
	"errors"
	"messagefeed/internal/config"
	"messagefeed/internal/domain"
	"strings"
	"testing"
	"time"
)

func TestAuthServiceLocalLoginAndAuthenticateSession(t *testing.T) {
	now := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	repository := newFakeAuthRepository(now)
	service := NewAuthService(repository, testAuthConfig(), WithAuthNow(func() time.Time { return now }), WithAuthRandomToken(func() (string, error) {
		return "session-token", nil
	}))

	result, err := service.LocalLogin(context.Background(), LocalLoginInput{
		Username: "owner",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("LocalLogin() error = %v", err)
	}
	if result.Token != "session-token" {
		t.Fatalf("Token = %q, want session-token", result.Token)
	}
	if result.User.ID != 1 {
		t.Fatalf("User.ID = %d, want 1", result.User.ID)
	}

	auth, err := service.AuthenticateSession(context.Background(), result.Token)
	if err != nil {
		t.Fatalf("AuthenticateSession() error = %v", err)
	}
	if !auth.Authenticated {
		t.Fatal("Authenticated = false, want true")
	}
	if auth.User.Username != "owner" {
		t.Fatalf("Username = %q, want owner", auth.User.Username)
	}
}

func TestAuthServiceRejectsInvalidCredentials(t *testing.T) {
	service := NewAuthService(newFakeAuthRepository(time.Now()), testAuthConfig())

	if _, err := service.LocalLogin(context.Background(), LocalLoginInput{Username: "owner", Password: "wrong"}); err == nil {
		t.Fatal("LocalLogin() error = nil, want invalid credentials error")
	}
}

func TestAuthServiceRateLimitsLocalLoginAttempts(t *testing.T) {
	now := time.Date(2026, 6, 23, 10, 30, 0, 0, time.UTC)
	service := NewAuthService(newFakeAuthRepository(now), testAuthConfig(), WithAuthNow(func() time.Time { return now }))

	for i := 0; i < authAttemptLimit; i++ {
		_, err := service.LocalLogin(context.Background(), LocalLoginInput{
			Username:      "owner",
			Password:      "wrong",
			RemoteAddress: "127.0.0.1",
		})
		if domain.ClassifyError(err) != domain.ErrorKindInvalidInput {
			t.Fatalf("attempt %d error kind = %s, want invalid_input", i+1, domain.ClassifyError(err))
		}
	}

	_, err := service.LocalLogin(context.Background(), LocalLoginInput{
		Username:      "owner",
		Password:      "secret",
		RemoteAddress: "127.0.0.1",
	})
	if domain.ClassifyError(err) != domain.ErrorKindRateLimited {
		t.Fatalf("error kind = %s, want rate_limited", domain.ClassifyError(err))
	}
}

func TestAuthServiceDefaultOwnerMigrationPasswordHash(t *testing.T) {
	const migrationHash = "$2a$10$DTKcuvnsad7405UJYtMIxOQDrpO6PN5bQJGgwgJDlJz8AIkcYicYO"
	if err := verifyPassword(migrationHash, "***REMOVED-FROM-GIT-HISTORY***"); err != nil {
		t.Fatalf("verifyPassword() error = %v", err)
	}
}

func TestAuthServiceWeChatWorkOAuthBind(t *testing.T) {
	now := time.Date(2026, 6, 23, 11, 0, 0, 0, time.UTC)
	repository := newFakeAuthRepository(now)
	cfg := testAuthConfig()
	oauth := fakeWeChatWorkOAuth{user: WeChatWorkOAuthUser{UserID: "aroen"}}
	service := NewAuthService(
		repository,
		cfg,
		WithAuthNow(func() time.Time { return now }),
		WithAuthRandomToken(func() (string, error) { return "fixed-token", nil }),
		WithAuthWeChatWorkOAuth(oauth),
	)

	oauthURL, err := service.BuildWeChatWorkOAuthURL(context.Background(), WeChatWorkOAuthURLInput{
		UserID:       1,
		Purpose:      "bind",
		RedirectPath: "/settings",
	})
	if err != nil {
		t.Fatalf("BuildWeChatWorkOAuthURL() error = %v", err)
	}
	if !strings.Contains(oauthURL.URL, "state=fixed-token") {
		t.Fatalf("oauth url does not contain state: %s", oauthURL.URL)
	}

	result, err := service.HandleWeChatWorkOAuthCallback(context.Background(), WeChatWorkOAuthCallbackInput{
		Code:  "oauth-code",
		State: "fixed-token",
	})
	if err != nil {
		t.Fatalf("HandleWeChatWorkOAuthCallback() error = %v", err)
	}
	if result.Binding.ExternalUserID != "aroen" {
		t.Fatalf("ExternalUserID = %q, want aroen", result.Binding.ExternalUserID)
	}
	if result.RedirectPath != "/settings" {
		t.Fatalf("RedirectPath = %q, want /settings", result.RedirectPath)
	}
}

func TestAuthServiceRegisterWithInvite(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	repository := newFakeAuthRepository(now)
	codeHash := hashSecret("invite-code")
	repository.invites[codeHash] = domain.AuthInviteCode{
		ID:          22,
		CodeHash:    codeHash,
		CreatedByID: 1,
		Role:        domain.UserRoleUser,
		MaxUses:     1,
		Status:      domain.AuthInviteCodeStatusActive,
		ExpiresAt:   ptrTime(now.Add(time.Hour)),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	service := NewAuthService(repository, testAuthConfig(), WithAuthNow(func() time.Time { return now }), WithAuthRandomToken(func() (string, error) {
		return "registered-session", nil
	}))

	result, err := service.RegisterWithInvite(context.Background(), RegisterWithInviteInput{
		InviteCode: "invite-code",
		Username:   "new_user",
		Password:   "strong-password",
	})
	if err != nil {
		t.Fatalf("RegisterWithInvite() error = %v", err)
	}
	if result.User.Username != "new_user" {
		t.Fatalf("Username = %q, want new_user", result.User.Username)
	}
	if repository.invites[codeHash].UseCount != 1 {
		t.Fatalf("UseCount = %d, want 1", repository.invites[codeHash].UseCount)
	}
}

func TestAuthServiceRegisterWithSixCharacterPassword(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 10, 0, 0, time.UTC)
	repository := newFakeAuthRepository(now)
	codeHash := hashSecret("short-password-invite")
	repository.invites[codeHash] = domain.AuthInviteCode{
		ID:          23,
		CodeHash:    codeHash,
		CreatedByID: 1,
		Role:        domain.UserRoleUser,
		MaxUses:     1,
		Status:      domain.AuthInviteCodeStatusActive,
		ExpiresAt:   ptrTime(now.Add(time.Hour)),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	service := NewAuthService(repository, testAuthConfig(), WithAuthNow(func() time.Time { return now }), WithAuthRandomToken(func() (string, error) {
		return "registered-short-password-session", nil
	}))

	result, err := service.RegisterWithInvite(context.Background(), RegisterWithInviteInput{
		InviteCode:    "short-password-invite",
		Username:      "new_user",
		Password:      "***REMOVED-FROM-GIT-HISTORY***",
		RemoteAddress: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("RegisterWithInvite() error = %v", err)
	}
	if result.User.Username != "new_user" {
		t.Fatalf("Username = %q, want new_user", result.User.Username)
	}
}

func TestAuthServiceRateLimitsRegisterAttempts(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 20, 0, 0, time.UTC)
	service := NewAuthService(newFakeAuthRepository(now), testAuthConfig(), WithAuthNow(func() time.Time { return now }))

	for i := 0; i < authAttemptLimit; i++ {
		_, err := service.RegisterWithInvite(context.Background(), RegisterWithInviteInput{
			Username:      "new_user",
			Password:      "***REMOVED-FROM-GIT-HISTORY***",
			RemoteAddress: "127.0.0.1",
		})
		if domain.ClassifyError(err) != domain.ErrorKindInvalidInput {
			t.Fatalf("attempt %d error kind = %s, want invalid_input", i+1, domain.ClassifyError(err))
		}
	}

	_, err := service.RegisterWithInvite(context.Background(), RegisterWithInviteInput{
		InviteCode:    "invite-code",
		Username:      "new_user",
		Password:      "***REMOVED-FROM-GIT-HISTORY***",
		RemoteAddress: "127.0.0.1",
	})
	if domain.ClassifyError(err) != domain.ErrorKindRateLimited {
		t.Fatalf("error kind = %s, want rate_limited", domain.ClassifyError(err))
	}
}

func TestAuthServiceChangePassword(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 30, 0, 0, time.UTC)
	repository := newFakeAuthRepository(now)
	service := NewAuthService(repository, testAuthConfig(), WithAuthNow(func() time.Time { return now }), WithAuthRandomToken(func() (string, error) {
		return "changed-session", nil
	}))

	changed, err := service.ChangePassword(context.Background(), ChangePasswordInput{
		UserID:          1,
		CurrentPassword: "secret",
		NewPassword:     "new-secret",
	})
	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if !changed.PasswordConfigured {
		t.Fatal("PasswordConfigured = false, want true")
	}
	if err := verifyPassword(repository.user.PasswordHash, "new-secret"); err != nil {
		t.Fatalf("stored password hash does not verify new password: %v", err)
	}

	if _, err := service.LocalLogin(context.Background(), LocalLoginInput{Username: "owner", Password: "new-secret"}); err != nil {
		t.Fatalf("LocalLogin() with changed password error = %v", err)
	}
}

func TestAuthServiceCreateListAndRevokeInvite(t *testing.T) {
	now := time.Date(2026, 6, 23, 13, 0, 0, 0, time.UTC)
	repository := newFakeAuthRepository(now)
	service := NewAuthService(repository, testAuthConfig(), WithAuthNow(func() time.Time { return now }), WithAuthRandomToken(func() (string, error) {
		return "invite-token", nil
	}))
	auth := CurrentAuth{
		Authenticated: true,
		User:          repository.user,
	}

	created, err := service.CreateInvite(context.Background(), CreateInviteInput{
		Creator:    repository.user,
		TTLSeconds: int64((2 * time.Hour).Seconds()),
	})
	if err != nil {
		t.Fatalf("CreateInvite() error = %v", err)
	}
	if created.Code != "invite-token" {
		t.Fatalf("Code = %q, want invite-token", created.Code)
	}
	if created.Invite.Status != string(domain.AuthInviteCodeStatusActive) {
		t.Fatalf("Invite.Status = %q, want active", created.Invite.Status)
	}
	if created.Invite.MaxUses != 1 {
		t.Fatalf("Invite.MaxUses = %d, want 1", created.Invite.MaxUses)
	}
	if _, ok := repository.invites["invite-token"]; ok {
		t.Fatal("repository stored raw invite code, want hashed code only")
	}
	if _, ok := repository.invites[hashSecret("invite-token")]; !ok {
		t.Fatal("repository did not store hashed invite code")
	}

	invites, err := service.ListInvites(context.Background(), auth)
	if err != nil {
		t.Fatalf("ListInvites() error = %v", err)
	}
	if len(invites) != 1 {
		t.Fatalf("len(invites) = %d, want 1", len(invites))
	}

	revoked, err := service.RevokeInvite(context.Background(), auth, created.Invite.ID)
	if err != nil {
		t.Fatalf("RevokeInvite() error = %v", err)
	}
	if revoked.Status != string(domain.AuthInviteCodeStatusRevoked) {
		t.Fatalf("revoked.Status = %q, want revoked", revoked.Status)
	}
}

func TestAuthServiceRejectsMultiUseInvite(t *testing.T) {
	now := time.Date(2026, 6, 23, 13, 30, 0, 0, time.UTC)
	repository := newFakeAuthRepository(now)
	service := NewAuthService(repository, testAuthConfig(), WithAuthNow(func() time.Time { return now }))

	if _, err := service.CreateInvite(context.Background(), CreateInviteInput{
		Creator: repository.user,
		MaxUses: 2,
	}); err == nil {
		t.Fatal("CreateInvite() error = nil, want invalid input error")
	}
}

func TestAuthServiceUpdateProfileAndBuildUserContext(t *testing.T) {
	now := time.Date(2026, 6, 23, 14, 0, 0, 0, time.UTC)
	repository := newFakeAuthRepository(now)
	service := NewAuthService(repository, testAuthConfig(), WithAuthNow(func() time.Time { return now }))

	profile, err := service.UpdateProfile(context.Background(), UpdateProfileInput{
		UserID:          1,
		DisplayName:     "Aroen",
		Email:           "aroen@example.com",
		TimeZone:        "Asia/Shanghai",
		Language:        "zh-CN",
		FocusTopics:     []string{"AI", "AI", "金融"},
		InstrumentFocus: []string{"AAPL"},
		ReplyStyle:      "plain_text_short",
	})
	if err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
	if profile.DisplayName != "Aroen" {
		t.Fatalf("DisplayName = %q, want Aroen", profile.DisplayName)
	}

	ctxResult, err := service.GetUserContext(context.Background(), CurrentAuth{Authenticated: true, User: repository.user})
	if err != nil {
		t.Fatalf("GetUserContext() error = %v", err)
	}
	if ctxResult.DataScope.UserID != 1 {
		t.Fatalf("DataScope.UserID = %d, want 1", ctxResult.DataScope.UserID)
	}
	if !strings.Contains(ctxResult.Prompt.PlainText, "只能读取和操作 user_id=1") {
		t.Fatalf("Prompt does not contain user data boundary: %q", ctxResult.Prompt.PlainText)
	}
}

func TestAuthServiceListAndRevokeSessions(t *testing.T) {
	now := time.Date(2026, 6, 23, 15, 0, 0, 0, time.UTC)
	repository := newFakeAuthRepository(now)
	service := NewAuthService(repository, testAuthConfig(), WithAuthNow(func() time.Time { return now }), WithAuthRandomToken(func() (string, error) {
		return "session-token", nil
	}))

	login, err := service.LocalLogin(context.Background(), LocalLoginInput{Username: "owner", Password: "secret"})
	if err != nil {
		t.Fatalf("LocalLogin() error = %v", err)
	}
	auth := CurrentAuth{Authenticated: true, User: login.User, Session: login.Session}
	sessions, err := service.ListSessions(context.Background(), auth)
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(sessions) != 1 || !sessions[0].Current {
		t.Fatalf("sessions = %#v, want one current session", sessions)
	}
	if err := service.RevokeSession(context.Background(), auth, login.Session.ID); err != nil {
		t.Fatalf("RevokeSession() error = %v", err)
	}
	sessions, err = service.ListSessions(context.Background(), auth)
	if err != nil {
		t.Fatalf("ListSessions() after revoke error = %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("len(sessions) = %d, want 0", len(sessions))
	}
}

func TestAuthServiceDeactivateAccountRejectsOwner(t *testing.T) {
	now := time.Date(2026, 6, 23, 16, 0, 0, 0, time.UTC)
	repository := newFakeAuthRepository(now)
	service := NewAuthService(repository, testAuthConfig(), WithAuthNow(func() time.Time { return now }))

	if err := service.DeactivateAccount(context.Background(), DeactivateAccountInput{UserID: 1, CurrentPassword: "secret"}); err == nil {
		t.Fatal("DeactivateAccount() error = nil, want owner protection error")
	}
}

func TestAuthServiceResolveExternalAccount(t *testing.T) {
	now := time.Date(2026, 6, 23, 17, 0, 0, 0, time.UTC)
	repository := newFakeAuthRepository(now)
	account := domain.ExternalAccount{
		ID:             9,
		UserID:         1,
		Provider:       domain.AgentProviderWeChatWorkApp,
		CorpID:         "corp-a",
		AgentID:        "1000002",
		ExternalUserID: "zhangsan",
		BindingStatus:  domain.ExternalAccountBindingStatusActive,
	}
	repository.accounts[account.ID] = account
	service := NewAuthService(repository, testAuthConfig(), WithAuthNow(func() time.Time { return now }))

	resolved, err := service.ResolveExternalAccount(context.Background(), domain.AgentProviderWeChatWorkApp, "corp-a", "1000002", "zhangsan")
	if err != nil {
		t.Fatalf("ResolveExternalAccount() error = %v", err)
	}
	if resolved.UserID != 1 {
		t.Fatalf("UserID = %d, want 1", resolved.UserID)
	}
	if repository.accounts[account.ID].LastSeenAt == nil {
		t.Fatal("LastSeenAt was not updated")
	}
}

func testAuthConfig() config.Config {
	cfg := config.Defaults()
	cfg.Runtime.PublicBaseURL = "https://messagefeed.example"
	cfg.Auth.OwnerUsername = "owner"
	cfg.Auth.OwnerPassword = "secret"
	cfg.Auth.SessionTTL = time.Hour
	cfg.Auth.OAuthStateTTL = 10 * time.Minute
	cfg.WeChatWork.CorpID = "ww0123456789abcdef"
	cfg.WeChatWork.AgentID = "1000002"
	cfg.WeChatWork.Secret = "wechat-secret"
	cfg.WeChatWork.CallbackToken = "token"
	cfg.WeChatWork.EncodingAESKey = "abcdefghijklmnopqrstuvwxyzABCDEFG1234567890"
	return cfg
}

type fakeWeChatWorkOAuth struct {
	user WeChatWorkOAuthUser
	err  error
}

func (f fakeWeChatWorkOAuth) ExchangeCode(ctx context.Context, code string) (WeChatWorkOAuthUser, error) {
	if f.err != nil {
		return WeChatWorkOAuthUser{}, f.err
	}
	return f.user, nil
}

type fakeAuthRepository struct {
	now      time.Time
	nextID   int64
	user     domain.User
	sessions map[string]domain.UserSession
	states   map[string]domain.AuthOAuthState
	accounts map[int64]domain.ExternalAccount
	invites  map[string]domain.AuthInviteCode
	profiles map[int64]domain.UserProfile
}

func newFakeAuthRepository(now time.Time) *fakeAuthRepository {
	return &fakeAuthRepository{
		now:    now,
		nextID: 1,
		user: domain.User{
			ID:          1,
			Username:    "owner",
			DisplayName: "owner",
			Role:        domain.UserRoleOwner,
			Status:      domain.UserStatusActive,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		sessions: map[string]domain.UserSession{},
		states:   map[string]domain.AuthOAuthState{},
		accounts: map[int64]domain.ExternalAccount{},
		invites:  map[string]domain.AuthInviteCode{},
		profiles: map[int64]domain.UserProfile{},
	}
}

func (r *fakeAuthRepository) EnsureOwner(ctx context.Context, username string) (domain.User, error) {
	r.user = domain.User{
		ID:          1,
		Username:    username,
		DisplayName: username,
		Role:        domain.UserRoleOwner,
		Status:      domain.UserStatusActive,
		CreatedAt:   r.now,
		UpdatedAt:   r.now,
	}
	return r.user, nil
}

func (r *fakeAuthRepository) GetUserByID(ctx context.Context, userID int64) (domain.User, error) {
	if r.user.ID == userID {
		return r.user, nil
	}
	return domain.User{}, domain.ErrNotFound
}

func (r *fakeAuthRepository) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
	if r.user.Username == username {
		return r.user, nil
	}
	return domain.User{}, domain.ErrNotFound
}

func (r *fakeAuthRepository) ListUsers(ctx context.Context) ([]domain.User, error) {
	return []domain.User{r.user}, nil
}

func (r *fakeAuthRepository) UpdateUserInfo(ctx context.Context, userID int64, displayName string, email string, now time.Time) (domain.User, error) {
	if r.user.ID != userID {
		return domain.User{}, domain.ErrNotFound
	}
	r.user.DisplayName = displayName
	r.user.Email = email
	r.user.UpdatedAt = now
	return r.user, nil
}

func (r *fakeAuthRepository) UpdateUserPassword(ctx context.Context, userID int64, passwordHash string, now time.Time) (domain.User, error) {
	if r.user.ID != userID {
		return domain.User{}, domain.ErrNotFound
	}
	r.user.PasswordHash = passwordHash
	r.user.UpdatedAt = now
	return r.user, nil
}

func (r *fakeAuthRepository) DeactivateUser(ctx context.Context, userID int64, now time.Time) (domain.User, error) {
	if r.user.ID != userID {
		return domain.User{}, domain.ErrNotFound
	}
	r.user.Status = domain.UserStatusDeleted
	r.user.UpdatedAt = now
	for tokenHash, session := range r.sessions {
		if session.UserID == userID && session.RevokedAt == nil {
			session.RevokedAt = &now
			r.sessions[tokenHash] = session
		}
	}
	return r.user, nil
}

func (r *fakeAuthRepository) GetUserProfile(ctx context.Context, userID int64) (domain.UserProfile, error) {
	profile, ok := r.profiles[userID]
	if !ok {
		return domain.UserProfile{}, domain.ErrNotFound
	}
	return profile, nil
}

func (r *fakeAuthRepository) UpsertUserProfile(ctx context.Context, profile domain.UserProfile) (domain.UserProfile, error) {
	r.profiles[profile.UserID] = profile
	return profile, nil
}

func (r *fakeAuthRepository) CreateSession(ctx context.Context, session domain.UserSession) (domain.UserSession, error) {
	session.ID = r.nextID
	r.nextID++
	r.sessions[session.SessionTokenHash] = session
	return session, nil
}

func (r *fakeAuthRepository) GetSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) (domain.UserSession, error) {
	session, ok := r.sessions[tokenHash]
	if !ok || session.ExpiresAt.Before(now) || session.RevokedAt != nil {
		return domain.UserSession{}, domain.ErrNotFound
	}
	return session, nil
}

func (r *fakeAuthRepository) ListSessions(ctx context.Context, userID int64, now time.Time) ([]domain.UserSession, error) {
	sessions := make([]domain.UserSession, 0, len(r.sessions))
	for _, session := range r.sessions {
		if session.UserID == userID && session.RevokedAt == nil && session.ExpiresAt.After(now) {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

func (r *fakeAuthRepository) TouchSession(ctx context.Context, sessionID int64, now time.Time) error {
	return nil
}

func (r *fakeAuthRepository) RevokeSessionByID(ctx context.Context, userID int64, sessionID int64, now time.Time) error {
	for tokenHash, session := range r.sessions {
		if session.ID == sessionID && session.UserID == userID && session.RevokedAt == nil {
			session.RevokedAt = &now
			r.sessions[tokenHash] = session
			return nil
		}
	}
	return domain.ErrNotFound
}

func (r *fakeAuthRepository) RevokeSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) error {
	session, ok := r.sessions[tokenHash]
	if !ok {
		return nil
	}
	session.RevokedAt = &now
	r.sessions[tokenHash] = session
	return nil
}

func (r *fakeAuthRepository) CreateOAuthState(ctx context.Context, state domain.AuthOAuthState) (domain.AuthOAuthState, error) {
	state.ID = r.nextID
	r.nextID++
	r.states[state.StateHash] = state
	return state, nil
}

func (r *fakeAuthRepository) ConsumeOAuthState(ctx context.Context, stateHash string, now time.Time) (domain.AuthOAuthState, error) {
	state, ok := r.states[stateHash]
	if !ok || state.ExpiresAt.Before(now) || state.ConsumedAt != nil {
		return domain.AuthOAuthState{}, domain.ErrNotFound
	}
	state.ConsumedAt = &now
	r.states[stateHash] = state
	return state, nil
}

func (r *fakeAuthRepository) BindExternalAccount(ctx context.Context, account domain.ExternalAccount) (domain.ExternalAccount, error) {
	if account.UserID < 1 || account.ExternalUserID == "" {
		return domain.ExternalAccount{}, errors.New("invalid account")
	}
	account.ID = r.nextID
	r.nextID++
	r.accounts[account.ID] = account
	return account, nil
}

func (r *fakeAuthRepository) ListExternalAccounts(ctx context.Context, userID int64) ([]domain.ExternalAccount, error) {
	accounts := make([]domain.ExternalAccount, 0, len(r.accounts))
	for _, account := range r.accounts {
		if account.UserID == userID {
			accounts = append(accounts, account)
		}
	}
	return accounts, nil
}

func (r *fakeAuthRepository) GetExternalAccountByIdentity(ctx context.Context, provider string, corpID string, agentID string, externalUserID string) (domain.ExternalAccount, error) {
	for _, account := range r.accounts {
		if account.Provider == provider && account.CorpID == corpID && account.AgentID == agentID && account.ExternalUserID == externalUserID {
			return account, nil
		}
	}
	return domain.ExternalAccount{}, domain.ErrNotFound
}

func (r *fakeAuthRepository) TouchExternalAccount(ctx context.Context, accountID int64, now time.Time) error {
	account, ok := r.accounts[accountID]
	if !ok {
		return domain.ErrNotFound
	}
	account.LastSeenAt = &now
	r.accounts[accountID] = account
	return nil
}

func (r *fakeAuthRepository) DisableExternalAccount(ctx context.Context, userID int64, accountID int64, now time.Time) (domain.ExternalAccount, error) {
	account, ok := r.accounts[accountID]
	if !ok || account.UserID != userID {
		return domain.ExternalAccount{}, domain.ErrNotFound
	}
	account.BindingStatus = domain.ExternalAccountBindingStatusDisabled
	r.accounts[accountID] = account
	return account, nil
}

func (r *fakeAuthRepository) CreateInviteCode(ctx context.Context, invite domain.AuthInviteCode) (domain.AuthInviteCode, error) {
	invite.ID = r.nextID
	r.nextID++
	r.invites[invite.CodeHash] = invite
	return invite, nil
}

func (r *fakeAuthRepository) ListInviteCodes(ctx context.Context, createdByUserID int64) ([]domain.AuthInviteCode, error) {
	invites := make([]domain.AuthInviteCode, 0, len(r.invites))
	for _, invite := range r.invites {
		if invite.CreatedByID == createdByUserID {
			invites = append(invites, invite)
		}
	}
	return invites, nil
}

func (r *fakeAuthRepository) RevokeInviteCode(ctx context.Context, createdByUserID int64, inviteID int64, now time.Time) (domain.AuthInviteCode, error) {
	for codeHash, invite := range r.invites {
		if invite.ID == inviteID && invite.CreatedByID == createdByUserID {
			invite.Status = domain.AuthInviteCodeStatusRevoked
			invite.UpdatedAt = now
			r.invites[codeHash] = invite
			return invite, nil
		}
	}
	return domain.AuthInviteCode{}, domain.ErrNotFound
}

func (r *fakeAuthRepository) CreateUserWithInvite(ctx context.Context, codeHash string, user domain.User, redemption domain.AuthInviteRedemption, now time.Time) (domain.User, domain.AuthInviteCode, error) {
	invite, ok := r.invites[codeHash]
	if !ok ||
		invite.Status != domain.AuthInviteCodeStatusActive ||
		invite.UseCount >= invite.MaxUses ||
		(invite.ExpiresAt != nil && !now.Before(*invite.ExpiresAt)) {
		return domain.User{}, domain.AuthInviteCode{}, domain.ErrConflict
	}
	user.ID = r.nextID
	r.nextID++
	user.Role = invite.Role
	user.Status = domain.UserStatusActive
	user.CreatedAt = now
	user.UpdatedAt = now
	r.user = user
	invite.UseCount++
	invite.UpdatedAt = now
	r.invites[codeHash] = invite
	return user, invite, nil
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
