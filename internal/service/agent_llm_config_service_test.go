package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
)

func TestAgentGuestClientHashUsesCanonicalIPAddress(t *testing.T) {
	service := &AgentLLMConfigService{codec: newAgentLLMConfigCodec("guest-address-acceptance")}
	first, err := service.guestClientHash("192.0.2.10")
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := service.guestClientHash("::ffff:192.0.2.10")
	if err != nil || first != canonical || len(first) != 64 {
		t.Fatal("等价 IP 未使用相同游客身份")
	}
	other, err := service.guestClientHash("192.0.2.11")
	if err != nil || first == other {
		t.Fatal("不同 IP 使用相同游客身份")
	}
	for _, invalid := range []string{"", "guest", "192.0.2.10, 192.0.2.11", "192.0.2.10:80"} {
		if _, err := service.guestClientHash(invalid); err == nil {
			t.Fatalf("无效来源地址未被拒绝：%q", invalid)
		}
	}
}

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

func TestAgentLLMRuntimeSelectsUserDefaultConfig(t *testing.T) {
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
	resolved, err := runtime.clientForUser(context.Background(), 9)
	if err != nil {
		t.Fatal(err)
	}
	response, err := resolved.client.Chat(context.Background(), llm.ChatRequest{
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
	if resolved.configID != 1 || resolved.configuration["config_key"] != agentLLMRuntimeCacheKey(store.configs[1]) {
		t.Fatalf("configuration = %#v", resolved.configuration)
	}
}

func TestAgentLLMRuntimeSelectsEnvironmentClientWithoutUserConfig(t *testing.T) {
	fallback := &fakeAgentLLMRuntimeClient{
		response: llm.ChatResponse{Provider: "fallback", Model: "model-a", Content: "fallback ok"},
	}
	runtime := NewAgentLLMRuntime(
		newAgentLLMConfigMemoryStore(),
		WithAgentLLMRuntimeDefaultClient(fallback),
		WithAgentLLMRuntimeSecret("secret-c"),
	)
	resolved, err := runtime.clientForUser(context.Background(), 11)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resolved.client != fallback || resolved.configID != 0 || fallback.calls != 0 {
		t.Fatalf("resolved client = %#v", resolved)
	}
}

func TestAgentLLMRuntimeRequiresActualCallContext(t *testing.T) {
	runtime := NewAgentLLMRuntime(nil)
	if _, err := runtime.Chat(context.Background(), llm.ChatRequest{}); err == nil {
		t.Fatal("缺少调用上下文时必须返回错误")
	}
	metadata := AgentLLMCallContext{UserID: 9, SessionID: 12, TurnID: 31, Stage: AgentLLMCallStageGeneration}
	ctx := WithAgentLLMCallContext(context.Background(), metadata)
	actual, err := agentLLMCallContextFromContext(ctx)
	if err != nil || actual.UserID != 9 || actual.SessionID != 12 || actual.TurnID != 31 || actual.PromptVersion != AgentLLMCallPromptVersion {
		t.Fatalf("metadata = %#v, error = %v", actual, err)
	}
	if _, err := runtime.Chat(ctx, llm.ChatRequest{}); err == nil {
		t.Fatal("未注入 RunManager 时必须返回错误")
	}
	metadata.TurnID = 0
	if _, err := agentLLMCallContextFromContext(WithAgentLLMCallContext(context.Background(), metadata)); err == nil {
		t.Fatal("缺少真实轮次时必须返回错误")
	}
	metadata.TurnID = 31
	ctx = withAgentLLMUserID(WithAgentLLMCallContext(context.Background(), metadata), 10)
	if _, err := agentLLMCallContextFromContext(ctx); err == nil {
		t.Fatal("用户身份不一致时必须返回错误")
	}
}

func TestAgentLLMCallSnapshotsPreserveMessagesAndResponseMetadata(t *testing.T) {
	metadata := AgentLLMCallContext{UserID: 9, SessionID: 12, TurnID: 31, Stage: AgentLLMCallStageGeneration, PromptVersion: AgentLLMCallPromptVersion}
	longContent := strings.Repeat("完整上下文", 1000)
	request := llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: longContent},
			{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "web.search", Arguments: `{"query":"上下文"}`}}},
			{Role: "tool", ToolCallID: "call-1", Content: longContent},
			{Role: "user", Content: "保留完整回答"},
		},
		ToolChoice: "auto", Temperature: 0.2, MaxTokens: 400,
	}
	snapshot := agentLLMRequestSnapshot(metadata, request, domain.AgentJSON{"config_id": int64(7), "config_key": "version-3"})
	body, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Request struct {
			Messages  []llm.ChatMessage `json:"messages"`
			MaxTokens int               `json:"max_tokens"`
		} `json:"request"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Request.Messages) != 4 || decoded.Request.Messages[0].Content != longContent || decoded.Request.Messages[2].Content != longContent || decoded.Request.Messages[1].ToolCalls[0].ID != "call-1" || decoded.Request.MaxTokens != 400 {
		t.Fatal("请求快照未保留完整消息及请求参数")
	}
	response := llm.ChatResponse{ResponseID: "response-1", Protocol: "chat_completions", FinishReason: "length", IncompleteReason: "length", Content: "S", Usage: &llm.TokenUsage{InputTokens: 10, OutputTokens: 1, TotalTokens: 11}}
	responseSnapshot := agentLLMResponseSnapshot(metadata, response, domain.AgentRunContextTrace{ID: 25, ContentHash: "request-hash"}, domain.ErrInvalidInput)
	if responseSnapshot["response_id"] != "response-1" || responseSnapshot["incomplete_reason"] != "length" || responseSnapshot["request_content_hash"] != "request-hash" || responseSnapshot["content"] != "S" || responseSnapshot["error"] == nil {
		t.Fatalf("response snapshot = %#v", responseSnapshot)
	}
}

func TestAgentMemoryClassificationRequiresTranscriptTurn(t *testing.T) {
	service := NewAgentConversationService(nil)
	classification, err := service.classifyAgentMemoryCandidateWithModel(context.Background(), domain.AgentTranscriptEntry{
		ID: 12, UserID: 42, SessionID: 22, Content: "长期回复偏好",
	}, domain.AgentMemoryTopic{}, false)
	if err == nil || classification.ShouldCapture {
		t.Fatalf("classification = %#v, error = %v", classification, err)
	}
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != "agent_llm_call_context_invalid" {
		t.Fatalf("error = %v", err)
	}
}

func TestAgentLLMRequestBudgetIncludesToolsAndRetainsMessages(test *testing.T) {
	request := llm.ChatRequest{Messages: []llm.ChatMessage{{Role: "user", Content: "保留完整的当前要求"}}, MaxTokens: 512}
	budget, err := agentLLMRequestBudget(AgentLLMCallStageGeneration, request)
	if err != nil || budget["includes_all_messages_and_tools"] != true || budget["counting_method"] != "utf8_byte_upper_bound" {
		test.Fatalf("完整请求预算无效：%v，%v", budget, err)
	}
	request.Tools = []llm.ToolDefinition{{Name: "large_tool", Description: strings.Repeat("完整工具定义", 10000)}}
	if _, err := agentLLMRequestBudget(AgentLLMCallStageGeneration, request); err == nil {
		test.Fatal("超出预算的工具定义未被拒绝")
	}
	if request.Messages[0].Content != "保留完整的当前要求" || len(request.Tools[0].Description) != len(strings.Repeat("完整工具定义", 10000)) {
		test.Fatal("预算检查修改了实际消息或工具定义")
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
