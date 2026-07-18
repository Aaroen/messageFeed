package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"messagefeed/internal/config"
	"messagefeed/internal/db"
	"messagefeed/internal/metrics"
	"messagefeed/internal/service"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"
)

type workerSet struct {
	source         *service.SourceSyncService
	notification   *service.NotificationWorkerService
	agentScheduler *service.AgentScheduledTaskWorkerService
	embedding      *service.AgentEmbeddingWorkerService
}

func (workers workerSet) validate(role config.AppRole, plan RolePlan) error {
	if plan.SourceWorker && workers.source == nil && role != config.AppRoleAll {
		return fmt.Errorf("source worker dependencies are not configured")
	}
	if plan.NotificationWorker && workers.notification == nil && role != config.AppRoleAll {
		return fmt.Errorf("notification worker dependencies are not configured")
	}
	if plan.AgentSchedulerWorker && workers.agentScheduler == nil && role != config.AppRoleAll {
		return fmt.Errorf("agent scheduler worker dependencies are not configured")
	}
	if plan.EmbeddingWorker && workers.embedding == nil && role != config.AppRoleAll {
		return fmt.Errorf("embedding worker dependencies are not configured")
	}
	return nil
}

func startWorkerLoops(ctx context.Context, logger *slog.Logger, nodeID string, plan RolePlan, workers workerSet, waitGroup *sync.WaitGroup) {
	start := func(role config.AppRole, run func()) {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			logger.Info("worker loop starting", "worker_role", role)
			run()
			logger.Info("worker loop stopped", "worker_role", role)
		}()
	}
	if plan.SourceWorker && workers.source != nil {
		start(config.AppRoleSourceWorker, func() { runSourceSyncWorker(ctx, logger, workerID(nodeID, config.AppRoleSourceWorker), workers.source) })
	}
	if plan.NotificationWorker && workers.notification != nil {
		start(config.AppRoleNotificationWorker, func() {
			runNotificationWorker(ctx, logger, workerID(nodeID, config.AppRoleNotificationWorker), workers.notification)
		})
	}
	if plan.AgentSchedulerWorker && workers.agentScheduler != nil {
		start(config.AppRoleAgentSchedulerWorker, func() {
			runAgentScheduledTaskWorker(ctx, logger, workerID(nodeID, config.AppRoleAgentSchedulerWorker), workers.agentScheduler)
		})
	}
	if plan.EmbeddingWorker && workers.embedding != nil {
		start(config.AppRoleEmbeddingWorker, func() {
			runAgentEmbeddingWorker(ctx, logger, workerID(nodeID, config.AppRoleEmbeddingWorker), workers.embedding)
		})
	}
}

func workerID(nodeID string, role config.AppRole) string {
	if nodeID == "" {
		nodeID = "unknown-node"
	}
	return nodeID + ":" + string(role)
}

func runSourceSyncWorker(ctx context.Context, logger *slog.Logger, workerID string, sourceSyncService *service.SourceSyncService) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		result, err := sourceSyncService.RunOnce(runCtx, service.RunSourceSyncOnceInput{
			WorkerID: workerID, LockName: "source-sync", LockTTL: 30 * time.Second,
			EnqueueLimit: 50, ClaimLimit: 50, DefaultMaxAttempts: 3,
		})
		cancel()
		if err != nil && ctx.Err() == nil {
			logger.Warn("source sync worker run failed", "error", err)
		} else if result.ClaimedCount > 0 || result.EnqueuedCount > 0 {
			logger.Info("source sync worker run completed", "enqueued", result.EnqueuedCount, "claimed", result.ClaimedCount, "success", result.SuccessCount, "failed", result.FailureCount, "retry", result.RetryCount)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func runNotificationWorker(ctx context.Context, logger *slog.Logger, workerID string, workerService *service.NotificationWorkerService) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		result, err := workerService.RunOnce(runCtx, service.RunNotificationWorkerOnceInput{WorkerID: workerID, Limit: 20})
		cancel()
		if err != nil && ctx.Err() == nil {
			logger.Warn("notification worker run failed", "error", err)
		} else if result.ClaimedCount > 0 {
			logger.Info("notification worker run completed", "claimed", result.ClaimedCount, "success", result.SucceededCount, "failed", result.FailedCount, "retry", result.RetryCount)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func runAgentScheduledTaskWorker(ctx context.Context, logger *slog.Logger, workerID string, workerService *service.AgentScheduledTaskWorkerService) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		result, err := workerService.RunDueOnce(runCtx, service.RunDueAgentScheduledTasksInput{WorkerID: workerID, Limit: 20})
		cancel()
		if err != nil && ctx.Err() == nil {
			logger.Warn("agent scheduled task worker run failed", "error", err)
		} else if result.Claimed > 0 {
			logger.Info("agent scheduled task worker run completed", "claimed", result.Claimed, "succeeded", result.Succeeded, "failed", result.Failed, "report_sent", result.ReportSent, "report_failed", result.ReportFailed, "report_skipped", result.ReportSkipped)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func runAgentEmbeddingWorker(ctx context.Context, logger *slog.Logger, workerID string, workerService *service.AgentEmbeddingWorkerService) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		runCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		result, err := workerService.RunOnce(runCtx, service.RunAgentEmbeddingWorkerOnceInput{WorkerID: workerID, Limit: 10})
		cancel()
		if err != nil && ctx.Err() == nil {
			logger.Warn("agent embedding worker run failed", "error", err)
		} else if result.ClaimedCount > 0 {
			logger.Info("agent embedding worker run completed", "claimed", result.ClaimedCount, "succeeded", result.SucceededCount, "failed", result.FailedCount, "skipped", result.SkippedCount)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func collectDatabaseMetrics(ctx context.Context, database *gorm.DB, logger *slog.Logger) {
	update := func() {
		stats, err := db.GetStats(database)
		if err != nil {
			logger.Warn("failed to collect database metrics", "error", err)
			return
		}
		metrics.DatabaseConnections.WithLabelValues("open").Set(float64(stats.OpenConns))
		metrics.DatabaseConnections.WithLabelValues("in_use").Set(float64(stats.InUse))
		metrics.DatabaseConnections.WithLabelValues("idle").Set(float64(stats.Idle))
		metrics.DatabaseWaitCount.Set(float64(stats.WaitCount))
		metrics.DatabaseWaitDurationSeconds.Set(stats.WaitDuration.Seconds())
	}
	update()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			update()
		}
	}
}

func newWorkerOperationsHandler(role config.AppRole, ready *atomic.Bool, checkDatabase func(context.Context) error) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{"status":"ok","role":%q}`, role)
	})
	mux.HandleFunc("/readyz", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if !ready.Load() {
			writer.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(writer, `{"status":"not_ready","role":%q}`, role)
			return
		}
		if checkDatabase != nil {
			checkCtx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
			err := checkDatabase(checkCtx)
			cancel()
			if err != nil {
				writer.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprintf(writer, `{"status":"not_ready","role":%q,"reason":"database"}`, role)
				return
			}
		}
		fmt.Fprintf(writer, `{"status":"ready","role":%q}`, role)
	})
	mux.Handle("/metrics", promhttp.HandlerFor(metrics.Gatherer, promhttp.HandlerOpts{}))
	return mux
}
