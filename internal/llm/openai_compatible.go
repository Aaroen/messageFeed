package llm

import (
	"bytes"
	"context"
	"encoding/base64"
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

// llmDefaultHTTPTimeout 是非流式模型单次请求的思考等待窗口。
// 参考项目通常把模型长响应与普通短 HTTP 请求区分处理；本项目暂不启用流式，
// 因此先给非流式请求保留 180 秒，避免 30 秒级默认值误打断长思考模型。
const llmDefaultHTTPTimeout = 180 * time.Second
const llmDefaultHTTPMaxAttempts = 6
const llmRetryableHTTPStatusDelay = 30 * time.Second

var sleepLLMRetryDelay = sleepLLMRetryTimer

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
	Provider     string
	BaseURL      string
	APIKey       string
	Model        string
	ProtocolMode string
	Timeout      time.Duration
	MaxAttempts  int
	HTTPClient   *http.Client
}

type OpenAICompatibleClient struct {
	provider          string
	baseURL           string
	apiKey            string
	model             string
	httpClient        *http.Client
	protocolMode      string
	maxAttempts       int
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
	ID                string                `json:"id"`
	Model             string                `json:"model"`
	Object            string                `json:"object"`
	Status            string                `json:"status"`
	Output            []responsesOutputItem `json:"output"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details,omitempty"`
	Error *struct {
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
	protocolMode := normalizeOpenAICompatibleProtocolMode(config.ProtocolMode)
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = llmDefaultHTTPTimeout
	}
	maxAttempts := config.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = llmDefaultHTTPMaxAttempts
	}
	if maxAttempts > 50 {
		maxAttempts = 50
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout:   timeout,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		}
	}
	return &OpenAICompatibleClient{
		provider:     provider,
		baseURL:      baseURL,
		apiKey:       apiKey,
		model:        model,
		httpClient:   httpClient,
		protocolMode: protocolMode,
		maxAttempts:  maxAttempts,
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
	switch c.protocolMode {
	case "responses":
		return []llmProtocol{llmProtocolResponses, llmProtocolChatCompletions}
	case "chat_completions":
		return []llmProtocol{llmProtocolChatCompletions, llmProtocolResponses}
	}
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
// 协议尝试顺序由 protocolOrder 统一决定，默认优先 Responses，再降级到 Chat Completions。
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
	if encodedCall, ok := encodedToolCallFromText(content, "encoded_call_0_0"); ok {
		toolCalls = append(toolCalls, encodedCall)
		content = ""
	}
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
	content, encodedToolCalls := responsesOutputTextAndEncodedToolCalls(decoded.Output)
	toolCalls := domainToolCallsFromResponses(decoded.Output)
	toolCalls = append(toolCalls, encodedToolCalls...)
	if strings.EqualFold(decoded.Status, "incomplete") {
		return ChatResponse{}, newLLMRouteError(llmProtocolResponses, statusCode, responsesIncompleteMessage(decoded), true, nil)
	}
	if content == "" && len(toolCalls) == 0 {
		return ChatResponse{}, newLLMRouteError(llmProtocolResponses, statusCode, "llm response is empty", false, nil)
	}
	if decoded.Usage != nil {
		recordLLMUsage(c.provider, c.model, decoded.Usage.InputTokens, decoded.Usage.OutputTokens, decoded.Usage.TotalTokens, span)
	}
	return ChatResponse{Provider: c.provider, Model: c.model, Content: content, ToolCalls: toolCalls}, nil
}

func responsesIncompleteMessage(response responsesResponse) string {
	reason := ""
	if response.IncompleteDetails != nil {
		reason = strings.TrimSpace(response.IncompleteDetails.Reason)
	}
	if reason == "" {
		return "llm response is incomplete"
	}
	return "llm response is incomplete: " + reason
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
	text, _ := responsesOutputTextAndEncodedToolCalls(items)
	return text
}

func responsesOutputTextAndEncodedToolCalls(items []responsesOutputItem) (string, []ToolCall) {
	var builder strings.Builder
	calls := make([]ToolCall, 0)
	for itemIndex, item := range items {
		if item.Type != "message" {
			continue
		}
		for contentIndex, content := range item.Content {
			if content.Type != "output_text" && content.Type != "text" {
				continue
			}
			text := strings.TrimSpace(content.Text)
			if text == "" {
				continue
			}
			if call, ok := encodedToolCallFromText(text, fmt.Sprintf("encoded_call_%d_%d", itemIndex, contentIndex)); ok {
				calls = append(calls, call)
				continue
			}
			if builder.Len() > 0 {
				builder.WriteString("\n")
			}
			builder.WriteString(text)
		}
	}
	return builder.String(), calls
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

// encodedToolCallFromText 兼容部分上游把工具调用编码进 output_text 的非标准形态。
// 只有在 base64 解码后明确匹配 "Functions.<tool>:<json>" 时才转换为 ToolCall。
func encodedToolCallFromText(text string, callID string) (ToolCall, bool) {
	decoded, ok := decodeBase64Text(strings.TrimSpace(text))
	if !ok {
		return ToolCall{}, false
	}
	payload := strings.TrimSpace(strings.TrimLeftFunc(string(decoded), func(r rune) bool {
		return r >= 0 && r < ' '
	}))
	payload = strings.TrimSpace(payload)
	if !strings.HasPrefix(payload, "Functions.") {
		return ToolCall{}, false
	}
	payload = strings.TrimPrefix(payload, "Functions.")
	separator := strings.Index(payload, ":")
	if separator <= 0 {
		return ToolCall{}, false
	}
	name := strings.TrimSpace(payload[:separator])
	arguments, ok := firstJSONObjectPrefix(payload[separator+1:])
	if name == "" || !ok {
		return ToolCall{}, false
	}
	if strings.TrimSpace(callID) == "" {
		callID = "encoded_call"
	}
	return ToolCall{ID: callID, Name: name, Arguments: arguments}, true
}

func decodeBase64Text(text string) ([]byte, bool) {
	candidate := strings.Join(strings.Fields(strings.TrimSpace(text)), "")
	if len(candidate) < 12 {
		return nil, false
	}
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(candidate)
		if err == nil && len(decoded) > 0 {
			return decoded, true
		}
	}
	return nil, false
}

func firstJSONObjectPrefix(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "{") {
		return "", false
	}
	depth := 0
	inString := false
	escaped := false
	for index, value := range text {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if value == '\\' {
				escaped = true
				continue
			}
			if value == '"' {
				inString = false
			}
			continue
		}
		switch value {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				candidate := text[:index+1]
				return candidate, json.Valid([]byte(candidate))
			}
			if depth < 0 {
				return "", false
			}
		}
	}
	return "", false
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
	maxAttempts := c.maxAttempts
	if maxAttempts < 1 {
		maxAttempts = llmDefaultHTTPMaxAttempts
	}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
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
			if attempt < maxAttempts {
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
		if isRetryableLLMHTTPStatus(httpResponse.StatusCode) && attempt < maxAttempts {
			lastErr = domain.NewAppError(domain.ErrorKindUnavailable, "llm_retryable_status", llmResponseSnippet(responseBody), "llm.openai_compatible.chat", true, nil)
			if sleepErr := sleepLLMRetryableHTTPStatus(ctx, attempt); sleepErr != nil {
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
	return sleepLLMRetryDelay(ctx, delay)
}

func sleepLLMRetryableHTTPStatus(ctx context.Context, attempt int) error {
	// 429/5xx 通常表示上游限流或负载饱和。后台任务等待一个较长窗口再重试，
	// 避免刚排队就把用户任务判失败。
	return sleepLLMRetryDelay(ctx, llmRetryableHTTPStatusDelay)
}

func sleepLLMRetryTimer(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
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

func normalizeOpenAICompatibleProtocolMode(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "responses":
		return "responses"
	case "chat_completions", "chat-completions", "chat", "completions":
		return "chat_completions"
	default:
		return "auto"
	}
}

func llmHTTPHost(baseURL string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" {
		return "unknown"
	}
	return parsed.Host
}
