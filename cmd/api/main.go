package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	// defaultBindAddr 是本地开发阶段的默认监听地址。
	// 当未设置 BIND_ADDR 环境变量时，服务使用该地址启动。
	// 后续配置模块完善后，该默认值会迁移到 internal/config 中统一管理。
	defaultBindAddr = "127.0.0.1:60001"

	// serviceName 用于根路径响应，便于在浏览器或 curl 中确认当前响应服务。
	serviceName = "messageFeed"
)

func main() {
	// 从第一个可执行入口开始使用 slog，确保后续 handler、service 和 repository
	// 等层次沿用结构化日志，而不是分散使用临时标准输出。
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// BIND_ADDR 是运行时监听范围的显式开关。
	// 本机访问、局域网访问和 Tailscale 访问都应由该变量决定，
	// DEPLOYMENT_MODE 只表达部署拓扑，不隐式决定监听地址。
	bindAddr := envOrDefault("BIND_ADDR", defaultBindAddr)

	// 第一版路由保持最小集合：
	// "/" 用于人工确认 API 进程可达；
	// "/healthz" 作为阶段一要求的存活检查端点。
	mux := http.NewServeMux()
	mux.HandleFunc("/", rootHandler)
	mux.HandleFunc("/healthz", healthzHandler)

	// ReadHeaderTimeout 用于限制客户端长期占用连接但不完整发送请求头的情况。
	// 请求日志中间件包裹整个路由树，使当前和后续新增端点都具备基础访问日志。
	server := &http.Server{
		Addr:              bindAddr,
		Handler:           logRequests(logger, mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// ListenAndServe 在独立 goroutine 中运行，使主 goroutine 可以同时监听系统信号。
	// errCh 使用缓冲通道，避免服务在 select 开始前失败时阻塞错误上报。
	errCh := make(chan error, 1)
	go func() {
		logger.Info("api server starting", "bind_addr", bindAddr)
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
		shutdown(ctx, logger, server)
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api server failed", "error", err)
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

// logRequests 在每个请求结束后记录基础访问日志。
// 当前阶段使用标准 http.ResponseWriter，暂不记录响应状态码；
// 后续 handler 包可以通过 response recorder 补充状态码和响应体积等字段。
func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info(
			"http request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
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

// envOrDefault 读取单个环境变量，并在变量未设置时返回默认值。
// 空字符串在监听地址等配置中没有有效含义，因此也按未设置处理。
func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

// shutdown 为正在处理的请求保留一个短暂的完成窗口。
// 该生命周期模式后续会扩展到数据库连接、调度器和通知发送器等资源清理。
func shutdown(ctx context.Context, logger *slog.Logger, server *http.Server) {
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("api server shutdown failed", "error", err)
		return
	}
	logger.Info("api server stopped")
}
