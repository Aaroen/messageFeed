package domain

import "time"

const (
	AgentGuestClientDailyLimit = 20
	AgentGuestTotalDailyLimit  = 100
	AgentGuestRetentionDays    = 7
)

type AgentGuestTurn struct {
	ID               int64      `json:"id"`
	ConversationID   int64      `json:"conversation_id"`
	ClientHash       string     `json:"-"`
	ConfigID         int64      `json:"config_id"`
	Model            string     `json:"model"`
	InputText        string     `json:"input_text"`
	OutputText       string     `json:"output_text"`
	ReasoningContent string     `json:"reasoning_content"`
	Status           string     `json:"status"`
	CreatedAt        time.Time  `json:"created_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

type AgentLLMModel struct {
	ConfigID     int64  `json:"config_id" gorm:"column:config_id"`
	ProviderName string `json:"provider_name" gorm:"column:provider_name;->"`
	Model        string `json:"model" gorm:"column:model"`
	Enabled      bool   `json:"enabled"`
	GuestEnabled bool   `json:"guest_enabled"`
}

type AgentModelSelection struct {
	UserID   int64  `json:"user_id"`
	ConfigID int64  `json:"config_id"`
	Model    string `json:"model"`
}
