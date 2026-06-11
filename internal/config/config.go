package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultBindAddr       = "127.0.0.1:60001"
	DefaultPublicBaseURL  = "http://127.0.0.1:60001"
	DefaultAppNodeID      = "local-dev"
	DefaultDeploymentMode = "single_node"
	DefaultLogLevel       = "info"
)

// Config 汇总应用启动所需的基础配置。
// 第一阶段只从环境变量读取配置，后续可以在该结构上扩展配置文件加载与合并逻辑。
type Config struct {
	HTTP    HTTPConfig
	Runtime RuntimeConfig
	Log     LogConfig
}

// HTTPConfig 保存 HTTP 服务相关配置。
type HTTPConfig struct {
	BindAddr string
}

// RuntimeConfig 保存运行时身份、部署拓扑和公开访问地址。
type RuntimeConfig struct {
	PublicBaseURL     string
	AppNodeID         string
	DeploymentMode    string
	TrustedProxyCIDRs []string
}

// LogConfig 保存日志相关配置。
type LogConfig struct {
	Level string
}

// Load 从环境变量加载配置，并在返回前执行基础校验。
func Load() (Config, error) {
	cfg := Defaults()

	cfg.HTTP.BindAddr = envString("BIND_ADDR", cfg.HTTP.BindAddr)
	cfg.Runtime.PublicBaseURL = envString("PUBLIC_BASE_URL", cfg.Runtime.PublicBaseURL)
	cfg.Runtime.AppNodeID = envString("APP_NODE_ID", cfg.Runtime.AppNodeID)
	cfg.Runtime.DeploymentMode = envString("DEPLOYMENT_MODE", cfg.Runtime.DeploymentMode)
	cfg.Runtime.TrustedProxyCIDRs = envStringList("TRUSTED_PROXY_CIDRS", cfg.Runtime.TrustedProxyCIDRs)
	cfg.Log.Level = strings.ToLower(envString("LOG_LEVEL", cfg.Log.Level))

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Defaults 返回第一阶段的代码默认配置。
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
	}
}

// Validate 校验配置是否满足当前阶段的最低启动要求。
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

	return nil
}

// SlogLevel 将配置中的日志级别转换为 slog 可识别的级别。
func (cfg LogConfig) SlogLevel() slog.Level {
	level, ok := slogLevels[cfg.Level]
	if !ok {
		return slog.LevelInfo
	}
	return level
}

func envString(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

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

var slogLevels = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}
