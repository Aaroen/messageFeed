package repository

import (
	"context"
	"messagefeed/internal/domain"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AuthRepository struct {
	db *gorm.DB
}

func NewAuthRepository(db *gorm.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

type userModel struct {
	ID          int64 `gorm:"primaryKey"`
	Username    string
	Email       string
	DisplayName string
	Role        string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type userSessionModel struct {
	ID               int64 `gorm:"primaryKey"`
	UserID           int64 `gorm:"not null"`
	SessionTokenHash string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	UserAgentHash    string
	IPAddress        string
	LastSeenAt       time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type authOAuthStateModel struct {
	ID           int64 `gorm:"primaryKey"`
	StateHash    string
	UserID       int64 `gorm:"not null"`
	Provider     string
	Purpose      string
	RedirectPath string
	CorpID       string
	AgentID      string
	ExpiresAt    time.Time
	ConsumedAt   *time.Time
	Metadata     domain.AgentJSON `gorm:"column:metadata_json;serializer:json;type:jsonb;not null"`
	CreatedAt    time.Time
}

func (userModel) TableName() string           { return "users" }
func (userSessionModel) TableName() string    { return "user_sessions" }
func (authOAuthStateModel) TableName() string { return "auth_oauth_states" }

func (r *AuthRepository) EnsureOwner(ctx context.Context, username string) (domain.User, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.user.ensure_owner", "upsert", "users")
	var opErr error
	defer func() { finish(opErr) }()

	username = strings.TrimSpace(username)
	if username == "" {
		username = "owner"
	}
	now := time.Now().UTC()
	model := userModel{
		ID:          1,
		Username:    username,
		Email:       "owner@messagefeed.local",
		DisplayName: username,
		Role:        string(domain.UserRoleOwner),
		Status:      string(domain.UserStatusActive),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"username":     model.Username,
				"display_name": model.DisplayName,
				"role":         model.Role,
				"status":       model.Status,
				"updated_at":   now,
			}),
		}).
		Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.User{}, opErr
	}

	var persisted userModel
	if err := r.db.WithContext(ctx).Where("id = ?", model.ID).First(&persisted).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.User{}, opErr
	}
	return userModelToDomain(persisted), nil
}

func (r *AuthRepository) GetUserByID(ctx context.Context, userID int64) (domain.User, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.user.get", "select", "users")
	var opErr error
	defer func() { finish(opErr) }()

	var model userModel
	if err := r.db.WithContext(ctx).Where("id = ?", userID).First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.User{}, opErr
	}
	return userModelToDomain(model), nil
}

func (r *AuthRepository) CreateSession(ctx context.Context, session domain.UserSession) (domain.UserSession, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.session.create", "insert", "user_sessions")
	var opErr error
	defer func() { finish(opErr) }()

	model := userSessionModelFromDomain(normalizeUserSession(session))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.UserSession{}, opErr
	}
	return userSessionModelToDomain(model), nil
}

func (r *AuthRepository) GetSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) (domain.UserSession, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.session.get", "select", "user_sessions")
	var opErr error
	defer func() { finish(opErr) }()

	var model userSessionModel
	if err := r.db.WithContext(ctx).
		Where("session_token_hash = ? AND revoked_at IS NULL AND expires_at > ?", strings.TrimSpace(tokenHash), now.UTC()).
		First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.UserSession{}, opErr
	}
	return userSessionModelToDomain(model), nil
}

func (r *AuthRepository) TouchSession(ctx context.Context, sessionID int64, now time.Time) error {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.session.touch", "update", "user_sessions")
	var opErr error
	defer func() { finish(opErr) }()

	err := r.db.WithContext(ctx).
		Model(&userSessionModel{}).
		Where("id = ? AND revoked_at IS NULL", sessionID).
		Update("last_seen_at", now.UTC()).Error
	if err != nil {
		opErr = mapRepositoryError(err)
	}
	return opErr
}

func (r *AuthRepository) RevokeSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) error {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.session.revoke", "update", "user_sessions")
	var opErr error
	defer func() { finish(opErr) }()

	err := r.db.WithContext(ctx).
		Model(&userSessionModel{}).
		Where("session_token_hash = ? AND revoked_at IS NULL", strings.TrimSpace(tokenHash)).
		Update("revoked_at", now.UTC()).Error
	if err != nil {
		opErr = mapRepositoryError(err)
	}
	return opErr
}

func (r *AuthRepository) CreateOAuthState(ctx context.Context, state domain.AuthOAuthState) (domain.AuthOAuthState, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.oauth_state.create", "insert", "auth_oauth_states")
	var opErr error
	defer func() { finish(opErr) }()

	model := authOAuthStateModelFromDomain(normalizeAuthOAuthState(state))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AuthOAuthState{}, opErr
	}
	return authOAuthStateModelToDomain(model), nil
}

func (r *AuthRepository) ConsumeOAuthState(ctx context.Context, stateHash string, now time.Time) (domain.AuthOAuthState, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.oauth_state.consume", "update", "auth_oauth_states")
	var opErr error
	defer func() { finish(opErr) }()

	var model authOAuthStateModel
	result := r.db.WithContext(ctx).
		Model(&model).
		Clauses(clause.Returning{}).
		Where("state_hash = ? AND consumed_at IS NULL AND expires_at > ?", strings.TrimSpace(stateHash), now.UTC()).
		Updates(map[string]any{"consumed_at": now.UTC()})
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.AuthOAuthState{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.AuthOAuthState{}, opErr
	}
	return authOAuthStateModelToDomain(model), nil
}

func (r *AuthRepository) BindExternalAccount(ctx context.Context, account domain.ExternalAccount) (domain.ExternalAccount, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.external_account.bind", "upsert", "external_accounts")
	var opErr error
	defer func() { finish(opErr) }()

	account = normalizeExternalAccount(account)
	model := externalAccountModelFromDomain(account)
	now := time.Now().UTC()
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "provider"},
				{Name: "corp_id"},
				{Name: "agent_id"},
				{Name: "external_user_id"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"user_id":        model.UserID,
				"display_name":   model.DisplayName,
				"binding_status": string(domain.ExternalAccountBindingStatusActive),
				"verified_at":    model.VerifiedAt,
				"last_seen_at":   model.LastSeenAt,
				"updated_at":     now,
			}),
		}).
		Create(&model).Error
	if err != nil {
		opErr = mapRepositoryError(err)
		return domain.ExternalAccount{}, opErr
	}

	var persisted externalAccountModel
	if err := r.db.WithContext(ctx).
		Where("provider = ? AND corp_id = ? AND agent_id = ? AND external_user_id = ?", account.Provider, account.CorpID, account.AgentID, account.ExternalUserID).
		First(&persisted).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.ExternalAccount{}, opErr
	}
	return externalAccountModelToDomain(persisted), nil
}

func (r *AuthRepository) ListExternalAccounts(ctx context.Context, userID int64) ([]domain.ExternalAccount, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.external_account.list", "select", "external_accounts")
	var opErr error
	defer func() { finish(opErr) }()

	var models []externalAccountModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("updated_at DESC, id DESC").
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	accounts := make([]domain.ExternalAccount, 0, len(models))
	for _, model := range models {
		accounts = append(accounts, externalAccountModelToDomain(model))
	}
	return accounts, nil
}

func (r *AuthRepository) DisableExternalAccount(ctx context.Context, userID int64, accountID int64, now time.Time) (domain.ExternalAccount, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.external_account.disable", "update", "external_accounts")
	var opErr error
	defer func() { finish(opErr) }()

	var model externalAccountModel
	result := r.db.WithContext(ctx).
		Model(&model).
		Clauses(clause.Returning{}).
		Where("id = ? AND user_id = ?", accountID, userID).
		Updates(map[string]any{
			"binding_status": string(domain.ExternalAccountBindingStatusDisabled),
			"updated_at":     now.UTC(),
		})
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.ExternalAccount{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.ExternalAccount{}, opErr
	}
	return externalAccountModelToDomain(model), nil
}

func normalizeUserSession(session domain.UserSession) domain.UserSession {
	session.SessionTokenHash = strings.TrimSpace(session.SessionTokenHash)
	session.UserAgentHash = strings.TrimSpace(session.UserAgentHash)
	session.IPAddress = strings.TrimSpace(session.IPAddress)
	return session
}

func normalizeAuthOAuthState(state domain.AuthOAuthState) domain.AuthOAuthState {
	state.StateHash = strings.TrimSpace(state.StateHash)
	state.Provider = strings.TrimSpace(state.Provider)
	state.RedirectPath = strings.TrimSpace(state.RedirectPath)
	state.CorpID = strings.TrimSpace(state.CorpID)
	state.AgentID = strings.TrimSpace(state.AgentID)
	if !state.Purpose.Valid() {
		state.Purpose = domain.AuthOAuthPurposeBind
	}
	if state.Metadata == nil {
		state.Metadata = domain.AgentJSON{}
	}
	return state
}

func userModelToDomain(model userModel) domain.User {
	return domain.User{
		ID:          model.ID,
		Username:    model.Username,
		Email:       model.Email,
		DisplayName: model.DisplayName,
		Role:        domain.UserRole(model.Role),
		Status:      domain.UserStatus(model.Status),
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}
}

func userSessionModelFromDomain(session domain.UserSession) userSessionModel {
	return userSessionModel{
		ID:               session.ID,
		UserID:           session.UserID,
		SessionTokenHash: session.SessionTokenHash,
		ExpiresAt:        session.ExpiresAt,
		RevokedAt:        session.RevokedAt,
		UserAgentHash:    session.UserAgentHash,
		IPAddress:        session.IPAddress,
		LastSeenAt:       session.LastSeenAt,
		CreatedAt:        session.CreatedAt,
		UpdatedAt:        session.UpdatedAt,
	}
}

func userSessionModelToDomain(model userSessionModel) domain.UserSession {
	return domain.UserSession{
		ID:               model.ID,
		UserID:           model.UserID,
		SessionTokenHash: model.SessionTokenHash,
		ExpiresAt:        model.ExpiresAt,
		RevokedAt:        model.RevokedAt,
		UserAgentHash:    model.UserAgentHash,
		IPAddress:        model.IPAddress,
		LastSeenAt:       model.LastSeenAt,
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
	}
}

func authOAuthStateModelFromDomain(state domain.AuthOAuthState) authOAuthStateModel {
	return authOAuthStateModel{
		ID:           state.ID,
		StateHash:    state.StateHash,
		UserID:       state.UserID,
		Provider:     state.Provider,
		Purpose:      string(state.Purpose),
		RedirectPath: state.RedirectPath,
		CorpID:       state.CorpID,
		AgentID:      state.AgentID,
		ExpiresAt:    state.ExpiresAt,
		ConsumedAt:   state.ConsumedAt,
		Metadata:     cloneAgentJSON(state.Metadata),
		CreatedAt:    state.CreatedAt,
	}
}

func authOAuthStateModelToDomain(model authOAuthStateModel) domain.AuthOAuthState {
	return domain.AuthOAuthState{
		ID:           model.ID,
		StateHash:    model.StateHash,
		UserID:       model.UserID,
		Provider:     model.Provider,
		Purpose:      domain.AuthOAuthPurpose(model.Purpose),
		RedirectPath: model.RedirectPath,
		CorpID:       model.CorpID,
		AgentID:      model.AgentID,
		ExpiresAt:    model.ExpiresAt,
		ConsumedAt:   model.ConsumedAt,
		Metadata:     cloneAgentJSON(model.Metadata),
		CreatedAt:    model.CreatedAt,
	}
}
