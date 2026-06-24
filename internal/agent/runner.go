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
	Blocks       []ContextBlock
	Observations []CapabilityObservation
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

type TurnRunner struct {
	store          TurnStore
	auditLogger    AuditLogger
	contextBuilder ContextBuilder
	llmClient      ChatClient
	now            func() time.Time
	systemPrompt   string
	maxTokens      int
	temperature    float64
}

type TurnRunnerOptions struct {
	Store          TurnStore
	AuditLogger    AuditLogger
	ContextBuilder ContextBuilder
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
	return &TurnRunner{
		store:          options.Store,
		auditLogger:    options.AuditLogger,
		contextBuilder: options.ContextBuilder,
		llmClient:      options.LLMClient,
		now:            now,
		systemPrompt:   strings.TrimSpace(options.SystemPrompt),
		maxTokens:      options.MaxTokens,
		temperature:    temperature,
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
	response, err := r.llmClient.Chat(ctx, llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: input.MessageText},
		},
		Temperature: r.temperature,
		MaxTokens:   r.maxTokens,
	})
	if err != nil {
		return "", "", "", snapshot, err
	}
	return response.Content, response.Provider, response.Model, snapshot, nil
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
	if builder.Len() > 0 {
		builder.WriteString("\n\n")
	}
	builder.WriteString("能力边界：P0 仅允许只读查询、文本总结、写入 transcript 和审计。新增订阅、停用来源、通知配置、画像写入、金融告警或其他状态变更必须拒绝直接执行，并说明需要后续确认流程。")
	return builder.String()
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
