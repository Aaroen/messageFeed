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
		slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})).Error("notification service stopped with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if err := os.Setenv("APP_ROLE", string(config.AppRoleNotificationWorker)); err != nil {
		return fmt.Errorf("set notification role: %w", err)
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := observability.NewLogger(os.Stdout, cfg.Log.SlogLevel(), cfg)
	tracingShutdown, err := observability.InitTracing(context.Background(), cfg.Observability, cfg.Runtime.AppNodeID)
	if err != nil {
		return fmt.Errorf("initialize tracing: %w", err)
	}
	defer func() { _ = observability.ShutdownWithTimeout(context.Background(), tracingShutdown, 5*time.Second) }()
	application, err := bootstrap.New(cfg, logger)
	if err != nil {
		return fmt.Errorf("initialize notification service: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return application.Run(ctx)
}
