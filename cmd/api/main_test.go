package main

import (
	"context"
	"log/slog"
	"net/http"
	"testing"
	"time"
)

// TestShutdownWithoutDatabase 验证入口层优雅关闭可以在无数据库配置时完成。
// 路由行为已经迁移到 internal/handler 包测试，此处仅覆盖进程生命周期辅助函数。
func TestShutdownWithoutDatabase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	logger := slog.Default()
	server := &http.Server{}

	shutdown(ctx, logger, server, nil)
}
