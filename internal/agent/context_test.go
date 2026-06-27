package agent

import (
	"context"
	"messagefeed/internal/domain"
	"strings"
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
	if len(snapshot.Blocks) != 2 {
		t.Fatalf("block count = %d, want 2", len(snapshot.Blocks))
	}
	if snapshot.Blocks[0].Name != "用户上下文" {
		t.Fatalf("first block = %q", snapshot.Blocks[0].Name)
	}
	if snapshot.Blocks[1].Name != "可用能力边界" {
		t.Fatalf("boundary block = %q", snapshot.Blocks[1].Name)
	}
	if len(snapshot.Observations) != 1 {
		t.Fatalf("observation count = %d, want 1", len(snapshot.Observations))
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

func TestDefaultContextBuilderUsesInputCapabilityKeysForPlannedScope(t *testing.T) {
	calls := []string{}
	builder := NewDefaultContextBuilder(DefaultContextBuilderOptions{
		Executor: fakeCapabilityExecutor{
			block: ContextBlock{
				Name:    "联网搜索结果",
				Content: "工具：web.search\n查询：港股",
			},
			calls: &calls,
		},
		CapabilityKeys: []string{"feed.query_recent_items", "source.query_latest_items"},
	})

	snapshot, err := builder.Build(context.Background(), ContextBuildInput{
		UserID:         1,
		MessageText:    "搜索最新港股消息并分析",
		CapabilityKeys: []string{"feed.query_recent_items", "web.search"},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if strings.Join(calls, ",") != "" {
		t.Fatalf("capability calls = %#v", calls)
	}
	boundary := snapshot.Blocks[len(snapshot.Blocks)-1].Content
	if !strings.Contains(boundary, "web.search") || strings.Contains(boundary, "source.query_latest_items") {
		t.Fatalf("boundary = %q", boundary)
	}
}

func TestHistoryNeedClassificationAndRecentEvidence(t *testing.T) {
	for _, message := range []string{
		"我之前说过关注什么吗",
		"我发的第一条消息是什么",
		"刚才 Go 官方博客 那个继续",
		"最近有什么更新",
	} {
		if got := ClassifyHistoryNeed(message); got != HistoryNeedNone {
			t.Fatalf("history hint = %q for %q", got, message)
		}
	}

	recent := []ContextMessage{
		{Role: domain.AgentTranscriptRoleUser, Content: "我想关注 Go 官方博客"},
		{Role: domain.AgentTranscriptRoleAssistant, Content: "已理解。"},
	}
	if ShouldQueryConversationHistory(HistoryNeedRequired, "我之前说过什么偏好吗", recent) {
		t.Fatal("history should not be queried before model tool call")
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
	calls *[]string
}

func (e fakeCapabilityExecutor) Execute(_ context.Context, input CapabilityExecuteInput) (CapabilityExecuteResult, error) {
	if e.calls != nil {
		*e.calls = append(*e.calls, input.Capability.Key)
	}
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
