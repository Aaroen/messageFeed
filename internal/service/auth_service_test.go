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

func (r *fakeAuthRepository) UpdateUserPassword(ctx context.Context, userID int64, passwordHash string, now time.Time) (domain.User, error) {
	if r.user.ID != userID {
		return domain.User{}, domain.ErrNotFound
	}
	r.user.PasswordHash = passwordHash
	r.user.UpdatedAt = now
	return r.user, nil
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

func (r *fakeAuthRepository) TouchSession(ctx context.Context, sessionID int64, now time.Time) error {
	return nil
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
