package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"

	"golang.org/x/sync/singleflight"
)

type agentModelCatalogCache struct {
	entries  sync.Map
	requests singleflight.Group
}

type agentModelCatalogEntry struct {
	key     string
	models  []llm.Model
	err     error
	expires time.Time
}

type agentLLMModelStore interface {
	ListAgentLLMModels(context.Context, int64, int64) ([]domain.AgentLLMModel, error)
	SetAgentLLMModels(context.Context, int64, int64, []domain.AgentLLMModel) error
	ListGuestAgentModels(context.Context) ([]domain.AgentLLMModel, error)
	GetGuestAgentModelConfig(context.Context, int64, string) (domain.AgentLLMProviderConfig, error)
	ReserveGuestAgentRequest(context.Context, string, domain.AgentGuestTurn) (domain.AgentGuestTurn, error)
	CompleteGuestAgentRequest(context.Context, string, domain.AgentGuestTurn) error
	ListGuestAgentHistory(context.Context, string) ([]domain.AgentGuestTurn, error)
}

type AgentProviderModels struct {
	Models []llm.Model            `json:"models"`
	Saved  []domain.AgentLLMModel `json:"saved"`
}

type AgentProviderModelsInput struct {
	Models       []string          `json:"models"`
	GuestModels  []string          `json:"guest_models"`
	DisplayNames map[string]string `json:"display_names"`
	Icons        map[string]string `json:"icons"`
}

func (s *AgentLLMConfigService) SavedModels(ctx context.Context, auth CurrentAuth, configID int64) ([]domain.AgentLLMModel, error) {
	if err := requireAgentLLMConfigAuth(auth); err != nil {
		return nil, err
	}
	if _, err := s.store.GetAgentLLMProviderConfig(ctx, auth.User.ID, configID); err != nil {
		return nil, err
	}
	store, ok := s.store.(agentLLMModelStore)
	if !ok {
		return nil, fmt.Errorf("模型目录存储不可用")
	}
	return store.ListAgentLLMModels(ctx, auth.User.ID, configID)
}

func (s *AgentLLMConfigService) DiscoverModels(ctx context.Context, auth CurrentAuth, configID int64) (AgentProviderModels, error) {
	if err := requireAgentLLMConfigAuth(auth); err != nil {
		return AgentProviderModels{}, err
	}
	configuration, err := s.store.GetAgentLLMProviderConfig(ctx, auth.User.ID, configID)
	if err != nil {
		return AgentProviderModels{}, err
	}
	models, err := s.fetchModelCatalog(ctx, configuration, true)
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

func (s *AgentLLMConfigService) fetchModelCatalog(ctx context.Context, configuration domain.AgentLLMProviderConfig, refresh bool) ([]llm.Model, error) {
	key := configuration.BaseURL + "\x00" + configuration.APIKeyCiphertext
	if !refresh {
		if cached, exists := s.catalog.entries.Load(configuration.ID); exists {
			entry := cached.(agentModelCatalogEntry)
			if entry.key == key && time.Now().Before(entry.expires) {
				return entry.models, entry.err
			}
		}
	}
	result := s.catalog.requests.DoChan(fmt.Sprintf("%d:%s", configuration.ID, key), func() (any, error) {
		apiKey, err := s.codec.Open(configuration.APIKeyCiphertext)
		if err != nil {
			return nil, err
		}
		fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 8*time.Second)
		defer cancel()
		models, err := llm.FetchModels(fetchCtx, configuration.BaseURL, apiKey)
		ttl := 5 * time.Minute
		if err != nil {
			ttl = 15 * time.Second
		}
		s.catalog.entries.Store(configuration.ID, agentModelCatalogEntry{key: key, models: models, err: err, expires: time.Now().Add(ttl)})
		return models, err
	})
	select {
	case response := <-result:
		if response.Err != nil {
			return nil, response.Err
		}
		return response.Val.([]llm.Model), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *AgentLLMConfigService) ReasoningLevels(ctx context.Context, auth CurrentAuth, guest bool, configID int64, model string) ([]string, error) {
	var configuration domain.AgentLLMProviderConfig
	var err error
	if guest {
		store, ok := s.store.(agentLLMModelStore)
		if !ok {
			return nil, fmt.Errorf("模型目录存储不可用")
		}
		configuration, err = store.GetGuestAgentModelConfig(ctx, configID, model)
	} else {
		models, listErr := s.SelectableModels(ctx, auth)
		if listErr != nil {
			return nil, listErr
		}
		if !slices.ContainsFunc(models, func(entry domain.AgentLLMModel) bool { return entry.ConfigID == configID && entry.Model == model }) {
			return nil, domain.ErrNotFound
		}
		configuration, err = s.store.GetAgentLLMProviderConfig(ctx, auth.User.ID, configID)
	}
	if err != nil {
		return nil, err
	}
	return s.reasoningLevels(ctx, configuration, model)
}

func (s *AgentLLMConfigService) reasoningLevels(ctx context.Context, configuration domain.AgentLLMProviderConfig, model string) ([]string, error) {
	models, err := s.fetchModelCatalog(ctx, configuration, false)
	if err != nil {
		return nil, err
	}
	for _, entry := range models {
		if entry.ID == model && len(entry.ReasoningLevels) > 0 {
			return entry.ReasoningLevels, nil
		}
	}
	return []string{}, nil
}

func (s *AgentLLMConfigService) validateReasoningEffort(ctx context.Context, configuration domain.AgentLLMProviderConfig, effort string) error {
	if effort == "" {
		return nil
	}
	levels, err := s.reasoningLevels(ctx, configuration, configuration.Model)
	if err != nil {
		return err
	}
	if !slices.Contains(levels, effort) {
		return fmt.Errorf("%w: 当前模型不支持所选思考强度", domain.ErrInvalidInput)
	}
	return nil
}

func (s *AgentLLMConfigService) SaveModels(ctx context.Context, auth CurrentAuth, configID int64, input AgentProviderModelsInput) error {
	if err := requireAgentLLMConfigAuth(auth); err != nil {
		return err
	}
	if len(input.Models) > 4096 || len(input.GuestModels) > 4096 || len(input.DisplayNames) > 4096 || len(input.Icons) > 4096 {
		return fmt.Errorf("%w: 模型选择数量超过限制", domain.ErrInvalidInput)
	}
	if len(input.GuestModels) > 0 && (!auth.Authenticated || auth.User.Role != domain.UserRoleOwner) {
		return domain.NewAppError(domain.ErrorKindForbidden, "guest_models_owner_required", "只有管理员可以向游客开放模型", "service.agent_models.save", false, nil)
	}
	saved, err := s.SavedModels(ctx, auth, configID)
	if err != nil {
		return err
	}
	configuration, err := s.store.GetAgentLLMProviderConfig(ctx, auth.User.ID, configID)
	if err != nil {
		return err
	}
	aliases := make(map[string]string, len(saved))
	icons := make(map[string]string, len(saved))
	for _, model := range saved {
		aliases[model.Model] = model.DisplayName
		icons[model.Model] = model.Icon
	}
	selected := map[string]bool{}
	for _, model := range input.Models {
		if model == "" || model != strings.TrimSpace(model) || len(model) > 512 || selected[model] {
			return fmt.Errorf("%w: 模型标识无效或重复", domain.ErrInvalidInput)
		}
		selected[model] = true
	}
	if !selected[configuration.Model] {
		return fmt.Errorf("%w: 必须保留默认模型，修改默认模型后可以停用原模型", domain.ErrInvalidInput)
	}
	for model, name := range input.DisplayNames {
		name = strings.TrimSpace(name)
		if !selected[model] || len([]rune(name)) > 80 {
			return fmt.Errorf("%w: 模型显示名称最多 80 个字符，且必须属于已启用模型", domain.ErrInvalidInput)
		}
		aliases[model] = name
	}
	for model, icon := range input.Icons {
		if !selected[model] || (icon != "" && icon != "lightning" && icon != "brain") {
			return fmt.Errorf("%w: 图标无效或不属于已启用模型", domain.ErrInvalidInput)
		}
		icons[model] = icon
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
		models = append(models, domain.AgentLLMModel{ConfigID: configID, Model: model, DisplayName: aliases[model], Icon: icons[model], Enabled: true, GuestEnabled: guests[model]})
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
	return agent.WithModelContextWindow(context.WithValue(ctx, agentModelSelectionKey{}, selection), selection.ContextWindow)
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

func (r *AgentLLMRuntime) ValidateModelSelection(ctx context.Context, userID, configID int64, model, effort string) (domain.AgentModelSelection, error) {
	selection := domain.AgentModelSelection{UserID: userID, ConfigID: configID, Model: strings.TrimSpace(model), ReasoningEffort: effort}
	if selection.ConfigID < 1 || selection.Model == "" || len(selection.Model) > 512 {
		return selection, fmt.Errorf("%w: 模型选择无效", domain.ErrInvalidInput)
	}
	if _, err := r.clientForUser(withAgentModelSelection(ctx, selection), userID); err != nil {
		return domain.AgentModelSelection{}, err
	}
	if r.modelCatalog != nil {
		configuration, err := r.store.GetAgentLLMProviderConfig(ctx, userID, configID)
		if err != nil {
			return domain.AgentModelSelection{}, err
		}
		configuration.Model = selection.Model
		if err := r.modelCatalog.validateReasoningEffort(ctx, configuration, effort); err != nil {
			return domain.AgentModelSelection{}, err
		}
		selection.ContextWindow = r.modelCatalog.modelContextWindow(ctx, configuration)
	} else if effort != "" {
		return domain.AgentModelSelection{}, fmt.Errorf("模型目录不可用")
	}
	return selection, nil
}

func (s *AgentLLMConfigService) modelContextWindow(ctx context.Context, configuration domain.AgentLLMProviderConfig) int {
	catalogCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	models, err := s.fetchModelCatalog(catalogCtx, configuration, false)
	if err != nil {
		return 0
	}
	for _, model := range models {
		if model.ID == configuration.Model && model.ContextLength > 0 {
			return model.ContextLength
		}
	}
	return 0
}
