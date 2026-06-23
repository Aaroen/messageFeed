package domain

import "time"

type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

func (s UserStatus) Valid() bool {
	switch s {
	case UserStatusActive, UserStatusDisabled:
		return true
	default:
		return false
	}
}

type UserRole string

const (
	UserRoleOwner UserRole = "owner"
	UserRoleUser  UserRole = "user"
)

func (r UserRole) Valid() bool {
	switch r {
	case UserRoleOwner, UserRoleUser:
		return true
	default:
		return false
	}
}

type User struct {
	ID          int64
	Username    string
	Email       string
	DisplayName string
	Role        UserRole
	Status      UserStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type UserSession struct {
	ID               int64
	UserID           int64
	SessionTokenHash string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	UserAgentHash    string
	IPAddress        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	LastSeenAt       time.Time
}

type AuthOAuthPurpose string

const (
	AuthOAuthPurposeBind    AuthOAuthPurpose = "bind"
	AuthOAuthPurposeConfirm AuthOAuthPurpose = "confirm"
)

func (p AuthOAuthPurpose) Valid() bool {
	switch p {
	case AuthOAuthPurposeBind, AuthOAuthPurposeConfirm:
		return true
	default:
		return false
	}
}

type AuthOAuthState struct {
	ID           int64
	StateHash    string
	UserID       int64
	Provider     string
	Purpose      AuthOAuthPurpose
	RedirectPath string
	CorpID       string
	AgentID      string
	ExpiresAt    time.Time
	ConsumedAt   *time.Time
	Metadata     AgentJSON
	CreatedAt    time.Time
}
