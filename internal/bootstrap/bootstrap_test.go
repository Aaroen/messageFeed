package bootstrap

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"messagefeed/internal/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPlanForRoleSeparatesRuntimeBoundaries(t *testing.T) {
	tests := []struct {
		role       config.AppRole
		api        bool
		workers    bool
		migration  bool
		workerRole config.AppRole
	}{
		{role: config.AppRoleAll, api: true, workers: true},
		{role: config.AppRoleAPI, api: true},
		{role: config.AppRoleSourceWorker, workers: true, workerRole: config.AppRoleSourceWorker},
		{role: config.AppRoleNotificationWorker, workers: true, workerRole: config.AppRoleNotificationWorker},
		{role: config.AppRoleAgentSchedulerWorker, workers: true, workerRole: config.AppRoleAgentSchedulerWorker},
		{role: config.AppRoleEmbeddingWorker, workers: true, workerRole: config.AppRoleEmbeddingWorker},
		{role: config.AppRoleMigrate, migration: true},
	}
	for _, test := range tests {
		plan, err := PlanForRole(test.role)
		if err != nil {
			t.Fatalf("PlanForRole(%q) error = %v", test.role, err)
		}
		if plan.API != test.api || plan.HasWorkers() != test.workers || plan.Migrate != test.migration {
			t.Errorf("PlanForRole(%q) = %#v", test.role, plan)
		}
		if test.workerRole != "" {
			workerPlan, _ := PlanForRole(test.workerRole)
			if workerPlan.API || !workerPlan.HasWorkers() {
				t.Errorf("worker role %q includes an invalid boundary: %#v", test.workerRole, workerPlan)
			}
		}
	}

	if _, err := PlanForRole(config.AppRole("unknown")); err == nil {
		t.Fatal("PlanForRole(unknown) error = nil")
	}
}

func TestNewAPIApplicationDoesNotConstructWorkerOperations(t *testing.T) {
	cfg := config.Defaults()
	cfg.Runtime.AppRole = config.AppRoleAPI
	app, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if app.apiServer == nil {
		t.Fatal("apiServer = nil")
	}
	if app.operationsServer != nil {
		t.Fatal("API role unexpectedly constructed worker operations server")
	}
	if app.dependencies.workers != (workerSet{}) {
		t.Fatalf("API role unexpectedly constructed worker dependencies: %#v", app.dependencies.workers)
	}
}

func TestWorkerOperationsHandlerExposesOnlyOperationalEndpoints(t *testing.T) {
	var ready atomic.Bool
	handler := newWorkerOperationsHandler(config.AppRoleSourceWorker, &ready, nil)

	request := func(path string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		return recorder
	}

	if response := request("/healthz"); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "source-worker") {
		t.Fatalf("health response = %d %q", response.Code, response.Body.String())
	}
	if response := request("/readyz"); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("not-ready response code = %d", response.Code)
	}
	ready.Store(true)
	if response := request("/readyz"); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "ready") {
		t.Fatalf("ready response = %d %q", response.Code, response.Body.String())
	}
	if response := request("/api/healthz"); response.Code != http.StatusNotFound {
		t.Fatalf("worker business route response code = %d, want 404", response.Code)
	}
	if response := request("/metrics"); response.Code != http.StatusOK {
		t.Fatalf("metrics response code = %d", response.Code)
	}
}

func TestWorkerReadinessFailsWhenDatabaseCheckFails(t *testing.T) {
	var ready atomic.Bool
	ready.Store(true)
	handler := newWorkerOperationsHandler(config.AppRoleNotificationWorker, &ready, func(context.Context) error {
		return errors.New("database unavailable")
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), `"reason":"database"`) {
		t.Fatalf("database failure response = %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestMigrateRoleUsesRelativeMigrationsPath(t *testing.T) {
	cfg := config.Defaults()
	cfg.Runtime.AppRole = config.AppRoleMigrate
	cfg.Database.DSN = "postgres://migration-test"
	runner := &recordingMigrationRunner{}
	app, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), WithMigrationRunner(runner))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := app.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if runner.databaseURL != cfg.Database.DSN || runner.path != config.DefaultMigrationsPath {
		t.Fatalf("migration arguments = database %q path %q", runner.databaseURL, runner.path)
	}
}

type recordingMigrationRunner struct {
	databaseURL string
	path        string
}

func (runner *recordingMigrationRunner) Run(_ context.Context, databaseURL string, path string) error {
	runner.databaseURL = databaseURL
	runner.path = path
	return nil
}
