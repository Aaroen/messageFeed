package main

import (
	"context"
	"errors"
	"log/slog"
	"messagefeed/internal/config"
	"messagefeed/internal/db"
	"messagefeed/internal/fetcher"
	"messagefeed/internal/handler"
	"messagefeed/internal/metrics"
	"messagefeed/internal/observability"
	"messagefeed/internal/repository"
	appRuntime "messagefeed/internal/runtime"
	"messagefeed/internal/service"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gorm.io/gorm"
)

func main() {
	// 启动初期先使用 info 级别日志，以便在配置加载失败时仍能输出结构化错误。
	bootstrapLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// 配置模块统一负责默认值、环境变量覆盖和基础校验。
	// 入口层只使用已经校验过的配置，避免各处重复读取环境变量。
	cfg, err := config.Load()
	if err != nil {
		bootstrapLogger.Error("load config failed", "error", err)
		os.Exit(1)
	}

	logger := observability.NewLogger(os.Stdout, cfg.Log.SlogLevel(), cfg)

	observabilityShutdown, err := observability.InitTracing(context.Background(), cfg.Observability, cfg.Runtime.AppNodeID)
	if err != nil {
		logger.Error("initialize tracing failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := observability.ShutdownWithTimeout(context.Background(), observabilityShutdown, 5*time.Second); err != nil {
			logger.Error("shutdown tracing failed", "error", err)
		}
	}()

	logger.Info(
		"configuration loaded",
		"bind_addr", cfg.HTTP.BindAddr,
		"public_base_url", cfg.Runtime.PublicBaseURL,
		"app_node_id", cfg.Runtime.AppNodeID,
		"deployment_mode", cfg.Runtime.DeploymentMode,
		"database_configured", cfg.Database.DSN != "",
		"tracing_enabled", cfg.Observability.TraceEnabled,
		"otel_endpoint", cfg.Observability.OTLPEndpoint,
	)

	// 数据库连接（可选）
	// 当 DATABASE_URL 未配置时，database 为 nil，服务仍可启动但 /readyz 不检查数据库。
	var database *gorm.DB
	var sourceService *service.SourceService
	var timelineService *service.TimelineService
	var recommendationService *service.RecommendationService
	var itemService *service.ItemService
	var feedViewService *service.FeedViewService
	var sourceSyncService *service.SourceSyncService
	backgroundCtx, cancelBackground := context.WithCancel(context.Background())
	defer cancelBackground()
	if cfg.Database.DSN != "" {
		dbCfg := db.Config{
			DSN:             cfg.Database.DSN,
			MaxOpenConns:    cfg.Database.MaxOpenConns,
			MaxIdleConns:    cfg.Database.MaxIdleConns,
			ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
			Logger:          logger,
		}

		database, err = db.Open(dbCfg)
		if err != nil {
			logger.Error("failed to open database", "error", err)
			os.Exit(1)
		}

		// 测试数据库连接
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := db.Ping(ctx, database); err != nil {
			cancel()
			logger.Error("failed to ping database", "error", err)
			os.Exit(1)
		}
		cancel()

		logger.Info("database connected",
			"max_open_conns", cfg.Database.MaxOpenConns,
			"max_idle_conns", cfg.Database.MaxIdleConns,
		)

		sourceRepository := repository.NewSourceRepository(database)
		sourceCatalogRepository := repository.NewSourceCatalogRepository(database)
		sourceImportJobRepository := repository.NewSourceImportJobRepository(database)
		itemRepository := repository.NewItemRepository(database)
		userItemStateRepository := repository.NewUserItemStateRepository(database)
		feedViewPreferenceRepository := repository.NewFeedViewPreferenceRepository(database)
		sourceFetchJobRepository := repository.NewSourceFetchJobRepository(database)
		itemEventRepository := repository.NewItemEventRepository(database)
		taskLockRepository := repository.NewTaskLockRepository(database)
		feedFetcher := fetcher.NewClient()
		sourceService = service.NewSourceService(
			sourceRepository,
			service.WithSourceCatalogRepository(sourceCatalogRepository),
			service.WithSourceImportJobRepository(sourceImportJobRepository),
			service.WithSourceFetchJobRepository(sourceFetchJobRepository),
			service.WithItemRepository(itemRepository),
			service.WithFeedFetcher(feedFetcher),
		)
		sourceSyncService = service.NewSourceSyncService(
			sourceRepository,
			itemRepository,
			feedFetcher,
			sourceFetchJobRepository,
			itemEventRepository,
			service.WithSourceSyncTaskLocker(taskLockRepository),
		)
		timelineService = service.NewTimelineService(itemRepository)
		recommendationService = service.NewRecommendationService(sourceCatalogRepository, feedFetcher)
		recommendationService.SetLocalHistoryRepositories(sourceRepository, itemRepository)
		itemService = service.NewItemService(userItemStateRepository)
		feedViewService = service.NewFeedViewService(feedViewPreferenceRepository)

		// 启动数据库连接池指标采集器
		go collectDatabaseMetrics(database, logger)
		go runSourceSyncWorker(backgroundCtx, logger, cfg.Runtime.AppNodeID, sourceSyncService)
	} else {
		logger.Warn("database not configured, running in database-less mode")
	}

	// 节点信息在启动时构建为快照。
	// 后续 /api/runtime/node 直接返回该快照，避免每次请求重新读取环境变量。
	nodeInfo := appRuntime.NewNodeInfo(appRuntime.NodeOptions{
		NodeID:            cfg.Runtime.AppNodeID,
		DeploymentMode:    cfg.Runtime.DeploymentMode,
		PublicBaseURL:     cfg.Runtime.PublicBaseURL,
		BindAddr:          cfg.HTTP.BindAddr,
		TrustedProxyCIDRs: cfg.Runtime.TrustedProxyCIDRs,
		StartedAt:         time.Now().UTC(),
	})

	router := handler.NewRouter(handler.RouterOptions{
		Logger:                logger,
		Database:              database,
		NodeInfo:              nodeInfo,
		Now:                   time.Now,
		SourceService:         sourceService,
		TimelineService:       timelineService,
		RecommendationService: recommendationService,
		ItemService:           itemService,
		FeedViewService:       feedViewService,
		ServiceName:           cfg.Observability.ServiceName,
	})

	// ReadHeaderTimeout 用于限制客户端长期占用连接但不完整发送请求头的情况。
	// Gin 路由树内部已经装配 request id、访问日志、错误恢复和指标中间件。
	server := &http.Server{
		Addr:              cfg.HTTP.BindAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// ListenAndServe 在独立 goroutine 中运行，使主 goroutine 可以同时监听系统信号。
	// errCh 使用缓冲通道，避免服务在 select 开始前失败时阻塞错误上报。
	errCh := make(chan error, 1)
	go func() {
		logger.Info("api server starting", "bind_addr", cfg.HTTP.BindAddr)
		errCh <- server.ListenAndServe()
	}()

	// 捕获 Ctrl+C 和容器停止信号。
	// SIGTERM 对后续 Docker Compose 场景尤其重要，因为容器通常先收到该信号，
	// 随后才会进入强制终止流程。
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	// 当前最小入口只处理两类退出原因：
	// 一是收到关闭信号，二是服务启动或运行失败。
	// http.ErrServerClosed 属于优雅关闭的预期结果，不作为异常处理。
	select {
	case sig := <-stopCh:
		logger.Info("api server stopping", "signal", sig.String())
		cancelBackground()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdown(ctx, logger, server, database)
	case err := <-errCh:
		cancelBackground()
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api server failed", "error", err)
			if database != nil {
				_ = db.Close(database)
			}
			os.Exit(1)
		}
	}
}

func runSourceSyncWorker(ctx context.Context, logger *slog.Logger, workerID string, sourceSyncService *service.SourceSyncService) {
	if sourceSyncService == nil {
		return
	}
	if workerID == "" {
		workerID = "api"
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		runCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
		result, err := sourceSyncService.RunOnce(runCtx, service.RunSourceSyncOnceInput{
			WorkerID:           workerID,
			LockName:           "source-sync",
			LockTTL:            30 * time.Second,
			EnqueueLimit:       50,
			ClaimLimit:         5,
			DefaultMaxAttempts: 3,
		})
		cancel()
		if err != nil && ctx.Err() == nil {
			logger.Warn("source sync worker run failed", "error", err)
		} else if result.ClaimedCount > 0 || result.EnqueuedCount > 0 {
			logger.Info(
				"source sync worker run completed",
				"enqueued", result.EnqueuedCount,
				"claimed", result.ClaimedCount,
				"success", result.SuccessCount,
				"failed", result.FailureCount,
				"retry", result.RetryCount,
			)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// shutdown 为正在处理的请求保留一个短暂的完成窗口。
// 该生命周期模式后续会扩展到数据库连接、调度器和通知发送器等资源清理。
func shutdown(ctx context.Context, logger *slog.Logger, server *http.Server, database *gorm.DB) {
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("api server shutdown failed", "error", err)
	} else {
		logger.Info("api server stopped")
	}

	// 关闭数据库连接池
	if database != nil {
		if err := db.Close(database); err != nil {
			logger.Error("database close failed", "error", err)
		} else {
			logger.Info("database closed")
		}
	}
}

// collectDatabaseMetrics 定期采集数据库连接池指标并上报到 Prometheus。
// 该函数在独立 goroutine 中运行，每 15 秒更新一次指标。
func collectDatabaseMetrics(database *gorm.DB, logger *slog.Logger) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats, err := db.GetStats(database)
		if err != nil {
			logger.Warn("failed to collect database metrics", "error", err)
			continue
		}

		metrics.DatabaseConnections.WithLabelValues("open").Set(float64(stats.OpenConns))
		metrics.DatabaseConnections.WithLabelValues("in_use").Set(float64(stats.InUse))
		metrics.DatabaseConnections.WithLabelValues("idle").Set(float64(stats.Idle))
		metrics.DatabaseWaitCount.Set(float64(stats.WaitCount))
		metrics.DatabaseWaitDurationSeconds.Set(stats.WaitDuration.Seconds())
	}
}
