package runtime

import (
	"testing"
	"time"
)

// TestNewNodeInfoUsesProvidedValues 验证 NewNodeInfo 会按输入参数构建节点信息，
// 并将启动时间规范化为 UTC。
func TestNewNodeInfoUsesProvidedValues(t *testing.T) {
	startedAt := time.Date(2026, 6, 12, 9, 30, 0, 0, time.FixedZone("CST", 8*60*60))

	nodeInfo := NewNodeInfo(NodeOptions{
		NodeID:            "node-a",
		DeploymentMode:    "single_node",
		PublicBaseURL:     "http://127.0.0.1:60001",
		BindAddr:          "127.0.0.1:60001",
		TrustedProxyCIDRs: []string{"100.64.0.0/10"},
		StartedAt:         startedAt,
	})

	if nodeInfo.NodeID != "node-a" {
		t.Fatalf("NodeID = %q", nodeInfo.NodeID)
	}
	if nodeInfo.DeploymentMode != "single_node" {
		t.Fatalf("DeploymentMode = %q", nodeInfo.DeploymentMode)
	}
	if nodeInfo.PublicBaseURL != "http://127.0.0.1:60001" {
		t.Fatalf("PublicBaseURL = %q", nodeInfo.PublicBaseURL)
	}
	if nodeInfo.BindAddr != "127.0.0.1:60001" {
		t.Fatalf("BindAddr = %q", nodeInfo.BindAddr)
	}
	if got, want := len(nodeInfo.TrustedProxyCIDRs), 1; got != want {
		t.Fatalf("TrustedProxyCIDRs length = %d, want %d", got, want)
	}
	if !nodeInfo.StartedAt.Equal(startedAt.UTC()) {
		t.Fatalf("StartedAt = %s, want %s", nodeInfo.StartedAt, startedAt.UTC())
	}
}

// TestNewNodeInfoDefaultsStartedAt 验证调用方未传入启动时间时，
// runtime 模块会自动生成非零的 UTC 启动时间。
func TestNewNodeInfoDefaultsStartedAt(t *testing.T) {
	nodeInfo := NewNodeInfo(NodeOptions{})

	if nodeInfo.StartedAt.IsZero() {
		t.Fatal("StartedAt is zero")
	}
	if nodeInfo.StartedAt.Location() != time.UTC {
		t.Fatalf("StartedAt location = %s, want UTC", nodeInfo.StartedAt.Location())
	}
}

// TestNewNodeInfoCopiesTrustedProxyCIDRs 验证 NewNodeInfo 会复制可信代理网段切片，
// 避免调用方修改原始切片后影响已经构建的节点快照。
func TestNewNodeInfoCopiesTrustedProxyCIDRs(t *testing.T) {
	cidrs := []string{"100.64.0.0/10"}

	nodeInfo := NewNodeInfo(NodeOptions{
		TrustedProxyCIDRs: cidrs,
	})
	cidrs[0] = "192.168.0.0/16"

	if nodeInfo.TrustedProxyCIDRs[0] != "100.64.0.0/10" {
		t.Fatalf("TrustedProxyCIDRs[0] = %q", nodeInfo.TrustedProxyCIDRs[0])
	}
}
