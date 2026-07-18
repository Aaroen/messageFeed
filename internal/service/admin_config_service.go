package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"messagefeed/internal/config"
	appdb "messagefeed/internal/db"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"messagefeed/internal/metrics"
	"messagefeed/internal/notifier"
	"messagefeed/internal/observability"

	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gorm"
)

const (
	defaultAdminLLMTestMessage        = "请回复 OK，使用普通微信聊天文本。"
	defaultAdminWeChatWorkTestMessage = "messageFeed 管理后台测试消息"
	adminLLMTestMaxTokens             = 64
	adminMaskedSecretPlaceholder      = "********"
)

type AdminConfigLLM interface {
	Chat(ctx context.Context, request llm.ChatRequest) (llm.ChatResponse, error)
}

type AdminConfigWeChatWorkSender interface {
	SendText(ctx context.Context, message notifier.WeChatWorkTextMessage) (notifier.WeChatWorkSendResult, error)
}

type AdminConfigService struct {
	cfg                config.Config
	database           *gorm.DB
	llmClient          AdminConfigLLM
	weChatWorkSender   AdminConfigWeChatWorkSender
	weChatWorkCallback bool
	now                func() time.Time
}

type AdminConfigServiceOption func(*AdminConfigService)

func WithAdminConfigLLM(client AdminConfigLLM) AdminConfigServiceOption {
	return func(service *AdminConfigService) {
		service.llmClient = client
	}
}

func WithAdminConfigDatabase(database *gorm.DB) AdminConfigServiceOption {
	return func(service *AdminConfigService) {
		service.database = database
	}
}

func WithAdminConfigWeChatWorkSender(sender AdminConfigWeChatWorkSender) AdminConfigServiceOption {
	return func(service *AdminConfigService) {
		service.weChatWorkSender = sender
	}
}

func WithAdminConfigWeChatWorkCallbackConfigured(configured bool) AdminConfigServiceOption {
	return func(service *AdminConfigService) {
		service.weChatWorkCallback = configured
	}
}

func WithAdminConfigNow(now func() time.Time) AdminConfigServiceOption {
	return func(service *AdminConfigService) {
		if now != nil {
			service.now = now
		}
	}
}

func NewAdminConfigService(cfg config.Config, options ...AdminConfigServiceOption) *AdminConfigService {
	service := &AdminConfigService{
		cfg: cfg,
		now: time.Now,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

type AdminConfigStatus struct {
	UpdatedAt     time.Time                        `json:"updated_at"`
	Runtime       AdminRuntimeConfigStatus         `json:"runtime"`
	Database      AdminDatabaseConfigStatus        `json:"database"`
	Auth          AdminAuthConfigStatus            `json:"auth"`
	WeChatWork    AdminWeChatWorkConfigStatus      `json:"wechat_work"`
	LLM           AdminLLMConfigStatus             `json:"llm"`
	Observability AdminObservabilityConfigStatus   `json:"observability"`
	Endpoints     AdminConfigEndpointStatus        `json:"endpoints"`
	Requirements  []AdminConfigRequirementCategory `json:"requirements"`
}

type AdminRuntimeConfigStatus struct {
	Environment    string `json:"environment"`
	ServiceName    string `json:"service_name"`
	ServiceVersion string `json:"service_version"`
	AppNodeID      string `json:"app_node_id"`
	DeploymentMode string `json:"deployment_mode"`
	AppRole        string `json:"app_role"`
	PublicBaseURL  string `json:"public_base_url"`
	BindAddr       string `json:"bind_addr"`
}

type AdminDatabaseConfigStatus struct {
	Configured bool `json:"configured"`
}

type AdminAuthConfigStatus struct {
	LocalLoginEnabled    bool   `json:"local_login_enabled"`
	SessionCookie        string `json:"session_cookie"`
	SessionSecure        bool   `json:"session_secure"`
	SessionTTLSeconds    int64  `json:"session_ttl_seconds"`
	OAuthStateTTLSeconds int64  `json:"oauth_state_ttl_seconds"`
}

type AdminWeChatWorkConfigStatus struct {
	Enabled            bool   `json:"enabled"`
	OAuthConfigured    bool   `json:"oauth_configured"`
	CallbackConfigured bool   `json:"callback_configured"`
	SenderConfigured   bool   `json:"sender_configured"`
	CorpIDMasked       string `json:"corp_id_masked,omitempty"`
	AgentID            string `json:"agent_id,omitempty"`
	CallbackURL        string `json:"callback_url,omitempty"`
	OAuthCallbackURL   string `json:"oauth_callback_url,omitempty"`
}

type AdminLLMConfigStatus struct {
	Enabled       bool   `json:"enabled"`
	ClientReady   bool   `json:"client_ready"`
	Provider      string `json:"provider,omitempty"`
	Model         string `json:"model,omitempty"`
	BaseURL       string `json:"base_url,omitempty"`
	APIKeyPresent bool   `json:"api_key_present"`
}

type AdminObservabilityConfigStatus struct {
	TraceEnabled       bool                            `json:"trace_enabled"`
	OTLPEndpointSet    bool                            `json:"otlp_endpoint_set"`
	OTLPInsecure       bool                            `json:"otlp_insecure"`
	TraceSampleRatio   float64                         `json:"trace_sample_ratio"`
	PrometheusEndpoint string                          `json:"prometheus_endpoint"`
	GrafanaURL         string                          `json:"grafana_url"`
	Agent              AdminAgentObservabilityStatus   `json:"agent"`
	Metrics            []AdminObservabilityMetricEntry `json:"metrics"`
}

type AdminAgentObservabilityStatus struct {
	Configured                        bool    `json:"configured"`
	Ready                             bool    `json:"ready"`
	ErrorMessage                      string  `json:"error_message,omitempty"`
	TraceEventRows                    int64   `json:"trace_event_rows"`
	RecallTraceRows                   int64   `json:"recall_trace_rows"`
	EmbeddingTraceRows                int64   `json:"embedding_trace_rows"`
	MemoryTopicRows                   int64   `json:"memory_topic_rows"`
	MemoryChunkRows                   int64   `json:"memory_chunk_rows"`
	MemoryChunkReadyRows              int64   `json:"memory_chunk_ready_rows"`
	MemoryChunkEmbeddingCoverageRatio float64 `json:"memory_chunk_embedding_coverage_ratio"`
	PendingEmbeddingJobs              int64   `json:"pending_embedding_jobs"`
	RunningEmbeddingJobs              int64   `json:"running_embedding_jobs"`
	FailedEmbeddingJobs               int64   `json:"failed_embedding_jobs"`
	LastEmbeddingJobUpdatedAt         string  `json:"last_embedding_job_updated_at,omitempty"`
	LastEmbeddingError                string  `json:"last_embedding_error,omitempty"`
	EmbeddingWorkerEnabled            bool    `json:"embedding_worker_enabled"`
	EmbeddingWorkerConfigured         bool    `json:"embedding_worker_configured"`
	EmbeddingModelConfigured          bool    `json:"embedding_model_configured"`
}

type AdminObservabilityMetricEntry struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
}

type AdminConfigEndpointStatus struct {
	WeChatWorkCallback string `json:"wechat_work_callback"`
	Metrics            string `json:"metrics"`
	Health             string `json:"health"`
	Readiness          string `json:"readiness"`
}

type AdminConfigRequirementCategory struct {
	Name  string                   `json:"name"`
	Items []AdminConfigRequirement `json:"items"`
}

type AdminConfigRequirement struct {
	Key        string `json:"key"`
	Configured bool   `json:"configured"`
	Secret     bool   `json:"secret"`
}

type AdminLLMTestInput struct {
	Message string `json:"message"`
}

type AdminLLMTestResult struct {
	Status       string `json:"status"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	LatencyMS    int64  `json:"latency_ms"`
	ResponseText string `json:"response_text"`
	CheckedAt    string `json:"checked_at"`
}

type AdminWeChatWorkTestInput struct {
	ToUser  string `json:"to_user"`
	Content string `json:"content"`
}

type AdminWeChatWorkTestResult struct {
	Status         string `json:"status"`
	ErrCode        int    `json:"errcode"`
	ErrMsg         string `json:"errmsg,omitempty"`
	MessageID      string `json:"message_id,omitempty"`
	InvalidUser    string `json:"invalid_user,omitempty"`
	InvalidParty   string `json:"invalid_party,omitempty"`
	InvalidTag     string `json:"invalid_tag,omitempty"`
	UnlicensedUser string `json:"unlicensed_user,omitempty"`
	LatencyMS      int64  `json:"latency_ms"`
	CheckedAt      string `json:"checked_at"`
}

func (s *AdminConfigService) Status(ctx context.Context) (AdminConfigStatus, error) {
	if s == nil {
		return AdminConfigStatus{}, domain.NewAppError(domain.ErrorKindUnavailable, "admin_config_unavailable", "admin config service is unavailable", "service.admin_config.status", true, nil)
	}
	ctx, span := observability.StartSpan(ctx, "service.admin_config.status")
	defer observability.EndSpan(span, nil)

	publicBaseURL := strings.TrimRight(s.cfg.Runtime.PublicBaseURL, "/")
	callbackURL := joinPublicURL(publicBaseURL, "/api/v1/channels/wechat-work/app/callback")
	oauthCallbackURL := joinPublicURL(publicBaseURL, "/api/v1/auth/wechat-work/callback")
	status := AdminConfigStatus{
		UpdatedAt: s.now().UTC(),
		Runtime: AdminRuntimeConfigStatus{
			Environment:    s.cfg.Observability.Environment,
			ServiceName:    s.cfg.Observability.ServiceName,
			ServiceVersion: s.cfg.Observability.ServiceVersion,
			AppNodeID:      s.cfg.Runtime.AppNodeID,
			DeploymentMode: s.cfg.Runtime.DeploymentMode,
			AppRole:        string(s.cfg.Runtime.AppRole),
			PublicBaseURL:  s.cfg.Runtime.PublicBaseURL,
			BindAddr:       s.cfg.HTTP.BindAddr,
		},
		Database: AdminDatabaseConfigStatus{
			Configured: strings.TrimSpace(s.cfg.Database.DSN) != "",
		},
		Auth: AdminAuthConfigStatus{
			LocalLoginEnabled:    s.cfg.Auth.LocalLoginEnabled(),
			SessionCookie:        s.cfg.Auth.SessionCookie,
			SessionSecure:        s.cfg.Auth.SessionSecure,
			SessionTTLSeconds:    int64(s.cfg.Auth.SessionTTL.Seconds()),
			OAuthStateTTLSeconds: int64(s.cfg.Auth.OAuthStateTTL.Seconds()),
		},
		WeChatWork: AdminWeChatWorkConfigStatus{
			Enabled:            s.cfg.WeChatWork.Enabled(),
			OAuthConfigured:    s.weChatWorkOAuthConfigured(),
			CallbackConfigured: s.weChatWorkCallback,
			SenderConfigured:   s.weChatWorkSender != nil,
			CorpIDMasked:       maskConfigValue(s.cfg.WeChatWork.CorpID),
			AgentID:            s.cfg.WeChatWork.AgentID,
			CallbackURL:        callbackURL,
			OAuthCallbackURL:   oauthCallbackURL,
		},
		LLM: AdminLLMConfigStatus{
			Enabled:       s.cfg.LLM.Enabled(),
			ClientReady:   s.llmClient != nil,
			Provider:      s.cfg.LLM.Provider,
			Model:         s.cfg.LLM.Model,
			BaseURL:       s.cfg.LLM.BaseURL,
			APIKeyPresent: strings.TrimSpace(s.cfg.LLM.APIKey) != "",
		},
		Observability: AdminObservabilityConfigStatus{
			TraceEnabled:       s.cfg.Observability.TraceEnabled,
			OTLPEndpointSet:    strings.TrimSpace(s.cfg.Observability.OTLPEndpoint) != "",
			OTLPInsecure:       s.cfg.Observability.OTLPInsecure,
			TraceSampleRatio:   s.cfg.Observability.TraceSampleRatio,
			PrometheusEndpoint: "http://127.0.0.1:9090",
			GrafanaURL:         "http://127.0.0.1:3000/d/messagefeed-overview/messagefeed-overview",
			Agent:              s.agentObservabilityStatus(ctx),
			Metrics:            adminAgentObservabilityMetrics(),
		},
		Endpoints: AdminConfigEndpointStatus{
			WeChatWorkCallback: callbackURL,
			Metrics:            joinPublicURL(publicBaseURL, "/metrics"),
			Health:             joinPublicURL(publicBaseURL, "/healthz"),
			Readiness:          joinPublicURL(publicBaseURL, "/readyz"),
		},
		Requirements: []AdminConfigRequirementCategory{
			{
				Name: "wechat_work",
				Items: []AdminConfigRequirement{
					{Key: "WECHAT_WORK_CORP_ID", Configured: s.cfg.WeChatWork.CorpID != "", Secret: false},
					{Key: "WECHAT_WORK_AGENT_ID", Configured: s.cfg.WeChatWork.AgentID != "", Secret: false},
					{Key: "WECHAT_WORK_SECRET", Configured: s.cfg.WeChatWork.Secret != "", Secret: true},
					{Key: "WECHAT_WORK_CALLBACK_TOKEN", Configured: s.cfg.WeChatWork.CallbackToken != "", Secret: true},
					{Key: "WECHAT_WORK_ENCODING_AES_KEY", Configured: s.cfg.WeChatWork.EncodingAESKey != "", Secret: true},
				},
			},
			{
				Name: "auth",
				Items: []AdminConfigRequirement{
					{Key: "AUTH_OWNER_USERNAME", Configured: s.cfg.Auth.OwnerUsername != "", Secret: false},
					{Key: "AUTH_OWNER_PASSWORD", Configured: s.cfg.Auth.OwnerPassword != "", Secret: true},
					{Key: "AUTH_SESSION_COOKIE_NAME", Configured: s.cfg.Auth.SessionCookie != "", Secret: false},
					{Key: "AUTH_SESSION_TTL", Configured: s.cfg.Auth.SessionTTL > 0, Secret: false},
					{Key: "AUTH_OAUTH_STATE_TTL", Configured: s.cfg.Auth.OAuthStateTTL > 0, Secret: false},
				},
			},
			{
				Name: "llm",
				Items: []AdminConfigRequirement{
					{Key: "LLM_PROVIDER", Configured: s.cfg.LLM.Provider != "", Secret: false},
					{Key: "LLM_BASE_URL", Configured: s.cfg.LLM.Provider == "openai" || s.cfg.LLM.BaseURL != "", Secret: false},
					{Key: "LLM_API_KEY", Configured: s.cfg.LLM.APIKey != "", Secret: true},
					{Key: "LLM_MODEL", Configured: s.cfg.LLM.Model != "", Secret: false},
				},
			},
		},
	}
	span.SetAttributes(
		attribute.Bool("admin_config.wechat_work.enabled", status.WeChatWork.Enabled),
		attribute.Bool("admin_config.llm.enabled", status.LLM.Enabled),
	)
	return status, nil
}

func (s *AdminConfigService) agentObservabilityStatus(ctx context.Context) AdminAgentObservabilityStatus {
	status := AdminAgentObservabilityStatus{
		Configured:                s.database != nil,
		EmbeddingWorkerEnabled:    s.cfg.Embedding.Enabled(),
		EmbeddingWorkerConfigured: s.cfg.Embedding.Enabled() && strings.TrimSpace(s.cfg.Embedding.Model) != "",
		EmbeddingModelConfigured:  strings.TrimSpace(s.cfg.Embedding.Model) != "",
	}
	if s.database == nil {
		status.ErrorMessage = "database is not configured"
		return status
	}
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	dbStatus, err := appdb.CheckAgentObservabilityStatus(checkCtx, s.database)
	if err != nil {
		status.ErrorMessage = err.Error()
		return status
	}
	status.Ready = true
	status.TraceEventRows = dbStatus.TraceEventRows
	status.RecallTraceRows = dbStatus.RecallTraceRows
	status.EmbeddingTraceRows = dbStatus.EmbeddingTraceRows
	status.MemoryTopicRows = dbStatus.MemoryTopicRows
	status.MemoryChunkRows = dbStatus.MemoryChunkRows
	status.MemoryChunkReadyRows = dbStatus.MemoryChunkReadyRows
	status.MemoryChunkEmbeddingCoverageRatio = dbStatus.MemoryChunkEmbeddingCoverageRate
	status.PendingEmbeddingJobs = dbStatus.PendingEmbeddingJobs
	status.RunningEmbeddingJobs = dbStatus.RunningEmbeddingJobs
	status.FailedEmbeddingJobs = dbStatus.FailedEmbeddingJobs
	status.LastEmbeddingError = truncateRunes(dbStatus.LastEmbeddingError, 240)
	if dbStatus.LastEmbeddingJobUpdatedAt != nil {
		status.LastEmbeddingJobUpdatedAt = dbStatus.LastEmbeddingJobUpdatedAt.UTC().Format(time.RFC3339)
	}
	s.recordAgentObservabilityMetrics(dbStatus)
	return status
}

func (s *AdminConfigService) recordAgentObservabilityMetrics(status appdb.AgentObservabilityStatus) {
	model := strings.TrimSpace(s.cfg.Embedding.Model)
	if model == "" {
		model = "unknown"
	}
	metrics.AgentEmbeddingQueueDepth.WithLabelValues("pending").Set(float64(status.PendingEmbeddingJobs))
	metrics.AgentEmbeddingQueueDepth.WithLabelValues("running").Set(float64(status.RunningEmbeddingJobs))
	metrics.AgentEmbeddingQueueDepth.WithLabelValues("failed").Set(float64(status.FailedEmbeddingJobs))
	metrics.AgentEmbeddingCoverageRatio.WithLabelValues("memory_chunk", model).Set(status.MemoryChunkEmbeddingCoverageRate)
	metrics.AgentMemoryStaleEmbeddings.WithLabelValues("memory_chunk", model).Set(0)
}

func adminAgentObservabilityMetrics() []AdminObservabilityMetricEntry {
	return []AdminObservabilityMetricEntry{
		{Name: "messagefeed_agent_trace_events_total", Purpose: "Agent waterfall 事件吞吐与状态"},
		{Name: "messagefeed_agent_trace_event_duration_seconds", Purpose: "Agent 内部分段耗时"},
		{Name: "messagefeed_agent_planner_requests_total", Purpose: "主 Agent planner 请求结果"},
		{Name: "messagefeed_agent_subagent_dispatches_total", Purpose: "子 Agent 下发结果"},
		{Name: "messagefeed_agent_tool_executions_total", Purpose: "工具调用次数与状态"},
		{Name: "messagefeed_agent_recall_requests_total", Purpose: "RAG 召回请求与降级"},
		{Name: "messagefeed_agent_recall_duration_seconds", Purpose: "RAG fulltext/vector/relation 分段耗时"},
		{Name: "messagefeed_agent_embedding_requests_total", Purpose: "embedding 模型调用结果"},
		{Name: "messagefeed_agent_embedding_jobs_total", Purpose: "embedding job 生命周期"},
		{Name: "messagefeed_agent_embedding_queue_depth", Purpose: "embedding 队列积压"},
		{Name: "messagefeed_agent_embedding_coverage_ratio", Purpose: "fact/chunk embedding 覆盖率"},
		{Name: "messagefeed_agent_memory_stale_embeddings", Purpose: "content hash 变化后的 stale embedding 数量"},
		{Name: "messagefeed_agent_memory_topics_total", Purpose: "记忆主题生成数量"},
		{Name: "messagefeed_agent_memory_chunks_total", Purpose: "记忆 chunk 生成数量"},
	}
}

func (s *AdminConfigService) weChatWorkOAuthConfigured() bool {
	if s == nil {
		return false
	}
	return strings.TrimSpace(s.cfg.WeChatWork.CorpID) != "" &&
		strings.TrimSpace(s.cfg.WeChatWork.AgentID) != "" &&
		strings.TrimSpace(s.cfg.WeChatWork.Secret) != "" &&
		strings.TrimSpace(s.cfg.Runtime.PublicBaseURL) != ""
}

func (s *AdminConfigService) TestLLM(ctx context.Context, input AdminLLMTestInput) (AdminLLMTestResult, error) {
	if s == nil || s.llmClient == nil || !s.cfg.LLM.Enabled() {
		return AdminLLMTestResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "admin_config_llm_unavailable", "llm client is not configured", "service.admin_config.test_llm", false, nil)
	}
	ctx, span := observability.StartSpan(ctx, "service.admin_config.test_llm",
		attribute.String("llm.provider", s.cfg.LLM.Provider),
		attribute.String("llm.model", s.cfg.LLM.Model),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	message := strings.TrimSpace(input.Message)
	if message == "" {
		message = defaultAdminLLMTestMessage
	}

	startedAt := time.Now()
	response, err := s.llmClient.Chat(ctx, llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: llm.AdminConfigTestSystemPrompt},
			{Role: "user", Content: message},
		},
		Temperature: 0,
		MaxTokens:   adminLLMTestMaxTokens,
	})
	if err != nil {
		opErr = err
		return AdminLLMTestResult{}, err
	}
	result := AdminLLMTestResult{
		Status:       "succeeded",
		Provider:     response.Provider,
		Model:        response.Model,
		LatencyMS:    time.Since(startedAt).Milliseconds(),
		ResponseText: truncateRunes(strings.TrimSpace(response.Content), 240),
		CheckedAt:    s.now().UTC().Format(time.RFC3339),
	}
	span.SetAttributes(attribute.Int64("admin_config.test_latency_ms", result.LatencyMS))
	return result, nil
}

func (s *AdminConfigService) TestWeChatWork(ctx context.Context, input AdminWeChatWorkTestInput) (AdminWeChatWorkTestResult, error) {
	if s == nil || s.weChatWorkSender == nil || !s.cfg.WeChatWork.Enabled() {
		return AdminWeChatWorkTestResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "admin_config_wechat_work_unavailable", "wechat work sender is not configured", "service.admin_config.test_wechat_work", false, nil)
	}
	ctx, span := observability.StartSpan(ctx, "service.admin_config.test_wechat_work")
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	toUser := strings.TrimSpace(input.ToUser)
	if toUser == "" {
		err := fmt.Errorf("%w: to_user is required", domain.ErrInvalidInput)
		opErr = err
		return AdminWeChatWorkTestResult{}, err
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		content = defaultAdminWeChatWorkTestMessage
	}

	startedAt := time.Now()
	sendResult, err := s.weChatWorkSender.SendText(ctx, notifier.WeChatWorkTextMessage{
		ToUser:  toUser,
		Content: content,
	})
	if err != nil {
		opErr = err
		return AdminWeChatWorkTestResult{}, err
	}
	result := AdminWeChatWorkTestResult{
		Status:         "succeeded",
		ErrCode:        sendResult.ErrCode,
		ErrMsg:         sendResult.ErrMsg,
		MessageID:      sendResult.MessageID,
		InvalidUser:    sendResult.InvalidUser,
		InvalidParty:   sendResult.InvalidParty,
		InvalidTag:     sendResult.InvalidTag,
		UnlicensedUser: sendResult.UnlicensedUser,
		LatencyMS:      time.Since(startedAt).Milliseconds(),
		CheckedAt:      s.now().UTC().Format(time.RFC3339),
	}
	span.SetAttributes(
		attribute.Int("wechat_work.errcode", result.ErrCode),
		attribute.Int64("admin_config.test_latency_ms", result.LatencyMS),
	)
	return result, nil
}

func joinPublicURL(baseURL string, path string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	path = "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	if baseURL == "" {
		return path
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return baseURL + path
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func maskConfigValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= 8 {
		return adminMaskedSecretPlaceholder
	}
	return string(runes[:4]) + adminMaskedSecretPlaceholder + string(runes[len(runes)-4:])
}

func truncateRunes(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
