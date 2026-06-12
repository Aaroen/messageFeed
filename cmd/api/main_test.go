package main

import (
	"encoding/json"
	appRuntime "messagefeed/internal/runtime"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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
