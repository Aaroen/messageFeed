package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
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
}

// DefaultConfig 返回数据库连接的默认配置。
// 生产环境应通过环境变量覆盖这些默认值。
func DefaultConfig() Config {
	return Config{
		DSN:             "",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
	}
}

// Open 打开数据库连接并配置连接池。
// 该函数不执行连接测试，调用方应在初始化后调用 Ping 或 CheckHealth。
func Open(cfg Config) (*gorm.DB, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("database DSN is empty")
	}

	// GORM 日志配置：将 GORM 日志桥接到 slog。
	// 第一阶段使用默认日志级别，后续可根据环境变量调整。
	gormLogger := logger.Default.LogMode(logger.Info)

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
