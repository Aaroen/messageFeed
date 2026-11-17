package domain

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
