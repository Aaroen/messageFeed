package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Registry 是全局 Prometheus 指标注册表。
// 第一阶段使用默认注册表，后续可扩展为自定义注册表以支持多租户或测试隔离。
var Registry = prometheus.DefaultRegisterer

// Gatherer 是全局 Prometheus 指标采集器。
var Gatherer = prometheus.DefaultGatherer

// ==================== HTTP 请求指标 ====================

// HTTPRequestsTotal 记录 HTTP 请求总数，按方法、路径和状态码分类。
var HTTPRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_http_requests_total",
		Help: "HTTP 请求总数，按方法、路径和状态码分类",
	},
	[]string{"method", "path", "status"},
)

// HTTPRequestDuration 记录 HTTP 请求耗时分布（秒），按方法和路径分类。
// 使用直方图记录耗时分位数（p50、p90、p99）。
var HTTPRequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_http_request_duration_seconds",
		Help:    "HTTP 请求耗时分布（秒），按方法和路径分类",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
	},
	[]string{"method", "path"},
)

// HTTPRequestSize 记录 HTTP 请求体大小分布（字节），按方法和路径分类。
var HTTPRequestSize = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_http_request_size_bytes",
		Help:    "HTTP 请求体大小分布（字节），按方法和路径分类",
		Buckets: prometheus.ExponentialBuckets(100, 10, 8), // 100B 到 10MB
	},
	[]string{"method", "path"},
)

// HTTPResponseSize 记录 HTTP 响应体大小分布（字节），按方法和路径分类。
var HTTPResponseSize = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_http_response_size_bytes",
		Help:    "HTTP 响应体大小分布（字节），按方法和路径分类",
		Buckets: prometheus.ExponentialBuckets(100, 10, 8), // 100B 到 10MB
	},
	[]string{"method", "path"},
)

// ==================== 数据库指标 ====================

// DatabaseConnections 记录数据库连接池状态，按状态分类（open、in_use、idle）。
var DatabaseConnections = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "messagefeed_database_connections",
		Help: "数据库连接池状态，按状态分类（open、in_use、idle）",
	},
	[]string{"state"},
)

// DatabaseWaitCount 记录数据库连接池等待连接累计次数。
var DatabaseWaitCount = promauto.NewGauge(
	prometheus.GaugeOpts{
		Name: "messagefeed_database_wait_count",
		Help: "数据库连接池等待连接累计次数",
	},
)

// DatabaseWaitDurationSeconds 记录数据库连接池等待连接累计耗时。
var DatabaseWaitDurationSeconds = promauto.NewGauge(
	prometheus.GaugeOpts{
		Name: "messagefeed_database_wait_duration_seconds",
		Help: "数据库连接池等待连接累计耗时（秒）",
	},
)

// DatabaseQueriesTotal 记录数据库查询总数，按操作类型和表名分类。
var DatabaseQueriesTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_database_queries_total",
		Help: "数据库查询总数，按操作类型和表名分类",
	},
	[]string{"operation", "table"},
)

// DatabaseQueryDuration 记录数据库查询耗时分布（秒），按操作类型和表名分类。
var DatabaseQueryDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_database_query_duration_seconds",
		Help:    "数据库查询耗时分布（秒），按操作类型和表名分类",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
	},
	[]string{"operation", "table"},
)

// ==================== 应用级指标（预留，后续阶段填充）====================

// FeedFetchesTotal 记录 RSS Feed 抓取总数，按来源和状态分类。
var FeedFetchesTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_feed_fetches_total",
		Help: "RSS Feed 抓取总数，按来源和状态分类",
	},
	[]string{"source", "status"},
)

// FeedFetchDuration 记录 RSS Feed 抓取耗时分布（秒）。
var FeedFetchDuration = promauto.NewHistogram(
	prometheus.HistogramOpts{
		Name:    "messagefeed_feed_fetch_duration_seconds",
		Help:    "RSS Feed 抓取耗时分布（秒）",
		Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0},
	},
)

// FeedFetchItemsTotal 记录抓取后解析出的条目总数，按来源和写入结果分类。
var FeedFetchItemsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_feed_fetch_items_total",
		Help: "抓取后解析出的条目总数，按来源和写入结果分类",
	},
	[]string{"source", "result"},
)

// ExternalHTTPRequestsTotal 记录外部 HTTP 调用次数。
var ExternalHTTPRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_external_http_requests_total",
		Help: "外部 HTTP 调用次数，按操作、主机和状态分类",
	},
	[]string{"operation", "host", "status"},
)

// ExternalHTTPRequestDuration 记录外部 HTTP 调用耗时。
var ExternalHTTPRequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_external_http_request_duration_seconds",
		Help:    "外部 HTTP 调用耗时分布（秒）",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0},
	},
	[]string{"operation", "host"},
)

// NotificationsTotal 记录通知发送总数，按通道和状态分类。
var NotificationsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_notifications_total",
		Help: "通知发送总数，按通道和状态分类",
	},
	[]string{"channel", "status"},
)

// NotificationDuration 记录通知发送耗时，按通道和状态分类。
var NotificationDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_notification_duration_seconds",
		Help:    "通知发送耗时分布（秒），按通道和状态分类",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0},
	},
	[]string{"channel", "status"},
)

// WeChatWorkCallbacksTotal 记录企业微信回调处理数，按操作和状态分类。
var WeChatWorkCallbacksTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_wechat_work_callbacks_total",
		Help: "企业微信回调处理总数，按操作和状态分类",
	},
	[]string{"operation", "status"},
)

// WeChatWorkCallbackDuration 记录企业微信回调处理耗时。
var WeChatWorkCallbackDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_wechat_work_callback_duration_seconds",
		Help:    "企业微信回调处理耗时分布（秒），按操作和状态分类",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
	},
	[]string{"operation", "status"},
)

// AgentInboundMessagesTotal 记录 Agent 入站消息数，按 provider、消息类型和处理状态分类。
var AgentInboundMessagesTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_agent_inbound_messages_total",
		Help: "Agent 入站消息总数，按 provider、消息类型和处理状态分类",
	},
	[]string{"provider", "msg_type", "status"},
)

// AgentTurnsTotal 记录 Agent turn 处理结果数，按 provider 和状态分类。
var AgentTurnsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_agent_turns_total",
		Help: "Agent turn 处理总数，按 provider 和状态分类",
	},
	[]string{"provider", "status"},
)

// AgentTurnDuration 记录 Agent turn 总耗时。
var AgentTurnDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_agent_turn_duration_seconds",
		Help:    "Agent turn 总耗时分布（秒），按 provider 和状态分类",
		Buckets: []float64{0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 20.0, 30.0, 60.0},
	},
	[]string{"provider", "status"},
)

// AgentReplyChunksTotal 记录 Agent 回复发送分片数。
var AgentReplyChunksTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_agent_reply_chunks_total",
		Help: "Agent 回复发送分片总数，按 provider 和状态分类",
	},
	[]string{"provider", "status"},
)

// AgentReplyBytes 记录 Agent 回复内容大小。
var AgentReplyBytes = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_agent_reply_bytes",
		Help:    "Agent 回复内容字节数分布，按 provider 和状态分类",
		Buckets: []float64{100, 250, 500, 1000, 1500, 2048, 4096, 8192, 16384},
	},
	[]string{"provider", "status"},
)

// AgentTraceEventsTotal 记录 Agent 内部 waterfall 事件数。
var AgentTraceEventsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_agent_trace_events_total",
		Help: "Agent 内部 waterfall 事件总数，按事件类型和状态分类",
	},
	[]string{"event_kind", "status"},
)

// AgentTraceEventDuration 记录 Agent 内部 waterfall 事件耗时。
var AgentTraceEventDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_agent_trace_event_duration_seconds",
		Help:    "Agent 内部 waterfall 事件耗时分布（秒），按事件类型和状态分类",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0, 120.0},
	},
	[]string{"event_kind", "status"},
)

// AgentPlannerRequestsTotal 记录主 Agent planner 结果。
var AgentPlannerRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_agent_planner_requests_total",
		Help: "主 Agent planner 请求总数，按状态、历史召回需求和审批需求分类",
	},
	[]string{"status", "needs_history_recall", "needs_approval"},
)

// AgentTaskRoutesTotal 记录主 Agent 任务分级结果。
var AgentTaskRoutesTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_agent_task_routes_total",
		Help: "主 Agent 任务分级总数，按任务类型、状态和预估延迟分类",
	},
	[]string{"task_type", "status", "latency_class"},
)

// AgentTaskRouteDuration 记录主 Agent 任务分级耗时。
var AgentTaskRouteDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_agent_task_route_duration_seconds",
		Help:    "主 Agent 任务分级耗时分布（秒），按任务类型、状态和预估延迟分类",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
	},
	[]string{"task_type", "status", "latency_class"},
)

// AgentSubagentDispatchesTotal 记录子 Agent 下发结果。
var AgentSubagentDispatchesTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_agent_subagent_dispatches_total",
		Help: "子 Agent 下发总数，按 capability 和状态分类",
	},
	[]string{"capability", "status"},
)

// AgentToolExecutionsTotal 记录 Agent 工具执行结果。
var AgentToolExecutionsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_agent_tool_executions_total",
		Help: "Agent 工具执行总数，按 capability、工具和状态分类",
	},
	[]string{"capability", "tool", "status"},
)

// AgentToolExecutionDuration 记录 Agent 工具执行耗时。
var AgentToolExecutionDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_agent_tool_execution_duration_seconds",
		Help:    "Agent 工具执行耗时分布（秒），按 capability、工具和状态分类",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0},
	},
	[]string{"capability", "tool", "status"},
)

// AgentApprovalsTotal 记录审批与治理决策结果。
var AgentApprovalsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_agent_approvals_total",
		Help: "Agent 审批与治理决策总数，按决策和风险级别分类",
	},
	[]string{"decision", "risk_level"},
)

// AgentRecallRequestsTotal 记录 Agent 历史召回请求结果。
var AgentRecallRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_agent_recall_requests_total",
		Help: "Agent 历史召回请求总数，按模式、状态和降级原因分类",
	},
	[]string{"mode", "status", "fallback_reason"},
)

// AgentRecallDuration 记录 Agent 历史召回分段耗时。
var AgentRecallDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_agent_recall_duration_seconds",
		Help:    "Agent 历史召回分段耗时分布（秒），按模式、阶段和状态分类",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0},
	},
	[]string{"mode", "stage", "status"},
)

// AgentRecallHits 记录 Agent 历史召回命中数。
var AgentRecallHits = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_agent_recall_hits",
		Help:    "Agent 历史召回命中数分布，按模式和来源分类",
		Buckets: []float64{0, 1, 2, 3, 5, 8, 13, 21, 34},
	},
	[]string{"mode", "source"},
)

// AgentMemoryTopicsTotal 记录 Agent memory topic 创建与关闭。
var AgentMemoryTopicsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_agent_memory_topics_total",
		Help: "Agent memory topic 事件总数，按状态和原因分类",
	},
	[]string{"status", "reason"},
)

// AgentMemoryChunksTotal 记录 Agent memory chunk 形成结果。
var AgentMemoryChunksTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_agent_memory_chunks_total",
		Help: "Agent memory chunk 事件总数，按记忆类型、原因和状态分类",
	},
	[]string{"memory_kind", "reason", "status"},
)

// AgentEmbeddingRequestsTotal 记录 Agent embedding 请求结果。
var AgentEmbeddingRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_agent_embedding_requests_total",
		Help: "Agent embedding 请求总数，按 provider、模型、操作和状态分类",
	},
	[]string{"provider", "model", "operation", "status"},
)

// AgentEmbeddingDuration 记录 Agent embedding 请求耗时。
var AgentEmbeddingDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_agent_embedding_duration_seconds",
		Help:    "Agent embedding 请求耗时分布（秒），按 provider、模型、操作和状态分类",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0},
	},
	[]string{"provider", "model", "operation", "status"},
)

// AgentEmbeddingBatchSize 记录 Agent embedding 批大小。
var AgentEmbeddingBatchSize = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_agent_embedding_batch_size",
		Help:    "Agent embedding 批大小分布，按 provider、模型和操作分类",
		Buckets: []float64{1, 2, 4, 8, 16, 32, 64, 128},
	},
	[]string{"provider", "model", "operation"},
)

// AgentEmbeddingInputChars 记录 Agent embedding 输入字符数。
var AgentEmbeddingInputChars = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_agent_embedding_input_chars",
		Help:    "Agent embedding 输入字符数分布，按 provider、模型和操作分类",
		Buckets: []float64{32, 128, 512, 1024, 2048, 4096, 8192, 16384, 32768},
	},
	[]string{"provider", "model", "operation"},
)

// AgentEmbeddingJobsTotal 记录 embedding job 处理结果。
var AgentEmbeddingJobsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_agent_embedding_jobs_total",
		Help: "Agent embedding job 处理总数，按状态和原因分类",
	},
	[]string{"status", "reason"},
)

// AgentEmbeddingJobDuration 记录 embedding job 处理耗时。
var AgentEmbeddingJobDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_agent_embedding_job_duration_seconds",
		Help:    "Agent embedding job 处理耗时分布（秒），按状态分类",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0, 180.0},
	},
	[]string{"status"},
)

// AgentEmbeddingQueueDepth 记录 embedding job 队列深度。
var AgentEmbeddingQueueDepth = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "messagefeed_agent_embedding_queue_depth",
		Help: "Agent embedding job 队列深度，按状态分类",
	},
	[]string{"status"},
)

// AgentEmbeddingCoverageRatio 记录 fact/chunk embedding 覆盖率。
var AgentEmbeddingCoverageRatio = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "messagefeed_agent_embedding_coverage_ratio",
		Help: "Agent fact/chunk embedding 覆盖率，按 fact_type 和模型分类",
	},
	[]string{"fact_type", "embedding_model"},
)

// AgentMemoryStaleEmbeddings 记录 content hash 已变化的 stale embedding 数量。
var AgentMemoryStaleEmbeddings = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "messagefeed_agent_memory_stale_embeddings",
		Help: "Agent stale embedding 数量，按 fact_type 和模型分类",
	},
	[]string{"fact_type", "embedding_model"},
)

// LLMRequestsTotal 记录大模型请求总数，按模型、操作和状态分类。
var LLMRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_llm_requests_total",
		Help: "大模型请求总数，按模型、操作和状态分类",
	},
	[]string{"provider", "model", "operation", "status"},
)

// LLMRequestDuration 记录大模型请求耗时分布（秒），按模型、操作和状态分类。
var LLMRequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_llm_request_duration_seconds",
		Help:    "大模型请求耗时分布（秒），按模型、操作和状态分类",
		Buckets: []float64{0.5, 1.0, 2.0, 5.0, 10.0, 20.0, 30.0, 60.0, 120.0},
	},
	[]string{"provider", "model", "operation", "status"},
)

// LLMTokensTotal 记录大模型 token 消耗总数，按模型和类型（input、output）分类。
var LLMTokensTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_llm_tokens_total",
		Help: "大模型 token 消耗总数，按模型和类型（input、output）分类",
	},
	[]string{"provider", "model", "token_type"},
)
