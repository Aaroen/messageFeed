package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
)

type agentLLMModelStore interface {
	ListAgentLLMModels(context.Context, int64, int64) ([]domain.AgentLLMModel, error)
	SetAgentLLMModels(context.Context, int64, int64, []domain.AgentLLMModel) error
	ListGuestAgentModels(context.Context) ([]domain.AgentLLMModel, error)
	GetGuestAgentModelConfig(context.Context, int64, string) (domain.AgentLLMProviderConfig, error)
	ReserveGuestAgentRequest(context.Context, string, int64, string) (int64, error)
}

type AgentProviderModels struct {
	Models []llm.Model            `json:"models"`
	Saved  []domain.AgentLLMModel `json:"saved"`
}

type AgentProviderModelsInput struct {
	Models      []string `json:"models"`
	GuestModels []string `json:"guest_models"`
}

func (s *AgentLLMConfigService) DiscoverModels(ctx context.Context, auth CurrentAuth, configID int64) (AgentProviderModels, error) {
	if err := requireAgentLLMConfigAuth(auth); err != nil {
		return AgentProviderModels{}, err
	}
	configuration, err := s.store.GetAgentLLMProviderConfig(ctx, auth.User.ID, configID)
	if err != nil {
		return AgentProviderModels{}, err
	}
	key, err := s.codec.Open(configuration.APIKeyCiphertext)
	if err != nil {
		return AgentProviderModels{}, err
	}
	models, err := llm.FetchModels(ctx, configuration.BaseURL, key)
	if err != nil {
		return AgentProviderModels{}, err
	}
	store, ok := s.store.(agentLLMModelStore)
	if !ok {
		return AgentProviderModels{}, fmt.Errorf("模型目录存储不可用")
	}
	saved, err := store.ListAgentLLMModels(ctx, auth.User.ID, configID)
	return AgentProviderModels{Models: models, Saved: saved}, err
}

func (s *AgentLLMConfigService) SaveModels(ctx context.Context, auth CurrentAuth, configID int64, input AgentProviderModelsInput) error {
	if err := requireAgentLLMConfigAuth(auth); err != nil {
		return err
	}
	if len(input.Models) > 4096 || len(input.GuestModels) > 4096 {
		return fmt.Errorf("%w: 模型选择数量超过限制", domain.ErrInvalidInput)
	}
	if len(input.GuestModels) > 0 && (!auth.Authenticated || auth.User.Role != domain.UserRoleOwner) {
		return domain.NewAppError(domain.ErrorKindForbidden, "guest_models_owner_required", "只有管理员可以向游客开放模型", "service.agent_models.save", false, nil)
	}
	catalog, err := s.DiscoverModels(ctx, auth, configID)
	if err != nil {
		return err
	}
	configuration, err := s.store.GetAgentLLMProviderConfig(ctx, auth.User.ID, configID)
	if err != nil {
		return err
	}
	available := map[string]bool{}
	for _, model := range catalog.Models {
		available[model.ID] = true
	}
	selected := map[string]bool{}
	for _, model := range input.Models {
		if len(model) > 512 || !available[model] || selected[model] {
			return fmt.Errorf("%w: 选择的模型不在供应商目录中或重复", domain.ErrInvalidInput)
		}
		selected[model] = true
	}
	if !selected[configuration.Model] {
		return fmt.Errorf("%w: 必须保留默认模型，修改默认模型后可以停用原模型", domain.ErrInvalidInput)
	}
	guests := map[string]bool{}
	for _, model := range input.GuestModels {
		if !selected[model] || guests[model] {
			return fmt.Errorf("%w: 游客模型必须属于已启用模型", domain.ErrInvalidInput)
		}
		guests[model] = true
	}
	models := make([]domain.AgentLLMModel, 0, len(input.Models))
	for _, model := range input.Models {
		models = append(models, domain.AgentLLMModel{ConfigID: configID, Model: model, Enabled: true, GuestEnabled: guests[model]})
	}
	return s.store.(agentLLMModelStore).SetAgentLLMModels(ctx, auth.User.ID, configID, models)
}

func (s *AgentLLMConfigService) SelectableModels(ctx context.Context, auth CurrentAuth) ([]domain.AgentLLMModel, error) {
	if err := requireAgentLLMConfigAuth(auth); err != nil {
		return nil, err
	}
	store, ok := s.store.(agentLLMModelStore)
	if !ok {
		return nil, fmt.Errorf("模型目录存储不可用")
	}
	return store.ListAgentLLMModels(ctx, auth.User.ID, 0)
}

func (s *AgentLLMConfigService) GuestModels(ctx context.Context) ([]domain.AgentLLMModel, error) {
	store, ok := s.store.(agentLLMModelStore)
	if !ok {
		return nil, fmt.Errorf("模型目录存储不可用")
	}
	return store.ListGuestAgentModels(ctx)
}

type agentModelSelectionKey struct{}

func withAgentModelSelection(ctx context.Context, selection domain.AgentModelSelection) context.Context {
	return context.WithValue(ctx, agentModelSelectionKey{}, selection)
}

func agentSelectionFromInbound(ctx context.Context, inbound domain.AgentInboundMessage) (context.Context, error) {
	value, exists := inbound.Payload["model_selection"]
	if !exists {
		return ctx, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ctx, err
	}
	var selection domain.AgentModelSelection
	if err := json.Unmarshal(encoded, &selection); err != nil {
		return ctx, err
	}
	if selection.ConfigID < 1 || strings.TrimSpace(selection.Model) == "" || selection.UserID != inbound.UserID {
		return ctx, fmt.Errorf("%w: 回合模型选择记录无效", domain.ErrInvalidInput)
	}
	return withAgentModelSelection(ctx, selection), nil
}

func (r *AgentLLMRuntime) ValidateModelSelection(ctx context.Context, userID, configID int64, model string) (domain.AgentModelSelection, error) {
	selection := domain.AgentModelSelection{UserID: userID, ConfigID: configID, Model: strings.TrimSpace(model)}
	if selection.ConfigID < 1 || selection.Model == "" || len(selection.Model) > 512 {
		return selection, fmt.Errorf("%w: 模型选择无效", domain.ErrInvalidInput)
	}
	_, err := r.clientForUser(withAgentModelSelection(ctx, selection), userID)
	return selection, err
}
