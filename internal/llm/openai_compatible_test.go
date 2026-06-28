package llm

import (
	"context"
	"encoding/json"
	"errors"
	"messagefeed/internal/domain"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestOpenAICompatibleClientUsesCustomBaseURL(t *testing.T) {
	var receivedPath string
	var receivedAuth string
	var receivedModel string
	var receivedUserContent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")
		var request responsesRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		receivedModel = request.Model
		if len(request.Input) > 1 {
			receivedUserContent = request.Input[1].Content
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"自定义 AI 回复"}]}]}`))
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(OpenAICompatibleConfig{
		Provider: "openai_compatible",
		BaseURL:  server.URL,
		APIKey:   "test-key",
		Model:    "custom-model",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleClient() error = %v", err)
	}

	response, err := client.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{
			{Role: "system", Content: "你是 messageFeed AI。"},
			{Role: "user", Content: "请总结最近内容"},
		},
		MaxTokens: 256,
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if receivedPath != "/responses" {
		t.Fatalf("path = %q, want /responses", receivedPath)
	}
	if receivedAuth != "Bearer test-key" {
		t.Fatalf("Authorization = %q", receivedAuth)
	}
	if receivedModel != "custom-model" {
		t.Fatalf("model = %q, want custom-model", receivedModel)
	}
	if receivedUserContent != "请总结最近内容" {
		t.Fatalf("user content = %q", receivedUserContent)
	}
	if response.Content != "自定义 AI 回复" {
		t.Fatalf("Content = %q", response.Content)
	}
	if response.Provider != "openai_compatible" {
		t.Fatalf("Provider = %q", response.Provider)
	}
}

func TestOpenAICompatibleClientReturnsProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "custom-model",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleClient() error = %v", err)
	}

	if _, err := client.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
	}); err == nil {
		t.Fatal("Chat() error = nil, want provider error")
	}
}

func TestOpenAICompatibleClientRetriesRetryableStatus(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"upstream saturated"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"retry ok"}]}]}`))
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "custom-model",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleClient() error = %v", err)
	}

	response, err := client.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if response.Content != "retry ok" {
		t.Fatalf("response = %#v", response)
	}
}

func TestOpenAICompatibleClientDefaultTimeoutIsThinkingWindow(t *testing.T) {
	client, err := NewOpenAICompatibleClient(OpenAICompatibleConfig{
		BaseURL: "https://llm.example/v1",
		APIKey:  "test-key",
		Model:   "custom-model",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleClient() error = %v", err)
	}
	if client.httpClient == nil {
		t.Fatal("http client is nil")
	}
	if client.httpClient.Timeout != 180*time.Second {
		t.Fatalf("timeout = %s, want 180s", client.httpClient.Timeout)
	}
}

func TestOpenAICompatibleClientClassifiesRequestTimeoutAsThinkingTimeout(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"late"}}]}`))
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "custom-model",
		HTTPClient: &http.Client{
			Timeout: 10 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleClient() error = %v", err)
	}

	_, err = client.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "需要较长思考"}},
	})
	if err == nil {
		t.Fatal("Chat() error = nil, want thinking timeout")
	}
	var appErr *domain.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %T %v, want AppError", err, err)
	}
	if appErr.Code != "llm_thinking_timeout" {
		t.Fatalf("code = %q, want llm_thinking_timeout", appErr.Code)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Fatalf("attempts = %d, want 1", got)
	}
}

func TestOpenAICompatibleClientSendsToolsAndParsesToolCalls(t *testing.T) {
	var receivedToolName string
	var receivedToolChoice any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request responsesRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(request.Tools) != 1 {
			t.Fatalf("tool count = %d, want 1", len(request.Tools))
		}
		receivedToolName = request.Tools[0].Name
		receivedToolChoice = request.ToolChoice
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"response","status":"completed","output":[{"type":"function_call","call_id":"call-1","name":"conversation__query_history","arguments":"{\"keyword\":\"偏好\"}"}]}`))
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "custom-model",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleClient() error = %v", err)
	}

	response, err := client.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "查一下历史"}},
		Tools: []ToolDefinition{
			{
				Name:        "conversation__query_history",
				Description: "查询历史聊天",
				InputSchema: map[string]any{"type": "object"},
			},
		},
		ToolChoice: "auto",
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if receivedToolName != "conversation__query_history" {
		t.Fatalf("tool name = %q", receivedToolName)
	}
	if receivedToolChoice != "auto" {
		t.Fatalf("tool choice = %#v", receivedToolChoice)
	}
	if len(response.ToolCalls) != 1 {
		t.Fatalf("tool calls = %#v", response.ToolCalls)
	}
	if response.ToolCalls[0].Name != "conversation__query_history" || response.ToolCalls[0].Arguments != `{"keyword":"偏好"}` {
		t.Fatalf("tool call = %#v", response.ToolCalls[0])
	}
}

func TestOpenAICompatibleClientFallsBackToChatCompletionsAndRemembersRoute(t *testing.T) {
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/responses":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"message":"not found"}}`))
		case "/chat/completions":
			var request chatCompletionRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("decode chat request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"chat ok"}}]}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "custom-model",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleClient() error = %v", err)
	}

	for i := 0; i < 2; i++ {
		response, err := client.Chat(context.Background(), ChatRequest{
			Messages: []ChatMessage{{Role: "user", Content: "hello"}},
		})
		if err != nil {
			t.Fatalf("Chat() attempt %d error = %v", i+1, err)
		}
		if response.Content != "chat ok" {
			t.Fatalf("response = %#v", response)
		}
	}
	want := []string{"/responses", "/chat/completions", "/chat/completions"}
	if len(paths) != len(want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths = %#v, want %#v", paths, want)
		}
	}
}

func TestOpenAICompatibleClientUsesResponsesFirstForToolsEvenWhenChatIsRemembered(t *testing.T) {
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/chat/completions":
			t.Fatalf("tools request should try /responses before remembered chat route")
		case "/responses":
			var request responsesRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("decode responses request: %v", err)
			}
			if len(request.Tools) != 1 || request.Tools[0].Name != "conversation__query_history" {
				t.Fatalf("responses tools = %#v", request.Tools)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"object":"response","status":"completed","output":[{"type":"function_call","call_id":"call-1","name":"conversation__query_history","arguments":"{\"query\":\"偏好\"}"}]}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "custom-model",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleClient() error = %v", err)
	}
	client.rememberProtocol(llmProtocolChatCompletions)

	response, err := client.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "查历史"}},
		Tools: []ToolDefinition{{
			Name:        "conversation__query_history",
			Description: "查询历史",
			InputSchema: map[string]any{"type": "object"},
		}},
		ToolChoice: "required",
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != "conversation__query_history" {
		t.Fatalf("tool calls = %#v", response.ToolCalls)
	}
	want := []string{"/responses"}
	if len(paths) != len(want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths = %#v, want %#v", paths, want)
		}
	}
}

func TestOpenAICompatibleClientFallsBackToChatCompletionsForToolsWhenResponsesUnavailable(t *testing.T) {
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/responses":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"message":"not found"}}`))
		case "/chat/completions":
			var request chatCompletionRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("decode chat request: %v", err)
			}
			if len(request.Tools) != 1 || request.Tools[0].Function.Name != "conversation__query_history" {
				t.Fatalf("chat tools = %#v", request.Tools)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call-1","type":"function","function":{"name":"conversation__query_history","arguments":"{\"query\":\"偏好\"}"}}]}}]}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "custom-model",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleClient() error = %v", err)
	}

	response, err := client.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "查历史"}},
		Tools: []ToolDefinition{{
			Name:        "conversation__query_history",
			Description: "查询历史",
			InputSchema: map[string]any{"type": "object"},
		}},
		ToolChoice: "required",
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != "conversation__query_history" {
		t.Fatalf("tool calls = %#v", response.ToolCalls)
	}
	want := []string{"/responses", "/chat/completions"}
	if len(paths) != len(want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths = %#v, want %#v", paths, want)
		}
	}
}
