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
	ID           int64 `gorm:"primaryKey"`
	Username     string
	Email        string
	DisplayName  string
	PasswordHash string
	Role         string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type userProfileModel struct {
	UserID                 int64 `gorm:"primaryKey"`
	TimeZone               string
	Language               string
	Region                 string
	Bio                    string
	FocusTopics            []string `gorm:"serializer:json;type:jsonb;not null"`
	BlockedTopics          []string `gorm:"serializer:json;type:jsonb;not null"`
	MarketFocus            []string `gorm:"serializer:json;type:jsonb;not null"`
	InstrumentFocus        []string `gorm:"serializer:json;type:jsonb;not null"`
	RiskPreference         string
	NotificationQuietHours string
	AgentNotes             string
	ReplyStyle             string
	CreatedAt              time.Time
	UpdatedAt              time.Time
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

type authInviteCodeModel struct {
	ID              int64 `gorm:"primaryKey"`
	CodeHash        string
	CreatedByUserID int64 `gorm:"column:created_by_user_id;not null"`
	Role            string
	MaxUses         int
	UseCount        int
	Status          string
	ExpiresAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type authInviteRedemptionModel struct {
	ID            int64 `gorm:"primaryKey"`
	InviteCodeID  int64 `gorm:"not null"`
	UserID        int64 `gorm:"not null"`
	RedeemedAt    time.Time
	IPAddress     string
	UserAgentHash string
}

func (userModel) TableName() string                 { return "users" }
func (userProfileModel) TableName() string          { return "user_profiles" }
func (userSessionModel) TableName() string          { return "user_sessions" }
func (authOAuthStateModel) TableName() string       { return "auth_oauth_states" }
func (authInviteCodeModel) TableName() string       { return "auth_invite_codes" }
func (authInviteRedemptionModel) TableName() string { return "auth_invite_redemptions" }

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

func (r *AuthRepository) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.user.get_by_username", "select", "users")
	var opErr error
	defer func() { finish(opErr) }()

	var model userModel
	if err := r.db.WithContext(ctx).
		Where("username = ?", strings.TrimSpace(username)).
		First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.User{}, opErr
	}
	return userModelToDomain(model), nil
}

func (r *AuthRepository) ListUsers(ctx context.Context) ([]domain.User, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.user.list", "select", "users")
	var opErr error
	defer func() { finish(opErr) }()

	var models []userModel
	if err := r.db.WithContext(ctx).
		Order("id ASC").
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	users := make([]domain.User, 0, len(models))
	for _, model := range models {
		users = append(users, userModelToDomain(model))
	}
	return users, nil
}

func (r *AuthRepository) UpdateUserInfo(ctx context.Context, userID int64, displayName string, email string, now time.Time) (domain.User, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.user.update_info", "update", "users")
	var opErr error
	defer func() { finish(opErr) }()

	var model userModel
	result := r.db.WithContext(ctx).
		Model(&model).
		Clauses(clause.Returning{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"display_name": strings.TrimSpace(displayName),
			"email":        strings.TrimSpace(email),
			"updated_at":   now.UTC(),
		})
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.User{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.User{}, opErr
	}
	return userModelToDomain(model), nil
}

func (r *AuthRepository) UpdateUserPassword(ctx context.Context, userID int64, passwordHash string, now time.Time) (domain.User, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.user.update_password", "update", "users")
	var opErr error
	defer func() { finish(opErr) }()

	var model userModel
	result := r.db.WithContext(ctx).
		Model(&model).
		Clauses(clause.Returning{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"password_hash": strings.TrimSpace(passwordHash),
			"updated_at":    now.UTC(),
		})
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.User{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.User{}, opErr
	}
	return userModelToDomain(model), nil
}

func (r *AuthRepository) DeactivateUser(ctx context.Context, userID int64, now time.Time) (domain.User, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.user.deactivate", "update", "users")
	var opErr error
	defer func() { finish(opErr) }()

	var model userModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.
			Model(&model).
			Clauses(clause.Returning{}).
			Where("id = ?", userID).
			Updates(map[string]any{
				"status":     string(domain.UserStatusDeleted),
				"updated_at": now.UTC(),
			})
		if result.Error != nil {
			return mapRepositoryError(result.Error)
		}
		if result.RowsAffected == 0 {
			return domain.ErrNotFound
		}
		if err := tx.
			Model(&userSessionModel{}).
			Where("user_id = ? AND revoked_at IS NULL", userID).
			Update("revoked_at", now.UTC()).Error; err != nil {
			return mapRepositoryError(err)
		}
		return nil
	})
	if err != nil {
		opErr = err
		return domain.User{}, err
	}
	return userModelToDomain(model), nil
}

func (r *AuthRepository) GetUserProfile(ctx context.Context, userID int64) (domain.UserProfile, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.user_profile.get", "select", "user_profiles")
	var opErr error
	defer func() { finish(opErr) }()

	var model userProfileModel
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.UserProfile{}, opErr
	}
	return userProfileModelToDomain(model), nil
}

func (r *AuthRepository) UpsertUserProfile(ctx context.Context, profile domain.UserProfile) (domain.UserProfile, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.user_profile.upsert", "upsert", "user_profiles")
	var opErr error
	defer func() { finish(opErr) }()

	profile = normalizeUserProfile(profile)
	model := userProfileModelFromDomain(profile)
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"timezone",
				"language",
				"region",
				"bio",
				"focus_topics",
				"blocked_topics",
				"market_focus",
				"instrument_focus",
				"risk_preference",
				"notification_quiet_hours",
				"agent_notes",
				"reply_style",
				"updated_at",
			}),
		}).
		Create(&model).Error
	if err != nil {
		opErr = mapRepositoryError(err)
		return domain.UserProfile{}, opErr
	}

	var persisted userProfileModel
	if err := r.db.WithContext(ctx).Where("user_id = ?", model.UserID).First(&persisted).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.UserProfile{}, opErr
	}
	return userProfileModelToDomain(persisted), nil
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

func (r *AuthRepository) ListSessions(ctx context.Context, userID int64, now time.Time) ([]domain.UserSession, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.session.list", "select", "user_sessions")
	var opErr error
	defer func() { finish(opErr) }()

	var models []userSessionModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND revoked_at IS NULL AND expires_at > ?", userID, now.UTC()).
		Order("last_seen_at DESC, id DESC").
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	sessions := make([]domain.UserSession, 0, len(models))
	for _, model := range models {
		sessions = append(sessions, userSessionModelToDomain(model))
	}
	return sessions, nil
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

func (r *AuthRepository) RevokeSessionByID(ctx context.Context, userID int64, sessionID int64, now time.Time) error {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.session.revoke_by_id", "update", "user_sessions")
	var opErr error
	defer func() { finish(opErr) }()

	result := r.db.WithContext(ctx).
		Model(&userSessionModel{}).
		Where("id = ? AND user_id = ? AND revoked_at IS NULL", sessionID, userID).
		Update("revoked_at", now.UTC())
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
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

func (r *AuthRepository) GetExternalAccountByIdentity(ctx context.Context, provider string, corpID string, agentID string, externalUserID string) (domain.ExternalAccount, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.external_account.get_by_identity", "select", "external_accounts")
	var opErr error
	defer func() { finish(opErr) }()

	var model externalAccountModel
	if err := r.db.WithContext(ctx).
		Where(
			"provider = ? AND corp_id = ? AND agent_id = ? AND external_user_id = ?",
			strings.TrimSpace(provider),
			strings.TrimSpace(corpID),
			strings.TrimSpace(agentID),
			strings.TrimSpace(externalUserID),
		).
		First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.ExternalAccount{}, opErr
	}
	return externalAccountModelToDomain(model), nil
}

func (r *AuthRepository) TouchExternalAccount(ctx context.Context, accountID int64, now time.Time) error {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.external_account.touch", "update", "external_accounts")
	var opErr error
	defer func() { finish(opErr) }()

	result := r.db.WithContext(ctx).
		Model(&externalAccountModel{}).
		Where("id = ?", accountID).
		Updates(map[string]any{
			"last_seen_at": now.UTC(),
			"updated_at":   now.UTC(),
		})
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
	}
	return opErr
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

func (r *AuthRepository) CreateInviteCode(ctx context.Context, invite domain.AuthInviteCode) (domain.AuthInviteCode, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.invite.create", "insert", "auth_invite_codes")
	var opErr error
	defer func() { finish(opErr) }()

	model := authInviteCodeModelFromDomain(normalizeInviteCode(invite))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AuthInviteCode{}, opErr
	}
	return authInviteCodeModelToDomain(model), nil
}

func (r *AuthRepository) ListInviteCodes(ctx context.Context, createdByUserID int64) ([]domain.AuthInviteCode, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.invite.list", "select", "auth_invite_codes")
	var opErr error
	defer func() { finish(opErr) }()

	var models []authInviteCodeModel
	if err := r.db.WithContext(ctx).
		Where("created_by_user_id = ?", createdByUserID).
		Where("status <> ?", string(domain.AuthInviteCodeStatusRevoked)).
		Order("created_at DESC, id DESC").
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	invites := make([]domain.AuthInviteCode, 0, len(models))
	for _, model := range models {
		invites = append(invites, authInviteCodeModelToDomain(model))
	}
	return invites, nil
}

func (r *AuthRepository) RevokeInviteCode(ctx context.Context, createdByUserID int64, inviteID int64, now time.Time) (domain.AuthInviteCode, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.invite.revoke", "update", "auth_invite_codes")
	var opErr error
	defer func() { finish(opErr) }()

	var model authInviteCodeModel
	result := r.db.WithContext(ctx).
		Model(&model).
		Clauses(clause.Returning{}).
		Where("id = ? AND created_by_user_id = ?", inviteID, createdByUserID).
		Updates(map[string]any{
			"status":     string(domain.AuthInviteCodeStatusRevoked),
			"updated_at": now.UTC(),
		})
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.AuthInviteCode{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.AuthInviteCode{}, opErr
	}
	return authInviteCodeModelToDomain(model), nil
}

func (r *AuthRepository) CreateUserWithInvite(ctx context.Context, codeHash string, user domain.User, redemption domain.AuthInviteRedemption, now time.Time) (domain.User, domain.AuthInviteCode, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.auth.invite.create_user", "transaction", "auth_invite_codes")
	var opErr error
	defer func() { finish(opErr) }()

	var createdUser domain.User
	var usedInvite domain.AuthInviteCode
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inviteModel authInviteCodeModel
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("code_hash = ?", strings.TrimSpace(codeHash)).
			First(&inviteModel).Error; err != nil {
			return mapRepositoryError(err)
		}
		invite := authInviteCodeModelToDomain(inviteModel)
		if invite.Status != domain.AuthInviteCodeStatusActive ||
			invite.UseCount >= invite.MaxUses ||
			(invite.ExpiresAt != nil && !now.UTC().Before(invite.ExpiresAt.UTC())) {
			return domain.ErrConflict
		}

		user.Role = invite.Role
		if !user.Role.Valid() {
			user.Role = domain.UserRoleUser
		}
		user.Status = domain.UserStatusActive
		userModel := userModelFromDomain(normalizeUser(user))
		userModel.CreatedAt = now.UTC()
		userModel.UpdatedAt = now.UTC()
		if err := tx.Create(&userModel).Error; err != nil {
			return mapRepositoryError(err)
		}

		if err := tx.Model(&authInviteCodeModel{}).
			Where("id = ?", inviteModel.ID).
			Updates(map[string]any{
				"use_count":  gorm.Expr("use_count + 1"),
				"updated_at": now.UTC(),
			}).Error; err != nil {
			return mapRepositoryError(err)
		}

		redemptionModel := authInviteRedemptionModelFromDomain(domain.AuthInviteRedemption{
			InviteCodeID:  inviteModel.ID,
			UserID:        userModel.ID,
			RedeemedAt:    now.UTC(),
			IPAddress:     redemption.IPAddress,
			UserAgentHash: redemption.UserAgentHash,
		})
		if err := tx.Create(&redemptionModel).Error; err != nil {
			return mapRepositoryError(err)
		}

		var refreshedInvite authInviteCodeModel
		if err := tx.Where("id = ?", inviteModel.ID).First(&refreshedInvite).Error; err != nil {
			return mapRepositoryError(err)
		}
		createdUser = userModelToDomain(userModel)
		usedInvite = authInviteCodeModelToDomain(refreshedInvite)
		return nil
	})
	if err != nil {
		opErr = err
		return domain.User{}, domain.AuthInviteCode{}, err
	}
	return createdUser, usedInvite, nil
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

func normalizeUser(user domain.User) domain.User {
	user.Username = strings.TrimSpace(user.Username)
	user.Email = strings.TrimSpace(user.Email)
	user.DisplayName = strings.TrimSpace(user.DisplayName)
	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}
	user.PasswordHash = strings.TrimSpace(user.PasswordHash)
	if !user.Role.Valid() {
		user.Role = domain.UserRoleUser
	}
	if !user.Status.Valid() {
		user.Status = domain.UserStatusActive
	}
	return user
}

func normalizeUserProfile(profile domain.UserProfile) domain.UserProfile {
	profile.TimeZone = strings.TrimSpace(profile.TimeZone)
	if profile.TimeZone == "" {
		profile.TimeZone = "Asia/Shanghai"
	}
	profile.Language = strings.TrimSpace(profile.Language)
	if profile.Language == "" {
		profile.Language = "zh-CN"
	}
	profile.Region = strings.TrimSpace(profile.Region)
	profile.Bio = strings.TrimSpace(profile.Bio)
	profile.FocusTopics = normalizeStringList(profile.FocusTopics)
	profile.BlockedTopics = normalizeStringList(profile.BlockedTopics)
	profile.MarketFocus = normalizeStringList(profile.MarketFocus)
	profile.InstrumentFocus = normalizeStringList(profile.InstrumentFocus)
	profile.RiskPreference = strings.TrimSpace(profile.RiskPreference)
	profile.NotificationQuietHours = strings.TrimSpace(profile.NotificationQuietHours)
	profile.AgentNotes = strings.TrimSpace(profile.AgentNotes)
	profile.ReplyStyle = strings.TrimSpace(profile.ReplyStyle)
	if profile.ReplyStyle == "" {
		profile.ReplyStyle = "plain_text_short"
	}
	return profile
}

func normalizeStringList(values []string) []string {
	if values == nil {
		return []string{}
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeInviteCode(invite domain.AuthInviteCode) domain.AuthInviteCode {
	invite.CodeHash = strings.TrimSpace(invite.CodeHash)
	if !invite.Role.Valid() {
		invite.Role = domain.UserRoleUser
	}
	if invite.MaxUses < 1 {
		invite.MaxUses = 1
	}
	if invite.UseCount < 0 {
		invite.UseCount = 0
	}
	if !invite.Status.Valid() {
		invite.Status = domain.AuthInviteCodeStatusActive
	}
	return invite
}

func userModelFromDomain(user domain.User) userModel {
	return userModel{
		ID:           user.ID,
		Username:     user.Username,
		Email:        user.Email,
		DisplayName:  user.DisplayName,
		PasswordHash: user.PasswordHash,
		Role:         string(user.Role),
		Status:       string(user.Status),
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
}

func userModelToDomain(model userModel) domain.User {
	return domain.User{
		ID:           model.ID,
		Username:     model.Username,
		Email:        model.Email,
		DisplayName:  model.DisplayName,
		PasswordHash: model.PasswordHash,
		Role:         domain.UserRole(model.Role),
		Status:       domain.UserStatus(model.Status),
		CreatedAt:    model.CreatedAt,
		UpdatedAt:    model.UpdatedAt,
	}
}

func userProfileModelFromDomain(profile domain.UserProfile) userProfileModel {
	return userProfileModel{
		UserID:                 profile.UserID,
		TimeZone:               profile.TimeZone,
		Language:               profile.Language,
		Region:                 profile.Region,
		Bio:                    profile.Bio,
		FocusTopics:            append([]string(nil), profile.FocusTopics...),
		BlockedTopics:          append([]string(nil), profile.BlockedTopics...),
		MarketFocus:            append([]string(nil), profile.MarketFocus...),
		InstrumentFocus:        append([]string(nil), profile.InstrumentFocus...),
		RiskPreference:         profile.RiskPreference,
		NotificationQuietHours: profile.NotificationQuietHours,
		AgentNotes:             profile.AgentNotes,
		ReplyStyle:             profile.ReplyStyle,
		CreatedAt:              profile.CreatedAt,
		UpdatedAt:              profile.UpdatedAt,
	}
}

func userProfileModelToDomain(model userProfileModel) domain.UserProfile {
	return domain.UserProfile{
		UserID:                 model.UserID,
		TimeZone:               model.TimeZone,
		Language:               model.Language,
		Region:                 model.Region,
		Bio:                    model.Bio,
		FocusTopics:            append([]string(nil), model.FocusTopics...),
		BlockedTopics:          append([]string(nil), model.BlockedTopics...),
		MarketFocus:            append([]string(nil), model.MarketFocus...),
		InstrumentFocus:        append([]string(nil), model.InstrumentFocus...),
		RiskPreference:         model.RiskPreference,
		NotificationQuietHours: model.NotificationQuietHours,
		AgentNotes:             model.AgentNotes,
		ReplyStyle:             model.ReplyStyle,
		CreatedAt:              model.CreatedAt,
		UpdatedAt:              model.UpdatedAt,
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

func authInviteCodeModelFromDomain(invite domain.AuthInviteCode) authInviteCodeModel {
	return authInviteCodeModel{
		ID:              invite.ID,
		CodeHash:        invite.CodeHash,
		CreatedByUserID: invite.CreatedByID,
		Role:            string(invite.Role),
		MaxUses:         invite.MaxUses,
		UseCount:        invite.UseCount,
		Status:          string(invite.Status),
		ExpiresAt:       invite.ExpiresAt,
		CreatedAt:       invite.CreatedAt,
		UpdatedAt:       invite.UpdatedAt,
	}
}

func authInviteCodeModelToDomain(model authInviteCodeModel) domain.AuthInviteCode {
	return domain.AuthInviteCode{
		ID:          model.ID,
		CodeHash:    model.CodeHash,
		CreatedByID: model.CreatedByUserID,
		Role:        domain.UserRole(model.Role),
		MaxUses:     model.MaxUses,
		UseCount:    model.UseCount,
		Status:      domain.AuthInviteCodeStatus(model.Status),
		ExpiresAt:   model.ExpiresAt,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}
}

func authInviteRedemptionModelFromDomain(redemption domain.AuthInviteRedemption) authInviteRedemptionModel {
	return authInviteRedemptionModel{
		ID:            redemption.ID,
		InviteCodeID:  redemption.InviteCodeID,
		UserID:        redemption.UserID,
		RedeemedAt:    redemption.RedeemedAt,
		IPAddress:     redemption.IPAddress,
		UserAgentHash: redemption.UserAgentHash,
	}
}
