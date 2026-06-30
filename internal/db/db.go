package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"messagefeed/internal/observability"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config 保存数据库连接配置。
// 该结构从环境变量或配置文件加载，由 internal/config 模块统一管理。
type Config struct {
	// DSN 是 PostgreSQL 数据源名称（Data Source Name）。
	// 格式：postgres://用户名:密码@主机:端口/数据库名?参数
	// 示例：postgres://messagefeed:password@localhost:5432/messagefeed?sslmode=disable
	DSN string

	// MaxOpenConns 是连接池最大连接数。
	// 默认值通常为 25，根据实际负载调整。
	MaxOpenConns int

	// MaxIdleConns 是连接池最大空闲连接数。
	// 默认值通常为 5，建议设置为 MaxOpenConns 的 20%-40%。
	MaxIdleConns int

	// ConnMaxLifetime 是单个连接的最大生命周期。
	// 避免连接长期占用，默认 1 小时。
	ConnMaxLifetime time.Duration

	// Logger 将 GORM 查询日志接入应用结构化日志。
	Logger *slog.Logger
}

// DefaultConfig 返回数据库连接的默认配置。
// 生产环境应通过环境变量覆盖这些默认值。
func DefaultConfig() Config {
	return Config{
		DSN:             "",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		Logger:          slog.Default(),
	}
}

// Open 打开数据库连接并配置连接池。
// 该函数不执行连接测试，调用方应在初始化后调用 Ping 或 CheckHealth。
func Open(cfg Config) (*gorm.DB, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("database DSN is empty")
	}

	gormLogger := newSlogGORMLogger(cfg.Logger).LogMode(logger.Warn)

	// 打开数据库连接
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
		Logger: gormLogger,
		// NowFunc 使用 UTC 时间，与配置、日志、运行时保持一致。
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// 获取底层 *sql.DB 并配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return db, nil
}

type slogGORMLogger struct {
	logger        *slog.Logger
	level         logger.LogLevel
	slowThreshold time.Duration
}

func newSlogGORMLogger(base *slog.Logger) slogGORMLogger {
	if base == nil {
		base = slog.Default()
	}
	return slogGORMLogger{
		logger:        base,
		level:         logger.Warn,
		slowThreshold: 200 * time.Millisecond,
	}
}

func (l slogGORMLogger) LogMode(level logger.LogLevel) logger.Interface {
	l.level = level
	return l
}

func (l slogGORMLogger) Info(ctx context.Context, msg string, args ...interface{}) {
	if l.level >= logger.Info {
		l.logger.DebugContext(ctx, msg, args...)
	}
}

func (l slogGORMLogger) Warn(ctx context.Context, msg string, args ...interface{}) {
	if l.level >= logger.Warn {
		l.logger.WarnContext(ctx, msg, args...)
	}
}

func (l slogGORMLogger) Error(ctx context.Context, msg string, args ...interface{}) {
	if l.level >= logger.Error {
		l.logger.ErrorContext(ctx, msg, args...)
	}
}

func (l slogGORMLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level == logger.Silent {
		return
	}

	duration := time.Since(begin)
	if err == nil && duration < l.slowThreshold && l.level < logger.Info {
		return
	}

	sql, rows := fc()
	attrs := []any{
		"trace_id", observability.TraceID(ctx),
		"span_id", observability.SpanID(ctx),
		"duration_ms", duration.Milliseconds(),
		"rows", rows,
		"sql", sql,
	}

	switch {
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound) && l.level >= logger.Error:
		l.logger.ErrorContext(ctx, "database query failed", append(attrs, "error", err)...)
	case duration >= l.slowThreshold && l.level >= logger.Warn:
		l.logger.WarnContext(ctx, "database slow query", attrs...)
	case l.level >= logger.Info:
		l.logger.DebugContext(ctx, "database query", attrs...)
	}
}

// Ping 执行数据库连接测试。
// 该函数应在服务启动时调用，确保数据库可达且凭证正确。
func Ping(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get underlying sql.DB: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	return nil
}

// Close 关闭数据库连接池。
// 该函数应在服务优雅关闭时调用，等待所有进行中的查询完成。
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get underlying sql.DB: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}

	return nil
}

// Stats 返回数据库连接池统计信息。
// 该函数用于健康检查和指标上报。
type Stats struct {
	MaxOpenConns      int           // 最大连接数
	OpenConns         int           // 当前打开连接数
	InUse             int           // 正在使用的连接数
	Idle              int           // 空闲连接数
	WaitCount         int64         // 等待连接的总次数
	WaitDuration      time.Duration // 等待连接的总时长
	MaxIdleClosed     int64         // 因达到 MaxIdleConns 而关闭的连接数
	MaxLifetimeClosed int64         // 因达到 ConnMaxLifetime 而关闭的连接数
}

// MigrationStatus 表示 golang-migrate 在 schema_migrations 中记录的当前迁移状态。
type MigrationStatus struct {
	Version int64
	Dirty   bool
}

type PgvectorStatus struct {
	Installed bool
	Version   string
}

type AgentFactIndexStatus struct {
	ArchiveRows    int64
	EmbeddingRows  int64
	LastIndexedAt  *time.Time
	LastEmbeddedAt *time.Time
}

// GetStats 获取数据库连接池统计信息。
func GetStats(db *gorm.DB) (Stats, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return Stats{}, fmt.Errorf("get underlying sql.DB: %w", err)
	}

	stats := sqlDB.Stats()
	return Stats{
		MaxOpenConns:      stats.MaxOpenConnections,
		OpenConns:         stats.OpenConnections,
		InUse:             stats.InUse,
		Idle:              stats.Idle,
		WaitCount:         stats.WaitCount,
		WaitDuration:      stats.WaitDuration,
		MaxIdleClosed:     stats.MaxIdleClosed,
		MaxLifetimeClosed: stats.MaxLifetimeClosed,
	}, nil
}

// CheckHealth 检查数据库连接健康状态。
// 该函数用于 /readyz 端点，判断数据库是否可用。
func CheckHealth(ctx context.Context, db *gorm.DB, logger *slog.Logger) error {
	// 执行简单查询测试连接
	if err := Ping(ctx, db); err != nil {
		logger.Error("database health check failed", "error", err)
		return fmt.Errorf("database ping failed: %w", err)
	}

	// 检查连接池状态
	stats, err := GetStats(db)
	if err != nil {
		logger.Warn("failed to get database stats", "error", err)
		// 获取统计信息失败不影响健康检查结果
	} else {
		logger.Debug("database stats",
			"open_conns", stats.OpenConns,
			"in_use", stats.InUse,
			"idle", stats.Idle,
			"wait_count", stats.WaitCount,
		)
	}

	return nil
}

// CheckMigrationStatus 检查 golang-migrate 版本表是否存在且未处于 dirty 状态。
func CheckMigrationStatus(ctx context.Context, db *gorm.DB) (MigrationStatus, error) {
	var status MigrationStatus
	row := db.WithContext(ctx).Raw("SELECT version, dirty FROM schema_migrations LIMIT 1").Row()
	if err := row.Scan(&status.Version, &status.Dirty); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MigrationStatus{}, fmt.Errorf("schema_migrations has no applied version")
		}
		return MigrationStatus{}, fmt.Errorf("query schema_migrations: %w", err)
	}
	if status.Dirty {
		return status, fmt.Errorf("schema_migrations is dirty at version %d", status.Version)
	}
	return status, nil
}

func CheckPgvectorStatus(ctx context.Context, db *gorm.DB) (PgvectorStatus, error) {
	var version string
	row := db.WithContext(ctx).Raw("SELECT extversion FROM pg_extension WHERE extname = 'vector' LIMIT 1").Row()
	if err := row.Scan(&version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PgvectorStatus{}, fmt.Errorf("pgvector extension is not installed")
		}
		return PgvectorStatus{}, fmt.Errorf("query pgvector extension: %w", err)
	}
	return PgvectorStatus{Installed: true, Version: version}, nil
}

func CheckAgentFactIndexStatus(ctx context.Context, db *gorm.DB) (AgentFactIndexStatus, error) {
	var status AgentFactIndexStatus
	if err := db.WithContext(ctx).Raw("SELECT COUNT(*) FROM agent_fact_archive_index").Row().Scan(&status.ArchiveRows); err != nil {
		return AgentFactIndexStatus{}, fmt.Errorf("query agent fact archive index: %w", err)
	}
	if err := db.WithContext(ctx).Raw("SELECT COUNT(*) FROM agent_fact_embeddings").Row().Scan(&status.EmbeddingRows); err != nil {
		return AgentFactIndexStatus{}, fmt.Errorf("query agent fact embeddings: %w", err)
	}
	var lastIndexed sql.NullTime
	if err := db.WithContext(ctx).Raw("SELECT MAX(updated_at) FROM agent_fact_archive_index").Row().Scan(&lastIndexed); err == nil && lastIndexed.Valid {
		value := lastIndexed.Time
		status.LastIndexedAt = &value
	}
	var lastEmbedded sql.NullTime
	if err := db.WithContext(ctx).Raw("SELECT MAX(updated_at) FROM agent_fact_embeddings").Row().Scan(&lastEmbedded); err == nil && lastEmbedded.Valid {
		value := lastEmbedded.Time
		status.LastEmbeddedAt = &value
	}
	return status, nil
}
