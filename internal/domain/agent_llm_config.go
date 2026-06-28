package domain

import "time"

type AgentLLMProtocolMode string

const (
	AgentLLMProtocolModeAuto            AgentLLMProtocolMode = "auto"
	AgentLLMProtocolModeResponses       AgentLLMProtocolMode = "responses"
	AgentLLMProtocolModeChatCompletions AgentLLMProtocolMode = "chat_completions"
)

func (mode AgentLLMProtocolMode) Valid() bool {
	switch mode {
	case AgentLLMProtocolModeAuto, AgentLLMProtocolModeResponses, AgentLLMProtocolModeChatCompletions:
		return true
	default:
		return false
	}
}

type AgentLLMProviderConfig struct {
	ID               int64
	UserID           int64
	Name             string
	Provider         string
	BaseURL          string
	Model            string
	APIKeyCiphertext string
	APIKeyHint       string
	ProtocolMode     AgentLLMProtocolMode
	Enabled          bool
	IsDefault        bool
	TimeoutSeconds   int
	MaxRetries       int
	LastUsedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
