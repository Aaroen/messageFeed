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
	if cfg.Log.SlogLevel() != slog.LevelInfo {
		t.Fatalf("SlogLevel = %v, want %v", cfg.Log.SlogLevel(), slog.LevelInfo)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("BIND_ADDR", "0.0.0.0:60002")
	t.Setenv("PUBLIC_BASE_URL", "http://messagefeed.test:60002")
	t.Setenv("APP_NODE_ID", "node-a")
	t.Setenv("DEPLOYMENT_MODE", "cluster")
	t.Setenv("TRUSTED_PROXY_CIDRS", "100.64.0.0/10, 192.168.0.0/16")
	t.Setenv("LOG_LEVEL", "debug")

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
	if got, want := len(cfg.Runtime.TrustedProxyCIDRs), 2; got != want {
		t.Fatalf("TrustedProxyCIDRs length = %d, want %d", got, want)
	}
	if cfg.Log.SlogLevel() != slog.LevelDebug {
		t.Fatalf("SlogLevel = %v, want %v", cfg.Log.SlogLevel(), slog.LevelDebug)
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

func TestLoadRejectsInvalidTrustedProxyCIDR(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "not-a-cidr")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid TRUSTED_PROXY_CIDRS error")
	}
}
