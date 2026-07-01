package service

import (
	"context"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"strings"
	"sync"
	"time"
)

const (
	defaultAgentOwnerUserID = int64(1)
	// agentReplyMaxTokens 为 0 时不向模型请求写入 max_tokens / max_output_tokens。
	// 当前 Agent 需要先保证复杂任务可完整收敛，暂不在最终回复层设置固定最高上限。
	agentReplyMaxTokens = 0
	// defaultAgentProcessTimeout 是单轮后台执行上限。
	// 真实模型规划、联网搜索和工具收敛可能超过几十秒，因此生产默认给到 10 分钟。
	defaultAgentProcessTimeout = 10 * time.Minute
	// defaultAgentNotificationTimeout 只约束企业微信发送动作。
	// 通知超时必须独立于执行超时，避免主流程超时后连失败消息也发不出去。
	defaultAgentNotificationTimeout = 15 * time.Second
	// defaultAgentProgressNotifyInterval 是异步长任务面向企微用户的低频进度心跳。
	// 间隔保持分钟级，避免把内部 trace 噪声转化为用户侧打扰。
	defaultAgentProgressNotifyInterval = 2 * time.Minute
	// defaultAgentMemoryCaptureTimeout 只约束后台长期记忆候选捕获。
	// 该流程会调用真实模型分类，不能阻塞企业微信回调确认。
	defaultAgentMemoryCaptureTimeout = 2 * time.Minute
	// defaultAgentStopWaitTimeout 是用户主动停止后等待 goroutine 退出的确认窗口。
	// 超过该时间仍未退出时，不把计划标记为已停止，避免掩盖无法取消的后台执行。
	defaultAgentStopWaitTimeout = 5 * time.Second
)

type AgentConversationService struct {
	repository             AgentConversationRepository
	llmClient              AgentConversationLLM
	embeddingClient        llm.EmbeddingClient
	embeddingModel         string
	sender                 AgentConversationSender
	resolver               AgentExternalAccountResolver
	userCtx                AgentUserContextProvider
	recentItems            AgentRecentItemsProvider
	sourceProvider         AgentSourceProvider
	notificationJobs       AgentNotificationJobStore
	scheduledTasks         AgentScheduleEvalRepository
	webFetcher             agentWebFetcher
	turnRunner             *agent.TurnRunner
	runManager             *agent.RunManager
	planner                *agent.Planner
	capabilityRegistry     *agent.CapabilityRegistry
	policyEngine           *agent.PolicyEngine
	now                    func() time.Time
	ownerID                int64
	publicBaseURL          string
	processInline          bool
	processTimeout         time.Duration
	notificationTimeout    time.Duration
	progressNotifyInterval time.Duration
	lockMu                 sync.Mutex
	sessionLocks           map[int64]*sync.Mutex
	activeProcessMu        sync.Mutex
	activeByTurnID         map[int64]*agentActiveProcess
	activeByPlanID         map[int64]*agentActiveProcess
	derivedParentByTurnID  map[int64]domain.AgentPlan
}

type AgentConversationServiceOption func(*AgentConversationService)

func WithAgentConversationLLM(client AgentConversationLLM) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		service.llmClient = client
	}
}

func WithAgentConversationEmbeddingClient(client llm.EmbeddingClient, model string) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		service.embeddingClient = client
		service.embeddingModel = strings.TrimSpace(model)
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

func WithAgentConversationNotificationTimeout(timeout time.Duration) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		if timeout > 0 {
			service.notificationTimeout = timeout
		}
	}
}

func WithAgentConversationProgressNotifyInterval(interval time.Duration) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		if interval > 0 {
			service.progressNotifyInterval = interval
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
		repository:             repository,
		capabilityRegistry:     agent.NewP0CapabilityRegistry(),
		policyEngine:           agent.NewPolicyEngine(),
		now:                    time.Now,
		ownerID:                defaultAgentOwnerUserID,
		processTimeout:         defaultAgentProcessTimeout,
		notificationTimeout:    defaultAgentNotificationTimeout,
		progressNotifyInterval: defaultAgentProgressNotifyInterval,
		sessionLocks:           map[int64]*sync.Mutex{},
		activeByTurnID:         map[int64]*agentActiveProcess{},
		activeByPlanID:         map[int64]*agentActiveProcess{},
		derivedParentByTurnID:  map[int64]domain.AgentPlan{},
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
			repository:    s.repository,
			factRetriever: newAgentFactRetriever(s.repository, s.embeddingClient, s.embeddingModel, s.now),
			now:           s.now,
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
		ToolKeys:       []string{"feed.query_recent_items", "source.query_latest_items", "conversation.query_history", "agent.schedule_task", "agent.schedule_message", "web.search", "web.fetch_page", "web.extract_page", "repo.search", "repo.inspect_remote", "content.summarize_text"},
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
