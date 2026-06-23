package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"messagefeed/internal/db"
	"messagefeed/internal/metrics"
	"messagefeed/internal/observability"
	appRuntime "messagefeed/internal/runtime"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	otelgin "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"gorm.io/gorm"
)

const serviceName = "messageFeed"

// RouterOptions 汇总路由层需要的只读依赖。
// 业务 service 接入后应继续通过该结构注入，不在 handler 内部直接构建依赖。
type RouterOptions struct {
	Logger                *slog.Logger
	Database              *gorm.DB
	NodeInfo              appRuntime.NodeInfo
	Now                   func() time.Time
	SourceService         sourceService
	TimelineService       timelineService
	RecommendationService recommendationService
	ItemService           itemStateService
	FeedViewService       feedViewService
	WeChatWorkAppCallback wechatWorkAppCallback
	ServiceName           string
}

// NewRouter 创建 Gin 路由树。
// 基础端点保持阶段一响应语义，业务端点统一预留在 /api/v1 路由组下。
func NewRouter(options RouterOptions) *gin.Engine {
	if gin.Mode() == gin.DebugMode {
		gin.SetMode(gin.ReleaseMode)
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.ServiceName == "" {
		options.ServiceName = serviceName
	}

	router := gin.New()
	router.Use(RequestID(), otelgin.Middleware(options.ServiceName), UserContext(), CORS(), Recovery(options.Logger), AccessLog(options.Logger))

	router.GET("/", rootHandler)
	router.GET("/healthz", healthzHandler)
	router.GET("/readyz", readyzHandler(options.Database, options.Logger, options.Now))
	router.GET("/api/runtime/node", runtimeNodeHandler(options.NodeInfo))
	router.GET("/metrics", gin.WrapH(promhttp.HandlerFor(metrics.Gatherer, promhttp.HandlerOpts{})))

	apiV1 := router.Group("/api/v1")
	registerSourceRoutes(apiV1, options.SourceService)
	registerItemRoutes(apiV1, options.TimelineService, options.ItemService, options.RecommendationService)
	registerFeedViewRoutes(apiV1, options.FeedViewService)
	registerWeChatWorkRoutes(apiV1, options.WeChatWorkAppCallback)

	router.NoRoute(func(c *gin.Context) {
		Error(c, http.StatusNotFound, http.StatusNotFound, "not found")
	})

	return router
}

func rootHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service": serviceName,
		"status":  "ok",
	})
}

func healthzHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func readyzHandler(database *gorm.DB, logger *slog.Logger, now func() time.Time) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := observability.StartSpan(c.Request.Context(), "handler.readyz")
		defer observability.EndSpan(span, nil)
		c.Request = c.Request.WithContext(ctx)

		checks := []appRuntime.ReadinessCheck{
			{
				Name:    "process",
				Status:  appRuntime.ReadinessReady,
				Message: "api process is running",
			},
		}

		if database != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
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
				migrationStatus, err := db.CheckMigrationStatus(ctx, database)
				if err != nil {
					checks = append(checks, appRuntime.ReadinessCheck{
						Name:    "migrations",
						Status:  appRuntime.ReadinessNotReady,
						Message: "database migrations not ready",
					})
				} else {
					checks = append(checks, appRuntime.ReadinessCheck{
						Name:    "migrations",
						Status:  appRuntime.ReadinessReady,
						Message: fmt.Sprintf("database migrations at version %d", migrationStatus.Version),
					})
				}
			}
		}

		report := appRuntime.NewReadinessReport(checks, now().UTC())
		statusCode := http.StatusOK
		if !report.Ready() {
			statusCode = http.StatusServiceUnavailable
		}
		c.JSON(statusCode, report)
	}
}

func runtimeNodeHandler(nodeInfo appRuntime.NodeInfo) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, nodeInfo)
	}
}
