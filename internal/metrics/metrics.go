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

// LLMRequestsTotal 记录大模型请求总数，按模型和操作分类。
var LLMRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_llm_requests_total",
		Help: "大模型请求总数，按模型和操作分类",
	},
	[]string{"provider", "model", "operation"},
)

// LLMRequestDuration 记录大模型请求耗时分布（秒）。
var LLMRequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagefeed_llm_request_duration_seconds",
		Help:    "大模型请求耗时分布（秒）",
		Buckets: []float64{0.5, 1.0, 2.0, 5.0, 10.0, 20.0, 30.0, 60.0, 120.0},
	},
	[]string{"provider", "model", "operation"},
)

// LLMTokensTotal 记录大模型 token 消耗总数，按模型和类型（input、output）分类。
var LLMTokensTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagefeed_llm_tokens_total",
		Help: "大模型 token 消耗总数，按模型和类型（input、output）分类",
	},
	[]string{"provider", "model", "token_type"},
)
