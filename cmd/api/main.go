package main

import (
	"context"
	"fmt"
	"log/slog"
	"messagefeed/internal/bootstrap"
	"messagefeed/internal/config"
	"messagefeed/internal/observability"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})).
			Error("messagefeed stopped with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := observability.NewLogger(os.Stdout, cfg.Log.SlogLevel(), cfg)
	tracingShutdown, err := observability.InitTracing(context.Background(), cfg.Observability, cfg.Runtime.AppNodeID)
	if err != nil {
		return fmt.Errorf("initialize tracing: %w", err)
	}
	defer func() {
		if shutdownErr := observability.ShutdownWithTimeout(context.Background(), tracingShutdown, 5*time.Second); shutdownErr != nil {
			logger.Error("shutdown tracing failed", "error", shutdownErr)
		}
	}()

	logger.Info(
		"configuration loaded",
		"bind_addr", cfg.HTTP.BindAddr,
		"worker_metrics_addr", cfg.HTTP.WorkerMetricsAddr,
		"public_base_url", cfg.Runtime.PublicBaseURL,
		"database_configured", cfg.Database.DSN != "",
		"wechat_work_configured", cfg.WeChatWork.Enabled(),
		"llm_configured", cfg.LLM.Enabled(),
		"embedding_configured", cfg.Embedding.Enabled(),
		"tracing_enabled", cfg.Observability.TraceEnabled,
		"otel_endpoint", cfg.Observability.OTLPEndpoint,
	)

	application, err := bootstrap.New(cfg, logger)
	if err != nil {
		return fmt.Errorf("initialize application: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := application.Run(ctx); err != nil {
		return fmt.Errorf("run application: %w", err)
	}
	return nil
}
