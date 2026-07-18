package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"messagefeed/internal/config"
	"messagefeed/internal/db"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

type Application struct {
	cfg              config.Config
	logger           *slog.Logger
	plan             RolePlan
	database         *gorm.DB
	dependencies     dependencies
	apiServer        *http.Server
	operationsServer *http.Server
	ready            atomic.Bool
	migrations       MigrationRunner
}

type Option func(*Application)

func WithMigrationRunner(runner MigrationRunner) Option {
	return func(application *Application) {
		if runner != nil {
			application.migrations = runner
		}
	}
}

func New(cfg config.Config, logger *slog.Logger, options ...Option) (*Application, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	plan, err := PlanForRole(cfg.Runtime.AppRole)
	if err != nil {
		return nil, err
	}
	application := &Application{
		cfg:    cfg,
		logger: logger,
		plan:   plan,
		migrations: commandMigrationRunner{
			lockTimeout: cfg.Migrations.LockTimeout,
			phase:       cfg.Migrations.Phase,
		},
	}
	for _, option := range options {
		option(application)
	}
	if plan.Migrate {
		return application, nil
	}

	if cfg.Database.DSN != "" {
		application.database, err = db.Open(db.Config{
			DSN: cfg.Database.DSN, MaxOpenConns: cfg.Database.MaxOpenConns,
			MaxIdleConns: cfg.Database.MaxIdleConns, ConnMaxLifetime: cfg.Database.ConnMaxLifetime, Logger: logger,
		})
		if err != nil {
			return nil, fmt.Errorf("open database: %w", err)
		}
		pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = db.Ping(pingCtx, application.database)
		cancel()
		if err != nil {
			_ = db.Close(application.database)
			return nil, fmt.Errorf("ping database: %w", err)
		}
	} else {
		logger.Warn("database not configured, running in database-less mode")
	}

	application.dependencies, err = buildDependencies(cfg, plan, application.database, logger)
	if err != nil {
		if application.database != nil {
			_ = db.Close(application.database)
		}
		return nil, err
	}
	if err := application.dependencies.workers.validate(cfg.Runtime.AppRole, plan); err != nil {
		if application.database != nil {
			_ = db.Close(application.database)
		}
		return nil, err
	}
	if plan.API {
		application.apiServer = &http.Server{Addr: cfg.HTTP.BindAddr, Handler: application.dependencies.router, ReadHeaderTimeout: 5 * time.Second}
	}
	if plan.HasWorkers() && !plan.API {
		application.operationsServer = &http.Server{
			Addr: cfg.HTTP.WorkerMetricsAddr, Handler: newWorkerOperationsHandler(cfg.Runtime.AppRole, &application.ready, func(ctx context.Context) error {
				if application.database == nil {
					return fmt.Errorf("database is not configured")
				}
				return db.Ping(ctx, application.database)
			}), ReadHeaderTimeout: 5 * time.Second,
		}
	}
	return application, nil
}

func (application *Application) Run(ctx context.Context) error {
	if application == nil {
		return fmt.Errorf("application is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if application.plan.Migrate {
		if application.migrations == nil {
			return fmt.Errorf("migration runner is not configured")
		}
		application.logger.Info(
			"migration role starting",
			"migrations_path", application.cfg.Migrations.Path,
			"migration_phase", application.cfg.Migrations.Phase,
			"lock_timeout", application.cfg.Migrations.LockTimeout,
		)
		if err := application.migrations.Run(ctx, application.cfg.Database.DSN, application.cfg.Migrations.Path); err != nil {
			return err
		}
		application.logger.Info("migration role completed")
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, 2)
	var waitGroup sync.WaitGroup

	if application.database != nil {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			collectDatabaseMetrics(runCtx, application.database, application.logger)
		}()
	}
	startServer := func(name string, server *http.Server) {
		if server == nil {
			return
		}
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			application.logger.Info(name+" server starting", "addr", server.Addr)
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				select {
				case errCh <- fmt.Errorf("%s server: %w", name, err):
				default:
				}
			}
		}()
	}
	startServer("api", application.apiServer)
	startServer("worker operations", application.operationsServer)
	startWorkerLoops(runCtx, application.logger, application.cfg.Runtime.AppNodeID, application.plan, application.dependencies.workers, &waitGroup)
	application.ready.Store(true)
	application.logger.Info("application role started")

	var runErr error
	select {
	case <-ctx.Done():
		application.logger.Info("application role stopping", "reason", ctx.Err())
	case runErr = <-errCh:
		application.logger.Error("application runtime failed", "error", runErr)
	}
	application.ready.Store(false)
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	for name, server := range map[string]*http.Server{"api": application.apiServer, "worker operations": application.operationsServer} {
		if server == nil {
			continue
		}
		if err := server.Shutdown(shutdownCtx); err != nil && runErr == nil {
			runErr = fmt.Errorf("shutdown %s server: %w", name, err)
		}
	}

	waitDone := make(chan struct{})
	go func() {
		waitGroup.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-shutdownCtx.Done():
		if runErr == nil {
			runErr = fmt.Errorf("wait for application workers: %w", shutdownCtx.Err())
		}
	}
	if application.database != nil {
		if err := db.Close(application.database); err != nil && runErr == nil {
			runErr = fmt.Errorf("close database: %w", err)
		} else if err == nil {
			application.logger.Info("database closed")
		}
	}
	application.logger.Info("application role stopped")
	return runErr
}
