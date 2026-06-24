package service

import (
	"context"
	"fmt"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"messagefeed/internal/metrics"
	"messagefeed/internal/notifier"
	"messagefeed/internal/observability"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const (
	defaultAgentOwnerUserID = int64(1)
	agentSystemPrompt       = "你是 messageFeed 的企业微信聊天助手。回复只能围绕本项目内的信息聚合、订阅源、阅读和设置。使用普通微信聊天文本，不使用 Markdown、表格、标题、加粗、代码块、列表符号、星号或反引号。默认回复 2 到 5 句，最多约 300 字；用户明确要求详细说明时，也使用适合手机阅读的短段落，不要写成文档。"
	agentReplyMaxTokens     = 768
)

type AgentConversationRepository interface {
	CreateInboundMessage(ctx context.Context, message domain.AgentInboundMessage) (domain.AgentInboundMessage, bool, error)
	UpdateInboundMessageStatus(ctx context.Context, userID int64, id int64, status domain.AgentInboundMessageStatus, now time.Time) (domain.AgentInboundMessage, error)
	GetOrCreateSession(ctx context.Context, session domain.AgentSession) (domain.AgentSession, error)
	CreateTurn(ctx context.Context, turn domain.AgentTurn) (domain.AgentTurn, error)
	UpdateTurn(ctx context.Context, turn domain.AgentTurn) (domain.AgentTurn, error)
	AppendTranscriptEntry(ctx context.Context, entry domain.AgentTranscriptEntry) (domain.AgentTranscriptEntry, error)
	CreateAuditLog(ctx context.Context, log domain.AgentAuditLog) (domain.AgentAuditLog, error)
}

type AgentExternalAccountResolver interface {
	ResolveExternalAccount(ctx context.Context, provider string, corpID string, agentID string, externalUserID string) (domain.ExternalAccount, error)
}

type AgentUserContextProvider interface {
	BuildAgentUserContext(ctx context.Context, userID int64) (UserContextResult, error)
}

type AgentConversationLLM interface {
	Chat(ctx context.Context, request llm.ChatRequest) (llm.ChatResponse, error)
}

type AgentConversationSender interface {
	SendText(ctx context.Context, message notifier.WeChatWorkTextMessage) (notifier.WeChatWorkSendResult, error)
}

type AgentRecentItemsProvider interface {
	ListItems(ctx context.Context, input ListItemsInput) (ListItemsResult, error)
}

type AgentSourceProvider interface {
	ListSources(ctx context.Context, userID int64) ([]domain.Source, error)
}

type AgentAIFeedPublisher interface {
	PublishEntry(ctx context.Context, input PublishAIFeedEntryInput) (PublishAIFeedEntryResult, error)
}

type AgentConversationService struct {
	repository         AgentConversationRepository
	llmClient          AgentConversationLLM
	sender             AgentConversationSender
	resolver           AgentExternalAccountResolver
	userCtx            AgentUserContextProvider
	recentItems        AgentRecentItemsProvider
	sourceProvider     AgentSourceProvider
	aiFeedPublisher    AgentAIFeedPublisher
	capabilityRegistry *agent.CapabilityRegistry
	policyEngine       *agent.PolicyEngine
	now                func() time.Time
	ownerID            int64
	processInline      bool
	processTimeout     time.Duration
	lockMu             sync.Mutex
	sessionLocks       map[int64]*sync.Mutex
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

func WithAgentConversationExternalAccountResolver(resolver AgentExternalAccountResolver) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		service.resolver = resolver
	}
}

func WithAgentConversationUserContextProvider(provider AgentUserContextProvider) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		service.userCtx = provider
	}
}

func WithAgentConversationRecentItemsProvider(provider AgentRecentItemsProvider) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		service.recentItems = provider
	}
}

func WithAgentConversationSourceProvider(provider AgentSourceProvider) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		service.sourceProvider = provider
	}
}

func WithAgentConversationAIFeedPublisher(publisher AgentAIFeedPublisher) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		service.aiFeedPublisher = publisher
	}
}

func WithAgentConversationInlineProcessing(enabled bool) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		service.processInline = enabled
	}
}

func WithAgentConversationProcessTimeout(timeout time.Duration) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		if timeout > 0 {
			service.processTimeout = timeout
		}
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
		repository:         repository,
		capabilityRegistry: agent.NewP0CapabilityRegistry(),
		policyEngine:       agent.NewPolicyEngine(),
		now:                time.Now,
		ownerID:            defaultAgentOwnerUserID,
		processTimeout:     30 * time.Second,
		sessionLocks:       map[int64]*sync.Mutex{},
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
	BindingRequired bool
	ProcessingAsync bool
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
	if s.resolver == nil {
		status = "failed"
		opErr = domain.NewAppError(domain.ErrorKindUnavailable, "agent_identity_resolver_unavailable", "external account resolver is unavailable", "service.agent.receive_wechat_work", true, nil)
		metrics.AgentInboundMessagesTotal.WithLabelValues(input.Provider, input.MsgType, "failed").Inc()
		return ReceiveWeChatWorkAppMessageResult{}, opErr
	}
	account, err := s.resolver.ResolveExternalAccount(ctx, input.Provider, input.CorpID, input.AgentID, input.ExternalUserID)
	if err != nil {
		if domain.ClassifyError(err) == domain.ErrorKindNotFound {
			status = "binding_required"
			reply := "请先登录 messageFeed，在设置页完成企业微信绑定后再发送消息。"
			sendResult := notifier.WeChatWorkSendResult{}
			sendCount := 0
			if s.sender != nil {
				sendResult, sendCount, _ = s.sendWeChatWorkReply(ctx, input.ExternalUserID, reply)
				metrics.AgentReplyChunksTotal.WithLabelValues(input.Provider, "binding_required").Add(float64(sendCount))
			}
			metrics.AgentInboundMessagesTotal.WithLabelValues(input.Provider, input.MsgType, "binding_required").Inc()
			return ReceiveWeChatWorkAppMessageResult{
				Reply:           reply,
				SendResult:      sendResult,
				BindingRequired: true,
			}, nil
		}
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

	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		EventType: "wechat_work.inbound_queued",
		Status:    "queued",
		Message:   "wechat work inbound message queued for turn processing",
		Metadata: domain.AgentJSON{
			"provider_message_id": input.ProviderMessageID,
			"msg_type":            input.MsgType,
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: now,
	})

	result := ReceiveWeChatWorkAppMessageResult{
		ExternalAccount: account,
		InboundMessage:  inbound,
		Session:         session,
		Turn:            turn,
		ProcessingAsync: !s.processInline,
	}
	if s.processInline {
		processed, err := s.processTurn(context.WithoutCancel(ctx), account, inbound, session, turn, input)
		if err != nil {
			status = "failed"
			opErr = err
			return processed, err
		}
		return processed, nil
	}

	processCtx := context.WithoutCancel(ctx)
	go func() {
		ctx, cancel := context.WithTimeout(processCtx, s.processTimeout)
		defer cancel()
		_, _ = s.processTurn(ctx, account, inbound, session, turn, input)
	}()

	return result, nil
}

func (s *AgentConversationService) processTurn(
	ctx context.Context,
	account domain.ExternalAccount,
	inbound domain.AgentInboundMessage,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
) (ReceiveWeChatWorkAppMessageResult, error) {
	lock := s.sessionLock(session.ID)
	lock.Lock()
	defer lock.Unlock()

	ctx, span := observability.StartSpan(ctx, "service.agent.process_turn",
		attribute.Int64("agent.session_id", session.ID),
		attribute.Int64("agent.turn_id", turn.ID),
		attribute.Int64("auth.user_id", account.UserID),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	reply, modelProvider, model, observations, err := s.generateReply(ctx, account.UserID, input)
	if err != nil {
		opErr = err
		_, _ = s.repository.UpdateInboundMessageStatus(ctx, account.UserID, inbound.ID, domain.AgentInboundMessageStatusFailed, s.now().UTC())
		return s.failTurn(ctx, account.UserID, session.ID, turn, input, err)
	}
	span.SetAttributes(
		attribute.String("llm.provider", modelProvider),
		attribute.String("llm.model", model),
		attribute.Int("agent.reply_bytes", len([]byte(reply))),
		attribute.Int("agent.observation_count", len(observations)),
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
			"observations":   observationMetadata(observations),
		},
		CreatedAt: s.now().UTC(),
	})

	sendResult := notifier.WeChatWorkSendResult{}
	sendCount := 0
	if s.sender != nil {
		sendResult, sendCount, err = s.sendWeChatWorkReply(ctx, input.ExternalUserID, reply)
		if err != nil {
			opErr = err
			metrics.AgentReplyBytes.WithLabelValues(input.Provider, "failed").Observe(float64(len([]byte(reply))))
			metrics.AgentReplyChunksTotal.WithLabelValues(input.Provider, "failed").Add(float64(sendCount))
			_, _ = s.repository.UpdateInboundMessageStatus(ctx, account.UserID, inbound.ID, domain.AgentInboundMessageStatusFailed, s.now().UTC())
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
		opErr = err
		return ReceiveWeChatWorkAppMessageResult{}, err
	}
	inbound, _ = s.repository.UpdateInboundMessageStatus(ctx, account.UserID, inbound.ID, domain.AgentInboundMessageStatusSucceeded, finishedAt)
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
			"observations":        observationMetadata(observations),
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: finishedAt,
	})
	s.publishTurnReport(ctx, account.UserID, input, reply, observations, finishedAt)

	return ReceiveWeChatWorkAppMessageResult{
		ExternalAccount: account,
		InboundMessage:  inbound,
		Session:         session,
		Turn:            turn,
		Reply:           reply,
		SendResult:      sendResult,
	}, nil
}

func (s *AgentConversationService) sessionLock(sessionID int64) *sync.Mutex {
	s.lockMu.Lock()
	defer s.lockMu.Unlock()
	if s.sessionLocks == nil {
		s.sessionLocks = map[int64]*sync.Mutex{}
	}
	lock := s.sessionLocks[sessionID]
	if lock == nil {
		lock = &sync.Mutex{}
		s.sessionLocks[sessionID] = lock
	}
	return lock
}

type agentCapabilityObservation struct {
	Capability string
	Decision   string
	Status     string
	Summary    string
}

func (s *AgentConversationService) generateReply(ctx context.Context, userID int64, input ReceiveWeChatWorkAppMessageInput) (string, string, string, []agentCapabilityObservation, error) {
	ctx, span := observability.StartSpan(ctx, "service.agent.generate_reply",
		attribute.String("agent.provider", input.Provider),
		attribute.String("message.type", input.MsgType),
		attribute.Int64("auth.user_id", userID),
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
		return "当前仅支持文本消息。", "", "", nil, nil
	}
	if s.llmClient == nil {
		return "已收到：" + input.TextContent, "", "", nil, nil
	}
	systemPrompt := agentSystemPrompt
	if s.userCtx != nil {
		userContext, err := s.userCtx.BuildAgentUserContext(ctx, userID)
		if err != nil {
			replyErr = err
			return "", "", "", nil, err
		}
		if strings.TrimSpace(userContext.Prompt.PlainText) != "" {
			systemPrompt += "\n\n用户上下文：\n" + userContext.Prompt.PlainText
		}
	}
	toolContext, observations := s.buildP0CapabilityContext(ctx, userID, input)
	if strings.TrimSpace(toolContext) != "" {
		systemPrompt += "\n\n只读工具结果：\n" + toolContext
	}
	systemPrompt += "\n\n能力边界：P0 仅允许只读查询、文本总结、写入 transcript 和审计。新增订阅、停用来源、通知配置、画像写入、金融告警或其他状态变更必须拒绝直接执行，并说明需要后续确认流程。"
	response, err := s.llmClient.Chat(ctx, llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: input.TextContent},
		},
		Temperature: 0.2,
		MaxTokens:   agentReplyMaxTokens,
	})
	if err != nil {
		replyErr = err
		return "", "", "", observations, err
	}
	span.SetAttributes(
		attribute.String("llm.provider", response.Provider),
		attribute.String("llm.model", response.Model),
		attribute.Int("agent.reply_bytes", len([]byte(response.Content))),
	)
	return response.Content, response.Provider, response.Model, observations, nil
}

func (s *AgentConversationService) buildP0CapabilityContext(ctx context.Context, userID int64, input ReceiveWeChatWorkAppMessageInput) (string, []agentCapabilityObservation) {
	observations := make([]agentCapabilityObservation, 0, 2)
	var builder strings.Builder
	recentSummary, recentObservation := s.executeRecentItemsCapability(ctx, userID)
	if recentObservation.Capability != "" {
		observations = append(observations, recentObservation)
	}
	if recentSummary != "" {
		builder.WriteString(recentSummary)
	}
	sourceSummary, sourceObservation := s.executeSourceLatestItemsCapability(ctx, userID, input.TextContent)
	if sourceObservation.Capability != "" {
		observations = append(observations, sourceObservation)
	}
	if sourceSummary != "" {
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(sourceSummary)
	}
	return builder.String(), observations
}

func (s *AgentConversationService) executeRecentItemsCapability(ctx context.Context, userID int64) (string, agentCapabilityObservation) {
	capability, ok := s.capabilityRegistry.Get("feed.query_recent_items")
	if !ok {
		return "", agentCapabilityObservation{Capability: "feed.query_recent_items", Decision: string(agent.PolicyDecisionForbidden), Status: "skipped", Summary: "capability is not registered"}
	}
	decision := s.policyEngine.Decide(ctx, agent.PolicyInput{Capability: capability, UserID: userID})
	observation := agentCapabilityObservation{Capability: capability.Key, Decision: string(decision.Decision)}
	if decision.Decision != agent.PolicyDecisionAllow {
		observation.Status = "blocked"
		observation.Summary = decision.Reason
		return "", observation
	}
	if s.recentItems == nil {
		observation.Status = "skipped"
		observation.Summary = "recent items provider is unavailable"
		return "", observation
	}
	result, err := s.recentItems.ListItems(ctx, ListItemsInput{
		UserID:        userID,
		Limit:         5,
		Offset:        0,
		IncludeHidden: false,
		Order:         string(domain.ItemSortOrderDesc),
	})
	if err != nil {
		observation.Status = "failed"
		observation.Summary = err.Error()
		return "", observation
	}
	observation.Status = "succeeded"
	observation.Summary = fmt.Sprintf("loaded %d recent items", len(result.Items))
	if len(result.Items) == 0 {
		return "最近条目：暂无可用条目。", observation
	}
	var builder strings.Builder
	builder.WriteString("最近条目：")
	for i, item := range result.Items {
		builder.WriteString("\n")
		builder.WriteString(strconv.Itoa(i + 1))
		builder.WriteString(". ")
		builder.WriteString(item.Title)
		if item.SourceName != "" {
			builder.WriteString("（")
			builder.WriteString(item.SourceName)
			builder.WriteString("）")
		}
		if item.Summary != "" {
			builder.WriteString("：")
			builder.WriteString(truncateError(item.Summary, 160))
		}
	}
	return builder.String(), observation
}

func (s *AgentConversationService) executeSourceLatestItemsCapability(ctx context.Context, userID int64, text string) (string, agentCapabilityObservation) {
	capability, ok := s.capabilityRegistry.Get("source.query_latest_items")
	if !ok {
		return "", agentCapabilityObservation{Capability: "source.query_latest_items", Decision: string(agent.PolicyDecisionForbidden), Status: "skipped", Summary: "capability is not registered"}
	}
	decision := s.policyEngine.Decide(ctx, agent.PolicyInput{Capability: capability, UserID: userID})
	observation := agentCapabilityObservation{Capability: capability.Key, Decision: string(decision.Decision)}
	if decision.Decision != agent.PolicyDecisionAllow {
		observation.Status = "blocked"
		observation.Summary = decision.Reason
		return "", observation
	}
	if s.sourceProvider == nil || s.recentItems == nil {
		observation.Status = "skipped"
		observation.Summary = "source or item provider is unavailable"
		return "", observation
	}
	source, found, err := s.matchSourceByText(ctx, userID, text)
	if err != nil {
		observation.Status = "failed"
		observation.Summary = err.Error()
		return "", observation
	}
	if !found {
		observation.Status = "skipped"
		observation.Summary = "no source name matched user input"
		return "", observation
	}
	result, err := s.recentItems.ListItems(ctx, ListItemsInput{
		UserID:        userID,
		SourceID:      source.ID,
		Limit:         3,
		Offset:        0,
		IncludeHidden: false,
		Order:         string(domain.ItemSortOrderDesc),
	})
	if err != nil {
		observation.Status = "failed"
		observation.Summary = err.Error()
		return "", observation
	}
	observation.Status = "succeeded"
	observation.Summary = fmt.Sprintf("loaded %d latest items for source %s", len(result.Items), source.Name)
	var builder strings.Builder
	builder.WriteString("匹配来源 ")
	builder.WriteString(source.Name)
	builder.WriteString(" 的最新条目：")
	if len(result.Items) == 0 {
		builder.WriteString("暂无可用条目。")
		return builder.String(), observation
	}
	for i, item := range result.Items {
		builder.WriteString("\n")
		builder.WriteString(strconv.Itoa(i + 1))
		builder.WriteString(". ")
		builder.WriteString(item.Title)
	}
	return builder.String(), observation
}

func (s *AgentConversationService) matchSourceByText(ctx context.Context, userID int64, text string) (domain.Source, bool, error) {
	sources, err := s.sourceProvider.ListSources(ctx, userID)
	if err != nil {
		return domain.Source{}, false, err
	}
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return domain.Source{}, false, nil
	}
	for _, source := range sources {
		name := strings.ToLower(strings.TrimSpace(source.Name))
		if name != "" && strings.Contains(text, name) {
			return source, true, nil
		}
	}
	return domain.Source{}, false, nil
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

func (s *AgentConversationService) publishTurnReport(ctx context.Context, userID int64, input ReceiveWeChatWorkAppMessageInput, reply string, observations []agentCapabilityObservation, now time.Time) {
	if s == nil || s.aiFeedPublisher == nil || userID < 1 {
		return
	}
	title := "企业微信对话处理报告"
	summary := "已处理一条企业微信文本消息并发送回复。"
	content := buildTurnReportContent(input, reply, observations)
	_, _ = s.aiFeedPublisher.PublishEntry(ctx, PublishAIFeedEntryInput{
		UserID:      userID,
		Kind:        domain.AIFeedEntryKindAgentOperationLog,
		Title:       title,
		Summary:     summary,
		Content:     content,
		DedupeKey:   "wechat-work-turn-" + input.ProviderMessageID,
		PublishedAt: now,
	})
}

func buildTurnReportContent(input ReceiveWeChatWorkAppMessageInput, reply string, observations []agentCapabilityObservation) string {
	var builder strings.Builder
	builder.WriteString("输入：")
	builder.WriteString(input.TextContent)
	builder.WriteString("\n回复：")
	builder.WriteString(reply)
	if len(observations) > 0 {
		builder.WriteString("\n工具调用：")
		for _, observation := range observations {
			builder.WriteString("\n- ")
			builder.WriteString(observation.Capability)
			builder.WriteString(" ")
			builder.WriteString(observation.Decision)
			builder.WriteString(" ")
			builder.WriteString(observation.Status)
			if observation.Summary != "" {
				builder.WriteString("：")
				builder.WriteString(observation.Summary)
			}
		}
	}
	return builder.String()
}

func observationMetadata(observations []agentCapabilityObservation) []domain.AgentJSON {
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
