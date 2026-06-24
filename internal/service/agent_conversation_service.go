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
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const (
	defaultAgentOwnerUserID = int64(1)
	agentReplyMaxTokens     = 768
)

type AgentConversationRepository interface {
	CreateInboundMessage(ctx context.Context, message domain.AgentInboundMessage) (domain.AgentInboundMessage, bool, error)
	UpdateInboundMessageStatus(ctx context.Context, userID int64, id int64, status domain.AgentInboundMessageStatus, now time.Time) (domain.AgentInboundMessage, error)
	GetOrCreateSession(ctx context.Context, session domain.AgentSession) (domain.AgentSession, error)
	GetAgentSession(ctx context.Context, userID int64, sessionID int64) (domain.AgentSession, error)
	TouchAgentSession(ctx context.Context, userID int64, sessionID int64, now time.Time) error
	CreateTurn(ctx context.Context, turn domain.AgentTurn) (domain.AgentTurn, error)
	UpdateTurn(ctx context.Context, turn domain.AgentTurn) (domain.AgentTurn, error)
	AppendTranscriptEntry(ctx context.Context, entry domain.AgentTranscriptEntry) (domain.AgentTranscriptEntry, error)
	ListRecentTranscriptEntries(ctx context.Context, options domain.AgentTranscriptListOptions) ([]domain.AgentTranscriptEntry, error)
	QueryTranscriptEntries(ctx context.Context, options domain.AgentTranscriptQueryOptions) ([]domain.AgentTranscriptEntry, error)
	CreateRecallEvent(ctx context.Context, event domain.AgentRecallEvent) (domain.AgentRecallEvent, error)
	CreateAuditLog(ctx context.Context, log domain.AgentAuditLog) (domain.AgentAuditLog, error)
	CreateAgentRun(ctx context.Context, run domain.AgentRun) (domain.AgentRun, error)
	UpdateAgentRun(ctx context.Context, run domain.AgentRun) (domain.AgentRun, error)
	CreateAgentRunContextTrace(ctx context.Context, trace domain.AgentRunContextTrace) (domain.AgentRunContextTrace, error)
	CreateAgentObservation(ctx context.Context, observation domain.AgentObservation) (domain.AgentObservation, error)
	CreateAgentArtifact(ctx context.Context, artifact domain.AgentArtifact) (domain.AgentArtifact, error)
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

type AgentNotificationJobStore interface {
	CreateJob(ctx context.Context, job domain.NotificationJob) (domain.NotificationJob, error)
}

type AgentConversationService struct {
	repository         AgentConversationRepository
	llmClient          AgentConversationLLM
	sender             AgentConversationSender
	resolver           AgentExternalAccountResolver
	userCtx            AgentUserContextProvider
	recentItems        AgentRecentItemsProvider
	sourceProvider     AgentSourceProvider
	notificationJobs   AgentNotificationJobStore
	turnRunner         *agent.TurnRunner
	runManager         *agent.RunManager
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

func WithAgentConversationNotificationJobStore(store AgentNotificationJobStore) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		service.notificationJobs = store
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
	service.runManager = agent.NewRunManager(agent.RunManagerOptions{Store: repository, Now: service.now})
	service.rebuildTurnRunner()
	return service
}

func (s *AgentConversationService) rebuildTurnRunner() {
	if s == nil {
		return
	}
	contextBuilder := agent.NewDefaultContextBuilder(agent.DefaultContextBuilderOptions{
		Registry: s.capabilityRegistry,
		Policy:   s.policyEngine,
		UserContextProvider: agentUserContextBlockProvider{
			provider: s.userCtx,
			now:      s.now,
		},
		ConversationMemory: agentConversationMemoryProvider{
			repository: s.repository,
			now:        s.now,
		},
		Executor: s.agentCapabilityExecutor(),
		Now:      s.now,
	})
	s.turnRunner = agent.NewTurnRunner(agent.TurnRunnerOptions{
		Store:          s.repository,
		AuditLogger:    s,
		ContextBuilder: contextBuilder,
		ToolExecutor:   s.agentCapabilityExecutor(),
		ToolRegistry:   s.capabilityRegistry,
		ToolKeys:       []string{"conversation.query_history", "agent.schedule_message", "web.search", "web.fetch_page", "web.extract_page"},
		LLMClient:      s.llmClient,
		Now:            s.now,
		SystemPrompt:   llm.MessageFeedAgentSystemPrompt,
		MaxTokens:      agentReplyMaxTokens,
		Temperature:    0.2,
	})
}

func (s *AgentConversationService) agentCapabilityExecutor() agentRunRecordingExecutor {
	return agentRunRecordingExecutor{
		base: agentP0CapabilityExecutor{
			repository:       s.repository,
			recentItems:      s.recentItems,
			sourceProvider:   s.sourceProvider,
			notificationJobs: s.notificationJobs,
			now:              s.now,
		},
		runManager: s.runManager,
		now:        s.now,
	}
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

	session, err := s.resolveConversationSession(ctx, account, input, now)
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

func (s *AgentConversationService) resolveConversationSession(ctx context.Context, account domain.ExternalAccount, input ReceiveWeChatWorkAppMessageInput, now time.Time) (domain.AgentSession, error) {
	if account.ActiveAgentSessionID > 0 {
		session, err := s.repository.GetAgentSession(ctx, account.UserID, account.ActiveAgentSessionID)
		if err == nil && session.ExternalAccountID == account.ID && session.Status == domain.AgentSessionStatusActive {
			_ = s.repository.TouchAgentSession(ctx, account.UserID, session.ID, now)
			session.LastActiveAt = now
			return session, nil
		}
		if err != nil && domain.ClassifyError(err) != domain.ErrorKindNotFound {
			return domain.AgentSession{}, err
		}
	}
	return s.repository.GetOrCreateSession(ctx, domain.AgentSession{
		UserID:            account.UserID,
		ExternalAccountID: account.ID,
		Provider:          input.Provider,
		ChannelSessionKey: weChatWorkSessionKey(input),
		Status:            domain.AgentSessionStatusActive,
		Title:             "企业微信对话",
		StartedAt:         now,
		LastActiveAt:      now,
	})
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

	if s.turnRunner == nil {
		opErr = domain.NewAppError(domain.ErrorKindUnavailable, "agent_runner_unavailable", "agent turn runner is unavailable", "service.agent.process_turn", true, nil)
		return ReceiveWeChatWorkAppMessageResult{Turn: turn}, opErr
	}
	controllerRun, err := s.createControllerRun(ctx, account, inbound, session, turn, input)
	if err != nil {
		opErr = err
		return ReceiveWeChatWorkAppMessageResult{Turn: turn}, err
	}
	runResult, err := s.turnRunner.Run(ctx, agent.TurnRunInput{
		UserID:          account.UserID,
		Session:         session,
		Turn:            turn,
		InboundMessage:  inbound,
		ControllerRunID: controllerRun.ID,
		MessageType:     input.MsgType,
		MessageText:     input.TextContent,
		RequestID:       input.RequestID,
		TraceID:         input.TraceID,
	})
	if err != nil {
		s.recordControllerTrace(ctx, controllerRun, runResult, "controller_error")
		_, _ = s.runManager.FailRun(ctx, controllerRun, err)
		opErr = err
		return s.sendTurnFailureFeedback(ctx, account, inbound, session, turn, runResult.Turn, input, err), nil
	}
	s.recordControllerTrace(ctx, controllerRun, runResult, "controller_output")
	_, _ = s.runManager.CompleteRun(ctx, controllerRun, "turn_output")
	reply := runResult.Reply
	observations := runResult.Context.Observations
	turn = runResult.Turn
	span.SetAttributes(
		attribute.String("llm.provider", runResult.ModelProvider),
		attribute.String("llm.model", runResult.Model),
		attribute.Int("agent.reply_bytes", len([]byte(reply))),
		attribute.Int("agent.observation_count", len(observations)),
	)

	sendResult := notifier.WeChatWorkSendResult{}
	sendCount := 0
	if s.sender != nil {
		sendResult, sendCount, err = s.sendWeChatWorkReply(ctx, input.ExternalUserID, reply)
		if err != nil {
			opErr = err
			metrics.AgentReplyBytes.WithLabelValues(input.Provider, "failed").Observe(float64(len([]byte(reply))))
			metrics.AgentReplyChunksTotal.WithLabelValues(input.Provider, "failed").Add(float64(sendCount))
			_, _ = s.repository.UpdateInboundMessageStatus(ctx, account.UserID, inbound.ID, domain.AgentInboundMessageStatusFailed, s.now().UTC())
			_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
				SessionID: session.ID,
				TurnID:    turn.ID,
				UserID:    account.UserID,
				EventType: "wechat_work.reply_failed",
				Status:    "failed",
				Message:   err.Error(),
				Metadata:  domain.AgentJSON{"provider_message_id": input.ProviderMessageID},
				RequestID: input.RequestID,
				TraceID:   input.TraceID,
				CreatedAt: s.now().UTC(),
			})
			return ReceiveWeChatWorkAppMessageResult{Turn: turn}, err
		}
	}
	metrics.AgentReplyBytes.WithLabelValues(input.Provider, "succeeded").Observe(float64(len([]byte(reply))))
	metrics.AgentReplyChunksTotal.WithLabelValues(input.Provider, "succeeded").Add(float64(sendCount))

	finishedAt := s.now().UTC()
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
			"observations":        agent.ObservationMetadata(observations),
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

func (s *AgentConversationService) createControllerRun(
	ctx context.Context,
	account domain.ExternalAccount,
	inbound domain.AgentInboundMessage,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
) (domain.AgentRun, error) {
	if s.runManager == nil {
		return domain.AgentRun{}, nil
	}
	run, err := s.runManager.CreateControllerRun(ctx, agent.CreateRunInput{
		SessionID: session.ID,
		TurnID:    turn.ID,
		TaskPacket: domain.AgentJSON{
			"provider":            input.Provider,
			"provider_message_id": input.ProviderMessageID,
			"inbound_message_id":  inbound.ID,
			"user_id":             account.UserID,
			"message_type":        input.MsgType,
			"message":             safeSummary(input.TextContent, 1000),
		},
		CapabilityScope: []string{"feed.query_recent_items", "source.query_latest_items", "content.summarize_text"},
		ModelKey:        "controller:" + llmModelKey(s.llmClient),
		ContextBudget: domain.AgentJSON{
			"max_reply_tokens": agentReplyMaxTokens,
			"mode":             "p0_controller_single_executor",
		},
		TraceID: input.TraceID,
	})
	if err != nil {
		return domain.AgentRun{}, err
	}
	if run.ID > 0 {
		_, _ = s.runManager.SaveContextTrace(ctx, agent.SaveContextTraceInput{
			RunID:     run.ID,
			TraceKind: "controller_input",
			ModelKey:  run.ModelKey,
			Content: domain.AgentJSON{
				"task_packet":      run.TaskPacket,
				"capability_scope": run.CapabilityScope,
				"context_budget":   run.ContextBudget,
			},
			RedactionStatus: "redacted",
			TokenEstimate:   estimateTokenCount(input.TextContent),
		})
	}
	return run, nil
}

func (s *AgentConversationService) recordControllerTrace(ctx context.Context, run domain.AgentRun, result agent.TurnRunResult, traceKind string) {
	if s.runManager == nil || run.ID == 0 {
		return
	}
	observations := agent.ObservationMetadata(result.Context.Observations)
	content := domain.AgentJSON{
		"reply":             safeSummary(result.Reply, 2000),
		"model_provider":    result.ModelProvider,
		"model":             result.Model,
		"context_blocks":    contextBlockMetadata(result.Context.Blocks),
		"context_messages":  contextMessageMetadata(result.Context.Messages),
		"observations":      observations,
		"history_need_hint": string(result.Context.HistoryNeedHint),
		"redaction_policy":  "secret, token, webhook url and database dsn are excluded from trace content",
	}
	_, _ = s.runManager.SaveContextTrace(ctx, agent.SaveContextTraceInput{
		RunID:           run.ID,
		TraceKind:       traceKind,
		ModelKey:        run.ModelKey,
		Content:         content,
		RedactionStatus: "redacted",
		TokenEstimate:   estimateTokenCount(result.Reply),
	})
}

func contextBlockMetadata(blocks []agent.ContextBlock) []domain.AgentJSON {
	output := make([]domain.AgentJSON, 0, len(blocks))
	for _, block := range blocks {
		output = append(output, domain.AgentJSON{
			"name":           block.Name,
			"capability_key": block.CapabilityKey,
			"content":        safeSummary(block.Content, 2000),
			"item_count":     block.ItemCount,
			"truncated":      block.Truncated,
			"trust_level":    block.TrustLevel,
			"generated_at":   formatOptionalTime(&block.GeneratedAt),
		})
	}
	return output
}

func contextMessageMetadata(messages []agent.ContextMessage) []domain.AgentJSON {
	output := make([]domain.AgentJSON, 0, len(messages))
	for _, message := range messages {
		output = append(output, domain.AgentJSON{
			"role":                string(message.Role),
			"content":             safeSummary(message.Content, 1000),
			"transcript_entry_id": message.TranscriptEntryID,
			"turn_id":             message.TurnID,
			"created_at":          formatOptionalTime(&message.CreatedAt),
		})
	}
	return output
}

func llmModelKey(client AgentConversationLLM) string {
	if client == nil {
		return "fallback"
	}
	return "configured"
}

func (s *AgentConversationService) sendTurnFailureFeedback(
	ctx context.Context,
	account domain.ExternalAccount,
	inbound domain.AgentInboundMessage,
	session domain.AgentSession,
	originalTurn domain.AgentTurn,
	failedTurn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
	cause error,
) ReceiveWeChatWorkAppMessageResult {
	if failedTurn.ID == 0 {
		failedTurn = originalTurn
	}
	reply := agentTurnFailureFeedback(input.TextContent)
	sendResult := notifier.WeChatWorkSendResult{}
	sendCount := 0
	sendStatus := "skipped"
	if s.sender != nil {
		var sendErr error
		sendResult, sendCount, sendErr = s.sendWeChatWorkReply(ctx, input.ExternalUserID, reply)
		if sendErr != nil {
			sendStatus = "failed"
		} else {
			sendStatus = "succeeded"
		}
	}
	now := s.now().UTC()
	_, _ = s.repository.AppendTranscriptEntry(ctx, domain.AgentTranscriptEntry{
		SessionID: session.ID,
		TurnID:    failedTurn.ID,
		UserID:    account.UserID,
		Role:      domain.AgentTranscriptRoleAssistant,
		Content:   reply,
		Metadata: domain.AgentJSON{
			"fallback":       true,
			"failure_reason": truncateError(cause.Error(), 500),
			"send_status":    sendStatus,
		},
		CreatedAt: now,
	})
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    failedTurn.ID,
		UserID:    account.UserID,
		EventType: "agent.turn_failure_feedback",
		Status:    sendStatus,
		Message:   "agent turn failed and fallback feedback was generated",
		Metadata: domain.AgentJSON{
			"provider_message_id": input.ProviderMessageID,
			"send_count":          sendCount,
			"failure_reason":      truncateError(cause.Error(), 500),
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: now,
	})
	return ReceiveWeChatWorkAppMessageResult{
		ExternalAccount: account,
		InboundMessage:  inbound,
		Session:         session,
		Turn:            failedTurn,
		Reply:           reply,
		SendResult:      sendResult,
	}
}

func agentTurnFailureFeedback(text string) string {
	normalized := strings.TrimSpace(text)
	if strings.Contains(normalized, "提醒") || strings.Contains(normalized, "定时") {
		return "没有设置成功，后台创建提醒时出错。请稍后再试，或重新发送提醒时间和内容。"
	}
	return "这次处理没有成功，请稍后再试。"
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

func (s *AgentConversationService) Record(ctx context.Context, event agent.AuditEvent) error {
	if s == nil || s.repository == nil {
		return nil
	}
	_, err := s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: event.SessionID,
		TurnID:    event.TurnID,
		UserID:    event.UserID,
		EventType: event.EventType,
		Status:    event.Status,
		Message:   event.Message,
		Metadata:  event.Metadata,
		RequestID: event.RequestID,
		TraceID:   event.TraceID,
		CreatedAt: event.CreatedAt,
	})
	return err
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
