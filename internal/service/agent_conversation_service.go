package service

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"messagefeed/internal/metrics"
	"messagefeed/internal/notifier"
	"messagefeed/internal/observability"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const (
	defaultAgentOwnerUserID = int64(1)
	agentSystemPrompt       = "你是 messageFeed 的企业微信聊天助手。回复只能围绕本项目内的信息聚合、订阅源、阅读和设置。使用普通微信聊天文本，不使用 Markdown、表格、标题、加粗、代码块、列表符号、星号或反引号。默认回复 2 到 5 句，最多约 300 字；用户明确要求详细说明时，也使用适合手机阅读的短段落，不要写成文档。"
	agentReplyMaxTokens     = 768
)

type AgentConversationRepository interface {
	EnsureExternalAccount(ctx context.Context, account domain.ExternalAccount) (domain.ExternalAccount, error)
	CreateInboundMessage(ctx context.Context, message domain.AgentInboundMessage) (domain.AgentInboundMessage, bool, error)
	GetOrCreateSession(ctx context.Context, session domain.AgentSession) (domain.AgentSession, error)
	CreateTurn(ctx context.Context, turn domain.AgentTurn) (domain.AgentTurn, error)
	UpdateTurn(ctx context.Context, turn domain.AgentTurn) (domain.AgentTurn, error)
	AppendTranscriptEntry(ctx context.Context, entry domain.AgentTranscriptEntry) (domain.AgentTranscriptEntry, error)
	CreateAuditLog(ctx context.Context, log domain.AgentAuditLog) (domain.AgentAuditLog, error)
}

type AgentConversationLLM interface {
	Chat(ctx context.Context, request llm.ChatRequest) (llm.ChatResponse, error)
}

type AgentConversationSender interface {
	SendText(ctx context.Context, message notifier.WeChatWorkTextMessage) (notifier.WeChatWorkSendResult, error)
}

type AgentConversationService struct {
	repository AgentConversationRepository
	llmClient  AgentConversationLLM
	sender     AgentConversationSender
	now        func() time.Time
	ownerID    int64
}

type AgentConversationServiceOption func(*AgentConversationService)

func WithAgentConversationLLM(client AgentConversationLLM) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		service.llmClient = client
	}
}

func WithAgentConversationSender(sender AgentConversationSender) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		service.sender = sender
	}
}

func WithAgentConversationNow(now func() time.Time) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		if now != nil {
			service.now = now
		}
	}
}

func WithAgentConversationOwnerID(ownerID int64) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		if ownerID > 0 {
			service.ownerID = ownerID
		}
	}
}

func NewAgentConversationService(repository AgentConversationRepository, options ...AgentConversationServiceOption) *AgentConversationService {
	service := &AgentConversationService{
		repository: repository,
		now:        time.Now,
		ownerID:    defaultAgentOwnerUserID,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

type ReceiveWeChatWorkAppMessageInput struct {
	Provider          string
	ProviderMessageID string
	CorpID            string
	AgentID           string
	ExternalUserID    string
	ChatID            string
	ChatType          string
	MsgType           string
	TextContent       string
	EventType         string
	EventKey          string
	RawXML            string
	RequestID         string
	TraceID           string
}

type ReceiveWeChatWorkAppMessageResult struct {
	ExternalAccount domain.ExternalAccount
	InboundMessage  domain.AgentInboundMessage
	Session         domain.AgentSession
	Turn            domain.AgentTurn
	Reply           string
	SendResult      notifier.WeChatWorkSendResult
	Duplicate       bool
}

func (s *AgentConversationService) ReceiveWeChatWorkAppMessage(ctx context.Context, input ReceiveWeChatWorkAppMessageInput) (ReceiveWeChatWorkAppMessageResult, error) {
	startedAt := time.Now()
	if s == nil || s.repository == nil {
		metrics.AgentTurnsTotal.WithLabelValues(domain.AgentProviderWeChatWorkApp, "failed").Inc()
		metrics.AgentTurnDuration.WithLabelValues(domain.AgentProviderWeChatWorkApp, "failed").Observe(time.Since(startedAt).Seconds())
		return ReceiveWeChatWorkAppMessageResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_conversation_unavailable", "agent conversation service is unavailable", "service.agent.receive_wechat_work", true, nil)
	}
	input = normalizeReceiveWeChatWorkInput(input)
	status := "succeeded"
	ctx, span := observability.StartSpan(ctx, "service.agent.receive_wechat_work",
		attribute.String("agent.provider", input.Provider),
		attribute.String("message.type", input.MsgType),
		attribute.String("message.chat_type", input.ChatType),
		attribute.Int("message.text_chars", len([]rune(input.TextContent))),
	)
	var opErr error
	defer func() {
		span.SetAttributes(attribute.String("agent.turn.status", status))
		metrics.AgentTurnsTotal.WithLabelValues(input.Provider, status).Inc()
		metrics.AgentTurnDuration.WithLabelValues(input.Provider, status).Observe(time.Since(startedAt).Seconds())
		observability.EndSpan(span, opErr)
	}()

	if err := validateReceiveWeChatWorkInput(input); err != nil {
		status = "failed"
		opErr = err
		metrics.AgentInboundMessagesTotal.WithLabelValues(input.Provider, input.MsgType, "failed").Inc()
		return ReceiveWeChatWorkAppMessageResult{}, err
	}

	now := s.now().UTC()
	account, err := s.repository.EnsureExternalAccount(ctx, domain.ExternalAccount{
		UserID:         s.ownerID,
		Provider:       input.Provider,
		CorpID:         input.CorpID,
		AgentID:        input.AgentID,
		ExternalUserID: input.ExternalUserID,
		BindingStatus:  domain.ExternalAccountBindingStatusActive,
		VerifiedAt:     &now,
		LastSeenAt:     &now,
	})
	if err != nil {
		status = "failed"
		opErr = err
		metrics.AgentInboundMessagesTotal.WithLabelValues(input.Provider, input.MsgType, "failed").Inc()
		return ReceiveWeChatWorkAppMessageResult{}, err
	}
	if account.BindingStatus == domain.ExternalAccountBindingStatusDisabled {
		status = "failed"
		opErr = domain.NewAppError(domain.ErrorKindUnavailable, "agent_external_account_disabled", "external account binding is disabled", "service.agent.receive_wechat_work", false, nil)
		metrics.AgentInboundMessagesTotal.WithLabelValues(input.Provider, input.MsgType, "failed").Inc()
		return ReceiveWeChatWorkAppMessageResult{}, opErr
	}

	inbound, created, err := s.repository.CreateInboundMessage(ctx, domain.AgentInboundMessage{
		UserID:            account.UserID,
		ExternalAccountID: account.ID,
		Provider:          input.Provider,
		ProviderMessageID: input.ProviderMessageID,
		CorpID:            input.CorpID,
		AgentID:           input.AgentID,
		ExternalUserID:    input.ExternalUserID,
		ChatID:            input.ChatID,
		ChatType:          input.ChatType,
		MsgType:           input.MsgType,
		TextContent:       input.TextContent,
		Payload: domain.AgentJSON{
			"event_type": input.EventType,
			"event_key":  input.EventKey,
			"raw_xml":    input.RawXML,
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		Status:    domain.AgentInboundMessageStatusReceived,
	})
	if err != nil {
		status = "failed"
		opErr = err
		metrics.AgentInboundMessagesTotal.WithLabelValues(input.Provider, input.MsgType, "failed").Inc()
		return ReceiveWeChatWorkAppMessageResult{}, err
	}
	if !created {
		status = "duplicate"
		metrics.AgentInboundMessagesTotal.WithLabelValues(input.Provider, input.MsgType, "duplicate").Inc()
		return ReceiveWeChatWorkAppMessageResult{
			ExternalAccount: account,
			InboundMessage:  inbound,
			Duplicate:       true,
		}, nil
	}
	metrics.AgentInboundMessagesTotal.WithLabelValues(input.Provider, input.MsgType, "received").Inc()

	session, err := s.repository.GetOrCreateSession(ctx, domain.AgentSession{
		UserID:            account.UserID,
		ExternalAccountID: account.ID,
		Provider:          input.Provider,
		ChannelSessionKey: weChatWorkSessionKey(input),
		Status:            domain.AgentSessionStatusActive,
		Title:             "企业微信对话",
		StartedAt:         now,
		LastActiveAt:      now,
	})
	if err != nil {
		status = "failed"
		opErr = err
		return ReceiveWeChatWorkAppMessageResult{}, err
	}
	span.SetAttributes(attribute.Int64("agent.session_id", session.ID))

	turn, err := s.repository.CreateTurn(ctx, domain.AgentTurn{
		SessionID:        session.ID,
		InboundMessageID: inbound.ID,
		UserID:           account.UserID,
		Status:           domain.AgentTurnStatusRunning,
		InputText:        input.TextContent,
		StartedAt:        now,
	})
	if err != nil {
		status = "failed"
		opErr = err
		return ReceiveWeChatWorkAppMessageResult{}, err
	}
	span.SetAttributes(attribute.Int64("agent.turn_id", turn.ID))

	_, _ = s.repository.AppendTranscriptEntry(ctx, domain.AgentTranscriptEntry{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		Role:      domain.AgentTranscriptRoleUser,
		Content:   input.TextContent,
		Metadata:  domain.AgentJSON{"provider_message_id": input.ProviderMessageID},
		CreatedAt: now,
	})

	reply, modelProvider, model, err := s.generateReply(ctx, input)
	if err != nil {
		status = "failed"
		opErr = err
		return s.failTurn(ctx, account.UserID, session.ID, turn, input, err)
	}
	span.SetAttributes(
		attribute.String("llm.provider", modelProvider),
		attribute.String("llm.model", model),
		attribute.Int("agent.reply_bytes", len([]byte(reply))),
	)

	_, _ = s.repository.AppendTranscriptEntry(ctx, domain.AgentTranscriptEntry{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		Role:      domain.AgentTranscriptRoleAssistant,
		Content:   reply,
		Metadata: domain.AgentJSON{
			"model_provider": modelProvider,
			"model":          model,
		},
		CreatedAt: s.now().UTC(),
	})

	sendResult := notifier.WeChatWorkSendResult{}
	sendCount := 0
	if s.sender != nil {
		sendResult, sendCount, err = s.sendWeChatWorkReply(ctx, input.ExternalUserID, reply)
		if err != nil {
			status = "failed"
			opErr = err
			metrics.AgentReplyBytes.WithLabelValues(input.Provider, "failed").Observe(float64(len([]byte(reply))))
			metrics.AgentReplyChunksTotal.WithLabelValues(input.Provider, "failed").Add(float64(sendCount))
			return s.failTurn(ctx, account.UserID, session.ID, turn, input, err)
		}
	}
	metrics.AgentReplyBytes.WithLabelValues(input.Provider, "succeeded").Observe(float64(len([]byte(reply))))
	metrics.AgentReplyChunksTotal.WithLabelValues(input.Provider, "succeeded").Add(float64(sendCount))

	finishedAt := s.now().UTC()
	turn.Status = domain.AgentTurnStatusSucceeded
	turn.OutputText = reply
	turn.ModelProvider = modelProvider
	turn.Model = model
	turn.FinishedAt = &finishedAt
	turn, err = s.repository.UpdateTurn(ctx, turn)
	if err != nil {
		status = "failed"
		opErr = err
		return ReceiveWeChatWorkAppMessageResult{}, err
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		EventType: "wechat_work.reply_sent",
		Status:    "succeeded",
		Message:   "wechat work reply sent",
		Metadata: domain.AgentJSON{
			"provider_message_id": input.ProviderMessageID,
			"wechat_msgid":        sendResult.MessageID,
			"invalid_user":        sendResult.InvalidUser,
			"send_count":          sendCount,
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: finishedAt,
	})

	return ReceiveWeChatWorkAppMessageResult{
		ExternalAccount: account,
		InboundMessage:  inbound,
		Session:         session,
		Turn:            turn,
		Reply:           reply,
		SendResult:      sendResult,
	}, nil
}

func (s *AgentConversationService) generateReply(ctx context.Context, input ReceiveWeChatWorkAppMessageInput) (string, string, string, error) {
	ctx, span := observability.StartSpan(ctx, "service.agent.generate_reply",
		attribute.String("agent.provider", input.Provider),
		attribute.String("message.type", input.MsgType),
	)
	var replyErr error
	defer func() {
		status := "success"
		if replyErr != nil {
			status = "failed"
		}
		span.SetAttributes(attribute.String("agent.reply_generation.status", status))
		observability.EndSpan(span, replyErr)
	}()

	if input.MsgType != "text" {
		return "当前仅支持文本消息。", "", "", nil
	}
	if s.llmClient == nil {
		return "已收到：" + input.TextContent, "", "", nil
	}
	response, err := s.llmClient.Chat(ctx, llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: agentSystemPrompt},
			{Role: "user", Content: input.TextContent},
		},
		Temperature: 0.2,
		MaxTokens:   agentReplyMaxTokens,
	})
	if err != nil {
		replyErr = err
		return "", "", "", err
	}
	span.SetAttributes(
		attribute.String("llm.provider", response.Provider),
		attribute.String("llm.model", response.Model),
		attribute.Int("agent.reply_bytes", len([]byte(response.Content))),
	)
	return response.Content, response.Provider, response.Model, nil
}

func (s *AgentConversationService) sendWeChatWorkReply(ctx context.Context, toUser string, reply string) (notifier.WeChatWorkSendResult, int, error) {
	chunks := splitUTF8Bytes(reply, notifier.WeChatWorkTextByteLimit)
	ctx, span := observability.StartSpan(ctx, "service.agent.send_wechat_work_reply",
		attribute.Int("agent.reply_chunks", len(chunks)),
		attribute.Int("agent.reply_bytes", len([]byte(reply))),
	)
	var sendErr error
	defer func() {
		status := "success"
		if sendErr != nil {
			status = "failed"
		}
		span.SetAttributes(attribute.String("agent.reply_send.status", status))
		observability.EndSpan(span, sendErr)
	}()

	var result notifier.WeChatWorkSendResult
	for i, chunk := range chunks {
		var err error
		span.SetAttributes(attribute.Int("agent.reply_chunk_index", i+1))
		result, err = s.sender.SendText(ctx, notifier.WeChatWorkTextMessage{
			ToUser:  toUser,
			Content: chunk,
		})
		if err != nil {
			sendErr = err
			return result, i, err
		}
	}
	return result, len(chunks), nil
}

func (s *AgentConversationService) failTurn(ctx context.Context, userID int64, sessionID int64, turn domain.AgentTurn, input ReceiveWeChatWorkAppMessageInput, cause error) (ReceiveWeChatWorkAppMessageResult, error) {
	now := s.now().UTC()
	turn.Status = domain.AgentTurnStatusFailed
	turn.ErrorMessage = cause.Error()
	turn.FinishedAt = &now
	if turn.ID > 0 {
		_, _ = s.repository.UpdateTurn(ctx, turn)
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: sessionID,
		TurnID:    turn.ID,
		UserID:    userID,
		EventType: "wechat_work.reply_failed",
		Status:    "failed",
		Message:   cause.Error(),
		Metadata:  domain.AgentJSON{"provider_message_id": input.ProviderMessageID},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: now,
	})
	return ReceiveWeChatWorkAppMessageResult{Turn: turn}, cause
}

func splitUTF8Bytes(value string, limit int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if limit <= 0 || len(value) <= limit {
		return []string{value}
	}
	chunks := make([]string, 0, len(value)/limit+1)
	var builder strings.Builder
	currentBytes := 0
	for _, r := range value {
		part := string(r)
		partBytes := len(part)
		if currentBytes > 0 && currentBytes+partBytes > limit {
			chunks = append(chunks, strings.TrimSpace(builder.String()))
			builder.Reset()
			currentBytes = 0
		}
		builder.WriteString(part)
		currentBytes += partBytes
	}
	if tail := strings.TrimSpace(builder.String()); tail != "" {
		chunks = append(chunks, tail)
	}
	return chunks
}

func normalizeReceiveWeChatWorkInput(input ReceiveWeChatWorkAppMessageInput) ReceiveWeChatWorkAppMessageInput {
	input.Provider = strings.TrimSpace(input.Provider)
	if input.Provider == "" {
		input.Provider = domain.AgentProviderWeChatWorkApp
	}
	input.ProviderMessageID = strings.TrimSpace(input.ProviderMessageID)
	input.CorpID = strings.TrimSpace(input.CorpID)
	input.AgentID = strings.TrimSpace(input.AgentID)
	input.ExternalUserID = strings.TrimSpace(input.ExternalUserID)
	input.ChatID = strings.TrimSpace(input.ChatID)
	if input.ChatID == "" {
		input.ChatID = input.ExternalUserID
	}
	input.ChatType = strings.TrimSpace(input.ChatType)
	if input.ChatType == "" {
		input.ChatType = "direct"
	}
	input.MsgType = strings.TrimSpace(input.MsgType)
	input.TextContent = strings.TrimSpace(input.TextContent)
	input.EventType = strings.TrimSpace(input.EventType)
	input.EventKey = strings.TrimSpace(input.EventKey)
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.TraceID = strings.TrimSpace(input.TraceID)
	return input
}

func validateReceiveWeChatWorkInput(input ReceiveWeChatWorkAppMessageInput) error {
	if input.ProviderMessageID == "" {
		return fmt.Errorf("%w: provider message id is required", domain.ErrInvalidInput)
	}
	if input.CorpID == "" {
		return fmt.Errorf("%w: corp id is required", domain.ErrInvalidInput)
	}
	if input.AgentID == "" {
		return fmt.Errorf("%w: agent id is required", domain.ErrInvalidInput)
	}
	if input.ExternalUserID == "" {
		return fmt.Errorf("%w: external user id is required", domain.ErrInvalidInput)
	}
	if input.MsgType == "" {
		return fmt.Errorf("%w: message type is required", domain.ErrInvalidInput)
	}
	if input.MsgType == "text" && input.TextContent == "" {
		return fmt.Errorf("%w: text content is required", domain.ErrInvalidInput)
	}
	return nil
}

func weChatWorkSessionKey(input ReceiveWeChatWorkAppMessageInput) string {
	return input.CorpID + ":" + input.AgentID + ":" + input.ExternalUserID
}
