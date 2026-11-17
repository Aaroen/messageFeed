package repository

import (
	"context"
	"fmt"
	"time"

	"messagefeed/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *AgentRepository) ListAgentLLMModels(ctx context.Context, userID, configID int64) ([]domain.AgentLLMModel, error) {
	models := []domain.AgentLLMModel{}
	query := r.db.WithContext(ctx).Table("agent_llm_models m").Select("m.*, c.name AS provider_name").Joins("JOIN agent_llm_provider_configs c ON c.id = m.config_id").Where("c.user_id = ?", userID)
	if configID > 0 {
		query = query.Where("c.id = ?", configID)
	} else {
		query = query.Where("c.enabled = TRUE AND m.enabled = TRUE")
	}
	if err := query.Order("c.is_default DESC, c.id, (m.model = c.model) DESC, m.model").Find(&models).Error; err != nil {
		return nil, mapRepositoryError(err)
	}
	return models, nil
}

func (r *AgentRepository) SetAgentLLMModels(ctx context.Context, userID, configID int64, models []domain.AgentLLMModel) error {
	return r.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var config agentLLMProviderConfigModel
		if err := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", configID, userID).First(&config).Error; err != nil {
			return mapRepositoryError(err)
		}
		defaultEnabled := false
		for _, model := range models {
			if model.Model == config.Model && model.Enabled {
				defaultEnabled = true
			}
		}
		if !defaultEnabled {
			return fmt.Errorf("%w: 当前默认模型必须保持启用", domain.ErrInvalidInput)
		}
		if err := transaction.Table("agent_llm_models").Where("config_id = ?", configID).Updates(map[string]any{"enabled": false, "guest_enabled": false}).Error; err != nil {
			return err
		}
		for _, model := range models {
			if model.ConfigID != configID || model.Model == "" {
				return fmt.Errorf("%w: 模型所属供应商无效", domain.ErrInvalidInput)
			}
			if err := transaction.Table("agent_llm_models").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "config_id"}, {Name: "model"}}, DoUpdates: clause.AssignmentColumns([]string{"enabled", "guest_enabled"})}).Create(&model).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *AgentRepository) ListGuestAgentModels(ctx context.Context) ([]domain.AgentLLMModel, error) {
	models := []domain.AgentLLMModel{}
	err := r.db.WithContext(ctx).Table("agent_llm_models m").Select("m.*, c.name AS provider_name").Joins("JOIN agent_llm_provider_configs c ON c.id = m.config_id").Joins("JOIN users u ON u.id = c.user_id").Where("u.role = ? AND u.status = ? AND c.enabled = TRUE AND m.enabled = TRUE AND m.guest_enabled = TRUE", domain.UserRoleOwner, domain.UserStatusActive).Order("c.id, m.model").Find(&models).Error
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return models, nil
}

func (r *AgentRepository) GetGuestAgentModelConfig(ctx context.Context, configID int64, model string) (domain.AgentLLMProviderConfig, error) {
	var config agentLLMProviderConfigModel
	err := r.db.WithContext(ctx).Table("agent_llm_provider_configs c").Select("c.*").Joins("JOIN agent_llm_models m ON m.config_id = c.id").Joins("JOIN users u ON u.id = c.user_id").Where("c.id = ? AND m.model = ? AND u.role = ? AND u.status = ? AND c.enabled = TRUE AND m.enabled = TRUE AND m.guest_enabled = TRUE", configID, model, domain.UserRoleOwner, domain.UserStatusActive).Take(&config).Error
	if err != nil {
		return domain.AgentLLMProviderConfig{}, mapRepositoryError(err)
	}
	configuration := agentLLMProviderConfigModelToDomain(config)
	configuration.Model = model
	return configuration, nil
}

func (r *AgentRepository) ReserveGuestAgentRequest(ctx context.Context, clientHash string, configID int64, model string) (int64, error) {
	var request struct {
		ID         int64
		ClientHash string
		ConfigID   int64
		Model      string
		CreatedAt  time.Time
	}
	err := r.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if err := transaction.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "agent-guest-model-quota").Error; err != nil {
			return err
		}
		var clientCount, totalCount int64
		query := transaction.Table("agent_guest_model_requests").Where("created_at >= CURRENT_DATE")
		if err := query.Count(&totalCount).Error; err != nil {
			return err
		}
		if err := query.Where("client_hash = ?", clientHash).Count(&clientCount).Error; err != nil {
			return err
		}
		if clientCount >= 20 || totalCount >= 200 {
			return domain.NewAppError(domain.ErrorKindRateLimited, "guest_model_quota_exceeded", "游客模型调用额度已用完", "repository.agent_guest.reserve", false, nil)
		}
		request.ClientHash, request.ConfigID, request.Model, request.CreatedAt = clientHash, configID, model, time.Now().UTC()
		return transaction.Table("agent_guest_model_requests").Create(&request).Error
	})
	return request.ID, err
}
