package handler

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"messagefeed/internal/metrics"
	"messagefeed/internal/observability"

	"github.com/gin-gonic/gin"
	"github.com/rs/cors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
		c.Request = c.Request.WithContext(observability.WithRequestID(c.Request.Context(), requestID))
		c.Header(requestIDHeader, requestID)
		c.Next()
	}
}

// CORS 允许本地 Vite 前端在开发期直接调用 API。
// 生产环境仍建议通过 Nginx 反向代理统一入口，避免浏览器跨域。
func CORS() gin.HandlerFunc {
	allowedOrigins := map[string]struct{}{
		"http://localhost:5173":  {},
		"http://127.0.0.1:5173":  {},
		"http://[::1]:5173":      {},
		"http://[::1]:5173/":     {},
		"http://localhost:5173/": {},
	}
	handler := cors.New(cors.Options{
		AllowOriginFunc: func(origin string) bool {
			_, ok := allowedOrigins[origin]
			return ok
		},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"*",
		},
		ExposedHeaders: []string{
			requestIDHeader,
		},
		AllowCredentials: true,
		MaxAge:           int((12 * time.Hour).Seconds()),
	})

	return func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request, func(_ http.ResponseWriter, request *http.Request) {
			c.Request = request
			c.Next()
		})
		if c.Request.Method == http.MethodOptions && c.Request.Header.Get("Access-Control-Request-Method") != "" {
			c.Abort()
		}
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
		ctx := c.Request.Context()
		traceID := observability.TraceID(ctx)
		spanID := observability.SpanID(ctx)
		if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
			span.SetAttributes(
				attribute.String("http.request_id", requestID(c)),
				attribute.Int("http.response.status_code", status),
			)
		}

		attrs := []any{
			"request_id", requestID(c),
			"trace_id", traceID,
			"span_id", spanID,
			"method", method,
			"path", path,
			"status", status,
			"duration_ms", duration.Milliseconds(),
		}
		if status >= http.StatusBadRequest {
			if errorCode, ok := c.Get(errorCodeKey); ok {
				attrs = append(attrs, "error_code", errorCode)
			}
			if errorMessage, ok := c.Get(errorMessageKey); ok {
				attrs = append(attrs, "error_message", errorMessage)
			}
		}

		if status < http.StatusBadRequest && isRoutineObservationPath(path) {
			logger.Debug("http request", attrs...)
		} else {
			logger.Info("http request", attrs...)
		}

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

func isRoutineObservationPath(path string) bool {
	return path == "/healthz" || path == "/metrics"
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
			"trace_id", observability.TraceID(c.Request.Context()),
			"span_id", observability.SpanID(c.Request.Context()),
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
