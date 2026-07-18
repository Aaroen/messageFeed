package config

import (
	"log/slog"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	if cfg.HTTP.BindAddr != DefaultBindAddr {
		t.Fatalf("BindAddr = %q, want %q", cfg.HTTP.BindAddr, DefaultBindAddr)
	}
	if cfg.Runtime.PublicBaseURL != DefaultPublicBaseURL {
		t.Fatalf("PublicBaseURL = %q, want %q", cfg.Runtime.PublicBaseURL, DefaultPublicBaseURL)
	}
	if cfg.Runtime.AppNodeID != DefaultAppNodeID {
		t.Fatalf("AppNodeID = %q, want %q", cfg.Runtime.AppNodeID, DefaultAppNodeID)
	}
	if cfg.Runtime.DeploymentMode != DefaultDeploymentMode {
		t.Fatalf("DeploymentMode = %q, want %q", cfg.Runtime.DeploymentMode, DefaultDeploymentMode)
	}
	if cfg.Runtime.AppRole != DefaultAppRole {
		t.Fatalf("AppRole = %q, want %q", cfg.Runtime.AppRole, DefaultAppRole)
	}
	if cfg.HTTP.WorkerMetricsAddr != DefaultWorkerMetricsAddr {
		t.Fatalf("WorkerMetricsAddr = %q, want %q", cfg.HTTP.WorkerMetricsAddr, DefaultWorkerMetricsAddr)
	}
	if cfg.Migrations.Path != DefaultMigrationsPath {
		t.Fatalf("Migrations.Path = %q, want %q", cfg.Migrations.Path, DefaultMigrationsPath)
	}
	if cfg.Log.SlogLevel() != slog.LevelInfo {
		t.Fatalf("SlogLevel = %v, want %v", cfg.Log.SlogLevel(), slog.LevelInfo)
	}
	if cfg.Observability.Environment != DefaultEnvironment {
		t.Fatalf("Environment = %q, want %q", cfg.Observability.Environment, DefaultEnvironment)
	}
	if cfg.Observability.ServiceName != DefaultObservabilityService {
		t.Fatalf("ServiceName = %q, want %q", cfg.Observability.ServiceName, DefaultObservabilityService)
	}
	if cfg.Observability.TraceEnabled {
		t.Fatal("TraceEnabled = true, want false")
	}
	if cfg.Auth.OwnerUsername != DefaultAuthOwnerUsername {
		t.Fatalf("OwnerUsername = %q, want %q", cfg.Auth.OwnerUsername, DefaultAuthOwnerUsername)
	}
	if cfg.Auth.SessionCookie != DefaultAuthSessionCookie {
		t.Fatalf("SessionCookie = %q, want %q", cfg.Auth.SessionCookie, DefaultAuthSessionCookie)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("BIND_ADDR", "0.0.0.0:60002")
	t.Setenv("PUBLIC_BASE_URL", "http://messagefeed.test:60002")
	t.Setenv("APP_NODE_ID", "node-a")
	t.Setenv("DEPLOYMENT_MODE", "cluster")
	t.Setenv("APP_ROLE", "api")
	t.Setenv("DATABASE_URL", "postgres://messagefeed:password@localhost:5432/messagefeed?sslmode=disable")
	t.Setenv("TRUSTED_PROXY_CIDRS", "100.64.0.0/10, 192.168.0.0/16")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("ENVIRONMENT", "test")
	t.Setenv("OTEL_SERVICE_NAME", "messagefeed-test")
	t.Setenv("OTEL_SERVICE_VERSION", "0.2.1")
	t.Setenv("OBSERVABILITY_TRACE_ENABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0.5")
	t.Setenv("AUTH_OWNER_USERNAME", "owner")
	t.Setenv("AUTH_OWNER_PASSWORD", "owner-secret")
	t.Setenv("AUTH_SESSION_COOKIE_NAME", "mf_session")
	t.Setenv("AUTH_SESSION_TTL", "3600")
	t.Setenv("AUTH_SESSION_COOKIE_SECURE", "true")
	t.Setenv("AUTH_OAUTH_STATE_TTL", "300")
	t.Setenv("AUTH_APPROVAL_TOKEN_TTL", "900")
	t.Setenv("WECHAT_WORK_CORP_ID", "ww0123456789abcdef")
	t.Setenv("WECHAT_WORK_AGENT_ID", "1000002")
	t.Setenv("WECHAT_WORK_SECRET", "wechat-work-secret")
	t.Setenv("WECHAT_WORK_CALLBACK_TOKEN", "callback-token")
	t.Setenv("WECHAT_WORK_ENCODING_AES_KEY", "abcdefghijklmnopqrstuvwxyzABCDEFG1234567890")
	t.Setenv("LLM_PROVIDER", "openai_compatible")
	t.Setenv("LLM_API_KEY", "llm-key")
	t.Setenv("LLM_BASE_URL", "https://llm.example/v1")
	t.Setenv("LLM_MODEL", "custom-model")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTP.BindAddr != "0.0.0.0:60002" {
		t.Fatalf("BindAddr = %q", cfg.HTTP.BindAddr)
	}
	if cfg.Runtime.PublicBaseURL != "http://messagefeed.test:60002" {
		t.Fatalf("PublicBaseURL = %q", cfg.Runtime.PublicBaseURL)
	}
	if cfg.Runtime.AppNodeID != "node-a" {
		t.Fatalf("AppNodeID = %q", cfg.Runtime.AppNodeID)
	}
	if cfg.Runtime.DeploymentMode != "cluster" {
		t.Fatalf("DeploymentMode = %q", cfg.Runtime.DeploymentMode)
	}
	if cfg.Runtime.AppRole != AppRoleAPI {
		t.Fatalf("AppRole = %q", cfg.Runtime.AppRole)
	}
	if got, want := len(cfg.Runtime.TrustedProxyCIDRs), 2; got != want {
		t.Fatalf("TrustedProxyCIDRs length = %d, want %d", got, want)
	}
	if cfg.Log.SlogLevel() != slog.LevelDebug {
		t.Fatalf("SlogLevel = %v, want %v", cfg.Log.SlogLevel(), slog.LevelDebug)
	}
	if cfg.Observability.Environment != "test" {
		t.Fatalf("Environment = %q", cfg.Observability.Environment)
	}
	if cfg.Observability.ServiceName != "messagefeed-test" {
		t.Fatalf("ServiceName = %q", cfg.Observability.ServiceName)
	}
	if cfg.Observability.ServiceVersion != "0.2.1" {
		t.Fatalf("ServiceVersion = %q", cfg.Observability.ServiceVersion)
	}
	if !cfg.Observability.TraceEnabled {
		t.Fatal("TraceEnabled = false, want true")
	}
	if cfg.Observability.OTLPEndpoint != "localhost:4317" {
		t.Fatalf("OTLPEndpoint = %q", cfg.Observability.OTLPEndpoint)
	}
	if cfg.Observability.TraceSampleRatio != 0.5 {
		t.Fatalf("TraceSampleRatio = %f, want 0.5", cfg.Observability.TraceSampleRatio)
	}
	if !cfg.WeChatWork.Enabled() {
		t.Fatal("WeChatWork.Enabled() = false, want true")
	}
	if !cfg.Auth.LocalLoginEnabled() {
		t.Fatal("Auth.LocalLoginEnabled() = false, want true")
	}
	if cfg.Auth.SessionCookie != "mf_session" {
		t.Fatalf("SessionCookie = %q", cfg.Auth.SessionCookie)
	}
	if !cfg.Auth.SessionSecure {
		t.Fatal("SessionSecure = false, want true")
	}
	if cfg.WeChatWork.CorpID != "ww0123456789abcdef" {
		t.Fatalf("WeChatWork.CorpID = %q", cfg.WeChatWork.CorpID)
	}
	if cfg.WeChatWork.AgentID != "1000002" {
		t.Fatalf("WeChatWork.AgentID = %q", cfg.WeChatWork.AgentID)
	}
	if !cfg.LLM.Enabled() {
		t.Fatal("LLM.Enabled() = false, want true")
	}
	if cfg.LLM.Provider != "openai_compatible" {
		t.Fatalf("LLM.Provider = %q", cfg.LLM.Provider)
	}
	if cfg.LLM.BaseURL != "https://llm.example/v1" {
		t.Fatalf("LLM.BaseURL = %q", cfg.LLM.BaseURL)
	}
}

func TestLoadRejectsInvalidBindAddr(t *testing.T) {
	t.Setenv("BIND_ADDR", "127.0.0.1:90001")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid BIND_ADDR error")
	}
}

func TestLoadRejectsInvalidDeploymentMode(t *testing.T) {
	t.Setenv("DEPLOYMENT_MODE", "local")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid DEPLOYMENT_MODE error")
	}
}

func TestLoadRejectsAllRoleInCluster(t *testing.T) {
	t.Setenv("DEPLOYMENT_MODE", "cluster")
	t.Setenv("DATABASE_URL", "postgres://messagefeed:password@localhost:5432/messagefeed?sslmode=disable")
	t.Setenv("APP_ROLE", "all")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want cluster all-role rejection")
	}
}

func TestLoadAllowsExplicitAllRoleInCluster(t *testing.T) {
	t.Setenv("DEPLOYMENT_MODE", "cluster")
	t.Setenv("DATABASE_URL", "postgres://messagefeed:password@localhost:5432/messagefeed?sslmode=disable")
	t.Setenv("APP_ROLE", "all")
	t.Setenv("ALLOW_ALL_ROLE_IN_CLUSTER", "true")

	if _, err := Load(); err != nil {
		t.Fatalf("Load() error = %v, want explicit compatibility allowance", err)
	}
}

func TestLoadRejectsUnsafeMigrationsPath(t *testing.T) {
	t.Setenv("MIGRATIONS_PATH", "../migrations")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want unsafe migrations path rejection")
	}
}

func TestLoadRejectsInvalidTrustedProxyCIDR(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "not-a-cidr")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid TRUSTED_PROXY_CIDRS error")
	}
}

func TestLoadRejectsEnabledTraceWithoutEndpoint(t *testing.T) {
	t.Setenv("OBSERVABILITY_TRACE_ENABLED", "true")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing OTLP endpoint error")
	}
}

func TestLoadRejectsInvalidTraceSampleRatio(t *testing.T) {
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "1.1")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid trace sampler error")
	}
}

func TestLoadRejectsInvalidAuthCookieName(t *testing.T) {
	t.Setenv("AUTH_SESSION_COOKIE_NAME", "bad cookie")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid auth cookie name error")
	}
}

func TestLoadRejectsInvalidAuthTTL(t *testing.T) {
	t.Setenv("AUTH_SESSION_TTL", "0")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid auth ttl error")
	}
}

func TestLoadRejectsPartialWeChatWorkConfig(t *testing.T) {
	t.Setenv("WECHAT_WORK_CORP_ID", "ww0123456789abcdef")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want partial WeChat Work config error")
	}
}

func TestLoadRejectsInvalidWeChatWorkEncodingAESKey(t *testing.T) {
	t.Setenv("WECHAT_WORK_CORP_ID", "ww0123456789abcdef")
	t.Setenv("WECHAT_WORK_AGENT_ID", "1000002")
	t.Setenv("WECHAT_WORK_SECRET", "wechat-work-secret")
	t.Setenv("WECHAT_WORK_CALLBACK_TOKEN", "callback-token")
	t.Setenv("WECHAT_WORK_ENCODING_AES_KEY", "short")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid WeChat Work encoding aes key error")
	}
}

func TestLoadRejectsPartialLLMConfig(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "openai_compatible")
	t.Setenv("LLM_API_KEY", "llm-key")
	t.Setenv("LLM_MODEL", "custom-model")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want partial LLM config error")
	}
}

func TestLoadAcceptsCustomLLMProvider(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "hyb")
	t.Setenv("LLM_API_KEY", "llm-key")
	t.Setenv("LLM_BASE_URL", "https://llm.example/v1")
	t.Setenv("LLM_MODEL", "custom-model")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LLM.Provider != "hyb" {
		t.Fatalf("LLM.Provider = %q", cfg.LLM.Provider)
	}
}

func TestLoadRejectsInvalidLLMProviderName(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "unknown/provider")
	t.Setenv("LLM_API_KEY", "llm-key")
	t.Setenv("LLM_BASE_URL", "https://llm.example/v1")
	t.Setenv("LLM_MODEL", "custom-model")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid LLM provider error")
	}
}
