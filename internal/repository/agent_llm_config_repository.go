package repository

import (
	"context"
	"messagefeed/internal/domain"
	"strings"
	"time"
)

type agentLLMProviderConfigModel struct {
	ID               int64 `gorm:"primaryKey"`
	UserID           int64 `gorm:"not null"`
	Name             string
	Provider         string
	BaseURL          string
	Model            string
	APIKeyCiphertext string
	APIKeyHint       string
	ProtocolMode     string
	Enabled          bool
	IsDefault        bool
	TimeoutSeconds   int
	MaxRetries       int
	LastUsedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (agentLLMProviderConfigModel) TableName() string {
	return "agent_llm_provider_configs"
}

func (r *AgentRepository) ListAgentLLMProviderConfigs(ctx context.Context, userID int64) ([]domain.AgentLLMProviderConfig, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_llm_config.list", "select", "agent_llm_provider_configs")
	var opErr error
	defer func() { finish(opErr) }()

	var models []agentLLMProviderConfigModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("is_default DESC, enabled DESC, id ASC").
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	configs := make([]domain.AgentLLMProviderConfig, 0, len(models))
	for _, model := range models {
		configs = append(configs, agentLLMProviderConfigModelToDomain(model))
	}
	return configs, nil
}

func (r *AgentRepository) GetAgentLLMProviderConfig(ctx context.Context, userID int64, id int64) (domain.AgentLLMProviderConfig, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_llm_config.get", "select", "agent_llm_provider_configs")
	var opErr error
	defer func() { finish(opErr) }()

	var model agentLLMProviderConfigModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND id = ?", userID, id).
		First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentLLMProviderConfig{}, opErr
	}
	return agentLLMProviderConfigModelToDomain(model), nil
}

func (r *AgentRepository) GetDefaultAgentLLMProviderConfig(ctx context.Context, userID int64) (domain.AgentLLMProviderConfig, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_llm_config.get_default", "select", "agent_llm_provider_configs")
	var opErr error
	defer func() { finish(opErr) }()

	var model agentLLMProviderConfigModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND enabled = TRUE AND is_default = TRUE", userID).
		First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentLLMProviderConfig{}, opErr
	}
	return agentLLMProviderConfigModelToDomain(model), nil
}

func (r *AgentRepository) CreateAgentLLMProviderConfig(ctx context.Context, config domain.AgentLLMProviderConfig) (domain.AgentLLMProviderConfig, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_llm_config.create", "insert", "agent_llm_provider_configs")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentLLMProviderConfigModelFromDomain(normalizeAgentLLMProviderConfig(config))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentLLMProviderConfig{}, opErr
	}
	return agentLLMProviderConfigModelToDomain(model), nil
}

func (r *AgentRepository) UpdateAgentLLMProviderConfig(ctx context.Context, config domain.AgentLLMProviderConfig) (domain.AgentLLMProviderConfig, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_llm_config.update", "update", "agent_llm_provider_configs")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentLLMProviderConfigModelFromDomain(normalizeAgentLLMProviderConfig(config))
	result := r.db.WithContext(ctx).
		Model(&agentLLMProviderConfigModel{}).
		Where("id = ? AND user_id = ?", config.ID, config.UserID).
		Select(
			"Name",
			"Provider",
			"BaseURL",
			"Model",
			"APIKeyCiphertext",
			"APIKeyHint",
			"ProtocolMode",
			"Enabled",
			"IsDefault",
			"TimeoutSeconds",
			"MaxRetries",
			"LastUsedAt",
			"UpdatedAt",
		).
		Updates(&model)
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.AgentLLMProviderConfig{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.AgentLLMProviderConfig{}, opErr
	}
	return r.GetAgentLLMProviderConfig(ctx, config.UserID, config.ID)
}

func (r *AgentRepository) ClearDefaultAgentLLMProviderConfigs(ctx context.Context, userID int64, exceptID int64, now time.Time) error {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_llm_config.clear_default", "update", "agent_llm_provider_configs")
	var opErr error
	defer func() { finish(opErr) }()

	query := r.db.WithContext(ctx).
		Model(&agentLLMProviderConfigModel{}).
		Where("user_id = ? AND is_default = TRUE", userID)
	if exceptID > 0 {
		query = query.Where("id <> ?", exceptID)
	}
	if err := query.Updates(map[string]any{
		"is_default": false,
		"updated_at": now.UTC(),
	}).Error; err != nil {
		opErr = mapRepositoryError(err)
		return opErr
	}
	return nil
}

func (r *AgentRepository) MarkAgentLLMProviderConfigUsed(ctx context.Context, userID int64, id int64, now time.Time) error {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_llm_config.mark_used", "update", "agent_llm_provider_configs")
	var opErr error
	defer func() { finish(opErr) }()

	result := r.db.WithContext(ctx).
		Model(&agentLLMProviderConfigModel{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{
			"last_used_at": now.UTC(),
			"updated_at":   now.UTC(),
		})
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return opErr
	}
	return nil
}

func normalizeAgentLLMProviderConfig(config domain.AgentLLMProviderConfig) domain.AgentLLMProviderConfig {
	config.Name = strings.TrimSpace(config.Name)
	config.Provider = strings.TrimSpace(strings.ToLower(config.Provider))
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	config.Model = strings.TrimSpace(config.Model)
	config.APIKeyCiphertext = strings.TrimSpace(config.APIKeyCiphertext)
	config.APIKeyHint = strings.TrimSpace(config.APIKeyHint)
	if !config.ProtocolMode.Valid() {
		config.ProtocolMode = domain.AgentLLMProtocolModeAuto
	}
	if config.TimeoutSeconds < 10 {
		config.TimeoutSeconds = 600
	}
	if config.TimeoutSeconds > 3600 {
		config.TimeoutSeconds = 3600
	}
	if config.MaxRetries < 1 {
		config.MaxRetries = 6
	}
	if config.MaxRetries > 50 {
		config.MaxRetries = 50
	}
	now := time.Now().UTC()
	if config.CreatedAt.IsZero() {
		config.CreatedAt = now
	}
	if config.UpdatedAt.IsZero() {
		config.UpdatedAt = config.CreatedAt
	}
	return config
}

func agentLLMProviderConfigModelFromDomain(config domain.AgentLLMProviderConfig) agentLLMProviderConfigModel {
	return agentLLMProviderConfigModel{
		ID:               config.ID,
		UserID:           config.UserID,
		Name:             config.Name,
		Provider:         config.Provider,
		BaseURL:          config.BaseURL,
		Model:            config.Model,
		APIKeyCiphertext: config.APIKeyCiphertext,
		APIKeyHint:       config.APIKeyHint,
		ProtocolMode:     string(config.ProtocolMode),
		Enabled:          config.Enabled,
		IsDefault:        config.IsDefault,
		TimeoutSeconds:   config.TimeoutSeconds,
		MaxRetries:       config.MaxRetries,
		LastUsedAt:       config.LastUsedAt,
		CreatedAt:        config.CreatedAt,
		UpdatedAt:        config.UpdatedAt,
	}
}

func agentLLMProviderConfigModelToDomain(model agentLLMProviderConfigModel) domain.AgentLLMProviderConfig {
	return domain.AgentLLMProviderConfig{
		ID:               model.ID,
		UserID:           model.UserID,
		Name:             model.Name,
		Provider:         model.Provider,
		BaseURL:          model.BaseURL,
		Model:            model.Model,
		APIKeyCiphertext: model.APIKeyCiphertext,
		APIKeyHint:       model.APIKeyHint,
		ProtocolMode:     domain.AgentLLMProtocolMode(model.ProtocolMode),
		Enabled:          model.Enabled,
		IsDefault:        model.IsDefault,
		TimeoutSeconds:   model.TimeoutSeconds,
		MaxRetries:       model.MaxRetries,
		LastUsedAt:       model.LastUsedAt,
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
	}
}
