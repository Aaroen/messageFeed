package domain

import "time"

type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
	UserStatusDeleted  UserStatus = "deleted"
)

func (s UserStatus) Valid() bool {
	switch s {
	case UserStatusActive, UserStatusDisabled, UserStatusDeleted:
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
	ID           int64
	Username     string
	Email        string
	DisplayName  string
	PasswordHash string
	Role         UserRole
	Status       UserStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserProfile struct {
	UserID                 int64
	TimeZone               string
	Language               string
	Region                 string
	Bio                    string
	FocusTopics            []string
	BlockedTopics          []string
	MarketFocus            []string
	InstrumentFocus        []string
	RiskPreference         string
	NotificationQuietHours string
	AgentNotes             string
	ReplyStyle             string
	CreatedAt              time.Time
	UpdatedAt              time.Time
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

type AuthInviteCodeStatus string

const (
	AuthInviteCodeStatusActive  AuthInviteCodeStatus = "active"
	AuthInviteCodeStatusRevoked AuthInviteCodeStatus = "revoked"
)

func (s AuthInviteCodeStatus) Valid() bool {
	switch s {
	case AuthInviteCodeStatusActive, AuthInviteCodeStatusRevoked:
		return true
	default:
		return false
	}
}

type AuthInviteCode struct {
	ID          int64
	CodeHash    string
	CreatedByID int64
	Role        UserRole
	MaxUses     int
	UseCount    int
	Status      AuthInviteCodeStatus
	ExpiresAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type AuthInviteRedemption struct {
	ID            int64
	InviteCodeID  int64
	UserID        int64
	RedeemedAt    time.Time
	IPAddress     string
	UserAgentHash string
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
