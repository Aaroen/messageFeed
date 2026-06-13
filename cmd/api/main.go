package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"messagefeed/internal/config"
	"messagefeed/internal/db"
	"messagefeed/internal/metrics"
	appRuntime "messagefeed/internal/runtime"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"
)

const (
	// serviceName 用于根路径响应，便于在浏览器或 curl 中确认当前响应服务。
	serviceName = "messageFeed"
)

func main() {
	// 启动初期先使用 info 级别日志，以便在配置加载失败时仍能输出结构化错误。
	bootstrapLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// 配置模块统一负责默认值、环境变量覆盖和基础校验。
	// 入口层只使用已经校验过的配置，避免各处重复读取环境变量。
	cfg, err := config.Load()
	if err != nil {
		bootstrapLogger.Error("load config failed", "error", err)
		os.Exit(1)
	}

	// 正式 logger 使用配置中的 LOG_LEVEL。
	// 从这里开始，后续 handler、service 和 repository 应沿用该 logger。
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.Log.SlogLevel(),
	}))
	logger.Info(
		"configuration loaded",
		"bind_addr", cfg.HTTP.BindAddr,
		"public_base_url", cfg.Runtime.PublicBaseURL,
		"app_node_id", cfg.Runtime.AppNodeID,
		"deployment_mode", cfg.Runtime.DeploymentMode,
		"database_configured", cfg.Database.DSN != "",
	)

	// 数据库连接（可选）
	// 当 DATABASE_URL 未配置时，database 为 nil，服务仍可启动但 /readyz 不检查数据库。
	var database *gorm.DB
	if cfg.Database.DSN != "" {
		dbCfg := db.Config{
			DSN:             cfg.Database.DSN,
			MaxOpenConns:    cfg.Database.MaxOpenConns,
			MaxIdleConns:    cfg.Database.MaxIdleConns,
			ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
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

		// 启动数据库连接池指标采集器
		go collectDatabaseMetrics(database, logger)
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

	// 路由注册
	// "/" 用于人工确认 API 进程可达；
	// "/healthz" 作为阶段一要求的存活检查端点；
	// "/readyz" 返回当前进程是否具备接收流量的条件；
	// "/metrics" 暴露 Prometheus 指标；
	// "/api/runtime/node" 返回当前节点的运行时身份与访问配置。
	mux := http.NewServeMux()
	mux.HandleFunc("/", rootHandler)
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/readyz", readyzHandler(database, logger, time.Now))
	mux.Handle("/metrics", promhttp.HandlerFor(metrics.Gatherer, promhttp.HandlerOpts{}))
	mux.HandleFunc("/api/runtime/node", runtimeNodeHandler(nodeInfo))

	// ReadHeaderTimeout 用于限制客户端长期占用连接但不完整发送请求头的情况。
	// 请求日志中间件包裹整个路由树，使当前和后续新增端点都具备基础访问日志。
	server := &http.Server{
		Addr:              cfg.HTTP.BindAddr,
		Handler:           logRequests(logger, mux),
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdown(ctx, logger, server, database)
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api server failed", "error", err)
			if database != nil {
				_ = db.Close(database)
			}
			os.Exit(1)
		}
	}
}

// rootHandler 提供一个浏览器可见的最小响应。
// 在真实 API 路由尚未加入前，该端点用于人工验证服务是否已经启动。
func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"service": serviceName,
		"status":  "ok",
	})
}

// healthzHandler 是存活检查端点。
// 它只证明 HTTP 进程可以响应请求；数据库连接和迁移状态后续由 /readyz 承担。
func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// readyzHandler 返回服务就绪状态。
// 第一阶段包含进程级检查和可选的数据库连接检查。
func readyzHandler(database *gorm.DB, logger *slog.Logger, now func() time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		checks := []appRuntime.ReadinessCheck{
			{
				Name:    "process",
				Status:  appRuntime.ReadinessReady,
				Message: "api process is running",
			},
		}

		// 数据库健康检查（仅当数据库已配置时）
		if database != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()

			if err := db.CheckHealth(ctx, database, logger); err != nil {
				checks = append(checks, appRuntime.ReadinessCheck{
					Name:    "database",
					Status:  appRuntime.ReadinessNotReady,
					Message: "database connection failed",
				})
			} else {
				checks = append(checks, appRuntime.ReadinessCheck{
					Name:    "database",
					Status:  appRuntime.ReadinessReady,
					Message: "database connection ok",
				})
			}
		}

		report := appRuntime.NewReadinessReport(checks, now().UTC())
		statusCode := http.StatusOK
		if !report.Ready() {
			statusCode = http.StatusServiceUnavailable
		}
		writeJSON(w, statusCode, report)
	}
}

// runtimeNodeHandler 返回当前节点的运行时信息。
// 该端点用于验证部署模式、节点标识、公开访问基址和实际监听地址是否符合预期。
func runtimeNodeHandler(nodeInfo appRuntime.NodeInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, nodeInfo)
	}
}

// logRequests 在每个请求结束后记录基础访问日志并更新 Prometheus 指标。
func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 包装 ResponseWriter 以捕获状态码
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		method := r.Method
		path := r.URL.Path
		status := wrapped.statusCode

		// 记录日志
		logger.Info(
			"http request",
			"method", method,
			"path", path,
			"status", status,
			"duration_ms", duration.Milliseconds(),
		)

		// 更新 Prometheus 指标
		metrics.HTTPRequestsTotal.WithLabelValues(method, path, http.StatusText(status)).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
	})
}

// responseWriter 包装 http.ResponseWriter 以捕获响应状态码。
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// writeJSON 统一当前最小端点的 JSON 响应写法。
// 在正式 handler 响应辅助函数引入前，该函数负责集中设置响应头和编码行为。
func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("write response failed", "error", err)
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
	}
}
