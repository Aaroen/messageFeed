package config

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultBindAddr 是本地单节点开发时的默认监听地址。
	// 该值只控制 HTTP 服务监听位置，不表达部署拓扑；如需局域网或 Tailscale 访问，
	// 应通过 BIND_ADDR 环境变量显式覆盖。
	DefaultBindAddr = "127.0.0.1:60001"

	// DefaultPublicBaseURL 是默认公开访问基址。
	// 该值用于返回给前端、通知内容或运行时节点信息，不一定等同于实际监听地址。
	DefaultPublicBaseURL = "http://127.0.0.1:60001"

	// DefaultAppNodeID 是本地开发默认节点标识。
	// 后续多节点部署时，每个节点应通过 APP_NODE_ID 设置稳定且唯一的节点 ID。
	DefaultAppNodeID = "local-dev"

	// DefaultDeploymentMode 表示第一阶段默认采用单节点部署拓扑。
	// 该值不应被用于推导监听范围,监听范围始终由 BIND_ADDR 决定。
	DefaultDeploymentMode = "single_node"

	// DefaultLogLevel 是默认日志级别。
	// 第一阶段使用 info 级别，便于在本地运行时观察启动、请求和关闭行为。
	DefaultLogLevel = "info"

	// DefaultDatabaseMaxOpenConns 是数据库连接池最大连接数。
	DefaultDatabaseMaxOpenConns = 25

	// DefaultDatabaseMaxIdleConns 是数据库连接池最大空闲连接数。
	DefaultDatabaseMaxIdleConns = 5

	// DefaultDatabaseConnMaxLifetime 是单个数据库连接的最大生命周期（秒）。
	DefaultDatabaseConnMaxLifetime = 3600

	DefaultEnvironment          = "development"
	DefaultObservabilityService = "messagefeed-api"
	DefaultServiceVersion       = "0.2.0"
	DefaultTraceSampleRatio     = 1.0
)

// Config 汇总应用启动所需的基础配置。
// 第一阶段只从环境变量读取配置，后续可以在该结构上扩展配置文件加载与合并逻辑。
type Config struct {
	HTTP          HTTPConfig
	Runtime       RuntimeConfig
	Log           LogConfig
	Database      DatabaseConfig
	Observability ObservabilityConfig
	WeChatWork    WeChatWorkConfig
}

// HTTPConfig 保存 HTTP 服务相关配置。
type HTTPConfig struct {
	// BindAddr 是 HTTP 服务实际监听地址，对应 BIND_ADDR 环境变量。
	// 示例：127.0.0.1:60001、0.0.0.0:60001、100.x.y.z:60001。
	BindAddr string
}

// RuntimeConfig 保存运行时身份、部署拓扑和公开访问地址。
type RuntimeConfig struct {
	// PublicBaseURL 是用户或外部系统访问服务时使用的基址，对应 PUBLIC_BASE_URL。
	PublicBaseURL string

	// AppNodeID 是当前进程的节点标识，对应 APP_NODE_ID。
	AppNodeID string

	// DeploymentMode 表示部署拓扑，对应 DEPLOYMENT_MODE。
	// 当前允许 single_node 和 cluster，第一阶段默认 single_node。
	DeploymentMode string

	// TrustedProxyCIDRs 是可信代理网段列表，对应 TRUSTED_PROXY_CIDRS。
	// 多个 CIDR 使用英文逗号分隔；第一阶段可以为空。
	TrustedProxyCIDRs []string
}

// LogConfig 保存日志相关配置。
type LogConfig struct {
	// Level 是日志级别，对应 LOG_LEVEL。
	// 当前允许 debug、info、warn 和 error。
	Level string
}

// DatabaseConfig 保存数据库连接配置。
type DatabaseConfig struct {
	// DSN 是 PostgreSQL 数据源名称，对应 DATABASE_URL。
	// 格式：postgres://用户名:密码@主机:端口/数据库名?参数
	// 示例：postgres://messagefeed:password@localhost:5432/messagefeed?sslmode=disable
	DSN string

	// MaxOpenConns 是连接池最大连接数，对应 DATABASE_MAX_OPEN_CONNS。
	MaxOpenConns int

	// MaxIdleConns 是连接池最大空闲连接数，对应 DATABASE_MAX_IDLE_CONNS。
	MaxIdleConns int

	// ConnMaxLifetime 是单个连接的最大生命周期，对应 DATABASE_CONN_MAX_LIFETIME。
	ConnMaxLifetime time.Duration
}

// ObservabilityConfig 保存日志、指标和链路追踪所需的运行时配置。
type ObservabilityConfig struct {
	Environment      string
	ServiceName      string
	ServiceVersion   string
	TraceEnabled     bool
	OTLPEndpoint     string
	OTLPInsecure     bool
	TraceSampleRatio float64
}

// WeChatWorkConfig 保存企业微信自建应用接收消息与回复所需配置。
type WeChatWorkConfig struct {
	CorpID         string
	AgentID        string
	Secret         string
	CallbackToken  string
	EncodingAESKey string
}

// Enabled 表示企业微信自建应用回调配置是否已经启用。
func (cfg WeChatWorkConfig) Enabled() bool {
	return cfg.CorpID != "" ||
		cfg.AgentID != "" ||
		cfg.Secret != "" ||
		cfg.CallbackToken != "" ||
		cfg.EncodingAESKey != ""
}

// Load 从环境变量加载配置，并在返回前执行基础校验。
// 当前不读取 YAML、TOML 或 JSON 配置文件，避免第一阶段引入路径、挂载和敏感信息落盘问题。
// 后续如需配置文件，可在 Defaults 和环境变量覆盖之间增加文件配置合并层。
func Load() (Config, error) {
	cfg := Defaults()

	cfg.HTTP.BindAddr = envString("BIND_ADDR", cfg.HTTP.BindAddr)
	cfg.Runtime.PublicBaseURL = envString("PUBLIC_BASE_URL", cfg.Runtime.PublicBaseURL)
	cfg.Runtime.AppNodeID = envString("APP_NODE_ID", cfg.Runtime.AppNodeID)
	cfg.Runtime.DeploymentMode = envString("DEPLOYMENT_MODE", cfg.Runtime.DeploymentMode)
	cfg.Runtime.TrustedProxyCIDRs = envStringList("TRUSTED_PROXY_CIDRS", cfg.Runtime.TrustedProxyCIDRs)
	cfg.Log.Level = strings.ToLower(envString("LOG_LEVEL", cfg.Log.Level))

	cfg.Database.DSN = envString("DATABASE_URL", cfg.Database.DSN)
	cfg.Database.MaxOpenConns = envInt("DATABASE_MAX_OPEN_CONNS", cfg.Database.MaxOpenConns)
	cfg.Database.MaxIdleConns = envInt("DATABASE_MAX_IDLE_CONNS", cfg.Database.MaxIdleConns)
	cfg.Database.ConnMaxLifetime = envDuration("DATABASE_CONN_MAX_LIFETIME", cfg.Database.ConnMaxLifetime)

	cfg.Observability.Environment = envString("ENVIRONMENT", cfg.Observability.Environment)
	cfg.Observability.ServiceName = envString("OTEL_SERVICE_NAME", cfg.Observability.ServiceName)
	cfg.Observability.ServiceVersion = envString("OTEL_SERVICE_VERSION", cfg.Observability.ServiceVersion)
	cfg.Observability.TraceEnabled = envBool("OBSERVABILITY_TRACE_ENABLED", cfg.Observability.TraceEnabled)
	cfg.Observability.OTLPEndpoint = envString("OTEL_EXPORTER_OTLP_ENDPOINT", cfg.Observability.OTLPEndpoint)
	cfg.Observability.OTLPInsecure = envBool("OTEL_EXPORTER_OTLP_INSECURE", cfg.Observability.OTLPInsecure)
	cfg.Observability.TraceSampleRatio = envFloat("OTEL_TRACES_SAMPLER_ARG", cfg.Observability.TraceSampleRatio)

	cfg.WeChatWork.CorpID = envString("WECHAT_WORK_CORP_ID", cfg.WeChatWork.CorpID)
	cfg.WeChatWork.AgentID = envString("WECHAT_WORK_AGENT_ID", cfg.WeChatWork.AgentID)
	cfg.WeChatWork.Secret = envString("WECHAT_WORK_SECRET", cfg.WeChatWork.Secret)
	cfg.WeChatWork.CallbackToken = envString("WECHAT_WORK_CALLBACK_TOKEN", cfg.WeChatWork.CallbackToken)
	cfg.WeChatWork.EncodingAESKey = envString("WECHAT_WORK_ENCODING_AES_KEY", cfg.WeChatWork.EncodingAESKey)

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Defaults 返回第一阶段的代码默认配置。
// 默认值只覆盖本地单节点可启动所需的非敏感配置；数据库密码、Webhook、模型密钥等
// 后续敏感配置不得在此处硬编码。
func Defaults() Config {
	return Config{
		HTTP: HTTPConfig{
			BindAddr: DefaultBindAddr,
		},
		Runtime: RuntimeConfig{
			PublicBaseURL:  DefaultPublicBaseURL,
			AppNodeID:      DefaultAppNodeID,
			DeploymentMode: DefaultDeploymentMode,
		},
		Log: LogConfig{
			Level: DefaultLogLevel,
		},
		Database: DatabaseConfig{
			DSN:             "", // 数据库 DSN 必须通过环境变量提供
			MaxOpenConns:    DefaultDatabaseMaxOpenConns,
			MaxIdleConns:    DefaultDatabaseMaxIdleConns,
			ConnMaxLifetime: DefaultDatabaseConnMaxLifetime * time.Second,
		},
		Observability: ObservabilityConfig{
			Environment:      DefaultEnvironment,
			ServiceName:      DefaultObservabilityService,
			ServiceVersion:   DefaultServiceVersion,
			TraceEnabled:     false,
			OTLPEndpoint:     "",
			OTLPInsecure:     true,
			TraceSampleRatio: DefaultTraceSampleRatio,
		},
		WeChatWork: WeChatWorkConfig{},
	}
}

// Validate 校验配置是否满足当前阶段的最低启动要求。
// 校验逻辑尽量在服务启动前暴露配置错误，避免进入监听阶段后才出现不可诊断的运行失败。
func (cfg Config) Validate() error {
	if err := validateBindAddr(cfg.HTTP.BindAddr); err != nil {
		return fmt.Errorf("invalid BIND_ADDR %q: %w", cfg.HTTP.BindAddr, err)
	}

	publicBaseURL, err := url.Parse(cfg.Runtime.PublicBaseURL)
	if err != nil {
		return fmt.Errorf("invalid PUBLIC_BASE_URL %q: %w", cfg.Runtime.PublicBaseURL, err)
	}
	if publicBaseURL.Scheme == "" || publicBaseURL.Host == "" {
		return fmt.Errorf("invalid PUBLIC_BASE_URL %q: scheme and host are required", cfg.Runtime.PublicBaseURL)
	}

	if cfg.Runtime.AppNodeID == "" {
		return fmt.Errorf("APP_NODE_ID must not be empty")
	}

	switch cfg.Runtime.DeploymentMode {
	case "single_node", "cluster":
	default:
		return fmt.Errorf("unsupported DEPLOYMENT_MODE %q", cfg.Runtime.DeploymentMode)
	}

	if _, ok := slogLevels[cfg.Log.Level]; !ok {
		return fmt.Errorf("unsupported LOG_LEVEL %q", cfg.Log.Level)
	}

	for _, cidr := range cfg.Runtime.TrustedProxyCIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("invalid TRUSTED_PROXY_CIDRS entry %q: %w", cidr, err)
		}
	}

	// 数据库配置校验：DSN 为空时不报错，允许无数据库模式启动（仅用于测试）
	// 生产环境应始终提供 DATABASE_URL
	if cfg.Database.DSN != "" {
		if cfg.Database.MaxOpenConns < 1 {
			return fmt.Errorf("DATABASE_MAX_OPEN_CONNS must be at least 1")
		}
		if cfg.Database.MaxIdleConns < 0 || cfg.Database.MaxIdleConns > cfg.Database.MaxOpenConns {
			return fmt.Errorf("DATABASE_MAX_IDLE_CONNS must be between 0 and DATABASE_MAX_OPEN_CONNS")
		}
		if cfg.Database.ConnMaxLifetime < 0 {
			return fmt.Errorf("DATABASE_CONN_MAX_LIFETIME must be non-negative")
		}
	}

	if strings.TrimSpace(cfg.Observability.Environment) == "" {
		return fmt.Errorf("ENVIRONMENT must not be empty")
	}
	if strings.TrimSpace(cfg.Observability.ServiceName) == "" {
		return fmt.Errorf("OTEL_SERVICE_NAME must not be empty")
	}
	if strings.TrimSpace(cfg.Observability.ServiceVersion) == "" {
		return fmt.Errorf("OTEL_SERVICE_VERSION must not be empty")
	}
	if cfg.Observability.TraceSampleRatio < 0 || cfg.Observability.TraceSampleRatio > 1 {
		return fmt.Errorf("OTEL_TRACES_SAMPLER_ARG must be between 0 and 1")
	}
	if cfg.Observability.TraceEnabled && strings.TrimSpace(cfg.Observability.OTLPEndpoint) == "" {
		return fmt.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT must not be empty when tracing is enabled")
	}

	if err := validateWeChatWorkConfig(cfg.WeChatWork); err != nil {
		return err
	}

	return nil
}

func validateWeChatWorkConfig(cfg WeChatWorkConfig) error {
	if !cfg.Enabled() {
		return nil
	}
	required := map[string]string{
		"WECHAT_WORK_CORP_ID":          cfg.CorpID,
		"WECHAT_WORK_AGENT_ID":         cfg.AgentID,
		"WECHAT_WORK_SECRET":           cfg.Secret,
		"WECHAT_WORK_CALLBACK_TOKEN":   cfg.CallbackToken,
		"WECHAT_WORK_ENCODING_AES_KEY": cfg.EncodingAESKey,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s must not be empty when WeChat Work callback is configured", name)
		}
	}
	if len(cfg.EncodingAESKey) != 43 {
		return fmt.Errorf("WECHAT_WORK_ENCODING_AES_KEY must be 43 characters")
	}
	aesKey, err := base64.StdEncoding.DecodeString(cfg.EncodingAESKey + "=")
	if err != nil {
		return fmt.Errorf("WECHAT_WORK_ENCODING_AES_KEY must be valid base64: %w", err)
	}
	if len(aesKey) != 32 {
		return fmt.Errorf("WECHAT_WORK_ENCODING_AES_KEY must decode to 32 bytes")
	}
	if _, err := strconv.ParseInt(cfg.AgentID, 10, 64); err != nil {
		return fmt.Errorf("WECHAT_WORK_AGENT_ID must be an integer: %w", err)
	}
	return nil
}

// SlogLevel 将配置中的日志级别转换为 slog 可识别的级别。
// 如果调用方绕过 Validate 直接使用该方法，未知日志级别会退回 info，避免日志完全丢失。
func (cfg LogConfig) SlogLevel() slog.Level {
	level, ok := slogLevels[cfg.Level]
	if !ok {
		return slog.LevelInfo
	}
	return level
}

// envString 读取单个字符串环境变量，并在变量为空时返回默认值。
// strings.TrimSpace 用于避免空白字符被误认为有效配置。
func envString(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

// envStringList 读取逗号分隔的字符串列表环境变量。
// 空项会被忽略，返回值会复制默认切片，避免调用方修改默认值底层数组。
func envStringList(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return append([]string(nil), fallback...)
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

// envInt 读取整数环境变量，并在变量为空或解析失败时返回默认值。
func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envFloat(key string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return value
}

// envDuration 读取时长环境变量（秒），并在变量为空或解析失败时返回默认值。
func envDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	seconds, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

// validateBindAddr 校验监听地址必须符合 host:port 形式，并显式限制端口范围。
// net.SplitHostPort 不会校验端口是否超过 65535，因此这里需要额外解析端口数值。
func validateBindAddr(bindAddr string) error {
	_, port, err := net.SplitHostPort(bindAddr)
	if err != nil {
		return err
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("port must be numeric: %w", err)
	}
	if portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	return nil
}

// slogLevels 定义配置文本与 slog 日志级别之间的映射。
// 该映射同时被 Validate 和 SlogLevel 使用，以保证校验与转换规则一致。
var slogLevels = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}
