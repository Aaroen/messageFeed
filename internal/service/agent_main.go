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
	EnsureExternalAccount(ctx context.Context, account domain.ExternalAccount) (domain.ExternalAccount, error)
	ListExternalAccounts(ctx context.Context, userID int64) ([]domain.ExternalAccount, error)
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
	CreateAgentPlan(ctx context.Context, plan domain.AgentPlan, steps []domain.AgentPlanStep) (domain.AgentPlan, error)
	ListAgentPlans(ctx context.Context, userID int64, sessionID int64, turnID int64, limit int) ([]domain.AgentPlan, error)
	ListAgentScheduledTasks(ctx context.Context, options domain.AgentScheduledTaskListOptions) ([]domain.AgentScheduledTask, error)
	UpdateAgentScheduledTask(ctx context.Context, task domain.AgentScheduledTask) (domain.AgentScheduledTask, error)
	UpdateAgentPlanStatus(ctx context.Context, userID int64, planID int64, status domain.AgentPlanStatus, now time.Time, errorMessage string) (domain.AgentPlan, error)
	UpdateAgentPlanMetadata(ctx context.Context, userID int64, planID int64, metadata domain.AgentJSON, now time.Time) (domain.AgentPlan, error)
	UpdateAgentPlanStepStatus(ctx context.Context, userID int64, step domain.AgentPlanStep) (domain.AgentPlanStep, error)
	CreateAgentApproval(ctx context.Context, approval domain.AgentApproval) (domain.AgentApproval, error)
	CreateAgentCapabilityAuditLog(ctx context.Context, log domain.AgentCapabilityAuditLog) (domain.AgentCapabilityAuditLog, error)
	GetAgentNotificationPreference(ctx context.Context, userID int64) (domain.AgentNotificationPreference, error)
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
	scheduledTasks     AgentScheduleEvalRepository
	webFetcher         agentWebFetcher
	turnRunner         *agent.TurnRunner
	runManager         *agent.RunManager
	planner            *agent.Planner
	capabilityRegistry *agent.CapabilityRegistry
	policyEngine       *agent.PolicyEngine
	now                func() time.Time
	ownerID            int64
	publicBaseURL      string
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

func WithAgentConversationScheduledTaskStore(store AgentScheduleEvalRepository) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		service.scheduledTasks = store
	}
}

func WithAgentConversationWebFetcher(fetcher func(context.Context, string) ([]byte, string, int, string, error)) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		service.webFetcher = agentWebFetcher(fetcher)
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

func WithAgentConversationPublicBaseURL(publicBaseURL string) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		service.publicBaseURL = strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
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
	if service.scheduledTasks == nil {
		if store, ok := any(repository).(AgentScheduleEvalRepository); ok {
			service.scheduledTasks = store
		}
	}
	service.runManager = agent.NewRunManager(agent.RunManagerOptions{Store: repository, Now: service.now})
	service.planner = agent.NewPlanner(agent.PlannerOptions{Registry: service.capabilityRegistry, Policy: service.policyEngine, Now: service.now})
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
		ToolKeys:       []string{"conversation.query_history", "agent.schedule_task", "agent.schedule_message", "web.search", "web.fetch_page", "web.extract_page", "repo.search", "repo.inspect_remote", "content.summarize_text"},
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
			scheduledTasks:   s.scheduledTasks,
			webFetcher:       s.webFetcher,
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
	Plan            domain.AgentPlan
	Reply           string
	SendResult      notifier.WeChatWorkSendResult
	Duplicate       bool
	BindingRequired bool
	ProcessingAsync bool
}

type ReceiveWebAgentTaskInput struct {
	Message   string
	SessionID int64
	Channel   string
	RequestID string
	TraceID   string
}

type AgentTurnResponse struct {
	ID               int64  `json:"id"`
	SessionID        int64  `json:"session_id"`
	InboundMessageID int64  `json:"inbound_message_id"`
	Status           string `json:"status"`
	InputText        string `json:"input_text"`
	OutputText       string `json:"output_text"`
	ErrorMessage     string `json:"error_message"`
	StartedAt        string `json:"started_at"`
	FinishedAt       string `json:"finished_at,omitempty"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

type ReceiveWebAgentTaskResult struct {
	Session     AgentSessionResponse `json:"session"`
	Turn        AgentTurnResponse    `json:"turn"`
	Plan        AgentPlanResponse    `json:"plan"`
	Reply       string               `json:"reply"`
	ProgressURL string               `json:"progress_url"`
	Duplicate   bool                 `json:"duplicate"`
}

func (s *AgentConversationService) approvalRequiredReply(plan domain.AgentPlan, token string) string {
	var builder strings.Builder
	builder.WriteString("该操作需要确认后才能继续。\n计划：")
	builder.WriteString(plan.Summary)
	builder.WriteString("\n状态锚点：approval_required/")
	builder.WriteString(string(plan.Status))
	builder.WriteString("\n影响：")
	builder.WriteString(plan.ImpactSummary)
	builder.WriteString("\n权限：")
	builder.WriteString(planPermissionSummary(plan))
	builder.WriteString("\n预算：")
	builder.WriteString(planBudgetSummary(plan))
	builder.WriteString("\n进度摘要：")
	builder.WriteString(s.agentPlanProgressText(plan))
	builder.WriteString("\n审批地址：")
	builder.WriteString(s.agentApprovalURL(token))
	if plan.ID > 0 {
		builder.WriteString("\n进度地址：")
		builder.WriteString(s.agentPlanURL(plan.ID))
	}
	builder.WriteString("\n下一步：打开审批地址确认或拒绝；如需查看实时执行细节，请打开进度地址。")
	builder.WriteString("\n")
	builder.WriteString(s.agentWeChatActionFallbackText(plan, token))
	return builder.String()
}

func (s *AgentConversationService) sendPlanStartedFeedback(
	ctx context.Context,
	account domain.ExternalAccount,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
	plan domain.AgentPlan,
) {
	if s == nil || plan.ID == 0 || !s.shouldSendWeChatWorkNotification(ctx, account.UserID, input, "process") {
		return
	}
	s.sendPlanProgressNotification(ctx, account, session, turn, input, plan, "started", "工作已开始")
}

func (s *AgentConversationService) sendPlanProgressNotification(
	ctx context.Context,
	account domain.ExternalAccount,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
	plan domain.AgentPlan,
	stage string,
	title string,
) {
	if s == nil || plan.ID == 0 {
		return
	}
	notificationKind := "process"
	if strings.Contains(stage, "failed") || stage == "failed" {
		notificationKind = "failure"
	}
	if !s.shouldSendWeChatWorkNotification(ctx, account.UserID, input, notificationKind) {
		return
	}
	stage = strings.TrimSpace(stage)
	if stage == "" {
		stage = "progress"
	}
	progressURL := s.agentPlanURL(plan.ID)
	reply := s.agentPlanProgressNotificationText(plan, stage, title)
	delivery := s.sendWeChatWorkProgressDelivery(ctx, input.ExternalUserID, plan, stage, title, reply)
	status := "succeeded"
	message := "agent plan progress notification sent"
	if delivery.FallbackStatus == "failed" {
		status = "failed"
		message = delivery.FallbackError
	} else if delivery.DeliveryMode == "text_fallback" {
		message = "agent plan progress notification sent with text fallback"
	}
	eventType := "agent.plan_progress_notification"
	if stage == "started" {
		eventType = "agent.plan_started_feedback"
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		EventType: eventType,
		Status:    status,
		Message:   message,
		Metadata: domain.AgentJSON{
			"plan_id":             plan.ID,
			"stage":               stage,
			"target_channel":      input.Provider,
			"target_ref":          input.ExternalUserID,
			"provider_message_id": input.ProviderMessageID,
			"wechat_msgid":        delivery.SendResult.MessageID,
			"send_count":          delivery.SendCount,
			"progress_url":        progressURL,
			"message_type":        delivery.DeliveryMode,
			"template_status":     delivery.TemplateStatus,
			"fallback_status":     delivery.FallbackStatus,
			"template_error":      delivery.TemplateError,
			"fallback_error":      delivery.FallbackError,
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentConversationService) agentPlanProgressNotificationText(plan domain.AgentPlan, stage string, title string) string {
	if stage == "started" {
		return s.agentPlanStartedReply(plan)
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "进度更新"
	}
	var builder strings.Builder
	builder.WriteString(title)
	builder.WriteString("。\n")
	builder.WriteString("进度：")
	builder.WriteString(s.agentPlanWeChatProgressText(plan))
	builder.WriteString("\n下一步：")
	builder.WriteString(agentProgressNextAction(string(plan.Status), true, plan, nil))
	builder.WriteString("\n详情：")
	builder.WriteString(s.agentPlanURL(plan.ID))
	if failedStep := firstFailedPlanStep(plan); failedStep.ID > 0 {
		builder.WriteString("\n失败步骤：")
		builder.WriteString(planStepLabel(failedStep))
		if failedStep.ErrorMessage != "" {
			builder.WriteString(" / ")
			builder.WriteString(safeSummary(failedStep.ErrorMessage, 160))
		}
	}
	return strings.TrimSpace(builder.String())
}

func (s *AgentConversationService) agentPlanStartedReply(plan domain.AgentPlan) string {
	var builder strings.Builder
	builder.WriteString("已开始处理")
	if strings.TrimSpace(plan.Goal) != "" {
		builder.WriteString("：")
		builder.WriteString(strings.TrimSpace(plan.Goal))
	}
	builder.WriteString("。\n")
	builder.WriteString("进度：")
	builder.WriteString(s.agentPlanWeChatProgressText(plan))
	if plan.ID > 0 {
		builder.WriteString("\n详情：")
		builder.WriteString(s.agentPlanURL(plan.ID))
	}
	return strings.TrimSpace(builder.String())
}

func (s *AgentConversationService) agentTurnCompletionReply(plan domain.AgentPlan, reply string) string {
	reply = strings.TrimSpace(reply)
	if reply != "" {
		return reply
	}
	status := "已完成"
	if plan.Status == domain.AgentPlanStatusFailed {
		status = "处理失败"
	}
	var builder strings.Builder
	builder.WriteString(status)
	if plan.ID > 0 {
		builder.WriteString("。详情：")
		builder.WriteString(s.agentPlanURL(plan.ID))
	}
	return builder.String()
}

func (s *AgentConversationService) agentWeChatActionFallbackText(plan domain.AgentPlan, approvalToken string) string {
	progressURL := s.agentPlanURL(plan.ID)
	approvalURL := progressURL
	if strings.TrimSpace(approvalToken) != "" {
		approvalURL = s.agentApprovalURL(approvalToken)
	}
	actions := []string{
		"view_progress=" + progressURL,
		"approval=" + approvalURL,
		"retry_plan=" + progressURL,
		"recover_plan=" + progressURL,
		"cancel_scheduled_task=" + progressURL,
	}
	return "企微动作组件：" + strings.Join(actions, "；")
}

func (s *AgentConversationService) sendWeChatWorkTaskAcceptedFeedback(
	ctx context.Context,
	account domain.ExternalAccount,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
) (string, notifier.WeChatWorkSendResult, int) {
	reply := agentTaskAcceptedFeedbackText()
	if s == nil || !s.shouldSendWeChatWorkNotification(ctx, account.UserID, input, "process") {
		return reply, notifier.WeChatWorkSendResult{}, 0
	}
	sendResult, sendCount, err := s.sendWeChatWorkReply(ctx, input.ExternalUserID, reply)
	status := "succeeded"
	message := "wechat work task acceptance feedback sent"
	if err != nil {
		status = "failed"
		message = strings.TrimSpace(err.Error())
	}
	metrics.AgentReplyChunksTotal.WithLabelValues(input.Provider, "accepted").Add(float64(sendCount))
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		EventType: "wechat_work.task_accepted_feedback",
		Status:    status,
		Message:   message,
		Metadata: domain.AgentJSON{
			"provider_message_id": input.ProviderMessageID,
			"target_channel":      input.Provider,
			"target_ref":          input.ExternalUserID,
			"wechat_msgid":        sendResult.MessageID,
			"send_count":          sendCount,
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: s.now().UTC(),
	})
	return reply, sendResult, sendCount
}

func agentTaskAcceptedFeedbackText() string {
	return "已收到任务，后台正在处理，请稍等。完成后会在这里返回结果。"
}

func (s *AgentConversationService) agentPlanProgressText(plan domain.AgentPlan) string {
	updatedAt := plan.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = s.now().UTC()
	}
	response := agentPlanResponse(plan, true)
	return AgentProgressTextSummary(AgentProgressSnapshot{
		SubjectType: "plan",
		SubjectID:   plan.ID,
		Status:      string(plan.Status),
		Summary:     plan.Summary,
		NextAction:  agentProgressNextAction(string(plan.Status), true, plan, nil),
		Version:     updatedAt.UnixNano(),
		EventCursor: fmt.Sprintf("plan:%d:%s", plan.ID, updatedAt.UTC().Format(time.RFC3339Nano)),
		UpdatedAt:   formatOptionalTime(&updatedAt),
		Plan:        &response,
	})
}

func (s *AgentConversationService) agentPlanWeChatProgressText(plan domain.AgentPlan) string {
	summary := strings.TrimSpace(plan.Goal)
	if summary == "" {
		summary = strings.TrimSpace(plan.Summary)
	}
	if summary == "" {
		summary = "任务处理中"
	}
	status := "处理中"
	switch plan.Status {
	case domain.AgentPlanStatusCompleted:
		status = "已完成"
	case domain.AgentPlanStatusFailed:
		status = "处理失败"
	case domain.AgentPlanStatusAwaitingApproval:
		status = "等待确认"
	case domain.AgentPlanStatusRejected:
		status = "已拒绝"
	case domain.AgentPlanStatusExecuting, domain.AgentPlanStatusApproved:
		status = "处理中"
	}
	return safeSummary(summary, 120) + "，" + status
}

func planStepLabel(step domain.AgentPlanStep) string {
	title := strings.TrimSpace(step.Title)
	if title == "" {
		title = strings.TrimSpace(step.CapabilityKey)
	}
	if title == "" {
		return "执行计划步骤"
	}
	if step.CapabilityKey == "" {
		return title
	}
	return title + "（" + step.CapabilityKey + "）"
}

func firstFailedPlanStep(plan domain.AgentPlan) domain.AgentPlanStep {
	for _, step := range plan.Steps {
		if step.Status == domain.AgentPlanStepStatusFailed {
			return step
		}
	}
	return domain.AgentPlanStep{}
}

func (s *AgentConversationService) agentPlanURL(planID int64) string {
	path := fmt.Sprintf("/agent/plans/%d", planID)
	if s == nil || s.publicBaseURL == "" {
		return path
	}
	return s.publicBaseURL + path
}

func (s *AgentConversationService) agentApprovalURL(token string) string {
	path := "/agent/approvals/" + strings.TrimSpace(token)
	if s == nil || s.publicBaseURL == "" {
		return path
	}
	return s.publicBaseURL + path
}

func (s *AgentConversationService) finishTurnWithReply(
	ctx context.Context,
	account domain.ExternalAccount,
	inbound domain.AgentInboundMessage,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
	reply string,
	observations []agent.CapabilityObservation,
	auditStatus string,
) (ReceiveWeChatWorkAppMessageResult, error) {
	now := s.now().UTC()
	reply = sanitizeAgentReportText(reply)
	_, _ = s.repository.AppendTranscriptEntry(ctx, domain.AgentTranscriptEntry{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		Role:      domain.AgentTranscriptRoleAssistant,
		Content:   reply,
		Metadata: domain.AgentJSON{
			"observations": agent.ObservationMetadata(observations),
		},
		CreatedAt: now,
	})
	finishedAt := now
	turn.Status = domain.AgentTurnStatusSucceeded
	turn.OutputText = reply
	turn.FinishedAt = &finishedAt
	turn, _ = s.repository.UpdateTurn(ctx, turn)

	sendResult := notifier.WeChatWorkSendResult{}
	sendCount := 0
	if s.shouldSendWeChatWorkNotification(ctx, account.UserID, input, "final") {
		var err error
		sendResult, sendCount, err = s.sendWeChatWorkReply(ctx, input.ExternalUserID, reply)
		if err != nil {
			_, _ = s.repository.UpdateInboundMessageStatus(ctx, account.UserID, inbound.ID, domain.AgentInboundMessageStatusFailed, s.now().UTC())
			return ReceiveWeChatWorkAppMessageResult{ExternalAccount: account, InboundMessage: inbound, Session: session, Turn: turn, Reply: reply}, err
		}
	}
	inbound, _ = s.repository.UpdateInboundMessageStatus(ctx, account.UserID, inbound.ID, domain.AgentInboundMessageStatusSucceeded, s.now().UTC())
	if auditStatus == "" {
		auditStatus = "succeeded"
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		EventType: "agent.turn_reply",
		Status:    auditStatus,
		Message:   "agent turn completed with direct reply",
		Metadata: domain.AgentJSON{
			"provider_message_id": input.ProviderMessageID,
			"send_count":          sendCount,
			"observations":        agent.ObservationMetadata(observations),
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: s.now().UTC(),
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

func (s *AgentConversationService) failTurnWithFeedback(
	ctx context.Context,
	account domain.ExternalAccount,
	inbound domain.AgentInboundMessage,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
	plan domain.AgentPlan,
	cause error,
) ReceiveWeChatWorkAppMessageResult {
	if cause == nil {
		cause = fmt.Errorf("agent turn failed")
	}
	now := s.now().UTC()
	failedTurn := turn
	failedTurn.Status = domain.AgentTurnStatusFailed
	failedTurn.ErrorMessage = cause.Error()
	failedTurn.FinishedAt = &now
	if failedTurn.ID > 0 {
		if updated, err := s.repository.UpdateTurn(ctx, failedTurn); err == nil {
			failedTurn = updated
		}
	}
	if inbound.ID > 0 {
		if updated, err := s.repository.UpdateInboundMessageStatus(ctx, account.UserID, inbound.ID, domain.AgentInboundMessageStatusFailed, now); err == nil {
			inbound = updated
		}
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    failedTurn.ID,
		UserID:    account.UserID,
		EventType: "agent.turn_failed",
		Status:    "failed",
		Message:   cause.Error(),
		Metadata: domain.AgentJSON{
			"provider_message_id": input.ProviderMessageID,
			"plan_id":             plan.ID,
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: now,
	})
	result := s.sendTurnFailureFeedback(ctx, account, inbound, session, turn, failedTurn, input, plan, cause)
	result.InboundMessage = inbound
	result.Plan = plan
	result.Turn = failedTurn
	return result
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
	plan domain.AgentPlan,
	cause error,
) ReceiveWeChatWorkAppMessageResult {
	if failedTurn.ID == 0 {
		failedTurn = originalTurn
	}
	reply := agentTurnFailureFeedback(input.TextContent)
	if !s.processInline {
		reply = s.agentTurnCompletionReply(plan, reply)
	}
	reply = sanitizeAgentReportText(reply)
	sendResult := notifier.WeChatWorkSendResult{}
	sendCount := 0
	sendStatus := "skipped"
	finalDelivery := agentWeChatFinalReportDeliveryResult{}
	if s.shouldSendWeChatWorkNotification(ctx, account.UserID, input, "failure") {
		var sendErr error
		finalDelivery, sendErr = s.sendWeChatWorkFinalReportDelivery(ctx, input.ExternalUserID, plan, reply, "failed")
		sendResult = finalDelivery.SendResult
		sendCount = finalDelivery.SendCount
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
			"message_type":        finalDelivery.DeliveryMode,
			"template_status":     finalDelivery.TemplateStatus,
			"text_status":         finalDelivery.TextStatus,
			"template_error":      finalDelivery.TemplateError,
			"text_error":          finalDelivery.TextError,
			"progress_url":        finalDelivery.ProgressURL,
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

func (s *AgentConversationService) shouldSendWeChatWorkReply(input ReceiveWeChatWorkAppMessageInput) bool {
	return s != nil &&
		s.sender != nil &&
		input.Provider == domain.AgentProviderWeChatWorkApp &&
		strings.TrimSpace(input.ExternalUserID) != ""
}

func (s *AgentConversationService) shouldSendWeChatWorkNotification(ctx context.Context, userID int64, input ReceiveWeChatWorkAppMessageInput, kind string) bool {
	if !s.shouldSendWeChatWorkReply(input) {
		return false
	}
	preference := s.agentNotificationPreference(ctx, userID)
	switch strings.TrimSpace(kind) {
	case "process":
		return preference.ProcessNotificationsEnabled
	case "failure":
		return preference.FailureNotificationsEnabled
	case "recovery":
		return preference.RecoveryNotificationsEnabled
	case "final":
		return preference.FinalReportsEnabled
	default:
		return true
	}
}

func (s *AgentConversationService) agentNotificationPreference(ctx context.Context, userID int64) domain.AgentNotificationPreference {
	if s == nil || s.repository == nil || userID < 1 {
		return defaultAgentNotificationPreference(userID, time.Time{})
	}
	preference, err := s.repository.GetAgentNotificationPreference(ctx, userID)
	if err != nil {
		return defaultAgentNotificationPreference(userID, s.now().UTC())
	}
	return preference
}

func (s *AgentConversationService) agentTaskAdmissionDecision(ctx context.Context, userID int64, entry string, currentScheduledTaskID int64) agentTaskAdmissionDecision {
	now := s.now().UTC()
	preference := s.agentNotificationPreference(ctx, userID)
	var plans []domain.AgentPlan
	var scheduledTasks []domain.AgentScheduledTask
	if s != nil && s.repository != nil && userID > 0 {
		plans, _ = s.repository.ListAgentPlans(ctx, userID, 0, 0, 100)
		scheduledTasks, _ = s.repository.ListAgentScheduledTasks(ctx, domain.AgentScheduledTaskListOptions{UserID: userID, Limit: 100})
	}
	return evaluateAgentTaskAdmission(agentTaskAdmissionInput{
		UserID:                 userID,
		Entry:                  entry,
		Preference:             preference,
		Plans:                  plans,
		ScheduledTasks:         scheduledTasks,
		CurrentScheduledTaskID: currentScheduledTaskID,
		Now:                    now,
	})
}

func normalizeWebAgentChannel(channel string) string {
	channel = strings.TrimSpace(channel)
	if channel == "" {
		return "web"
	}
	return channel
}

func webAgentSessionKey(userID int64, channel string) string {
	return fmt.Sprintf("web:user:%d:%s", userID, normalizeWebAgentChannel(channel))
}

func agentTurnResponse(turn domain.AgentTurn) AgentTurnResponse {
	return AgentTurnResponse{
		ID:               turn.ID,
		SessionID:        turn.SessionID,
		InboundMessageID: turn.InboundMessageID,
		Status:           string(turn.Status),
		InputText:        turn.InputText,
		OutputText:       turn.OutputText,
		ErrorMessage:     turn.ErrorMessage,
		StartedAt:        formatOptionalTime(&turn.StartedAt),
		FinishedAt:       formatOptionalTime(turn.FinishedAt),
		CreatedAt:        formatOptionalTime(&turn.CreatedAt),
		UpdatedAt:        formatOptionalTime(&turn.UpdatedAt),
	}
}
