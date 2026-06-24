package agent

import (
	"context"
	"testing"
	"time"
)

func TestDefaultContextBuilderBuildsUserContextCapabilityBlocksAndBoundary(t *testing.T) {
	now := time.Date(2026, 6, 24, 18, 30, 0, 0, time.UTC)
	builder := NewDefaultContextBuilder(DefaultContextBuilderOptions{
		UserContextProvider: fakeUserContextProvider{now: now},
		Executor: fakeCapabilityExecutor{
			block: ContextBlock{
				Name:    "最近条目",
				Content: "1. Go 1.26 发布",
			},
		},
		CapabilityKeys: []string{"feed.query_recent_items"},
		Now:            func() time.Time { return now },
	})

	snapshot, err := builder.Build(context.Background(), ContextBuildInput{
		UserID:      1,
		SessionID:   2,
		TurnID:      3,
		MessageText: "最近有什么",
		MessageType: "text",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(snapshot.Blocks) != 3 {
		t.Fatalf("block count = %d, want 3", len(snapshot.Blocks))
	}
	if snapshot.Blocks[0].Name != "用户上下文" {
		t.Fatalf("first block = %q", snapshot.Blocks[0].Name)
	}
	if snapshot.Blocks[1].CapabilityKey != "feed.query_recent_items" {
		t.Fatalf("capability block key = %q", snapshot.Blocks[1].CapabilityKey)
	}
	if snapshot.Blocks[2].Name != "可用能力边界" {
		t.Fatalf("boundary block = %q", snapshot.Blocks[2].Name)
	}
	if len(snapshot.Observations) != 2 {
		t.Fatalf("observation count = %d, want 2", len(snapshot.Observations))
	}
}

func TestDefaultContextBuilderSkipsUnregisteredCapability(t *testing.T) {
	builder := NewDefaultContextBuilder(DefaultContextBuilderOptions{
		CapabilityKeys: []string{"missing.capability"},
	})
	snapshot, err := builder.Build(context.Background(), ContextBuildInput{UserID: 1})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(snapshot.Observations) != 1 {
		t.Fatalf("observation count = %d, want 1", len(snapshot.Observations))
	}
	if snapshot.Observations[0].Decision != string(PolicyDecisionForbidden) {
		t.Fatalf("decision = %q, want forbidden", snapshot.Observations[0].Decision)
	}
}

type fakeUserContextProvider struct {
	now time.Time
}

func (p fakeUserContextProvider) BuildUserContextBlock(_ context.Context, _ int64) (ContextBlock, error) {
	return ContextBlock{
		Name:        "用户上下文",
		Content:     "当前用户：aroen",
		ItemCount:   1,
		GeneratedAt: p.now,
		TrustLevel:  "user_profile",
	}, nil
}

type fakeCapabilityExecutor struct {
	block ContextBlock
}

func (e fakeCapabilityExecutor) Execute(_ context.Context, input CapabilityExecuteInput) (CapabilityExecuteResult, error) {
	return CapabilityExecuteResult{
		Blocks: []ContextBlock{e.block},
		Observation: CapabilityObservation{
			Capability: input.Capability.Key,
			Decision:   string(PolicyDecisionAllow),
			Status:     "succeeded",
			Summary:    "loaded",
		},
	}, nil
}
