package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"messagefeed/internal/domain"
	"messagefeed/internal/metrics"
	"messagefeed/internal/observability"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
)

const defaultOpenAIBaseURL = "https://api.openai.com/v1"
const openAICompatibleChatOperation = "chat_completions"
const openAICompatibleExternalHTTPOperation = "llm_chat_completions"

type ChatMessage struct {
	Role       string
	Content    string
	Name       string
	ToolCallID string
	ToolCalls  []ToolCall
}

type ChatRequest struct {
	Messages    []ChatMessage
	Tools       []ToolDefinition
	ToolChoice  string
	Temperature float64
	MaxTokens   int
}

type ChatResponse struct {
	Provider  string
	Model     string
	Content   string
	ToolCalls []ToolCall
}

type Client interface {
	Chat(ctx context.Context, request ChatRequest) (ChatResponse, error)
}

type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
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
	Tools       []chatCompletionTool    `json:"tools,omitempty"`
	ToolChoice  any                     `json:"tool_choice,omitempty"`
	Temperature float64                 `json:"temperature,omitempty"`
	MaxTokens   int                     `json:"max_tokens,omitempty"`
}

type chatCompletionMessage struct {
	Role       string                   `json:"role"`
	Content    string                   `json:"content,omitempty"`
	Name       string                   `json:"name,omitempty"`
	ToolCallID string                   `json:"tool_call_id,omitempty"`
	ToolCalls  []chatCompletionToolCall `json:"tool_calls,omitempty"`
}

type chatCompletionTool struct {
	Type     string                     `json:"type"`
	Function chatCompletionToolFunction `json:"function"`
}

type chatCompletionToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type chatCompletionToolCall struct {
	ID       string                         `json:"id"`
	Type     string                         `json:"type"`
	Function chatCompletionToolCallFunction `json:"function"`
}

type chatCompletionToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatCompletionMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
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
		httpClient = &http.Client{
			Timeout:   30 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		}
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
	const operation = openAICompatibleChatOperation
	startedAt := time.Now()
	host := llmHTTPHost(c.baseURL)
	ctx, span := observability.StartSpan(ctx, "llm.openai_compatible.chat",
		attribute.String("llm.provider", c.provider),
		attribute.String("llm.model", c.model),
		attribute.String("llm.operation", operation),
		attribute.Int("llm.message_count", len(request.Messages)),
		attribute.String("http.request.host", host),
	)
	var chatErr error
	defer func() {
		status := "success"
		if chatErr != nil {
			status = "failed"
		}
		span.SetAttributes(attribute.String("llm.request.status", status))
		metrics.LLMRequestsTotal.WithLabelValues(c.provider, c.model, operation, status).Inc()
		metrics.LLMRequestDuration.WithLabelValues(c.provider, c.model, operation, status).Observe(time.Since(startedAt).Seconds())
		observability.EndSpan(span, chatErr)
	}()

	payload := chatCompletionRequest{
		Model:       c.model,
		Messages:    make([]chatCompletionMessage, 0, len(request.Messages)),
		Temperature: request.Temperature,
		MaxTokens:   request.MaxTokens,
	}
	for _, message := range request.Messages {
		role := strings.TrimSpace(message.Role)
		content := strings.TrimSpace(message.Content)
		name := strings.TrimSpace(message.Name)
		toolCallID := strings.TrimSpace(message.ToolCallID)
		toolCalls := chatCompletionToolCallsFromDomain(message.ToolCalls)
		if role == "" {
			continue
		}
		if content == "" && len(toolCalls) == 0 {
			continue
		}
		if role == "tool" {
			name = ""
		}
		payload.Messages = append(payload.Messages, chatCompletionMessage{
			Role:       role,
			Content:    content,
			Name:       name,
			ToolCallID: toolCallID,
			ToolCalls:  toolCalls,
		})
	}
	if len(payload.Messages) == 0 {
		chatErr = domain.NewAppError(domain.ErrorKindInvalidInput, "llm_empty_messages", "llm messages must not be empty", "llm.openai_compatible.chat", false, nil)
		return ChatResponse{}, chatErr
	}
	payload.Tools = chatCompletionToolsFromDomain(request.Tools)
	if strings.TrimSpace(request.ToolChoice) != "" {
		payload.ToolChoice = strings.TrimSpace(request.ToolChoice)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		chatErr = err
		return ChatResponse{}, err
	}
	span.SetAttributes(attribute.Int("http.request.body.size", len(body)))
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		chatErr = err
		return ChatResponse{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	httpStartedAt := time.Now()
	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		recordLLMExternalHTTPRequest(openAICompatibleExternalHTTPOperation, host, "error", time.Since(httpStartedAt))
		chatErr = domain.NewAppError(domain.ErrorKindUnavailable, "llm_request_failed", "llm request failed", "llm.openai_compatible.chat", true, err)
		return ChatResponse{}, chatErr
	}
	defer httpResponse.Body.Close()
	recordLLMExternalHTTPRequest(openAICompatibleExternalHTTPOperation, host, strconv.Itoa(httpResponse.StatusCode), time.Since(httpStartedAt))
	span.SetAttributes(attribute.Int("http.response.status_code", httpResponse.StatusCode))

	responseBody, err := io.ReadAll(io.LimitReader(httpResponse.Body, 1<<20))
	if err != nil {
		chatErr = err
		return ChatResponse{}, err
	}
	span.SetAttributes(attribute.Int("http.response.body.size", len(responseBody)))
	var decoded chatCompletionResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		chatErr = domain.NewAppError(domain.ErrorKindUnavailable, "llm_invalid_response", "llm response is invalid", "llm.openai_compatible.chat", true, err)
		return ChatResponse{}, chatErr
	}
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(http.StatusText(httpResponse.StatusCode))
		if decoded.Error != nil && strings.TrimSpace(decoded.Error.Message) != "" {
			message = decoded.Error.Message
		}
		chatErr = domain.NewAppError(domain.ErrorKindUnavailable, "llm_provider_error", message, "llm.openai_compatible.chat", true, nil)
		return ChatResponse{}, chatErr
	}
	if len(decoded.Choices) == 0 {
		chatErr = domain.NewAppError(domain.ErrorKindUnavailable, "llm_empty_response", "llm response is empty", "llm.openai_compatible.chat", true, nil)
		return ChatResponse{}, chatErr
	}
	content := strings.TrimSpace(decoded.Choices[0].Message.Content)
	toolCalls := domainToolCallsFromChatCompletion(decoded.Choices[0].Message.ToolCalls)
	if content == "" && len(toolCalls) == 0 {
		chatErr = domain.NewAppError(domain.ErrorKindUnavailable, "llm_empty_response", "llm response is empty", "llm.openai_compatible.chat", true, nil)
		return ChatResponse{}, chatErr
	}
	if decoded.Usage != nil {
		if decoded.Usage.PromptTokens > 0 {
			metrics.LLMTokensTotal.WithLabelValues(c.provider, c.model, "input").Add(float64(decoded.Usage.PromptTokens))
		}
		if decoded.Usage.CompletionTokens > 0 {
			metrics.LLMTokensTotal.WithLabelValues(c.provider, c.model, "output").Add(float64(decoded.Usage.CompletionTokens))
		}
		span.SetAttributes(
			attribute.Int("llm.usage.prompt_tokens", decoded.Usage.PromptTokens),
			attribute.Int("llm.usage.completion_tokens", decoded.Usage.CompletionTokens),
			attribute.Int("llm.usage.total_tokens", decoded.Usage.TotalTokens),
		)
	}
	return ChatResponse{
		Provider:  c.provider,
		Model:     c.model,
		Content:   content,
		ToolCalls: toolCalls,
	}, nil
}

func chatCompletionToolsFromDomain(tools []ToolDefinition) []chatCompletionTool {
	if len(tools) == 0 {
		return nil
	}
	output := make([]chatCompletionTool, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		parameters := tool.Parameters
		if parameters == nil {
			parameters = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		output = append(output, chatCompletionTool{
			Type: "function",
			Function: chatCompletionToolFunction{
				Name:        name,
				Description: strings.TrimSpace(tool.Description),
				Parameters:  parameters,
			},
		})
	}
	return output
}

func chatCompletionToolCallsFromDomain(calls []ToolCall) []chatCompletionToolCall {
	if len(calls) == 0 {
		return nil
	}
	output := make([]chatCompletionToolCall, 0, len(calls))
	for _, call := range calls {
		name := strings.TrimSpace(call.Name)
		if name == "" {
			continue
		}
		output = append(output, chatCompletionToolCall{
			ID:   strings.TrimSpace(call.ID),
			Type: "function",
			Function: chatCompletionToolCallFunction{
				Name:      name,
				Arguments: strings.TrimSpace(call.Arguments),
			},
		})
	}
	return output
}

func domainToolCallsFromChatCompletion(calls []chatCompletionToolCall) []ToolCall {
	if len(calls) == 0 {
		return nil
	}
	output := make([]ToolCall, 0, len(calls))
	for _, call := range calls {
		name := strings.TrimSpace(call.Function.Name)
		if name == "" {
			continue
		}
		output = append(output, ToolCall{
			ID:        strings.TrimSpace(call.ID),
			Name:      name,
			Arguments: strings.TrimSpace(call.Function.Arguments),
		})
	}
	return output
}

func recordLLMExternalHTTPRequest(operation string, host string, status string, duration time.Duration) {
	if host == "" {
		host = "unknown"
	}
	if status == "" {
		status = "unknown"
	}
	metrics.ExternalHTTPRequestsTotal.WithLabelValues(operation, host, status).Inc()
	metrics.ExternalHTTPRequestDuration.WithLabelValues(operation, host).Observe(duration.Seconds())
}

func llmHTTPHost(baseURL string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" {
		return "unknown"
	}
	return parsed.Host
}
