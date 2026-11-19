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
			if err := transaction.Table("agent_llm_models").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "config_id"}, {Name: "model"}}, DoUpdates: clause.AssignmentColumns([]string{"enabled", "guest_enabled", "display_name"})}).Create(&model).Error; err != nil {
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

func (r *AgentRepository) ListGuestAgentHistory(ctx context.Context, clientHash string) ([]domain.AgentGuestTurn, error) {
	if err := r.PurgeExpiredGuestAgentHistory(ctx); err != nil {
		return nil, err
	}
	turns := []domain.AgentGuestTurn{}
	if err := r.db.WithContext(ctx).Table("agent_guest_model_requests").Where("client_hash = ? AND created_at > NOW() - INTERVAL '7 days' AND input_text <> ''", clientHash).Order("created_at, id").Limit(200).Find(&turns).Error; err != nil {
		return nil, mapRepositoryError(err)
	}
	return turns, nil
}

func (r *AgentRepository) PurgeExpiredGuestAgentHistory(ctx context.Context) error {
	return r.db.WithContext(ctx).Exec("DELETE FROM agent_guest_model_requests WHERE created_at <= NOW() - INTERVAL '7 days'").Error
}

func (r *AgentRepository) ReserveGuestAgentRequest(ctx context.Context, clientHash string, request domain.AgentGuestTurn) (domain.AgentGuestTurn, error) {
	if clientHash == "" || request.ConfigID < 1 || request.Model == "" || request.InputText == "" || request.ConversationID < 0 {
		return domain.AgentGuestTurn{}, fmt.Errorf("%w: 游客对话参数无效", domain.ErrInvalidInput)
	}
	err := r.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if err := transaction.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "agent-guest-model-quota").Error; err != nil {
			return err
		}
		if request.ConversationID > 0 {
			if err := transaction.Table("agent_guest_model_requests").Where("client_hash = ? AND conversation_id = ? AND status = 'running' AND created_at < NOW() - INTERVAL '3 minutes'", clientHash, request.ConversationID).Updates(map[string]any{"status": "failed", "completed_at": time.Now().UTC()}).Error; err != nil {
				return err
			}
			var count int64
			if err := transaction.Table("agent_guest_model_requests").Where("client_hash = ? AND conversation_id = ? AND created_at > NOW() - INTERVAL '7 days'", clientHash, request.ConversationID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return domain.ErrNotFound
			}
			if err := transaction.Table("agent_guest_model_requests").Where("client_hash = ? AND conversation_id = ? AND status = 'running'", clientHash, request.ConversationID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return fmt.Errorf("%w: 当前游客对话仍在生成回答", domain.ErrConflict)
			}
		}
		now := time.Now().UTC()
		dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		var clientCount, totalCount int64
		query := transaction.Table("agent_guest_model_requests").Where("created_at >= ? AND created_at < ?", dayStart, dayStart.AddDate(0, 0, 1))
		if err := query.Count(&totalCount).Error; err != nil {
			return err
		}
		if err := query.Where("client_hash = ?", clientHash).Count(&clientCount).Error; err != nil {
			return err
		}
		if clientCount >= domain.AgentGuestClientDailyLimit {
			return domain.NewAppError(domain.ErrorKindRateLimited, "guest_client_daily_quota_exceeded", "当前 IP 今日已达到 20 次游客请求额度", "repository.agent_guest.reserve", false, nil)
		}
		if totalCount >= domain.AgentGuestTotalDailyLimit {
			return domain.NewAppError(domain.ErrorKindRateLimited, "guest_total_daily_quota_exceeded", "今日全部游客已达到 100 次请求额度", "repository.agent_guest.reserve", false, nil)
		}
		request.ID, request.ClientHash, request.CreatedAt, request.Status = 0, clientHash, now.UTC(), "running"
		request.OutputText, request.ReasoningContent, request.CompletedAt = "", "", nil
		if err := transaction.Table("agent_guest_model_requests").Create(&request).Error; err != nil {
			return err
		}
		if request.ConversationID == 0 {
			request.ConversationID = request.ID
			return transaction.Table("agent_guest_model_requests").Where("id = ?", request.ID).Update("conversation_id", request.ConversationID).Error
		}
		return nil
	})
	return request, err
}

func (r *AgentRepository) CompleteGuestAgentRequest(ctx context.Context, clientHash string, turn domain.AgentGuestTurn) error {
	if turn.Status != "succeeded" && turn.Status != "failed" {
		return fmt.Errorf("%w: 游客回答状态无效", domain.ErrInvalidInput)
	}
	result := r.db.WithContext(ctx).Table("agent_guest_model_requests").Where("id = ? AND client_hash = ? AND status = 'running'", turn.ID, clientHash).Updates(map[string]any{
		"output_text": turn.OutputText, "reasoning_content": turn.ReasoningContent, "status": turn.Status, "completed_at": time.Now().UTC(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrNotFound
	}
	return nil
}
