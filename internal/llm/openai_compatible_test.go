package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAICompatibleClientUsesCustomBaseURL(t *testing.T) {
	var receivedPath string
	var receivedAuth string
	var receivedModel string
	var receivedUserContent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")
		var request chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		receivedModel = request.Model
		if len(request.Messages) > 1 {
			receivedUserContent = request.Messages[1].Content
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"自定义 AI 回复"}}]}`))
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

	if receivedPath != "/chat/completions" {
		t.Fatalf("path = %q, want /chat/completions", receivedPath)
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
