package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"messagefeed/internal/config"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
)

const (
	defaultAgentLLMConfigTimeoutSeconds = 600
	defaultAgentLLMConfigMaxRetries     = 6
	agentLLMConfigTestMaxTokens         = 64
	agentLLMConfigDefaultTestMessage    = "请回复 OK。"
)

type AgentLLMProviderConfigStore interface {
	ListAgentLLMProviderConfigs(ctx context.Context, userID int64) ([]domain.AgentLLMProviderConfig, error)
	GetAgentLLMProviderConfig(ctx context.Context, userID int64, id int64) (domain.AgentLLMProviderConfig, error)
	GetDefaultAgentLLMProviderConfig(ctx context.Context, userID int64) (domain.AgentLLMProviderConfig, error)
	CreateAgentLLMProviderConfig(ctx context.Context, config domain.AgentLLMProviderConfig) (domain.AgentLLMProviderConfig, error)
	UpdateAgentLLMProviderConfig(ctx context.Context, config domain.AgentLLMProviderConfig) (domain.AgentLLMProviderConfig, error)
	ClearDefaultAgentLLMProviderConfigs(ctx context.Context, userID int64, exceptID int64, now time.Time) error
	MarkAgentLLMProviderConfigUsed(ctx context.Context, userID int64, id int64, now time.Time) error
}

type AgentLLMConfigService struct {
	store         AgentLLMProviderConfigStore
	codec         agentLLMConfigCodec
	defaultConfig config.LLMConfig
	now           func() time.Time
}

type AgentLLMConfigServiceOption func(*AgentLLMConfigService)

func WithAgentLLMConfigDefaultConfig(defaultConfig config.LLMConfig) AgentLLMConfigServiceOption {
	return func(service *AgentLLMConfigService) {
		service.defaultConfig = defaultConfig
	}
}

func WithAgentLLMConfigSecret(secret string) AgentLLMConfigServiceOption {
	return func(service *AgentLLMConfigService) {
		service.codec = newAgentLLMConfigCodec(secret)
	}
}

func WithAgentLLMConfigNow(now func() time.Time) AgentLLMConfigServiceOption {
	return func(service *AgentLLMConfigService) {
		if now != nil {
			service.now = now
		}
	}
}

func NewAgentLLMConfigService(store AgentLLMProviderConfigStore, options ...AgentLLMConfigServiceOption) *AgentLLMConfigService {
	service := &AgentLLMConfigService{
		store: store,
		codec: newAgentLLMConfigCodec(""),
		now:   time.Now,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

type AgentLLMProviderConfigListResult struct {
	Configs  []AgentLLMProviderConfigResponse `json:"configs"`
	Fallback AgentLLMFallbackConfigResponse   `json:"fallback"`
}

type AgentLLMFallbackConfigResponse struct {
	Enabled       bool   `json:"enabled"`
	Provider      string `json:"provider,omitempty"`
	Model         string `json:"model,omitempty"`
	BaseURL       string `json:"base_url,omitempty"`
	APIKeyPresent bool   `json:"api_key_present"`
}

type AgentLLMProviderConfigResponse struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Provider       string `json:"provider"`
	BaseURL        string `json:"base_url"`
	Model          string `json:"model"`
	APIKeyHint     string `json:"api_key_hint"`
	APIKeyPresent  bool   `json:"api_key_present"`
	ProtocolMode   string `json:"protocol_mode"`
	Enabled        bool   `json:"enabled"`
	IsDefault      bool   `json:"is_default"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	MaxRetries     int    `json:"max_retries"`
	LastUsedAt     string `json:"last_used_at,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type CreateAgentLLMProviderConfigInput struct {
	Name           string `json:"name"`
	Provider       string `json:"provider"`
	BaseURL        string `json:"base_url"`
	Model          string `json:"model"`
	APIKey         string `json:"api_key"`
	ProtocolMode   string `json:"protocol_mode"`
	Enabled        *bool  `json:"enabled"`
	IsDefault      bool   `json:"is_default"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	MaxRetries     int    `json:"max_retries"`
}

type UpdateAgentLLMProviderConfigInput struct {
	Name           *string `json:"name"`
	Provider       *string `json:"provider"`
	BaseURL        *string `json:"base_url"`
	Model          *string `json:"model"`
	APIKey         *string `json:"api_key"`
	ProtocolMode   *string `json:"protocol_mode"`
	Enabled        *bool   `json:"enabled"`
	IsDefault      *bool   `json:"is_default"`
	TimeoutSeconds *int    `json:"timeout_seconds"`
	MaxRetries     *int    `json:"max_retries"`
}

type TestAgentLLMProviderConfigInput struct {
	Message string `json:"message"`
}

type TestAgentLLMProviderConfigResult struct {
	Status       string `json:"status"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	LatencyMS    int64  `json:"latency_ms"`
	ResponseText string `json:"response_text"`
	CheckedAt    string `json:"checked_at"`
}

func (s *AgentLLMConfigService) ListConfigs(ctx context.Context, auth CurrentAuth) (AgentLLMProviderConfigListResult, error) {
	if err := requireAgentLLMConfigAuth(auth); err != nil {
		return AgentLLMProviderConfigListResult{}, err
	}
	if s == nil || s.store == nil {
		return AgentLLMProviderConfigListResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_llm_config_unavailable", "agent llm config service is unavailable", "service.agent_llm_config.list", true, nil)
	}
	configs, err := s.store.ListAgentLLMProviderConfigs(ctx, auth.User.ID)
	if err != nil {
		return AgentLLMProviderConfigListResult{}, err
	}
	responses := make([]AgentLLMProviderConfigResponse, 0, len(configs))
	for _, item := range configs {
		responses = append(responses, agentLLMProviderConfigResponse(item))
	}
	return AgentLLMProviderConfigListResult{
		Configs:  responses,
		Fallback: agentLLMFallbackConfigResponse(s.defaultConfig),
	}, nil
}

func (s *AgentLLMConfigService) CreateConfig(ctx context.Context, auth CurrentAuth, input CreateAgentLLMProviderConfigInput) (AgentLLMProviderConfigResponse, error) {
	if err := requireAgentLLMConfigAuth(auth); err != nil {
		return AgentLLMProviderConfigResponse{}, err
	}
	if s == nil || s.store == nil {
		return AgentLLMProviderConfigResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_llm_config_unavailable", "agent llm config service is unavailable", "service.agent_llm_config.create", true, nil)
	}
	apiKey := strings.TrimSpace(input.APIKey)
	if apiKey == "" {
		return AgentLLMProviderConfigResponse{}, domain.NewAppError(domain.ErrorKindInvalidInput, "agent_llm_api_key_required", "api key is required", "service.agent_llm_config.create", false, nil)
	}
	ciphertext, err := s.codec.Seal(apiKey)
	if err != nil {
		return AgentLLMProviderConfigResponse{}, err
	}
	now := s.now().UTC()
	config := domain.AgentLLMProviderConfig{
		UserID:           auth.User.ID,
		Name:             input.Name,
		Provider:         input.Provider,
		BaseURL:          input.BaseURL,
		Model:            input.Model,
		APIKeyCiphertext: ciphertext,
		APIKeyHint:       agentLLMAPIKeyHint(apiKey),
		ProtocolMode:     domain.AgentLLMProtocolMode(input.ProtocolMode),
		Enabled:          true,
		IsDefault:        input.IsDefault,
		TimeoutSeconds:   input.TimeoutSeconds,
		MaxRetries:       input.MaxRetries,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if input.Enabled != nil {
		config.Enabled = *input.Enabled
	}
	config = normalizeServiceAgentLLMProviderConfig(config)
	if err := validateServiceAgentLLMProviderConfig(config); err != nil {
		return AgentLLMProviderConfigResponse{}, err
	}
	existing, err := s.store.ListAgentLLMProviderConfigs(ctx, auth.User.ID)
	if err != nil {
		return AgentLLMProviderConfigResponse{}, err
	}
	if len(existing) == 0 {
		config.IsDefault = true
	}
	if config.IsDefault {
		if err := s.store.ClearDefaultAgentLLMProviderConfigs(ctx, auth.User.ID, 0, now); err != nil {
			return AgentLLMProviderConfigResponse{}, err
		}
	}
	created, err := s.store.CreateAgentLLMProviderConfig(ctx, config)
	if err != nil {
		return AgentLLMProviderConfigResponse{}, err
	}
	return agentLLMProviderConfigResponse(created), nil
}

func (s *AgentLLMConfigService) UpdateConfig(ctx context.Context, auth CurrentAuth, id int64, input UpdateAgentLLMProviderConfigInput) (AgentLLMProviderConfigResponse, error) {
	if err := requireAgentLLMConfigAuth(auth); err != nil {
		return AgentLLMProviderConfigResponse{}, err
	}
	if s == nil || s.store == nil {
		return AgentLLMProviderConfigResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_llm_config_unavailable", "agent llm config service is unavailable", "service.agent_llm_config.update", true, nil)
	}
	config, err := s.store.GetAgentLLMProviderConfig(ctx, auth.User.ID, id)
	if err != nil {
		return AgentLLMProviderConfigResponse{}, err
	}
	if input.Name != nil {
		config.Name = *input.Name
	}
	if input.Provider != nil {
		config.Provider = *input.Provider
	}
	if input.BaseURL != nil {
		config.BaseURL = *input.BaseURL
	}
	if input.Model != nil {
		config.Model = *input.Model
	}
	if input.APIKey != nil {
		apiKey := strings.TrimSpace(*input.APIKey)
		if apiKey == "" {
			return AgentLLMProviderConfigResponse{}, domain.NewAppError(domain.ErrorKindInvalidInput, "agent_llm_api_key_required", "api key is required", "service.agent_llm_config.update", false, nil)
		}
		ciphertext, sealErr := s.codec.Seal(apiKey)
		if sealErr != nil {
			return AgentLLMProviderConfigResponse{}, sealErr
		}
		config.APIKeyCiphertext = ciphertext
		config.APIKeyHint = agentLLMAPIKeyHint(apiKey)
	}
	if input.ProtocolMode != nil {
		config.ProtocolMode = domain.AgentLLMProtocolMode(*input.ProtocolMode)
	}
	if input.Enabled != nil {
		config.Enabled = *input.Enabled
	}
	if input.IsDefault != nil {
		config.IsDefault = *input.IsDefault
	}
	if input.TimeoutSeconds != nil {
		config.TimeoutSeconds = *input.TimeoutSeconds
	}
	if input.MaxRetries != nil {
		config.MaxRetries = *input.MaxRetries
	}
	config.UpdatedAt = s.now().UTC()
	config = normalizeServiceAgentLLMProviderConfig(config)
	if err := validateServiceAgentLLMProviderConfig(config); err != nil {
		return AgentLLMProviderConfigResponse{}, err
	}
	if config.IsDefault {
		if err := s.store.ClearDefaultAgentLLMProviderConfigs(ctx, auth.User.ID, config.ID, config.UpdatedAt); err != nil {
			return AgentLLMProviderConfigResponse{}, err
		}
	}
	updated, err := s.store.UpdateAgentLLMProviderConfig(ctx, config)
	if err != nil {
		return AgentLLMProviderConfigResponse{}, err
	}
	return agentLLMProviderConfigResponse(updated), nil
}

func (s *AgentLLMConfigService) SetDefaultConfig(ctx context.Context, auth CurrentAuth, id int64) (AgentLLMProviderConfigResponse, error) {
	if err := requireAgentLLMConfigAuth(auth); err != nil {
		return AgentLLMProviderConfigResponse{}, err
	}
	if s == nil || s.store == nil {
		return AgentLLMProviderConfigResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_llm_config_unavailable", "agent llm config service is unavailable", "service.agent_llm_config.default", true, nil)
	}
	config, err := s.store.GetAgentLLMProviderConfig(ctx, auth.User.ID, id)
	if err != nil {
		return AgentLLMProviderConfigResponse{}, err
	}
	now := s.now().UTC()
	if err := s.store.ClearDefaultAgentLLMProviderConfigs(ctx, auth.User.ID, config.ID, now); err != nil {
		return AgentLLMProviderConfigResponse{}, err
	}
	config.Enabled = true
	config.IsDefault = true
	config.UpdatedAt = now
	updated, err := s.store.UpdateAgentLLMProviderConfig(ctx, config)
	if err != nil {
		return AgentLLMProviderConfigResponse{}, err
	}
	return agentLLMProviderConfigResponse(updated), nil
}

func (s *AgentLLMConfigService) TestConfig(ctx context.Context, auth CurrentAuth, id int64, input TestAgentLLMProviderConfigInput) (TestAgentLLMProviderConfigResult, error) {
	if err := requireAgentLLMConfigAuth(auth); err != nil {
		return TestAgentLLMProviderConfigResult{}, err
	}
	if s == nil || s.store == nil {
		return TestAgentLLMProviderConfigResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_llm_config_unavailable", "agent llm config service is unavailable", "service.agent_llm_config.test", true, nil)
	}
	config, err := s.store.GetAgentLLMProviderConfig(ctx, auth.User.ID, id)
	if err != nil {
		return TestAgentLLMProviderConfigResult{}, err
	}
	client, err := s.clientForConfig(config)
	if err != nil {
		return TestAgentLLMProviderConfigResult{}, err
	}
	message := strings.TrimSpace(input.Message)
	if message == "" {
		message = agentLLMConfigDefaultTestMessage
	}
	startedAt := time.Now()
	response, err := client.Chat(ctx, llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: "你是连通性测试助手，只返回简短确认文本。"},
			{Role: "user", Content: message},
		},
		Temperature: 0.1,
		MaxTokens:   agentLLMConfigTestMaxTokens,
	})
	if err != nil {
		return TestAgentLLMProviderConfigResult{}, err
	}
	now := s.now().UTC()
	_ = s.store.MarkAgentLLMProviderConfigUsed(context.WithoutCancel(ctx), auth.User.ID, config.ID, now)
	return TestAgentLLMProviderConfigResult{
		Status:       "succeeded",
		Provider:     response.Provider,
		Model:        response.Model,
		LatencyMS:    time.Since(startedAt).Milliseconds(),
		ResponseText: response.Content,
		CheckedAt:    now.Format(time.RFC3339),
	}, nil
}

func (s *AgentLLMConfigService) clientForConfig(config domain.AgentLLMProviderConfig) (AgentConversationLLM, error) {
	apiKey, err := s.codec.Open(config.APIKeyCiphertext)
	if err != nil {
		return nil, err
	}
	return newAgentLLMClientFromConfig(config, apiKey)
}

type AgentLLMRuntime struct {
	store         AgentLLMProviderConfigStore
	defaultClient AgentConversationLLM
	codec         agentLLMConfigCodec
	now           func() time.Time
	mu            sync.Mutex
	cache         map[string]AgentConversationLLM
}

type AgentLLMRuntimeOption func(*AgentLLMRuntime)

func WithAgentLLMRuntimeDefaultClient(client AgentConversationLLM) AgentLLMRuntimeOption {
	return func(runtime *AgentLLMRuntime) {
		runtime.defaultClient = client
	}
}

func WithAgentLLMRuntimeSecret(secret string) AgentLLMRuntimeOption {
	return func(runtime *AgentLLMRuntime) {
		runtime.codec = newAgentLLMConfigCodec(secret)
	}
}

func WithAgentLLMRuntimeNow(now func() time.Time) AgentLLMRuntimeOption {
	return func(runtime *AgentLLMRuntime) {
		if now != nil {
			runtime.now = now
		}
	}
}

func NewAgentLLMRuntime(store AgentLLMProviderConfigStore, options ...AgentLLMRuntimeOption) *AgentLLMRuntime {
	runtime := &AgentLLMRuntime{
		store: store,
		codec: newAgentLLMConfigCodec(""),
		now:   time.Now,
		cache: map[string]AgentConversationLLM{},
	}
	for _, option := range options {
		option(runtime)
	}
	return runtime
}

func (r *AgentLLMRuntime) Chat(ctx context.Context, request llm.ChatRequest) (llm.ChatResponse, error) {
	userID := agentLLMUserIDFromContext(ctx)
	if r == nil || userID < 1 || r.store == nil {
		return chatWithAgentLLMFallback(ctx, r, request)
	}
	config, err := r.store.GetDefaultAgentLLMProviderConfig(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return chatWithAgentLLMFallback(ctx, r, request)
		}
		return llm.ChatResponse{}, err
	}
	client, err := r.clientForConfig(config)
	if err != nil {
		return llm.ChatResponse{}, err
	}
	response, err := client.Chat(ctx, request)
	if err == nil {
		_ = r.store.MarkAgentLLMProviderConfigUsed(context.WithoutCancel(ctx), userID, config.ID, r.now().UTC())
	}
	return response, err
}

func (r *AgentLLMRuntime) clientForConfig(config domain.AgentLLMProviderConfig) (AgentConversationLLM, error) {
	key := agentLLMRuntimeCacheKey(config)
	r.mu.Lock()
	if client := r.cache[key]; client != nil {
		r.mu.Unlock()
		return client, nil
	}
	r.mu.Unlock()
	apiKey, err := r.codec.Open(config.APIKeyCiphertext)
	if err != nil {
		return nil, err
	}
	client, err := newAgentLLMClientFromConfig(config, apiKey)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.cache[key] = client
	r.mu.Unlock()
	return client, nil
}

func chatWithAgentLLMFallback(ctx context.Context, runtime *AgentLLMRuntime, request llm.ChatRequest) (llm.ChatResponse, error) {
	if runtime == nil || runtime.defaultClient == nil {
		return llm.ChatResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "llm_unavailable", "llm client is unavailable", "service.agent_llm_runtime.chat", true, nil)
	}
	return runtime.defaultClient.Chat(ctx, request)
}

type agentLLMUserIDContextKey struct{}

func withAgentLLMUserID(ctx context.Context, userID int64) context.Context {
	if ctx == nil || userID < 1 {
		return ctx
	}
	return context.WithValue(ctx, agentLLMUserIDContextKey{}, userID)
}

func agentLLMUserIDFromContext(ctx context.Context) int64 {
	if ctx == nil {
		return 0
	}
	value, _ := ctx.Value(agentLLMUserIDContextKey{}).(int64)
	if value < 1 {
		return 0
	}
	return value
}

func newAgentLLMClientFromConfig(config domain.AgentLLMProviderConfig, apiKey string) (AgentConversationLLM, error) {
	return llm.NewOpenAICompatibleClient(llm.OpenAICompatibleConfig{
		Provider:     config.Provider,
		BaseURL:      config.BaseURL,
		APIKey:       apiKey,
		Model:        config.Model,
		ProtocolMode: string(config.ProtocolMode),
		Timeout:      time.Duration(config.TimeoutSeconds) * time.Second,
		MaxAttempts:  config.MaxRetries,
	})
}

func requireAgentLLMConfigAuth(auth CurrentAuth) error {
	if !auth.Authenticated || auth.User.ID < 1 {
		return fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	return nil
}

func normalizeServiceAgentLLMProviderConfig(config domain.AgentLLMProviderConfig) domain.AgentLLMProviderConfig {
	config.Name = strings.TrimSpace(config.Name)
	config.Provider = strings.TrimSpace(strings.ToLower(config.Provider))
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	config.Model = strings.TrimSpace(config.Model)
	if !config.ProtocolMode.Valid() {
		config.ProtocolMode = domain.AgentLLMProtocolModeAuto
	}
	if config.TimeoutSeconds < 10 {
		config.TimeoutSeconds = defaultAgentLLMConfigTimeoutSeconds
	}
	if config.TimeoutSeconds > 3600 {
		config.TimeoutSeconds = 3600
	}
	if config.MaxRetries < 1 {
		config.MaxRetries = defaultAgentLLMConfigMaxRetries
	}
	if config.MaxRetries > 50 {
		config.MaxRetries = 50
	}
	return config
}

func validateServiceAgentLLMProviderConfig(config domain.AgentLLMProviderConfig) error {
	if config.UserID < 1 {
		return domain.NewAppError(domain.ErrorKindInvalidInput, "agent_llm_user_required", "user id is required", "service.agent_llm_config.validate", false, nil)
	}
	if config.Name == "" {
		return domain.NewAppError(domain.ErrorKindInvalidInput, "agent_llm_name_required", "name is required", "service.agent_llm_config.validate", false, nil)
	}
	if config.Provider == "" || !isAgentLLMProviderName(config.Provider) {
		return domain.NewAppError(domain.ErrorKindInvalidInput, "agent_llm_provider_invalid", "provider is invalid", "service.agent_llm_config.validate", false, nil)
	}
	if config.Model == "" {
		return domain.NewAppError(domain.ErrorKindInvalidInput, "agent_llm_model_required", "model is required", "service.agent_llm_config.validate", false, nil)
	}
	if config.APIKeyCiphertext == "" {
		return domain.NewAppError(domain.ErrorKindInvalidInput, "agent_llm_api_key_required", "api key is required", "service.agent_llm_config.validate", false, nil)
	}
	if config.BaseURL != "" {
		parsed, err := url.Parse(config.BaseURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return domain.NewAppError(domain.ErrorKindInvalidInput, "agent_llm_base_url_invalid", "base url is invalid", "service.agent_llm_config.validate", false, err)
		}
	}
	return nil
}

func isAgentLLMProviderName(provider string) bool {
	for _, r := range provider {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '_' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return provider != ""
}

func agentLLMProviderConfigResponse(config domain.AgentLLMProviderConfig) AgentLLMProviderConfigResponse {
	response := AgentLLMProviderConfigResponse{
		ID:             config.ID,
		Name:           config.Name,
		Provider:       config.Provider,
		BaseURL:        config.BaseURL,
		Model:          config.Model,
		APIKeyHint:     config.APIKeyHint,
		APIKeyPresent:  strings.TrimSpace(config.APIKeyCiphertext) != "",
		ProtocolMode:   string(config.ProtocolMode),
		Enabled:        config.Enabled,
		IsDefault:      config.IsDefault,
		TimeoutSeconds: config.TimeoutSeconds,
		MaxRetries:     config.MaxRetries,
		CreatedAt:      config.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:      config.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if config.LastUsedAt != nil {
		response.LastUsedAt = config.LastUsedAt.UTC().Format(time.RFC3339)
	}
	return response
}

func agentLLMFallbackConfigResponse(config config.LLMConfig) AgentLLMFallbackConfigResponse {
	return AgentLLMFallbackConfigResponse{
		Enabled:       config.Enabled(),
		Provider:      config.Provider,
		Model:         config.Model,
		BaseURL:       config.BaseURL,
		APIKeyPresent: strings.TrimSpace(config.APIKey) != "",
	}
}

func agentLLMAPIKeyHint(apiKey string) string {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return ""
	}
	if len([]rune(apiKey)) <= 4 {
		return "****"
	}
	runes := []rune(apiKey)
	return "****" + string(runes[len(runes)-4:])
}

func agentLLMRuntimeCacheKey(config domain.AgentLLMProviderConfig) string {
	return fmt.Sprintf(
		"%d:%d:%s:%s:%s:%d:%d:%s",
		config.UserID,
		config.ID,
		config.UpdatedAt.UTC().Format(time.RFC3339Nano),
		config.Provider,
		config.Model,
		config.TimeoutSeconds,
		config.MaxRetries,
		config.ProtocolMode,
	)
}

type agentLLMConfigCodec struct {
	key [32]byte
}

func newAgentLLMConfigCodec(secret string) agentLLMConfigCodec {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		secret = "messagefeed-agent-llm-config-local"
	}
	return agentLLMConfigCodec{key: sha256.Sum256([]byte("agent_llm_config:v1:" + secret))}
}

func (codec agentLLMConfigCodec) Seal(plaintext string) (string, error) {
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return "", domain.NewAppError(domain.ErrorKindInvalidInput, "agent_llm_api_key_required", "api key is required", "service.agent_llm_config.seal", false, nil)
	}
	block, err := aes.NewCipher(codec.key[:])
	if err != nil {
		return "", domain.NewAppError(domain.ErrorKindInternal, "agent_llm_cipher_unavailable", "agent llm config cipher is unavailable", "service.agent_llm_config.seal", false, err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", domain.NewAppError(domain.ErrorKindInternal, "agent_llm_cipher_unavailable", "agent llm config cipher is unavailable", "service.agent_llm_config.seal", false, err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", domain.NewAppError(domain.ErrorKindInternal, "agent_llm_nonce_failed", "agent llm config nonce generation failed", "service.agent_llm_config.seal", true, err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return "v1." + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (codec agentLLMConfigCodec) Open(ciphertext string) (string, error) {
	ciphertext = strings.TrimSpace(ciphertext)
	if !strings.HasPrefix(ciphertext, "v1.") {
		return "", domain.NewAppError(domain.ErrorKindInvalidInput, "agent_llm_ciphertext_invalid", "agent llm config ciphertext is invalid", "service.agent_llm_config.open", false, nil)
	}
	body, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(ciphertext, "v1."))
	if err != nil {
		return "", domain.NewAppError(domain.ErrorKindInvalidInput, "agent_llm_ciphertext_invalid", "agent llm config ciphertext is invalid", "service.agent_llm_config.open", false, err)
	}
	block, err := aes.NewCipher(codec.key[:])
	if err != nil {
		return "", domain.NewAppError(domain.ErrorKindInternal, "agent_llm_cipher_unavailable", "agent llm config cipher is unavailable", "service.agent_llm_config.open", false, err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", domain.NewAppError(domain.ErrorKindInternal, "agent_llm_cipher_unavailable", "agent llm config cipher is unavailable", "service.agent_llm_config.open", false, err)
	}
	if len(body) < gcm.NonceSize() {
		return "", domain.NewAppError(domain.ErrorKindInvalidInput, "agent_llm_ciphertext_invalid", "agent llm config ciphertext is invalid", "service.agent_llm_config.open", false, nil)
	}
	nonce := body[:gcm.NonceSize()]
	payload := body[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", domain.NewAppError(domain.ErrorKindInvalidInput, "agent_llm_ciphertext_invalid", "agent llm config ciphertext is invalid", "service.agent_llm_config.open", false, err)
	}
	return strings.TrimSpace(string(plaintext)), nil
}

func AgentLLMConfigSecretFromConfig(cfg config.Config) string {
	for _, value := range []string{
		cfg.LLM.ConfigSecret,
		cfg.Auth.OwnerPassword,
		cfg.WeChatWork.EncodingAESKey,
		cfg.LLM.APIKey,
	} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "messagefeed-agent-llm-config-local"
}
