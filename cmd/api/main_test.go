package main

import (
	"encoding/json"
	"log/slog"
	appRuntime "messagefeed/internal/runtime"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// TestReadyzHandler 验证 /readyz 返回进程级 ready 报告。
// 当前阶段数据库为可选配置，因此测试时传入 nil 表示无数据库模式。
func TestReadyzHandler(t *testing.T) {
	checkedAt := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	recorder := httptest.NewRecorder()

	// 无数据库模式测试
	readyzHandler(nil, logger, func() time.Time {
		return checkedAt
	}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response appRuntime.ReadinessReport
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Status != appRuntime.ReadinessReady {
		t.Fatalf("Status = %q, want %q", response.Status, appRuntime.ReadinessReady)
	}
	// 无数据库模式只有 process 检查
	if got, want := len(response.Checks), 1; got != want {
		t.Fatalf("Checks length = %d, want %d", got, want)
	}
	if response.Checks[0].Name != "process" {
		t.Fatalf("process check name = %q", response.Checks[0].Name)
	}
	if !response.CheckedAt.Equal(checkedAt) {
		t.Fatalf("CheckedAt = %s, want %s", response.CheckedAt, checkedAt)
	}
}

// TestRuntimeNodeHandler 验证 /api/runtime/node 使用传入的节点快照返回 JSON。
// 该测试只覆盖 handler 行为，不启动真实端口，避免测试依赖本机网络状态。
func TestRuntimeNodeHandler(t *testing.T) {
	startedAt := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	nodeInfo := appRuntime.NodeInfo{
		NodeID:            "node-a",
		DeploymentMode:    "single_node",
		PublicBaseURL:     "http://127.0.0.1:60001",
		BindAddr:          "127.0.0.1:60001",
		TrustedProxyCIDRs: []string{"100.64.0.0/10"},
		StartedAt:         startedAt,
	}

	request := httptest.NewRequest(http.MethodGet, "/api/runtime/node", nil)
	recorder := httptest.NewRecorder()

	runtimeNodeHandler(nodeInfo).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response appRuntime.NodeInfo
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.NodeID != nodeInfo.NodeID {
		t.Fatalf("NodeID = %q, want %q", response.NodeID, nodeInfo.NodeID)
	}
	if !response.StartedAt.Equal(startedAt) {
		t.Fatalf("StartedAt = %s, want %s", response.StartedAt, startedAt)
	}
}
