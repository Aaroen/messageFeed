package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"messagefeed/internal/domain"
)

const defaultOpenAIBaseURL = "https://api.openai.com/v1"

type ChatMessage struct {
	Role    string
	Content string
}

type ChatRequest struct {
	Messages    []ChatMessage
	Temperature float64
	MaxTokens   int
}

type ChatResponse struct {
	Provider string
	Model    string
	Content  string
}

type Client interface {
	Chat(ctx context.Context, request ChatRequest) (ChatResponse, error)
}

type OpenAICompatibleConfig struct {
	Provider   string
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

type OpenAICompatibleClient struct {
	provider   string
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

type chatCompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []chatCompletionMessage `json:"messages"`
	Temperature float64                 `json:"temperature,omitempty"`
	MaxTokens   int                     `json:"max_tokens,omitempty"`
}

type chatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatCompletionMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func NewOpenAICompatibleClient(config OpenAICompatibleConfig) (*OpenAICompatibleClient, error) {
	provider := strings.TrimSpace(config.Provider)
	if provider == "" {
		provider = "openai_compatible"
	}
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	if parsed, err := url.Parse(baseURL); err != nil || parsed.Scheme == "" || parsed.Host == "" {
		if err == nil {
			err = fmt.Errorf("scheme and host are required")
		}
		return nil, domain.NewAppError(domain.ErrorKindInvalidInput, "llm_invalid_base_url", "invalid llm base url", "llm.openai_compatible.new", false, err)
	}
	apiKey := strings.TrimSpace(config.APIKey)
	if apiKey == "" {
		return nil, domain.NewAppError(domain.ErrorKindInvalidInput, "llm_missing_api_key", "llm api key is required", "llm.openai_compatible.new", false, nil)
	}
	model := strings.TrimSpace(config.Model)
	if model == "" {
		return nil, domain.NewAppError(domain.ErrorKindInvalidInput, "llm_missing_model", "llm model is required", "llm.openai_compatible.new", false, nil)
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &OpenAICompatibleClient{
		provider:   provider,
		baseURL:    baseURL,
		apiKey:     apiKey,
		model:      model,
		httpClient: httpClient,
	}, nil
}

func (c *OpenAICompatibleClient) Chat(ctx context.Context, request ChatRequest) (ChatResponse, error) {
	if c == nil || c.httpClient == nil {
		return ChatResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "llm_unavailable", "llm client is unavailable", "llm.openai_compatible.chat", true, nil)
	}
	payload := chatCompletionRequest{
		Model:       c.model,
		Messages:    make([]chatCompletionMessage, 0, len(request.Messages)),
		Temperature: request.Temperature,
		MaxTokens:   request.MaxTokens,
	}
	for _, message := range request.Messages {
		role := strings.TrimSpace(message.Role)
		content := strings.TrimSpace(message.Content)
		if role == "" || content == "" {
			continue
		}
		payload.Messages = append(payload.Messages, chatCompletionMessage{Role: role, Content: content})
	}
	if len(payload.Messages) == 0 {
		return ChatResponse{}, domain.NewAppError(domain.ErrorKindInvalidInput, "llm_empty_messages", "llm messages must not be empty", "llm.openai_compatible.chat", false, nil)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return ChatResponse{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatResponse{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return ChatResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "llm_request_failed", "llm request failed", "llm.openai_compatible.chat", true, err)
	}
	defer httpResponse.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(httpResponse.Body, 1<<20))
	if err != nil {
		return ChatResponse{}, err
	}
	var decoded chatCompletionResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return ChatResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "llm_invalid_response", "llm response is invalid", "llm.openai_compatible.chat", true, err)
	}
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(http.StatusText(httpResponse.StatusCode))
		if decoded.Error != nil && strings.TrimSpace(decoded.Error.Message) != "" {
			message = decoded.Error.Message
		}
		return ChatResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "llm_provider_error", message, "llm.openai_compatible.chat", true, nil)
	}
	if len(decoded.Choices) == 0 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return ChatResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "llm_empty_response", "llm response is empty", "llm.openai_compatible.chat", true, nil)
	}
	return ChatResponse{
		Provider: c.provider,
		Model:    c.model,
		Content:  strings.TrimSpace(decoded.Choices[0].Message.Content),
	}, nil
}
