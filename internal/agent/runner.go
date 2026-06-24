package agent

import (
	"context"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"messagefeed/internal/observability"
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
	ExecuteTool(ctx context.Context, input ToolExecuteInput) (ToolExecuteResult, error)
}

type ContextBuilder interface {
	Build(ctx context.Context, input ContextBuildInput) (ContextSnapshot, error)
}

type ContextBuildInput struct {
	UserID      int64
	SessionID   int64
	TurnID      int64
	MessageText string
	MessageType string
	RequestID   string
	TraceID     string
}

type ContextSnapshot struct {
	Blocks          []ContextBlock
	Messages        []ContextMessage
	Observations    []CapabilityObservation
	HistoryNeedHint HistoryNeedHint
}

type ContextBlock struct {
	Name          string
	CapabilityKey string
	Content       string
	ItemCount     int
	Truncated     bool
	GeneratedAt   time.Time
	TrustLevel    string
}

type ContextMessage struct {
	Role              domain.AgentTranscriptRole
	Content           string
	TranscriptEntryID int64
	TurnID            int64
	CreatedAt         time.Time
}

type CapabilityObservation struct {
	Capability string
	Decision   string
	Status     string
	Summary    string
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

type ToolExecuteInput struct {
	Capability     Capability
	UserID         int64
	SessionID      int64
	TurnID         int64
	Message        string
	ExternalUserID string
	ToolCallID     string
	RawArguments   string
	RequestID      string
	TraceID        string
}

type ToolExecuteResult struct {
	Content     string
	Observation CapabilityObservation
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
	UserID         int64
	Session        domain.AgentSession
	Turn           domain.AgentTurn
	InboundMessage domain.AgentInboundMessage
	MessageType    string
	MessageText    string
	RequestID      string
	TraceID        string
}

type TurnRunResult struct {
	Turn          domain.AgentTurn
	Reply         string
	ModelProvider string
	Model         string
	Context       ContextSnapshot
}

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
		return "当前仅支持文本消息。", "", "", ContextSnapshot{}, nil
	}
	if r.llmClient == nil {
		return "已收到：" + input.MessageText, "", "", ContextSnapshot{}, nil
	}

	snapshot := ContextSnapshot{}
	if r.contextBuilder != nil {
		var err error
		snapshot, err = r.contextBuilder.Build(ctx, ContextBuildInput{
			UserID:      input.UserID,
			SessionID:   input.Session.ID,
			TurnID:      input.Turn.ID,
			MessageText: input.MessageText,
			MessageType: input.MessageType,
			RequestID:   input.RequestID,
			TraceID:     input.TraceID,
		})
		if err != nil {
			return "", "", "", snapshot, err
		}
	}

	systemPrompt := r.buildSystemPrompt(snapshot)
	messages := r.buildChatMessages(systemPrompt, snapshot, input.MessageText)
	response, snapshot, err := r.chatWithTools(ctx, input, snapshot, messages)
	if err != nil {
		return "", "", "", snapshot, err
	}
	return response.Content, response.Provider, response.Model, snapshot, nil
}

func (r *TurnRunner) chatWithTools(ctx context.Context, input TurnRunInput, snapshot ContextSnapshot, messages []llm.ChatMessage) (llm.ChatResponse, ContextSnapshot, error) {
	tools := r.buildToolDefinitions()
	const maxToolRounds = 2
	for round := 0; round <= maxToolRounds; round++ {
		response, err := r.llmClient.Chat(ctx, llm.ChatRequest{
			Messages:    messages,
			Tools:       tools,
			ToolChoice:  toolChoiceForDefinitions(tools),
			Temperature: r.temperature,
			MaxTokens:   r.maxTokens,
		})
		if err != nil {
			return llm.ChatResponse{}, snapshot, err
		}
		if len(response.ToolCalls) == 0 {
			return response, snapshot, nil
		}
		if r.toolExecutor == nil {
			if strings.TrimSpace(response.Content) != "" {
				return response, snapshot, nil
			}
			return llm.ChatResponse{}, snapshot, domain.NewAppError(domain.ErrorKindUnavailable, "agent_tool_executor_unavailable", "agent tool executor is unavailable", "agent.turn_runner.tools", true, nil)
		}

		messages = append(messages, llm.ChatMessage{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		})
		for _, call := range response.ToolCalls {
			result, err := r.executeToolCall(ctx, input, call)
			if err != nil {
				return llm.ChatResponse{}, snapshot, err
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
			content := strings.TrimSpace(result.Content)
			if content == "" {
				content = "工具没有返回内容。"
			}
			messages = append(messages, llm.ChatMessage{
				Role:       "tool",
				ToolCallID: call.ID,
				Name:       call.Name,
				Content:    content,
			})
		}
	}
	return llm.ChatResponse{}, snapshot, domain.NewAppError(domain.ErrorKindUnavailable, "agent_tool_round_limit", "agent tool call round limit exceeded", "agent.turn_runner.tools", true, nil)
}

func (r *TurnRunner) executeToolCall(ctx context.Context, input TurnRunInput, call llm.ToolCall) (ToolExecuteResult, error) {
	key := capabilityKeyForToolName(call.Name)
	capability, ok := r.toolRegistry.Get(key)
	if !ok {
		return ToolExecuteResult{}, domain.NewAppError(domain.ErrorKindInvalidInput, "agent_unknown_tool", "agent tool is not registered", "agent.turn_runner.tools", false, nil)
	}
	if capability.Mutates || capability.Risk == CapabilityRiskHigh {
		return ToolExecuteResult{}, domain.NewAppError(domain.ErrorKindInvalidInput, "agent_tool_not_allowed", "agent tool is not allowed in current policy", "agent.turn_runner.tools", false, nil)
	}
	return r.toolExecutor.ExecuteTool(ctx, ToolExecuteInput{
		Capability:     capability,
		UserID:         input.UserID,
		SessionID:      input.Session.ID,
		TurnID:         input.Turn.ID,
		Message:        input.MessageText,
		ExternalUserID: input.InboundMessage.ExternalUserID,
		ToolCallID:     call.ID,
		RawArguments:   call.Arguments,
		RequestID:      input.RequestID,
		TraceID:        input.TraceID,
	})
}

func (r *TurnRunner) buildToolDefinitions() []llm.ToolDefinition {
	if r == nil || r.toolRegistry == nil || r.toolExecutor == nil {
		return nil
	}
	keys := append([]string(nil), r.toolKeys...)
	if len(keys) == 0 {
		keys = []string{"conversation.query_history"}
	}
	definitions := make([]llm.ToolDefinition, 0, len(keys))
	for _, key := range keys {
		capability, ok := r.toolRegistry.Get(key)
		if !ok || capability.Mutates || capability.Risk == CapabilityRiskHigh {
			continue
		}
		definitions = append(definitions, llm.ToolDefinition{
			Name:        toolNameForCapabilityKey(capability.Key),
			Description: capability.Description,
			Parameters:  capability.Parameters,
		})
	}
	return definitions
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

func (r *TurnRunner) buildSystemPrompt(snapshot ContextSnapshot) string {
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
	now := r.now().In(time.FixedZone("Asia/Shanghai", 8*60*60))
	builder.WriteString("当前时间：")
	builder.WriteString(now.Format("2006-01-02 15:04:05"))
	builder.WriteString(" Asia/Shanghai。")
	builder.WriteString("\n\n")
	builder.WriteString("能力边界：P0 仅允许只读查询、文本总结、写入 transcript 和审计。新增订阅、停用来源、通知配置、画像写入、金融告警或其他状态变更必须拒绝直接执行，并说明需要后续确认流程。")
	if r.toolExecutor != nil {
		builder.WriteString("\n可用工具：如需查询更早企微聊天原文，只能调用 conversation.query_history；询问第一条、最早或最开始消息时使用 earliest 模式；按时间查询历史时使用 time_range 模式和 time_hint。若工具返回 has_older=false 且有命中记录，应确认该记录就是当前 session 起点。若最近聊天窗口已有明确证据且不需要确认会话边界，不要调用历史查询工具。需要创建定时消息或提醒时使用 agent.schedule_message；模型必须结合当前时间和最近上下文，把用户的自然语言时间归一化为 scheduled_at，优先使用 RFC3339。除非用户已经明确确认创建，否则 confirmed 必须为 false；当用户回复“是的、确认、可以、对”等确认上一轮待创建提醒时，必须补全上一轮内容和时间并再次调用 agent.schedule_message，且 confirmed=true，不得只口头表示会创建。")
	}
	return builder.String()
}

func toolChoiceForDefinitions(tools []llm.ToolDefinition) string {
	if len(tools) == 0 {
		return ""
	}
	return "auto"
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
			"capability": observation.Capability,
			"decision":   observation.Decision,
			"status":     observation.Status,
			"summary":    observation.Summary,
		})
	}
	return output
}
