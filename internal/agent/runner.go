package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"messagefeed/internal/observability"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

type ChatClient interface {
	Chat(ctx context.Context, request llm.ChatRequest) (llm.ChatResponse, error)
}

type TurnStore interface {
	UpdateTurn(ctx context.Context, turn domain.AgentTurn) (domain.AgentTurn, error)
	AppendTranscriptEntry(ctx context.Context, entry domain.AgentTranscriptEntry) (domain.AgentTranscriptEntry, error)
	UpdateInboundMessageStatus(ctx context.Context, userID int64, id int64, status domain.AgentInboundMessageStatus, now time.Time) (domain.AgentInboundMessage, error)
}

type AuditLogger interface {
	Record(ctx context.Context, event AuditEvent) error
}

type ToolExecutor interface {
	CallTool(ctx context.Context, input MCPCallToolInput) (MCPCallToolResult, error)
}

type ContextBuilder interface {
	Build(ctx context.Context, input ContextBuildInput) (ContextSnapshot, error)
}

type ContextBuildInput struct {
	UserID          int64
	SessionID       int64
	TurnID          int64
	ControllerRunID int64
	CapabilityKeys  []string
	MessageText     string
	MessageType     string
	RequestID       string
	TraceID         string
	BudgetProfile   ContextBudgetProfile
}

type ContextSnapshot struct {
	Blocks          []ContextBlock
	Messages        []ContextMessage
	Observations    []CapabilityObservation
	HistoryNeedHint HistoryNeedHint
	Bundle          ContextBundle
	SemanticUnits   []ContextSemanticUnit
	BudgetProfile   ContextBudgetProfile
	BudgetReport    ContextBudgetReport
}

type ContextBlock struct {
	Name            string
	CapabilityKey   string
	Content         string
	ItemCount       int
	Truncated       bool
	GeneratedAt     time.Time
	TrustLevel      string
	Source          string
	EvidenceRefs    []string
	CanonicalRef    string
	TokenEstimate   int
	RetentionReason string
	OmittedReason   string
}

type ContextMessage struct {
	Role              domain.AgentTranscriptRole
	Content           string
	TranscriptEntryID int64
	TurnID            int64
	CreatedAt         time.Time
}

type CapabilityObservation struct {
	Capability     string
	Decision       string
	Status         string
	Summary        string
	RunID          int64
	ObservationRef string
	ArtifactRefs   []string
}

type AuditEvent struct {
	SessionID int64
	TurnID    int64
	UserID    int64
	EventType string
	Status    string
	Message   string
	Metadata  domain.AgentJSON
	RequestID string
	TraceID   string
	CreatedAt time.Time
}

type TurnRunner struct {
	store          TurnStore
	auditLogger    AuditLogger
	contextBuilder ContextBuilder
	toolExecutor   ToolExecutor
	toolRegistry   *CapabilityRegistry
	llmClient      ChatClient
	now            func() time.Time
	systemPrompt   string
	maxTokens      int
	temperature    float64
	toolKeys       []string
}

type TurnRunnerOptions struct {
	Store          TurnStore
	AuditLogger    AuditLogger
	ContextBuilder ContextBuilder
	ToolExecutor   ToolExecutor
	ToolRegistry   *CapabilityRegistry
	ToolKeys       []string
	LLMClient      ChatClient
	Now            func() time.Time
	SystemPrompt   string
	MaxTokens      int
	Temperature    float64
}

func NewTurnRunner(options TurnRunnerOptions) *TurnRunner {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	temperature := options.Temperature
	if temperature == 0 {
		temperature = 0.2
	}
	registry := options.ToolRegistry
	if registry == nil {
		registry = NewP0CapabilityRegistry()
	}
	return &TurnRunner{
		store:          options.Store,
		auditLogger:    options.AuditLogger,
		contextBuilder: options.ContextBuilder,
		toolExecutor:   options.ToolExecutor,
		toolRegistry:   registry,
		llmClient:      options.LLMClient,
		now:            now,
		systemPrompt:   strings.TrimSpace(options.SystemPrompt),
		maxTokens:      options.MaxTokens,
		temperature:    temperature,
		toolKeys:       append([]string(nil), options.ToolKeys...),
	}
}

type TurnRunInput struct {
	UserID          int64
	Session         domain.AgentSession
	Turn            domain.AgentTurn
	InboundMessage  domain.AgentInboundMessage
	ControllerRunID int64
	AllowedToolKeys []string
	MessageType     string
	MessageText     string
	RequestID       string
	TraceID         string
}

type TurnRunResult struct {
	Turn          domain.AgentTurn
	Reply         string
	ModelProvider string
	Model         string
	Context       ContextSnapshot
}

const emptyLLMResponseRetryLimit = 2
const requiredToolCallRetryLimit = 2
const promptedToolActionRetryLimit = 2
const plainTextReplyRepairLimit = 2

func (r *TurnRunner) Run(ctx context.Context, input TurnRunInput) (TurnRunResult, error) {
	ctx, span := observability.StartSpan(ctx, "agent.turn_runner.run",
		attribute.Int64("agent.session_id", input.Session.ID),
		attribute.Int64("agent.turn_id", input.Turn.ID),
		attribute.Int64("auth.user_id", input.UserID),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	reply, provider, model, snapshot, err := r.generateReply(ctx, input)
	if err != nil {
		opErr = err
		_ = r.markInbound(ctx, input, domain.AgentInboundMessageStatusFailed)
		failed := r.failTurn(ctx, input, err)
		return TurnRunResult{Turn: failed, Context: snapshot}, err
	}

	_, _ = r.store.AppendTranscriptEntry(ctx, domain.AgentTranscriptEntry{
		SessionID: input.Session.ID,
		TurnID:    input.Turn.ID,
		UserID:    input.UserID,
		Role:      domain.AgentTranscriptRoleAssistant,
		Content:   reply,
		Metadata: domain.AgentJSON{
			"model_provider": provider,
			"model":          model,
			"observations":   ObservationMetadata(snapshot.Observations),
		},
		CreatedAt: r.now().UTC(),
	})

	finishedAt := r.now().UTC()
	turn := input.Turn
	turn.Status = domain.AgentTurnStatusSucceeded
	turn.OutputText = reply
	turn.ModelProvider = provider
	turn.Model = model
	turn.FinishedAt = &finishedAt
	turn, err = r.store.UpdateTurn(ctx, turn)
	if err != nil {
		opErr = err
		return TurnRunResult{}, err
	}
	r.record(ctx, AuditEvent{
		SessionID: input.Session.ID,
		TurnID:    turn.ID,
		UserID:    input.UserID,
		EventType: "agent.turn_generated",
		Status:    "succeeded",
		Message:   "agent turn reply generated",
		Metadata: domain.AgentJSON{
			"model_provider": provider,
			"model":          model,
			"observations":   ObservationMetadata(snapshot.Observations),
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: finishedAt,
	})

	span.SetAttributes(
		attribute.String("llm.provider", provider),
		attribute.String("llm.model", model),
		attribute.Int("agent.reply_bytes", len([]byte(reply))),
		attribute.Int("agent.observation_count", len(snapshot.Observations)),
	)
	return TurnRunResult{
		Turn:          turn,
		Reply:         reply,
		ModelProvider: provider,
		Model:         model,
		Context:       snapshot,
	}, nil
}

func (r *TurnRunner) generateReply(ctx context.Context, input TurnRunInput) (string, string, string, ContextSnapshot, error) {
	if input.MessageType != "text" {
		return "", "", "", ContextSnapshot{}, domain.NewAppError(domain.ErrorKindInvalidInput, "agent_unsupported_message_type", "agent message type is unsupported", "agent.turn_runner.generate", true, nil)
	}

	snapshot := ContextSnapshot{}
	if r.contextBuilder != nil {
		var err error
		snapshot, err = r.contextBuilder.Build(ctx, ContextBuildInput{
			UserID:          input.UserID,
			SessionID:       input.Session.ID,
			TurnID:          input.Turn.ID,
			ControllerRunID: input.ControllerRunID,
			CapabilityKeys:  append([]string(nil), input.AllowedToolKeys...),
			MessageText:     input.MessageText,
			MessageType:     input.MessageType,
			RequestID:       input.RequestID,
			TraceID:         input.TraceID,
		})
		if err != nil {
			return "", "", "", snapshot, err
		}
	}
	if r.llmClient == nil {
		return "", "", "", snapshot, domain.NewAppError(domain.ErrorKindUnavailable, "agent_llm_client_unavailable", "agent llm client is unavailable", "agent.turn_runner.generate", true, nil)
	}

	systemPrompt := r.buildSystemPrompt(snapshot, input.MessageText)
	messages := r.buildChatMessages(systemPrompt, snapshot, input.MessageText)
	response, snapshot, finalMessages, err := r.chatWithTools(ctx, input, snapshot, messages)
	if err != nil {
		return "", "", "", snapshot, err
	}
	response, err = r.ensurePlainTextReply(ctx, input, finalMessages, response)
	if err != nil {
		return "", response.Provider, response.Model, snapshot, err
	}
	if strings.TrimSpace(response.Content) == "" {
		return "", response.Provider, response.Model, snapshot, domain.NewAppError(domain.ErrorKindUnavailable, "agent_empty_reply", "agent reply is empty", "agent.turn_runner.generate", true, nil)
	}
	return strings.TrimSpace(response.Content), response.Provider, response.Model, snapshot, nil
}

func (r *TurnRunner) chatWithTools(ctx context.Context, input TurnRunInput, snapshot ContextSnapshot, messages []llm.ChatMessage) (llm.ChatResponse, ContextSnapshot, []llm.ChatMessage, error) {
	tools := r.listMCPTools(input.AllowedToolKeys)
	const maxToolRounds = 50
	requiredToolRetries := 0
	toolInteractionObserved := false
	for round := 0; round <= maxToolRounds; round++ {
		hasToolObservation := toolInteractionObserved || hasObservationForMCPTools(snapshot.Observations, tools)
		response, effectiveMessages, err := r.chatWithEmptyResponseRetry(ctx, input, messages, tools, false, hasToolObservation, len(tools) > 0 && !hasToolObservation)
		if err != nil {
			return llm.ChatResponse{}, snapshot, messages, err
		}
		messages = effectiveMessages
		if len(response.ToolCalls) == 0 {
			// 主 Agent 已下发工具 scope 时，子 Agent 至少要产生一次真实 observation。
			// 如果模型首轮直接给出文本，runner 不把它当作完成结果，而是拉回工具执行路径。
			if len(tools) > 0 && !toolInteractionObserved && !hasObservationForMCPTools(snapshot.Observations, tools) {
				requiredToolRetries++
				if requiredToolRetries > requiredToolCallRetryLimit {
					return llm.ChatResponse{}, snapshot, messages, domain.NewAppError(domain.ErrorKindUnavailable, "agent_required_tool_skipped", "agent did not call an approved tool before answering", "agent.turn_runner.tools", true, nil)
				}
				r.record(ctx, AuditEvent{
					SessionID: input.Session.ID,
					TurnID:    input.Turn.ID,
					UserID:    input.UserID,
					EventType: "agent.required_tool_call_retry",
					Status:    "retrying",
					Message:   "agent required tool call retry scheduled",
					Metadata: domain.AgentJSON{
						"attempt":     requiredToolRetries,
						"max_retries": requiredToolCallRetryLimit,
						"tool_count":  len(tools),
					},
					RequestID: input.RequestID,
					TraceID:   input.TraceID,
					CreatedAt: r.now().UTC(),
				})
				if strings.TrimSpace(response.Content) != "" {
					messages = append(messages, llm.ChatMessage{Role: "assistant", Content: response.Content})
				}
				messages = append(messages, llm.ChatMessage{Role: "user", Content: requiredToolCallRetryPrompt(requiredToolRetries)})
				continue
			}
			return response, snapshot, messages, nil
		}
		if r.toolExecutor == nil {
			if strings.TrimSpace(response.Content) != "" {
				return response, snapshot, messages, nil
			}
			return llm.ChatResponse{}, snapshot, messages, domain.NewAppError(domain.ErrorKindUnavailable, "agent_tool_executor_unavailable", "agent tool executor is unavailable", "agent.turn_runner.tools", true, nil)
		}

		messages = append(messages, llm.ChatMessage{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		})
		for _, call := range response.ToolCalls {
			toolInteractionObserved = true
			result, err := r.executeToolCall(ctx, input, call)
			if err != nil {
				return llm.ChatResponse{}, snapshot, messages, err
			}
			observation := result.Observation
			if observation.Capability == "" {
				observation.Capability = capabilityKeyForToolName(call.Name)
			}
			if observation.Decision == "" {
				observation.Decision = string(PolicyDecisionAllow)
			}
			if observation.Status == "" {
				observation.Status = "succeeded"
			}
			snapshot.Observations = append(snapshot.Observations, observation)
			content := strings.TrimSpace(result.TextContent())
			if content == "" {
				content = emptyMCPToolResultPrompt()
			}
			messages = append(messages, llm.ChatMessage{
				Role:       "tool",
				ToolCallID: call.ID,
				Name:       call.Name,
				Content:    content,
			})
		}
	}
	if len(snapshot.Observations) > 0 {
		// 工具预算耗尽后进入收敛阶段：不再提供工具，只允许模型基于已有观察生成最终回答。
		messages = append(messages, llm.ChatMessage{
			Role:    "user",
			Content: "工具调用轮次已经达到上限。请只基于以上工具结果生成最终回答，不要再请求工具；如果证据不足，请直接说明证据不足。",
		})
		response, effectiveMessages, err := r.chatWithEmptyResponseRetry(ctx, input, messages, nil, true, true, false)
		if err != nil {
			return llm.ChatResponse{}, snapshot, messages, err
		}
		messages = effectiveMessages
		if strings.TrimSpace(response.Content) != "" {
			return response, snapshot, messages, nil
		}
	}
	return llm.ChatResponse{}, snapshot, messages, domain.NewAppError(domain.ErrorKindUnavailable, "agent_tool_round_limit", "agent tool call round limit exceeded", "agent.turn_runner.tools", true, nil)
}

// ensurePlainTextReply 对最终用户可见回答执行纯文本协议校验。
// 若模型输出 Markdown 形态，runner 会要求模型重写；后端不直接拼接或改写回答事实。
func (r *TurnRunner) ensurePlainTextReply(ctx context.Context, input TurnRunInput, messages []llm.ChatMessage, response llm.ChatResponse) (llm.ChatResponse, error) {
	if r == nil || r.llmClient == nil {
		return response, nil
	}
	content := strings.TrimSpace(response.Content)
	violation := PlainTextReplyViolation(content)
	if violation == "" {
		return response, nil
	}
	effectiveMessages := append([]llm.ChatMessage(nil), messages...)
	if content != "" {
		effectiveMessages = append(effectiveMessages, llm.ChatMessage{Role: "assistant", Content: content})
	}
	current := response
	for attempt := 0; attempt < plainTextReplyRepairLimit; attempt++ {
		r.record(ctx, AuditEvent{
			SessionID: input.Session.ID,
			TurnID:    input.Turn.ID,
			UserID:    input.UserID,
			EventType: "agent.plain_text_reply_repair_retry",
			Status:    "retrying",
			Message:   "agent reply plain text repair retry scheduled",
			Metadata: domain.AgentJSON{
				"attempt":   attempt + 1,
				"violation": violation,
			},
			RequestID: input.RequestID,
			TraceID:   input.TraceID,
			CreatedAt: r.now().UTC(),
		})
		repairMessages := append([]llm.ChatMessage(nil), effectiveMessages...)
		repairMessages = append(repairMessages, llm.ChatMessage{
			Role:    "user",
			Content: PlainTextReplyRepairPrompt(current.Content, violation, attempt+1),
		})
		repaired, _, err := r.chatWithEmptyResponseRetry(ctx, input, repairMessages, nil, true, false, false)
		if err != nil {
			return llm.ChatResponse{}, err
		}
		current = repaired
		content = strings.TrimSpace(current.Content)
		violation = PlainTextReplyViolation(content)
		if violation == "" {
			current.Content = content
			return current, nil
		}
		effectiveMessages = append(repairMessages, llm.ChatMessage{Role: "assistant", Content: content})
	}
	return llm.ChatResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_reply_plain_text_violation", "agent reply does not satisfy plain text format", "agent.turn_runner.reply_format", true, nil)
}

// chatWithEmptyResponseRetry 处理上游模型“请求成功但没有内容”的临界情况。
// 它只追加模型收敛提示并有限重试，不在后端生成业务结论，避免把固定分析逻辑塞进服务端。
func (r *TurnRunner) chatWithEmptyResponseRetry(
	ctx context.Context,
	input TurnRunInput,
	messages []llm.ChatMessage,
	tools []MCPToolDescriptor,
	finalOnly bool,
	hasObservations bool,
	requireToolCall bool,
) (llm.ChatResponse, []llm.ChatMessage, error) {
	effectiveMessages := append([]llm.ChatMessage(nil), messages...)
	effectiveTools := append([]MCPToolDescriptor(nil), tools...)
	if finalOnly {
		effectiveTools = nil
	}
	var lastErr error
	for attempt := 0; attempt <= emptyLLMResponseRetryLimit; attempt++ {
		toolChoice := toolChoiceForDefinitions(effectiveTools, requireToolCall)
		response, err := r.llmClient.Chat(ctx, llm.ChatRequest{
			Messages:    effectiveMessages,
			Tools:       llmToolsFromMCPTools(effectiveTools),
			ToolChoice:  toolChoice,
			Temperature: r.temperature,
			MaxTokens:   r.maxTokens,
		})
		if err != nil && toolChoice == "required" && isToolChoiceRequiredUnsupportedError(err) {
			// 部分 OpenAI-compatible 服务不完整支持 tool_choice=required。
			// 此处只做协议兼容降级，后续仍由 runner 检查是否真的产生了工具调用。
			r.record(ctx, AuditEvent{
				SessionID: input.Session.ID,
				TurnID:    input.Turn.ID,
				UserID:    input.UserID,
				EventType: "agent.tool_choice_required_fallback",
				Status:    "retrying",
				Message:   "tool choice required is unsupported, retrying with auto",
				Metadata: domain.AgentJSON{
					"tool_count": len(effectiveTools),
					"error":      err.Error(),
				},
				RequestID: input.RequestID,
				TraceID:   input.TraceID,
				CreatedAt: r.now().UTC(),
			})
			response, err = r.llmClient.Chat(ctx, llm.ChatRequest{
				Messages:    effectiveMessages,
				Tools:       llmToolsFromMCPTools(effectiveTools),
				ToolChoice:  toolChoiceForDefinitions(effectiveTools, false),
				Temperature: r.temperature,
				MaxTokens:   r.maxTokens,
			})
		}
		if err == nil && strings.TrimSpace(response.Content) == "" && len(response.ToolCalls) == 0 {
			err = domain.NewAppError(domain.ErrorKindUnavailable, "llm_empty_response", "llm response is empty", "agent.turn_runner.chat", true, nil)
		}
		if err == nil && len(response.ToolCalls) == 0 && isUnparsedToolCallMarkup(response.Content) {
			err = domain.NewAppError(domain.ErrorKindUnavailable, "llm_unparsed_tool_call", "llm returned unparsed tool call markup", "agent.turn_runner.chat", true, nil)
		}
		if err != nil && isRecoverableLLMResponseShapeError(err) && len(effectiveTools) > 0 {
			fallback, fallbackErr := r.chatWithPromptedToolAction(ctx, input, effectiveMessages, effectiveTools, requireToolCall, hasObservations, isUnparsedToolCallError(err))
			if fallbackErr == nil {
				r.record(ctx, AuditEvent{
					SessionID: input.Session.ID,
					TurnID:    input.Turn.ID,
					UserID:    input.UserID,
					EventType: "agent.prompted_tool_action_fallback",
					Status:    "succeeded",
					Message:   "prompted tool action fallback succeeded",
					Metadata: domain.AgentJSON{
						"native_error":       err.Error(),
						"require_tool_call":  requireToolCall,
						"has_observations":   hasObservations,
						"fallback_tool_call": len(fallback.ToolCalls) > 0,
					},
					RequestID: input.RequestID,
					TraceID:   input.TraceID,
					CreatedAt: r.now().UTC(),
				})
				return fallback, effectiveMessages, nil
			}
			r.record(ctx, AuditEvent{
				SessionID: input.Session.ID,
				TurnID:    input.Turn.ID,
				UserID:    input.UserID,
				EventType: "agent.prompted_tool_action_fallback",
				Status:    "failed",
				Message:   "prompted tool action fallback failed",
				Metadata: domain.AgentJSON{
					"native_error":      err.Error(),
					"fallback_error":    fallbackErr.Error(),
					"require_tool_call": requireToolCall,
					"has_observations":  hasObservations,
				},
				RequestID: input.RequestID,
				TraceID:   input.TraceID,
				CreatedAt: r.now().UTC(),
			})
		}
		if err == nil {
			return response, effectiveMessages, nil
		}
		lastErr = err
		if !isRecoverableLLMResponseShapeError(err) || attempt == emptyLLMResponseRetryLimit {
			return llm.ChatResponse{}, effectiveMessages, err
		}
		eventType := "agent.llm_empty_response_retry"
		eventMessage := "llm empty response retry scheduled"
		if isUnparsedToolCallError(err) {
			eventType = "agent.llm_unparsed_tool_call_retry"
			eventMessage = "llm unparsed tool call retry scheduled"
		}
		r.record(ctx, AuditEvent{
			SessionID: input.Session.ID,
			TurnID:    input.Turn.ID,
			UserID:    input.UserID,
			EventType: eventType,
			Status:    "retrying",
			Message:   eventMessage,
			Metadata: domain.AgentJSON{
				"attempt":          attempt + 1,
				"max_retries":      emptyLLMResponseRetryLimit,
				"final_only":       finalOnly,
				"has_observations": hasObservations,
				"error":            err.Error(),
			},
			RequestID: input.RequestID,
			TraceID:   input.TraceID,
			CreatedAt: r.now().UTC(),
		})
		effectiveMessages = append(effectiveMessages, llm.ChatMessage{
			Role:    "user",
			Content: llmResponseShapeRetryPrompt(err, attempt+1, finalOnly, hasObservations, len(effectiveTools) > 0),
		})
	}
	return llm.ChatResponse{}, effectiveMessages, lastErr
}

func (r *TurnRunner) chatWithPromptedToolAction(
	ctx context.Context,
	input TurnRunInput,
	messages []llm.ChatMessage,
	tools []MCPToolDescriptor,
	requireToolCall bool,
	hasObservations bool,
	repairMalformed bool,
) (llm.ChatResponse, error) {
	if r == nil || r.llmClient == nil {
		return llm.ChatResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_llm_client_unavailable", "agent llm client is unavailable", "agent.turn_runner.prompted_tool_action", true, nil)
	}
	// 兼容模式用于处理上游未返回原生 tool_calls 的情况。
	// runner 只负责有限重试和审计，具体格式修正要求集中在 runner_prompts.go。
	effectiveMessages := buildPromptedToolActionMessages(messages, tools, requireToolCall, hasObservations)
	var lastErr error
	retryLimit := 0
	if repairMalformed {
		retryLimit = promptedToolActionRetryLimit
	}
	for attempt := 0; attempt <= retryLimit; attempt++ {
		response, err := r.llmClient.Chat(ctx, llm.ChatRequest{
			Messages:    effectiveMessages,
			Temperature: 0.1,
			MaxTokens:   r.maxTokens,
		})
		if err != nil {
			return llm.ChatResponse{}, err
		}
		result, err := r.normalizePromptedToolAction(response, actionToolCallID(len(messages), attempt), tools, requireToolCall)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if attempt == retryLimit {
			return llm.ChatResponse{}, err
		}
		r.record(ctx, AuditEvent{
			SessionID: input.Session.ID,
			TurnID:    input.Turn.ID,
			UserID:    input.UserID,
			EventType: "agent.prompted_tool_action_repair_retry",
			Status:    "retrying",
			Message:   "prompted tool action response repair retry scheduled",
			Metadata: domain.AgentJSON{
				"attempt":             attempt + 1,
				"max_retries":         retryLimit,
				"require_tool_call":   requireToolCall,
				"has_observations":    hasObservations,
				"tool_count":          len(tools),
				"normalization_error": err.Error(),
			},
			RequestID: input.RequestID,
			TraceID:   input.TraceID,
			CreatedAt: r.now().UTC(),
		})
		effectiveMessages = append(effectiveMessages,
			llm.ChatMessage{Role: "assistant", Content: response.Content},
			llm.ChatMessage{Role: "user", Content: promptedToolActionRepairPrompt(err, attempt+1, requireToolCall, hasObservations, len(tools) > 0)},
		)
	}
	return llm.ChatResponse{}, lastErr
}

// normalizePromptedToolAction 将兼容 JSON 契约转换为内部统一的 ChatResponse。
// 该函数只校验协议形态和已授权工具，不根据用户语义补全缺失字段。
func (r *TurnRunner) normalizePromptedToolAction(
	response llm.ChatResponse,
	callID string,
	tools []MCPToolDescriptor,
	requireToolCall bool,
) (llm.ChatResponse, error) {
	action, err := parsePromptedToolAction(response.Content)
	if err != nil {
		return llm.ChatResponse{}, err
	}
	switch action.Action {
	case "tool_call":
		if !promptedToolActionAllowed(action, tools) {
			return llm.ChatResponse{}, fmt.Errorf("prompted tool action selected unavailable tool: %s", action.ToolName)
		}
		arguments, err := json.Marshal(action.Arguments)
		if err != nil {
			return llm.ChatResponse{}, fmt.Errorf("prompted tool action arguments marshal failed: %w", err)
		}
		return llm.ChatResponse{
			Provider: response.Provider,
			Model:    response.Model,
			ToolCalls: []llm.ToolCall{
				{
					ID:        callID,
					Name:      action.ToolName,
					Arguments: string(arguments),
				},
			},
		}, nil
	case "final":
		if requireToolCall {
			return llm.ChatResponse{}, fmt.Errorf("prompted tool action returned final while tool call is required")
		}
		if strings.TrimSpace(action.Content) == "" {
			return llm.ChatResponse{}, fmt.Errorf("prompted tool action final content is empty")
		}
		return llm.ChatResponse{Provider: response.Provider, Model: response.Model, Content: action.Content}, nil
	default:
		return llm.ChatResponse{}, fmt.Errorf("prompted tool action is unsupported: %s", action.Action)
	}
}

func actionToolCallID(baseMessageCount int, attempt int) string {
	return fmt.Sprintf("prompted_call_%d_%d", baseMessageCount+1, attempt+1)
}

func hasObservationForMCPTools(observations []CapabilityObservation, tools []MCPToolDescriptor) bool {
	if len(observations) == 0 || len(tools) == 0 {
		return false
	}
	toolKeys := map[string]struct{}{}
	for _, tool := range tools {
		key := strings.TrimSpace(tool.Name)
		if strings.TrimSpace(key) == "" {
			continue
		}
		toolKeys[key] = struct{}{}
	}
	for _, observation := range observations {
		key := strings.TrimSpace(observation.Capability)
		if _, ok := toolKeys[key]; ok {
			return true
		}
	}
	return false
}

func isToolChoiceRequiredUnsupportedError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	if !strings.Contains(lower, "tool_choice") || !strings.Contains(lower, "required") {
		return false
	}
	return strings.Contains(lower, "unsupported") ||
		strings.Contains(lower, "not supported") ||
		strings.Contains(lower, "not support") ||
		strings.Contains(lower, "invalid")
}

func isEmptyLLMResponseError(err error) bool {
	if err == nil {
		return false
	}
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		if appErr.Code == "llm_empty_response" || appErr.Code == "agent_empty_reply" {
			return true
		}
	}
	return strings.Contains(strings.ToLower(err.Error()), "llm response is empty")
}

func isUnparsedToolCallError(err error) bool {
	if err == nil {
		return false
	}
	var appErr *domain.AppError
	if errors.As(err, &appErr) && appErr.Code == "llm_unparsed_tool_call" {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "unparsed tool call")
}

func isRecoverableLLMResponseShapeError(err error) bool {
	return isEmptyLLMResponseError(err) || isUnparsedToolCallError(err)
}

func isUnparsedToolCallMarkup(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}
	return strings.Contains(content, "<|tool_calls_section_begin|>") ||
		strings.Contains(content, "<|tool_call_begin|>") ||
		strings.Contains(content, "<|tool_call_argument_begin|>") ||
		strings.Contains(content, "chatcmpl-tool-")
}

func (r *TurnRunner) executeToolCall(ctx context.Context, input TurnRunInput, call llm.ToolCall) (MCPCallToolResult, error) {
	key := capabilityKeyForToolName(call.Name)
	capability, ok := r.toolRegistry.Get(key)
	if !ok {
		return MCPCallToolResult{}, domain.NewAppError(domain.ErrorKindInvalidInput, "agent_unknown_tool", "agent tool is not registered", "agent.turn_runner.tools", false, nil)
	}
	if !r.toolAllowedInCurrentScope(key, input.AllowedToolKeys) {
		summary := "agent tool is outside approved capability scope"
		r.record(ctx, AuditEvent{
			SessionID: input.Session.ID,
			TurnID:    input.Turn.ID,
			UserID:    input.UserID,
			EventType: "agent.capability_scope_denied",
			Status:    "failed",
			Message:   summary,
			Metadata: domain.AgentJSON{
				"capability_key": key,
				"tool_call_id":   call.ID,
				"allowed_tools":  append([]string(nil), input.AllowedToolKeys...),
			},
			RequestID: input.RequestID,
			TraceID:   input.TraceID,
			CreatedAt: r.now().UTC(),
		})
		return NewMCPTextCallToolResult(mcpToolScopeDeniedText(), true, CapabilityObservation{
			Capability: key,
			Decision:   string(PolicyDecisionForbidden),
			Status:     "failed",
			Summary:    summary,
		}), nil
	}
	if (capability.Mutates && !capability.Schedulable) || capability.Risk == CapabilityRiskHigh {
		return MCPCallToolResult{}, domain.NewAppError(domain.ErrorKindInvalidInput, "agent_tool_not_allowed", "agent tool is not allowed in current policy", "agent.turn_runner.tools", false, nil)
	}
	tool := capability.MCPDescriptor()
	return r.toolExecutor.CallTool(ctx, MCPCallToolInput{
		Capability:      capability,
		Tool:            tool,
		UserID:          input.UserID,
		SessionID:       input.Session.ID,
		TurnID:          input.Turn.ID,
		ControllerRunID: input.ControllerRunID,
		Message:         input.MessageText,
		ExternalUserID:  input.InboundMessage.ExternalUserID,
		CallID:          call.ID,
		Name:            tool.Name,
		RawArguments:    call.Arguments,
		RequestID:       input.RequestID,
		TraceID:         input.TraceID,
	})
}

func (r *TurnRunner) toolAllowedInCurrentScope(key string, scopedKeys []string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	if scopedKeys != nil {
		for _, scoped := range scopedKeys {
			if scopeMatchesTool(strings.TrimSpace(scoped), key) {
				return true
			}
		}
		return false
	}
	keys := append([]string(nil), r.toolKeys...)
	if len(keys) == 0 {
		keys = []string{"conversation.query_history"}
	}
	for _, allowed := range keys {
		if strings.TrimSpace(allowed) == key {
			return true
		}
	}
	return false
}

func scopeMatchesTool(scope string, key string) bool {
	if scope == key {
		return true
	}
	return scope == "agent.schedule_task" && key == "agent.schedule_message"
}

func (r *TurnRunner) listMCPTools(scopedKeys []string) []MCPToolDescriptor {
	if r == nil || r.toolRegistry == nil || r.toolExecutor == nil {
		return nil
	}
	if scopedKeys != nil && len(scopedKeys) == 0 {
		return nil
	}
	keys := append([]string(nil), r.toolKeys...)
	if len(keys) == 0 {
		keys = []string{"conversation.query_history"}
	}
	tools := make([]MCPToolDescriptor, 0, len(keys))
	for _, key := range keys {
		if !r.toolAllowedInCurrentScope(key, scopedKeys) {
			continue
		}
		capability, ok := r.toolRegistry.Get(key)
		if !ok || (capability.Mutates && !capability.Schedulable) || capability.Risk == CapabilityRiskHigh {
			continue
		}
		tools = append(tools, capability.MCPDescriptor())
	}
	return tools
}

func (r *TurnRunner) buildChatMessages(systemPrompt string, snapshot ContextSnapshot, currentMessage string) []llm.ChatMessage {
	messages := []llm.ChatMessage{{Role: "system", Content: systemPrompt}}
	for _, message := range snapshot.Messages {
		role := strings.TrimSpace(string(message.Role))
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		if role != string(domain.AgentTranscriptRoleUser) && role != string(domain.AgentTranscriptRoleAssistant) {
			continue
		}
		messages = append(messages, llm.ChatMessage{Role: role, Content: content})
	}
	messages = append(messages, llm.ChatMessage{Role: "user", Content: strings.TrimSpace(currentMessage)})
	return messages
}

func (r *TurnRunner) buildSystemPrompt(snapshot ContextSnapshot, currentMessage string) string {
	var builder strings.Builder
	if r.systemPrompt != "" {
		builder.WriteString(r.systemPrompt)
	}
	for _, block := range snapshot.Blocks {
		content := strings.TrimSpace(block.Content)
		if content == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(block.Name)
		builder.WriteString("：\n")
		builder.WriteString(content)
	}
	if snapshot.HistoryNeedHint != "" {
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString("历史查询策略：\n")
		builder.WriteString(historyNeedPrompt(snapshot.HistoryNeedHint))
	}
	if builder.Len() > 0 {
		builder.WriteString("\n\n")
	}
	builder.WriteString("任务规格：由主 Agent 的结构化 PlanSpec 和当前工具观察确定；不要根据固定关键词自行改写用户意图。")
	builder.WriteString("\n\n")
	now := r.now().In(time.FixedZone("Asia/Shanghai", 8*60*60))
	builder.WriteString("当前时间：")
	builder.WriteString(now.Format("2006-01-02 15:04:05"))
	builder.WriteString(" Asia/Shanghai。")
	builder.WriteString("\n\n")
	builder.WriteString("能力边界：当前只允许执行已下发 capability scope 内的能力。只读本地查询、历史聊天查询、受限联网读取、远端仓库只读检查和文本总结可以执行；新增订阅、停用来源、通知配置、画像写入、金融告警或其他状态变更必须拒绝直接执行，并说明需要后续确认流程。联网信息必须保留来源、抓取时间和摘要，不得把外部内容改写为无来源事实；repo.inspect_remote 只能读取远端仓库元数据、README 和 license，不得克隆或写入本地文件。")
	if r.toolExecutor != nil {
		builder.WriteString("\n可用工具：需要读取订阅条目时调用 feed.query_recent_items；需要读取指定来源最新条目时调用 source.query_latest_items。需要查询更早企微聊天原文时调用 conversation.query_history，并由你显式提供 mode、query 或 time_hint。需要联网检索网页时使用 web.search；需要读取指定 URL 时使用 web.fetch_page；需要抽取网页标题、正文摘要和主要链接时使用 web.extract_page。需要搜索参考仓库时使用 repo.search；需要检查 GitHub 仓库时使用 repo.inspect_remote，并且不得克隆仓库。需要创建定时提醒、定时检索、定时总结、日报或周报任务时优先使用 agent.schedule_task；agent.schedule_message 仅作为旧提醒兼容入口。模型必须结合当前时间和最近上下文，把用户的自然语言时间归一化为 scheduled_at，优先使用 RFC3339。除非用户已经明确确认创建，否则 confirmed 必须为 false；确认后必须再次调用对应定时工具并传 confirmed=true，不得只口头表示会创建。")
	}
	return builder.String()
}

func toolChoiceForDefinitions(tools []MCPToolDescriptor, requireToolCall bool) string {
	if len(tools) == 0 {
		return ""
	}
	if requireToolCall {
		return "required"
	}
	return "auto"
}

func llmToolsFromMCPTools(tools []MCPToolDescriptor) []llm.ToolDefinition {
	if len(tools) == 0 {
		return nil
	}
	definitions := make([]llm.ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == "" {
			continue
		}
		definitions = append(definitions, LLMToolDefinitionFromMCP(tool))
	}
	return definitions
}

func toolNameForCapabilityKey(key string) string {
	return strings.ReplaceAll(strings.TrimSpace(key), ".", "__")
}

func capabilityKeyForToolName(name string) string {
	trimmed := strings.TrimSpace(name)
	if strings.Contains(trimmed, ".") {
		return trimmed
	}
	return strings.ReplaceAll(trimmed, "__", ".")
}

type fallbackEvidenceItem struct {
	Title       string
	Source      string
	PublishedAt string
	Summary     string
	URL         string
}

func fallbackEvidenceItems(message string, blocks []ContextBlock) []fallbackEvidenceItem {
	taskSpec := BuildTaskSpec(message)
	searchTask := taskSpec.RequestsSearch()
	webItems := make([]fallbackEvidenceItem, 0, 8)
	localItems := make([]fallbackEvidenceItem, 0, 8)
	items := make([]fallbackEvidenceItem, 0, 8)
	for _, block := range blocks {
		if !fallbackBlockIsUserVisible(block.CapabilityKey) {
			continue
		}
		parsed := parseFallbackEvidenceItems(block.Content)
		if strings.HasPrefix(strings.TrimSpace(block.CapabilityKey), "web.") {
			webItems = append(webItems, parsed...)
			continue
		}
		localItems = append(localItems, parsed...)
	}
	if searchTask && len(webItems) > 0 {
		if relevant := fallbackRankEvidenceItems(taskSpec, webItems); len(relevant) > 0 {
			return dedupeFallbackEvidenceItems(relevant)
		}
	}
	if searchTask {
		if relevant := fallbackRankEvidenceItems(taskSpec, localItems); len(relevant) > 0 {
			return dedupeFallbackEvidenceItems(relevant)
		}
		return nil
	}
	items = append(items, webItems...)
	items = append(items, localItems...)
	if len(items) == 0 && !searchTask {
		for _, block := range blocks {
			if !fallbackBlockIsUserVisible(block.CapabilityKey) {
				continue
			}
			title := strings.TrimSpace(block.Name)
			content := strings.TrimSpace(block.Content)
			if title == "" || content == "" {
				continue
			}
			items = append(items, fallbackEvidenceItem{Title: title, Summary: content})
		}
	}
	return dedupeFallbackEvidenceItems(items)
}

func fallbackRankEvidenceItems(taskSpec TaskSpec, items []fallbackEvidenceItem) []fallbackEvidenceItem {
	type scoredFallbackItem struct {
		item  fallbackEvidenceItem
		score EvidenceScore
		index int
	}
	scored := make([]scoredFallbackItem, 0, len(items))
	for index, item := range items {
		score := ScoreEvidence(taskSpec, EvidenceScoreInput{
			Title:       item.Title,
			Source:      item.Source,
			Summary:     item.Summary,
			URL:         item.URL,
			PublishedAt: item.PublishedAt,
		})
		if !score.Relevant {
			continue
		}
		scored = append(scored, scoredFallbackItem{item: item, score: score, index: index})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score.Score == scored[j].score.Score {
			return scored[i].index < scored[j].index
		}
		return scored[i].score.Score > scored[j].score.Score
	})
	filtered := make([]fallbackEvidenceItem, 0, len(scored))
	for _, item := range scored {
		filtered = append(filtered, item.item)
	}
	return filtered
}

func fallbackBlockIsUserVisible(capabilityKey string) bool {
	key := strings.TrimSpace(capabilityKey)
	return strings.HasPrefix(key, "feed.") ||
		strings.HasPrefix(key, "source.") ||
		strings.HasPrefix(key, "web.") ||
		strings.HasPrefix(key, "repo.")
}

func parseFallbackEvidenceItems(content string) []fallbackEvidenceItem {
	lines := strings.Split(content, "\n")
	items := make([]fallbackEvidenceItem, 0, 8)
	current := fallbackEvidenceItem{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "工具：") || strings.HasPrefix(line, "查询：") ||
			strings.HasPrefix(line, "来源：") || strings.HasPrefix(line, "抓取时间：") ||
			strings.HasPrefix(line, "HTTP 状态：") || strings.HasPrefix(line, "内容类型：") ||
			strings.HasPrefix(line, "证据引用：") || strings.HasPrefix(line, "Evidence ref：") ||
			strings.HasPrefix(line, "新鲜度提示：") || strings.HasPrefix(line, "结果：") ||
			strings.HasPrefix(line, "已读：") {
			continue
		}
		if title, ok := parseNumberedFallbackTitle(line); ok {
			if current.Title != "" {
				items = append(items, current)
			}
			current = fallbackEvidenceItem{}
			current.Title, current.Source = splitFallbackTitleSource(title)
			continue
		}
		switch {
		case strings.HasPrefix(line, "URL："):
			current.URL = strings.TrimSpace(strings.TrimPrefix(line, "URL："))
		case strings.HasPrefix(line, "摘要："):
			current.Summary = strings.TrimSpace(strings.TrimPrefix(line, "摘要："))
		case strings.HasPrefix(line, "发布时间："):
			current.PublishedAt = strings.TrimSpace(strings.TrimPrefix(line, "发布时间："))
		default:
			if current.Title != "" && current.Summary == "" && len([]rune(line)) > 20 {
				current.Summary = line
			}
		}
	}
	if current.Title != "" {
		items = append(items, current)
	}
	return items
}

func parseNumberedFallbackTitle(line string) (string, bool) {
	dot := strings.Index(line, ".")
	if dot <= 0 || dot > 3 {
		return "", false
	}
	for _, r := range line[:dot] {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	title := strings.TrimSpace(line[dot+1:])
	if title == "" {
		return "", false
	}
	return title, true
}

func splitFallbackTitleSource(title string) (string, string) {
	title = strings.TrimSpace(title)
	if strings.HasSuffix(title, "）") {
		if start := strings.LastIndex(title, "（"); start > 0 {
			source := strings.TrimSpace(strings.TrimSuffix(title[start+len("（"):], "）"))
			return strings.TrimSpace(title[:start]), source
		}
	}
	if strings.HasSuffix(title, ")") {
		if start := strings.LastIndex(title, "("); start > 0 {
			source := strings.TrimSpace(strings.TrimSuffix(title[start+1:], ")"))
			return strings.TrimSpace(title[:start]), source
		}
	}
	return title, ""
}

func dedupeFallbackEvidenceItems(items []fallbackEvidenceItem) []fallbackEvidenceItem {
	deduped := make([]fallbackEvidenceItem, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item.Title = strings.TrimSpace(item.Title)
		item.Source = strings.TrimSpace(item.Source)
		item.PublishedAt = strings.TrimSpace(item.PublishedAt)
		item.Summary = strings.TrimSpace(item.Summary)
		item.URL = strings.TrimSpace(item.URL)
		if item.Title == "" {
			continue
		}
		key := item.URL
		if key == "" {
			key = item.Title + "|" + item.Source
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, item)
	}
	return deduped
}

func (r *TurnRunner) failTurn(ctx context.Context, input TurnRunInput, cause error) domain.AgentTurn {
	now := r.now().UTC()
	turn := input.Turn
	turn.Status = domain.AgentTurnStatusFailed
	turn.ErrorMessage = cause.Error()
	turn.FinishedAt = &now
	if turn.ID > 0 && r.store != nil {
		updated, err := r.store.UpdateTurn(ctx, turn)
		if err == nil {
			turn = updated
		}
	}
	r.record(ctx, AuditEvent{
		SessionID: input.Session.ID,
		TurnID:    turn.ID,
		UserID:    input.UserID,
		EventType: "agent.turn_failed",
		Status:    "failed",
		Message:   cause.Error(),
		Metadata:  domain.AgentJSON{"provider_message_id": input.InboundMessage.ProviderMessageID},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: now,
	})
	return turn
}

func (r *TurnRunner) markInbound(ctx context.Context, input TurnRunInput, status domain.AgentInboundMessageStatus) error {
	if r.store == nil || input.InboundMessage.ID == 0 {
		return nil
	}
	_, err := r.store.UpdateInboundMessageStatus(ctx, input.UserID, input.InboundMessage.ID, status, r.now().UTC())
	return err
}

func (r *TurnRunner) record(ctx context.Context, event AuditEvent) {
	if r.auditLogger == nil {
		return
	}
	_ = r.auditLogger.Record(ctx, event)
}

func ObservationMetadata(observations []CapabilityObservation) []domain.AgentJSON {
	if len(observations) == 0 {
		return nil
	}
	output := make([]domain.AgentJSON, 0, len(observations))
	for _, observation := range observations {
		output = append(output, domain.AgentJSON{
			"capability":      observation.Capability,
			"decision":        observation.Decision,
			"status":          observation.Status,
			"summary":         observation.Summary,
			"run_id":          observation.RunID,
			"observation_ref": observation.ObservationRef,
			"artifact_refs":   append([]string(nil), observation.ArtifactRefs...),
		})
	}
	return output
}
