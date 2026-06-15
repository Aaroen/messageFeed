package handler

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"messagefeed/internal/metrics"

	"github.com/gin-gonic/gin"
)

const requestIDHeader = "X-Request-ID"

// RequestID 为每个请求设置稳定的 request id。
// 如果上游已经提供 X-Request-ID，则沿用该值，便于跨代理链路排查。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(requestIDHeader)
		if requestID == "" {
			requestID = newRequestID()
		}

		c.Set(requestIDKey, requestID)
		c.Header(requestIDHeader, requestID)
		c.Next()
	}
}

// AccessLog 记录基础访问日志，并同步更新 HTTP Prometheus 指标。
func AccessLog(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		duration := time.Since(start)
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		status := c.Writer.Status()

		logger.Info(
			"http request",
			"request_id", requestID(c),
			"method", method,
			"path", path,
			"status", status,
			"duration_ms", duration.Milliseconds(),
		)

		metrics.HTTPRequestsTotal.WithLabelValues(method, path, http.StatusText(status)).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
		if c.Request.ContentLength > 0 {
			metrics.HTTPRequestSize.WithLabelValues(method, path).Observe(float64(c.Request.ContentLength))
		}
		if responseSize := c.Writer.Size(); responseSize > 0 {
			metrics.HTTPResponseSize.WithLabelValues(method, path).Observe(float64(responseSize))
		}
	}
}

// Recovery 将 panic 转换为统一错误响应，同时保留结构化日志。
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		logger.Error(
			"http handler panic",
			"request_id", requestID(c),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"panic", recovered,
		)
		Error(c, http.StatusInternalServerError, http.StatusInternalServerError, "internal server error")
	})
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}
