package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"messagefeed/internal/domain"
	"messagefeed/internal/metrics"
	"messagefeed/internal/observability"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const defaultOpenAIBaseURL = "https://api.openai.com/v1"
const openAICompatibleChatOperation = "auto_route"
const openAICompatibleExternalHTTPOperation = "llm_openai_compatible"
const llmHTTPMaxAttempts = 6

// llmDefaultHTTPTimeout 是非流式模型单次请求的思考等待窗口。
// 参考项目通常把模型长响应与普通短 HTTP 请求区分处理；本项目暂不启用流式，
// 因此先给非流式请求保留 180 秒，避免 30 秒级默认值误打断长思考模型。
const llmDefaultHTTPTimeout = 180 * time.Second

type llmProtocol string

const (
	llmProtocolChatCompletions llmProtocol = "chat_completions"
	llmProtocolResponses       llmProtocol = "responses"
)

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
	Name         string
	Title        string
	Description  string
	InputSchema  map[string]any
	OutputSchema map[string]any
	Annotations  map[string]any
	Meta         map[string]any
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
	provider          string
	baseURL           string
	apiKey            string
	model             string
	httpClient        *http.Client
	routeMu           sync.Mutex
	preferredProtocol llmProtocol
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
	InputSchema map[string]any `json:"parameters,omitempty"`
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

type responsesRequest struct {
	Model           string               `json:"model"`
	Input           []responsesInputItem `json:"input"`
	Tools           []responsesTool      `json:"tools,omitempty"`
	ToolChoice      any                  `json:"tool_choice,omitempty"`
	Temperature     float64              `json:"temperature,omitempty"`
	MaxOutputTokens int                  `json:"max_output_tokens,omitempty"`
}

type responsesInputItem struct {
	Type      string `json:"type,omitempty"`
	Role      string `json:"role,omitempty"`
	Content   string `json:"content,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	Output    string `json:"output,omitempty"`
}

type responsesTool struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"parameters,omitempty"`
}

type responsesResponse struct {
	ID     string                `json:"id"`
	Model  string                `json:"model"`
	Object string                `json:"object"`
	Status string                `json:"status"`
	Output []responsesOutputItem `json:"output"`
	Error  *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

type responsesOutputItem struct {
	Type      string                   `json:"type"`
	ID        string                   `json:"id"`
	Status    string                   `json:"status"`
	Role      string                   `json:"role"`
	Content   []responsesOutputContent `json:"content,omitempty"`
	CallID    string                   `json:"call_id,omitempty"`
	Name      string                   `json:"name,omitempty"`
	Arguments string                   `json:"arguments,omitempty"`
}

type responsesOutputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
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
			Timeout:   llmDefaultHTTPTimeout,
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

	if len(compactChatMessages(request.Messages)) == 0 {
		chatErr = domain.NewAppError(domain.ErrorKindInvalidInput, "llm_empty_messages", "llm messages must not be empty", "llm.openai_compatible.chat", false, nil)
		return ChatResponse{}, chatErr
	}
	var lastErr error
	for _, protocol := range c.protocolOrder(request) {
		response, err := c.chatWithProtocol(ctx, request, protocol, host, span)
		if err == nil {
			c.rememberProtocol(protocol)
			span.SetAttributes(attribute.String("llm.route.protocol", string(protocol)))
			return response, nil
		}
		lastErr = err
		if !shouldTryNextLLMProtocol(err) {
			break
		}
	}
	if lastErr == nil {
		lastErr = domain.NewAppError(domain.ErrorKindUnavailable, "llm_route_failed", "llm route failed", "llm.openai_compatible.chat", true, nil)
	}
	chatErr = lastErr
	return ChatResponse{}, lastErr
}

// compactChatMessages 只用于确认请求至少含有一条可发送消息或工具结果。
func compactChatMessages(messages []ChatMessage) []ChatMessage {
	output := make([]ChatMessage, 0, len(messages))
	for _, message := range messages {
		if strings.TrimSpace(message.Role) == "" {
			continue
		}
		if strings.TrimSpace(message.Content) == "" && len(message.ToolCalls) == 0 {
			continue
		}
		output = append(output, message)
	}
	return output
}

// protocolOrder 返回当前客户端的协议尝试顺序。
// 默认优先 Responses；一旦某个协议成功，后续同一客户端会优先复用该协议。
// 当前接入的部分 OpenAI-compatible 服务在 /chat/completions 下能返回普通文本，
// 但带 tools 时不返回 tool_calls；Responses 原生 function_call 更稳定，因此作为默认主路由。
func (c *OpenAICompatibleClient) protocolOrder(request ChatRequest) []llmProtocol {
	c.routeMu.Lock()
	preferred := c.preferredProtocol
	c.routeMu.Unlock()
	if len(request.Tools) > 0 {
		return []llmProtocol{llmProtocolResponses, llmProtocolChatCompletions}
	}
	switch preferred {
	case llmProtocolChatCompletions:
		return []llmProtocol{llmProtocolChatCompletions, llmProtocolResponses}
	default:
		return []llmProtocol{llmProtocolResponses, llmProtocolChatCompletions}
	}
}

// rememberProtocol 记录本客户端上一次成功协议，实现轻量级智能路由记忆。
func (c *OpenAICompatibleClient) rememberProtocol(protocol llmProtocol) {
	if protocol == "" {
		return
	}
	c.routeMu.Lock()
	c.preferredProtocol = protocol
	c.routeMu.Unlock()
}

// chatWithProtocol 将统一的业务 ChatRequest 映射到指定上游协议。
// 当前默认协议为 Chat Completions；Responses 作为需要该协议的模型或服务的备用路由。
func (c *OpenAICompatibleClient) chatWithProtocol(ctx context.Context, request ChatRequest, protocol llmProtocol, host string, span trace.Span) (ChatResponse, error) {
	switch protocol {
	case llmProtocolResponses:
		return c.chatWithResponses(ctx, request, host, span)
	default:
		return c.chatWithChatCompletions(ctx, request, host, span)
	}
}

// chatWithChatCompletions 调用 /chat/completions，适配当前 .env 使用的模型服务。
func (c *OpenAICompatibleClient) chatWithChatCompletions(ctx context.Context, request ChatRequest, host string, span trace.Span) (ChatResponse, error) {
	payload := chatCompletionRequest{
		Model:       c.model,
		Messages:    chatCompletionMessagesFromDomain(request.Messages),
		Tools:       chatCompletionToolsFromDomain(request.Tools),
		Temperature: request.Temperature,
		MaxTokens:   request.MaxTokens,
	}
	if strings.TrimSpace(request.ToolChoice) != "" {
		payload.ToolChoice = strings.TrimSpace(request.ToolChoice)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ChatResponse{}, err
	}
	span.SetAttributes(attribute.Int("http.request.body.size", len(body)))
	responseBody, statusCode, err := c.doLLMHTTPRequest(ctx, "/chat/completions", body, host)
	if err != nil {
		return ChatResponse{}, err
	}
	span.SetAttributes(attribute.Int("http.response.status_code", statusCode))
	span.SetAttributes(attribute.Int("http.response.body.size", len(responseBody)))
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return ChatResponse{}, newLLMRouteError(llmProtocolChatCompletions, statusCode, providerErrorMessage(responseBody, statusCode), isLLMProtocolFallbackStatus(statusCode), nil)
	}
	var decoded chatCompletionResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return ChatResponse{}, newLLMRouteError(llmProtocolChatCompletions, statusCode, "llm response is invalid: "+llmResponseSnippet(responseBody), true, err)
	}
	if len(decoded.Choices) == 0 {
		return ChatResponse{}, newLLMRouteError(llmProtocolChatCompletions, statusCode, "llm response is empty", len(request.Tools) > 0, nil)
	}
	content := strings.TrimSpace(decoded.Choices[0].Message.Content)
	toolCalls := domainToolCallsFromChatCompletion(decoded.Choices[0].Message.ToolCalls)
	if content == "" && len(toolCalls) == 0 {
		return ChatResponse{}, newLLMRouteError(llmProtocolChatCompletions, statusCode, "llm response is empty", len(request.Tools) > 0, nil)
	}
	if decoded.Usage != nil {
		recordLLMUsage(c.provider, c.model, decoded.Usage.PromptTokens, decoded.Usage.CompletionTokens, decoded.Usage.TotalTokens, span)
	}
	return ChatResponse{Provider: c.provider, Model: c.model, Content: content, ToolCalls: toolCalls}, nil
}

func chatCompletionMessagesFromDomain(messages []ChatMessage) []chatCompletionMessage {
	output := make([]chatCompletionMessage, 0, len(messages))
	for _, message := range messages {
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
		output = append(output, chatCompletionMessage{
			Role:       role,
			Content:    content,
			Name:       name,
			ToolCallID: toolCallID,
			ToolCalls:  toolCalls,
		})
	}
	return output
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
		parameters := tool.InputSchema
		if parameters == nil {
			parameters = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		output = append(output, chatCompletionTool{
			Type: "function",
			Function: chatCompletionToolFunction{
				Name:        name,
				Description: strings.TrimSpace(tool.Description),
				InputSchema: parameters,
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

// chatWithResponses 调用 /responses，供只支持 Responses API 的模型服务使用。
func (c *OpenAICompatibleClient) chatWithResponses(ctx context.Context, request ChatRequest, host string, span trace.Span) (ChatResponse, error) {
	payload := responsesRequest{
		Model:           c.model,
		Input:           responsesInputFromChatMessages(request.Messages),
		Tools:           responsesToolsFromDomain(request.Tools),
		Temperature:     request.Temperature,
		MaxOutputTokens: request.MaxTokens,
	}
	if strings.TrimSpace(request.ToolChoice) != "" {
		payload.ToolChoice = strings.TrimSpace(request.ToolChoice)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ChatResponse{}, err
	}
	span.SetAttributes(attribute.Int("http.request.body.size", len(body)))
	responseBody, statusCode, err := c.doLLMHTTPRequest(ctx, "/responses", body, host)
	if err != nil {
		return ChatResponse{}, err
	}
	span.SetAttributes(attribute.Int("http.response.status_code", statusCode))
	span.SetAttributes(attribute.Int("http.response.body.size", len(responseBody)))
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return ChatResponse{}, newLLMRouteError(llmProtocolResponses, statusCode, providerErrorMessage(responseBody, statusCode), isLLMProtocolFallbackStatus(statusCode), nil)
	}
	var decoded responsesResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return ChatResponse{}, newLLMRouteError(llmProtocolResponses, statusCode, "llm response is invalid: "+llmResponseSnippet(responseBody), true, err)
	}
	if decoded.Status == "incomplete" {
		return ChatResponse{}, newLLMRouteError(llmProtocolResponses, statusCode, "llm response is incomplete", false, nil)
	}
	content := strings.TrimSpace(responsesOutputText(decoded.Output))
	toolCalls := domainToolCallsFromResponses(decoded.Output)
	if content == "" && len(toolCalls) == 0 {
		return ChatResponse{}, newLLMRouteError(llmProtocolResponses, statusCode, "llm response is empty", false, nil)
	}
	if decoded.Usage != nil {
		recordLLMUsage(c.provider, c.model, decoded.Usage.InputTokens, decoded.Usage.OutputTokens, decoded.Usage.TotalTokens, span)
	}
	return ChatResponse{Provider: c.provider, Model: c.model, Content: content, ToolCalls: toolCalls}, nil
}

func responsesInputFromChatMessages(messages []ChatMessage) []responsesInputItem {
	items := make([]responsesInputItem, 0, len(messages))
	for messageIndex, message := range messages {
		role := strings.TrimSpace(message.Role)
		content := strings.TrimSpace(message.Content)
		if role == "" {
			continue
		}
		if role == "tool" {
			if content == "" {
				continue
			}
			items = append(items, responsesInputItem{
				Type:   "function_call_output",
				CallID: strings.TrimSpace(message.ToolCallID),
				Output: content,
			})
			continue
		}
		if content != "" {
			items = append(items, responsesInputItem{
				Role:    role,
				Content: content,
			})
		}
		for callIndex, call := range message.ToolCalls {
			name := strings.TrimSpace(call.Name)
			if name == "" {
				continue
			}
			callID := strings.TrimSpace(call.ID)
			if callID == "" {
				callID = fmt.Sprintf("call_%d_%d", messageIndex, callIndex)
			}
			items = append(items, responsesInputItem{
				Type:      "function_call",
				CallID:    callID,
				Name:      name,
				Arguments: strings.TrimSpace(call.Arguments),
			})
		}
	}
	return items
}

func responsesToolsFromDomain(tools []ToolDefinition) []responsesTool {
	if len(tools) == 0 {
		return nil
	}
	output := make([]responsesTool, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		parameters := tool.InputSchema
		if parameters == nil {
			parameters = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		output = append(output, responsesTool{
			Type:        "function",
			Name:        name,
			Description: strings.TrimSpace(tool.Description),
			InputSchema: parameters,
		})
	}
	return output
}

func responsesOutputText(items []responsesOutputItem) string {
	var builder strings.Builder
	for _, item := range items {
		if item.Type != "message" {
			continue
		}
		for _, content := range item.Content {
			if content.Type != "output_text" && content.Type != "text" {
				continue
			}
			text := strings.TrimSpace(content.Text)
			if text == "" {
				continue
			}
			if builder.Len() > 0 {
				builder.WriteString("\n")
			}
			builder.WriteString(text)
		}
	}
	return builder.String()
}

func domainToolCallsFromResponses(items []responsesOutputItem) []ToolCall {
	calls := make([]ToolCall, 0)
	for _, item := range items {
		if item.Type != "function_call" {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		calls = append(calls, ToolCall{
			ID:        strings.TrimSpace(firstNonEmptyString(item.CallID, item.ID)),
			Name:      name,
			Arguments: strings.TrimSpace(item.Arguments),
		})
	}
	return calls
}

type llmRouteError struct {
	protocol         llmProtocol
	statusCode       int
	protocolMismatch bool
	err              error
}

func newLLMRouteError(protocol llmProtocol, statusCode int, message string, protocolMismatch bool, err error) error {
	if strings.TrimSpace(message) == "" {
		message = "llm route failed"
	}
	appErr := domain.NewAppError(domain.ErrorKindUnavailable, "llm_route_failed", message, "llm.openai_compatible.chat", true, err)
	return &llmRouteError{protocol: protocol, statusCode: statusCode, protocolMismatch: protocolMismatch, err: appErr}
}

func (e *llmRouteError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *llmRouteError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func shouldTryNextLLMProtocol(err error) bool {
	var routeErr *llmRouteError
	if !errors.As(err, &routeErr) {
		return false
	}
	if routeErr.protocolMismatch {
		return true
	}
	switch routeErr.statusCode {
	case http.StatusBadRequest, http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusUnsupportedMediaType:
		return true
	default:
		return false
	}
}

func providerErrorMessage(body []byte, statusCode int) string {
	message := strings.TrimSpace(http.StatusText(statusCode))
	var responsesErr responsesResponse
	if err := json.Unmarshal(body, &responsesErr); err == nil && responsesErr.Error != nil && strings.TrimSpace(responsesErr.Error.Message) != "" {
		return responsesErr.Error.Message
	}
	var chatErr chatCompletionResponse
	if err := json.Unmarshal(body, &chatErr); err == nil && chatErr.Error != nil && strings.TrimSpace(chatErr.Error.Message) != "" {
		return chatErr.Error.Message
	}
	if snippet := llmResponseSnippet(body); snippet != "" {
		return snippet
	}
	return message
}

func recordLLMUsage(provider string, model string, inputTokens int, outputTokens int, totalTokens int, span trace.Span) {
	if inputTokens > 0 {
		metrics.LLMTokensTotal.WithLabelValues(provider, model, "input").Add(float64(inputTokens))
	}
	if outputTokens > 0 {
		metrics.LLMTokensTotal.WithLabelValues(provider, model, "output").Add(float64(outputTokens))
	}
	span.SetAttributes(
		attribute.Int("llm.usage.prompt_tokens", inputTokens),
		attribute.Int("llm.usage.completion_tokens", outputTokens),
		attribute.Int("llm.usage.total_tokens", totalTokens),
	)
}

func (c *OpenAICompatibleClient) doLLMHTTPRequest(ctx context.Context, path string, body []byte, host string) ([]byte, int, error) {
	var lastErr error
	for attempt := 1; attempt <= llmHTTPMaxAttempts; attempt++ {
		httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return nil, 0, err
		}
		httpRequest.Header.Set("Authorization", "Bearer "+c.apiKey)
		httpRequest.Header.Set("Content-Type", "application/json")
		httpStartedAt := time.Now()
		httpResponse, err := c.httpClient.Do(httpRequest)
		if err != nil {
			recordLLMExternalHTTPRequest(openAICompatibleExternalHTTPOperation, host, "error", time.Since(httpStartedAt))
			if isLLMThinkingTimeoutError(err) {
				// 模型思考超时通常意味着上游已经长时间未产出结果。
				// 这里不做 HTTP 层重复提交，避免同一用户任务被上游模型并发执行多份。
				return nil, 0, domain.NewAppError(domain.ErrorKindUnavailable, "llm_thinking_timeout", "model thinking timed out", "llm.openai_compatible.chat", true, err)
			}
			lastErr = domain.NewAppError(domain.ErrorKindUnavailable, "llm_request_failed", "llm request failed", "llm.openai_compatible.chat", true, err)
			if attempt < llmHTTPMaxAttempts {
				if sleepErr := sleepLLMRetry(ctx, attempt); sleepErr != nil {
					return nil, 0, sleepErr
				}
				continue
			}
			return nil, 0, lastErr
		}
		responseBody, readErr := io.ReadAll(io.LimitReader(httpResponse.Body, 1<<20))
		_ = httpResponse.Body.Close()
		recordLLMExternalHTTPRequest(openAICompatibleExternalHTTPOperation, host, strconv.Itoa(httpResponse.StatusCode), time.Since(httpStartedAt))
		if readErr != nil {
			return nil, httpResponse.StatusCode, readErr
		}
		if isRetryableLLMHTTPStatus(httpResponse.StatusCode) && attempt < llmHTTPMaxAttempts {
			lastErr = domain.NewAppError(domain.ErrorKindUnavailable, "llm_retryable_status", llmResponseSnippet(responseBody), "llm.openai_compatible.chat", true, nil)
			if sleepErr := sleepLLMRetry(ctx, attempt); sleepErr != nil {
				return nil, httpResponse.StatusCode, sleepErr
			}
			continue
		}
		return responseBody, httpResponse.StatusCode, nil
	}
	if lastErr != nil {
		return nil, 0, lastErr
	}
	return nil, 0, domain.NewAppError(domain.ErrorKindUnavailable, "llm_request_failed", "llm request failed", "llm.openai_compatible.chat", true, nil)
}

func isLLMThinkingTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "client.timeout") || strings.Contains(lower, "timeout awaiting headers")
}

func isRetryableLLMHTTPStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

func isLLMProtocolFallbackStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusBadRequest, http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusUnsupportedMediaType:
		return true
	default:
		return false
	}
}

func sleepLLMRetry(ctx context.Context, attempt int) error {
	delay := time.Duration(1<<uint(attempt-1)) * 500 * time.Millisecond
	if delay > 8*time.Second {
		delay = 8 * time.Second
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func llmResponseSnippet(body []byte) string {
	snippet := strings.TrimSpace(string(body))
	if len([]rune(snippet)) > 300 {
		runes := []rune(snippet)
		snippet = string(runes[:300]) + "..."
	}
	if snippet == "" {
		return "empty body"
	}
	return snippet
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
