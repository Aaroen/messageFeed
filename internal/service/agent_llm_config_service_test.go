package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
)

func TestAgentLLMConfigServiceCreateMasksAndDefaults(t *testing.T) {
	store := newAgentLLMConfigMemoryStore()
	service := NewAgentLLMConfigService(
		store,
		WithAgentLLMConfigSecret("secret-a"),
		WithAgentLLMConfigNow(func() time.Time { return fixedAgentLLMConfigNow() }),
	)
	auth := CurrentAuth{Authenticated: true, User: domain.User{ID: 7}}

	created, err := service.CreateConfig(context.Background(), auth, CreateAgentLLMProviderConfigInput{
		Name:         "local",
		Provider:     "openai_compatible",
		BaseURL:      "http://127.0.0.1:15721/v1",
		Model:        "default",
		APIKey:       "sk-runtime-1234",
		ProtocolMode: "responses",
	})
	if err != nil {
		t.Fatalf("CreateConfig() error = %v", err)
	}
	if !created.IsDefault {
		t.Fatal("first config should become default")
	}
	if created.APIKeyHint != "****1234" || !created.APIKeyPresent {
		t.Fatalf("api key response = %#v", created)
	}
	if strings.Contains(store.configs[created.ID].APIKeyCiphertext, "sk-runtime-1234") {
		t.Fatal("repository stored raw api key")
	}
	opened, err := service.codec.Open(store.configs[created.ID].APIKeyCiphertext)
	if err != nil {
		t.Fatalf("decrypt api key: %v", err)
	}
	if opened != "sk-runtime-1234" {
		t.Fatalf("decrypted api key = %q", opened)
	}

	list, err := service.ListConfigs(context.Background(), auth)
	if err != nil {
		t.Fatalf("ListConfigs() error = %v", err)
	}
	if len(list.Configs) != 1 || !list.Configs[0].IsDefault {
		t.Fatalf("configs = %#v", list.Configs)
	}
}

func TestAgentLLMRuntimeUsesUserDefaultConfig(t *testing.T) {
	var requestedPath string
	var requestedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_1","object":"response","status":"completed","model":"runtime-model","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"runtime ok"}]}]}`))
	}))
	defer server.Close()

	codec := newAgentLLMConfigCodec("secret-b")
	ciphertext, err := codec.Seal("runtime-key")
	if err != nil {
		t.Fatalf("seal api key: %v", err)
	}
	store := newAgentLLMConfigMemoryStore()
	store.configs[1] = domain.AgentLLMProviderConfig{
		ID:               1,
		UserID:           9,
		Name:             "runtime",
		Provider:         "openai_compatible",
		BaseURL:          server.URL,
		Model:            "runtime-model",
		APIKeyCiphertext: ciphertext,
		APIKeyHint:       "****-key",
		ProtocolMode:     domain.AgentLLMProtocolModeResponses,
		Enabled:          true,
		IsDefault:        true,
		TimeoutSeconds:   10,
		MaxRetries:       1,
		CreatedAt:        fixedAgentLLMConfigNow(),
		UpdatedAt:        fixedAgentLLMConfigNow(),
	}
	runtime := NewAgentLLMRuntime(store, WithAgentLLMRuntimeSecret("secret-b"))
	response, err := runtime.Chat(withAgentLLMUserID(context.Background(), 9), llm.ChatRequest{
		Messages:  []llm.ChatMessage{{Role: "user", Content: "ping"}},
		MaxTokens: 64,
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if response.Content != "runtime ok" || response.Provider != "openai_compatible" || response.Model != "runtime-model" {
		t.Fatalf("response = %#v", response)
	}
	if requestedPath != "/responses" {
		t.Fatalf("requested path = %q", requestedPath)
	}
	if requestedAuth != "Bearer runtime-key" {
		t.Fatalf("authorization header = %q", requestedAuth)
	}
	if store.configs[1].LastUsedAt == nil {
		t.Fatal("last_used_at was not recorded")
	}
}

func TestAgentLLMRuntimeFallsBackWithoutUserConfig(t *testing.T) {
	fallback := &fakeAgentLLMRuntimeClient{
		response: llm.ChatResponse{Provider: "fallback", Model: "model-a", Content: "fallback ok"},
	}
	runtime := NewAgentLLMRuntime(
		newAgentLLMConfigMemoryStore(),
		WithAgentLLMRuntimeDefaultClient(fallback),
		WithAgentLLMRuntimeSecret("secret-c"),
	)
	response, err := runtime.Chat(withAgentLLMUserID(context.Background(), 11), llm.ChatRequest{
		Messages: []llm.ChatMessage{{Role: "user", Content: "ping"}},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if fallback.calls != 1 || response.Content != "fallback ok" {
		t.Fatalf("fallback calls = %d response = %#v", fallback.calls, response)
	}
}

func fixedAgentLLMConfigNow() time.Time {
	return time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
}

type fakeAgentLLMRuntimeClient struct {
	calls    int
	response llm.ChatResponse
	err      error
}

func (f *fakeAgentLLMRuntimeClient) Chat(_ context.Context, _ llm.ChatRequest) (llm.ChatResponse, error) {
	f.calls++
	if f.err != nil {
		return llm.ChatResponse{}, f.err
	}
	return f.response, nil
}

type agentLLMConfigMemoryStore struct {
	nextID  int64
	configs map[int64]domain.AgentLLMProviderConfig
}

func newAgentLLMConfigMemoryStore() *agentLLMConfigMemoryStore {
	return &agentLLMConfigMemoryStore{nextID: 1, configs: map[int64]domain.AgentLLMProviderConfig{}}
}

func (s *agentLLMConfigMemoryStore) ListAgentLLMProviderConfigs(_ context.Context, userID int64) ([]domain.AgentLLMProviderConfig, error) {
	configs := make([]domain.AgentLLMProviderConfig, 0)
	for _, config := range s.configs {
		if config.UserID == userID {
			configs = append(configs, config)
		}
	}
	return configs, nil
}

func (s *agentLLMConfigMemoryStore) GetAgentLLMProviderConfig(_ context.Context, userID int64, id int64) (domain.AgentLLMProviderConfig, error) {
	config, ok := s.configs[id]
	if !ok || config.UserID != userID {
		return domain.AgentLLMProviderConfig{}, domain.ErrNotFound
	}
	return config, nil
}

func (s *agentLLMConfigMemoryStore) GetDefaultAgentLLMProviderConfig(_ context.Context, userID int64) (domain.AgentLLMProviderConfig, error) {
	for _, config := range s.configs {
		if config.UserID == userID && config.Enabled && config.IsDefault {
			return config, nil
		}
	}
	return domain.AgentLLMProviderConfig{}, domain.ErrNotFound
}

func (s *agentLLMConfigMemoryStore) CreateAgentLLMProviderConfig(_ context.Context, config domain.AgentLLMProviderConfig) (domain.AgentLLMProviderConfig, error) {
	if config.ID == 0 {
		config.ID = s.nextID
		s.nextID++
	}
	if _, ok := s.configs[config.ID]; ok {
		return domain.AgentLLMProviderConfig{}, domain.ErrConflict
	}
	s.configs[config.ID] = config
	return config, nil
}

func (s *agentLLMConfigMemoryStore) UpdateAgentLLMProviderConfig(_ context.Context, config domain.AgentLLMProviderConfig) (domain.AgentLLMProviderConfig, error) {
	if _, ok := s.configs[config.ID]; !ok {
		return domain.AgentLLMProviderConfig{}, domain.ErrNotFound
	}
	s.configs[config.ID] = config
	return config, nil
}

func (s *agentLLMConfigMemoryStore) ClearDefaultAgentLLMProviderConfigs(_ context.Context, userID int64, exceptID int64, now time.Time) error {
	for id, config := range s.configs {
		if config.UserID != userID || id == exceptID {
			continue
		}
		config.IsDefault = false
		config.UpdatedAt = now
		s.configs[id] = config
	}
	return nil
}

func (s *agentLLMConfigMemoryStore) MarkAgentLLMProviderConfigUsed(_ context.Context, userID int64, id int64, now time.Time) error {
	config, ok := s.configs[id]
	if !ok || config.UserID != userID {
		return domain.ErrNotFound
	}
	usedAt := now
	config.LastUsedAt = &usedAt
	config.UpdatedAt = now
	s.configs[id] = config
	return nil
}
