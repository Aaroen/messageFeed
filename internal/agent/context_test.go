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

func TestDefaultContextBuilderBuildsContextBundleBudgetProfile(t *testing.T) {
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	builder := NewDefaultContextBuilder(DefaultContextBuilderOptions{
		ConversationMemory: fakeConversationMemoryProvider{
			memory: ConversationMemory{
				Messages: []ContextMessage{
					{Role: domain.AgentTranscriptRoleUser, Content: "上一轮问题", TranscriptEntryID: 1, TurnID: 1, CreatedAt: now.Add(-2 * time.Minute)},
					{Role: domain.AgentTranscriptRoleAssistant, Content: "上一轮回答", TranscriptEntryID: 2, TurnID: 1, CreatedAt: now.Add(-1 * time.Minute)},
				},
			},
		},
		Now: func() time.Time { return now },
	})

	snapshot, err := builder.Build(context.Background(), ContextBuildInput{
		UserID:        1,
		SessionID:     2,
		TurnID:        3,
		MessageText:   "继续刚才任务",
		BudgetProfile: ContextBudgetProfileMainPlanning,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if snapshot.BudgetProfile != ContextBudgetProfileMainPlanning {
		t.Fatalf("budget profile = %q", snapshot.BudgetProfile)
	}
	if snapshot.Bundle.BudgetProfile != ContextBudgetProfileMainPlanning {
		t.Fatalf("bundle budget profile = %q", snapshot.Bundle.BudgetProfile)
	}
	if snapshot.BudgetReport.TotalBudgetTokens != 64000 || snapshot.BudgetReport.RecentMessagesTokens != 32000 {
		t.Fatalf("budget report = %#v", snapshot.BudgetReport)
	}
	if snapshot.Bundle.CurrentMessage == nil || snapshot.Bundle.CurrentMessage.Content != "继续刚才任务" {
		t.Fatalf("current message = %#v", snapshot.Bundle.CurrentMessage)
	}
	if len(snapshot.Messages) != 2 {
		t.Fatalf("selected message count = %d, want 2", len(snapshot.Messages))
	}
	if len(snapshot.SemanticUnits) == 0 || snapshot.SemanticUnits[0].ID != "current_user_message" {
		t.Fatalf("semantic units = %#v", snapshot.SemanticUnits)
	}
}

func TestSelectSemanticUnitsByTokenBudgetKeepsWholeUnits(t *testing.T) {
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	messages := []ContextMessage{
		{Role: domain.AgentTranscriptRoleUser, Content: "第一轮问题，内容较长较长较长", TranscriptEntryID: 1, TurnID: 1, CreatedAt: now.Add(-6 * time.Minute)},
		{Role: domain.AgentTranscriptRoleAssistant, Content: "第一轮回答，内容较长较长较长", TranscriptEntryID: 2, TurnID: 1, CreatedAt: now.Add(-5 * time.Minute)},
		{Role: domain.AgentTranscriptRoleUser, Content: "第二轮问题，内容较长较长较长", TranscriptEntryID: 3, TurnID: 2, CreatedAt: now.Add(-4 * time.Minute)},
		{Role: domain.AgentTranscriptRoleAssistant, Content: "第二轮回答，内容较长较长较长", TranscriptEntryID: 4, TurnID: 2, CreatedAt: now.Add(-3 * time.Minute)},
		{Role: domain.AgentTranscriptRoleUser, Content: "最新问题", TranscriptEntryID: 5, TurnID: 3, CreatedAt: now.Add(-2 * time.Minute)},
		{Role: domain.AgentTranscriptRoleAssistant, Content: "最新回答", TranscriptEntryID: 6, TurnID: 3, CreatedAt: now.Add(-1 * time.Minute)},
	}
	units := BuildConversationSemanticUnits(messages)
	if len(units) != 3 {
		t.Fatalf("unit count = %d, want 3", len(units))
	}
	budget := units[2].TokenEstimate
	selected, report := SelectSemanticUnitsByTokenBudget(units, budget, ContextBudgetProfileSubagentAnalysis)
	selectedMessages := SelectedMessagesFromSemanticUnits(selected)

	if len(selectedMessages) != 2 {
		t.Fatalf("selected message count = %d, want 2", len(selectedMessages))
	}
	if selectedMessages[0].Content != "最新问题" || selectedMessages[1].Content != "最新回答" {
		t.Fatalf("selected messages = %#v", selectedMessages)
	}
	if report.SelectedUnitCount != 1 || report.SkippedUnitCount != 2 {
		t.Fatalf("budget report = %#v", report)
	}
	for _, unit := range selected {
		if strings.Contains(unit.Content, "第一轮") && unit.Selected {
			t.Fatalf("older unit should be skipped as a whole: %#v", unit)
		}
		if unit.OmittedReason != "" && strings.TrimSpace(unit.Content) == "" {
			t.Fatalf("skipped unit lost original projection content: %#v", unit)
		}
	}
}

func TestSelectSemanticUnitsByTokenBudgetProjectsOversizedUnitWithoutHardTruncation(t *testing.T) {
	unit := ContextSemanticUnit{
		ID:              "oversized",
		Type:            ContextSemanticUnitMessage,
		Source:          "recent_conversation",
		Content:         strings.Repeat("长内容", 40),
		Messages:        []ContextMessage{{Role: domain.AgentTranscriptRoleAssistant, Content: strings.Repeat("长内容", 40)}},
		Protected:       true,
		RetentionReason: "previous_assistant_answer",
	}
	ensureSemanticUnitTokenEstimate(&unit)
	original := unit.Content

	selected, report := SelectSemanticUnitsByTokenBudget([]ContextSemanticUnit{unit}, 1, ContextBudgetProfileSubagentAnalysis)
	if len(SelectedMessagesFromSemanticUnits(selected)) != 0 {
		t.Fatalf("oversized unit should not be selected under impossible budget")
	}
	if !selected[0].Projected || selected[0].OmittedReason == "" {
		t.Fatalf("oversized unit projection metadata missing: %#v", selected[0])
	}
	if selected[0].Content != original {
		t.Fatalf("oversized unit content was hard-truncated")
	}
	if report.OversizedUnitCount != 1 || report.SkippedUnitCount != 1 {
		t.Fatalf("budget report = %#v", report)
	}
}

func TestFormatContextMessagesDoesNotApplyFixedTwelveMessageWindow(t *testing.T) {
	messages := make([]ContextMessage, 0, 13)
	for i := 0; i < 13; i++ {
		messages = append(messages, ContextMessage{
			Role:    domain.AgentTranscriptRoleUser,
			Content: "消息" + string(rune('A'+i)),
		})
	}
	formatted := FormatContextMessages(messages)
	if !strings.Contains(formatted, "消息M") {
		t.Fatalf("formatted messages should include the thirteenth semantic input: %q", formatted)
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

type fakeConversationMemoryProvider struct {
	memory ConversationMemory
}

func (p fakeConversationMemoryProvider) BuildConversationMemory(_ context.Context, _ ContextBuildInput) (ConversationMemory, error) {
	return p.memory, nil
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
