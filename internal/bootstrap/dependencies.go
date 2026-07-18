package bootstrap

import (
	"fmt"
	"log/slog"
	"messagefeed/internal/channel/wechatwork"
	"messagefeed/internal/config"
	"messagefeed/internal/fetcher"
	"messagefeed/internal/handler"
	"messagefeed/internal/llm"
	"messagefeed/internal/notifier"
	"messagefeed/internal/repository"
	appRuntime "messagefeed/internal/runtime"
	"messagefeed/internal/service"
	"net/http"
	"time"

	"gorm.io/gorm"
)

type dependencies struct {
	router  http.Handler
	workers workerSet
}

func buildDependencies(cfg config.Config, plan RolePlan, database *gorm.DB, logger *slog.Logger) (dependencies, error) {
	var result dependencies

	var callback *wechatwork.AppCallbackCodec
	var sender *notifier.WeChatWorkAppClient
	if (plan.API || plan.NotificationWorker || plan.AgentSchedulerWorker) && cfg.WeChatWork.Enabled() {
		var err error
		sender, err = notifier.NewWeChatWorkAppClient(notifier.WeChatWorkAppConfig{CorpID: cfg.WeChatWork.CorpID, AgentID: cfg.WeChatWork.AgentID, Secret: cfg.WeChatWork.Secret})
		if err != nil {
			return dependencies{}, fmt.Errorf("initialize wechat work sender: %w", err)
		}
		if plan.API {
			callback, err = wechatwork.NewAppCallbackCodec(wechatwork.AppCallbackConfig{CorpID: cfg.WeChatWork.CorpID, AgentID: cfg.WeChatWork.AgentID, CallbackToken: cfg.WeChatWork.CallbackToken, EncodingAESKey: cfg.WeChatWork.EncodingAESKey})
			if err != nil {
				return dependencies{}, fmt.Errorf("initialize wechat work callback: %w", err)
			}
		}
	}
	if plan.NotificationWorker && sender == nil {
		return dependencies{}, fmt.Errorf("notification worker requires WeChat Work sender configuration")
	}

	var llmClient llm.Client
	if plan.API && cfg.LLM.Enabled() {
		baseURL := cfg.LLM.BaseURL
		if cfg.LLM.Provider == "openai" && baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		client, err := llm.NewOpenAICompatibleClient(llm.OpenAICompatibleConfig{Provider: cfg.LLM.Provider, BaseURL: baseURL, APIKey: cfg.LLM.APIKey, Model: cfg.LLM.Model})
		if err != nil {
			return dependencies{}, fmt.Errorf("initialize llm client: %w", err)
		}
		llmClient = client
	}

	var embeddingClient llm.EmbeddingClient
	if (plan.API || plan.EmbeddingWorker) && cfg.Embedding.Enabled() {
		baseURL := cfg.Embedding.BaseURL
		if cfg.Embedding.Provider == "openai" && baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		client, err := llm.NewOpenAICompatibleEmbeddingClient(llm.OpenAICompatibleEmbeddingConfig{Provider: cfg.Embedding.Provider, BaseURL: baseURL, APIKey: cfg.Embedding.APIKey, Model: cfg.Embedding.Model})
		if err != nil {
			return dependencies{}, fmt.Errorf("initialize embedding client: %w", err)
		}
		embeddingClient = client
	}
	if plan.EmbeddingWorker && embeddingClient == nil {
		return dependencies{}, fmt.Errorf("embedding worker requires embedding configuration")
	}

	if database == nil {
		if plan.API {
			adminConfigService := service.NewAdminConfigService(cfg, service.WithAdminConfigLLM(llmClient), service.WithAdminConfigWeChatWorkSender(sender), service.WithAdminConfigWeChatWorkCallbackConfigured(callback != nil))
			result.router = newRouter(cfg, logger, nil, callback, adminConfigService, apiServices{})
		}
		return result, nil
	}

	sourceRepository := repository.NewSourceRepository(database)
	sourceCatalogRepository := repository.NewSourceCatalogRepository(database)
	sourceImportJobRepository := repository.NewSourceImportJobRepository(database)
	itemRepository := repository.NewItemRepository(database)
	userItemStateRepository := repository.NewUserItemStateRepository(database)
	feedViewPreferenceRepository := repository.NewFeedViewPreferenceRepository(database)
	sourceFetchJobRepository := repository.NewSourceFetchJobRepository(database)
	itemEventRepository := repository.NewItemEventRepository(database)
	notificationRepository := repository.NewNotificationRepository(database)
	taskLockRepository := repository.NewTaskLockRepository(database)
	agentRepository := repository.NewAgentRepository(database)
	authRepository := repository.NewAuthRepository(database)
	agentApprovalRepository := repository.NewAgentApprovalRepository(database)
	feedFetcher := fetcher.NewClient()

	sourceService := service.NewSourceService(sourceRepository,
		service.WithSourceCatalogRepository(sourceCatalogRepository),
		service.WithSourceImportJobRepository(sourceImportJobRepository),
		service.WithSourceFetchJobRepository(sourceFetchJobRepository),
		service.WithItemRepository(itemRepository),
		service.WithFeedFetcher(feedFetcher),
	)
	if plan.SourceWorker {
		result.workers.source = service.NewSourceSyncService(sourceRepository, itemRepository, feedFetcher, sourceFetchJobRepository, itemEventRepository, service.WithSourceSyncTaskLocker(taskLockRepository))
	}
	if plan.NotificationWorker && sender != nil {
		result.workers.notification = service.NewNotificationWorkerService(notificationRepository, sender)
	}
	if plan.AgentSchedulerWorker {
		result.workers.agentScheduler = service.NewAgentScheduledTaskWorkerService(agentRepository)
		result.workers.agentScheduler.SetReportSender(sender)
	}
	if plan.EmbeddingWorker && embeddingClient != nil {
		result.workers.embedding = service.NewAgentEmbeddingWorkerService(agentRepository, embeddingClient, cfg.Embedding.Model, time.Now)
	}

	if !plan.API {
		return result, nil
	}

	var weChatWorkOAuth *service.WeChatWorkOAuthClient
	if cfg.WeChatWork.Enabled() {
		var err error
		weChatWorkOAuth, err = service.NewWeChatWorkOAuthClient(service.WeChatWorkOAuthConfig{CorpID: cfg.WeChatWork.CorpID, Secret: cfg.WeChatWork.Secret})
		if err != nil {
			return dependencies{}, fmt.Errorf("initialize wechat work oauth: %w", err)
		}
	}
	authService := service.NewAuthService(authRepository, cfg, service.WithAuthWeChatWorkOAuth(weChatWorkOAuth))
	timelineService := service.NewTimelineService(itemRepository)
	recommendationService := service.NewRecommendationService(sourceCatalogRepository, feedFetcher)
	recommendationService.SetLocalHistoryRepositories(sourceRepository, itemRepository)
	agentLLMConfigSecret := service.AgentLLMConfigSecretFromConfig(cfg)
	agentLLMRuntime := service.NewAgentLLMRuntime(agentRepository, service.WithAgentLLMRuntimeDefaultClient(llmClient), service.WithAgentLLMRuntimeSecret(agentLLMConfigSecret))
	agentConversationService := service.NewAgentConversationService(agentRepository,
		service.WithAgentConversationLLM(agentLLMRuntime),
		service.WithAgentConversationSender(sender),
		service.WithAgentConversationExternalAccountResolver(authService),
		service.WithAgentConversationUserContextProvider(authService),
		service.WithAgentConversationRecentItemsProvider(timelineService),
		service.WithAgentConversationSourceProvider(sourceService),
		service.WithAgentConversationNotificationJobStore(notificationRepository),
		service.WithAgentConversationPublicBaseURL(cfg.Runtime.PublicBaseURL),
		service.WithAgentConversationEmbeddingClient(embeddingClient, cfg.Embedding.Model),
	)
	services := apiServices{
		auth: authService, source: sourceService, timeline: timelineService,
		recommendation:    recommendationService,
		item:              service.NewItemService(userItemStateRepository),
		feedView:          service.NewFeedViewService(feedViewPreferenceRepository),
		agentConversation: agentConversationService,
		agentApproval:     service.NewAgentApprovalService(agentApprovalRepository),
		agentSession:      service.NewAgentSessionService(agentRepository, service.WithAgentSessionEmbeddingClient(embeddingClient, cfg.Embedding.Model)),
		agentSchedule:     service.NewAgentScheduleEvalService(agentRepository),
		agentLLMConfig:    service.NewAgentLLMConfigService(agentRepository, service.WithAgentLLMConfigDefaultConfig(cfg.LLM), service.WithAgentLLMConfigSecret(agentLLMConfigSecret)),
	}
	adminConfigService := service.NewAdminConfigService(cfg,
		service.WithAdminConfigDatabase(database), service.WithAdminConfigLLM(llmClient),
		service.WithAdminConfigWeChatWorkSender(sender), service.WithAdminConfigWeChatWorkCallbackConfigured(callback != nil),
	)
	result.router = newRouter(cfg, logger, database, callback, adminConfigService, services)
	return result, nil
}

type apiServices struct {
	auth              *service.AuthService
	source            *service.SourceService
	timeline          *service.TimelineService
	recommendation    *service.RecommendationService
	item              *service.ItemService
	feedView          *service.FeedViewService
	agentConversation *service.AgentConversationService
	agentApproval     *service.AgentApprovalService
	agentSession      *service.AgentSessionService
	agentSchedule     *service.AgentScheduleEvalService
	agentLLMConfig    *service.AgentLLMConfigService
}

func newRouter(cfg config.Config, logger *slog.Logger, database *gorm.DB, callback *wechatwork.AppCallbackCodec, adminConfigService *service.AdminConfigService, services apiServices) http.Handler {
	nodeInfo := appRuntime.NewNodeInfo(appRuntime.NodeOptions{
		NodeID: cfg.Runtime.AppNodeID, DeploymentMode: cfg.Runtime.DeploymentMode,
		AppRole: string(cfg.Runtime.AppRole), PublicBaseURL: cfg.Runtime.PublicBaseURL,
		BindAddr: cfg.HTTP.BindAddr, TrustedProxyCIDRs: cfg.Runtime.TrustedProxyCIDRs, StartedAt: time.Now().UTC(),
	})
	return handler.NewRouter(handler.RouterOptions{
		Logger: logger, Database: database, NodeInfo: nodeInfo, Now: time.Now,
		AuthService: services.auth, SourceService: services.source, TimelineService: services.timeline,
		RecommendationService: services.recommendation, ItemService: services.item, FeedViewService: services.feedView,
		WeChatWorkAppCallback: callback, WeChatWorkReceiver: services.agentConversation,
		AdminConfigService: adminConfigService, AgentApprovalService: services.agentApproval,
		AgentSessionService: services.agentSession, AgentTaskService: services.agentConversation,
		AgentEvalService: services.agentSchedule, AgentLLMConfigService: services.agentLLMConfig,
		ServiceName: cfg.Observability.ServiceName,
	})
}
