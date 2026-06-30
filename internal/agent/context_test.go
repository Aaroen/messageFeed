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

func TestDefaultContextBuilderBuildsShortTermProtectionBundle(t *testing.T) {
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	activePlan := &ContextBlock{
		Name:          "当前活动计划",
		Content:       "goal: 汇总上下文管理实现\nsteps:\n- #1 补齐保护区 [pending] capability=content.summarize_text",
		CanonicalRef:  "agent_plan:7",
		EvidenceRefs:  []string{"agent_plan:7", "agent_plan_step:8"},
		GeneratedAt:   now,
		TrustLevel:    "planner",
		Source:        "active_plan",
		CapabilityKey: "agent.plan",
	}
	builder := NewDefaultContextBuilder(DefaultContextBuilderOptions{
		UserContextProvider: fakeUserContextProvider{now: now},
		ConversationMemory: fakeConversationMemoryProvider{
			memory: ConversationMemory{
				Messages: []ContextMessage{
					{Role: domain.AgentTranscriptRoleUser, Content: "上一轮问题", TranscriptEntryID: 1, TurnID: 1, CreatedAt: now.Add(-2 * time.Minute)},
					{Role: domain.AgentTranscriptRoleAssistant, Content: "上一轮完整回答", TranscriptEntryID: 2, TurnID: 1, CreatedAt: now.Add(-time.Minute)},
				},
			},
		},
		Now: func() time.Time { return now },
	})

	snapshot, err := builder.Build(context.Background(), ContextBuildInput{
		UserID:        1,
		SessionID:     2,
		TurnID:        3,
		MessageText:   "继续实现",
		BudgetProfile: ContextBudgetProfileMainPlanning,
		ActiveGoal:    "汇总上下文管理实现",
		ActivePlan:    activePlan,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if snapshot.Bundle.CurrentMessage == nil || snapshot.Bundle.CurrentMessage.Content != "继续实现" {
		t.Fatalf("current message = %#v", snapshot.Bundle.CurrentMessage)
	}
	if snapshot.Bundle.ActivePlan == nil || snapshot.Bundle.ActivePlan.CanonicalRef != "plan:7" {
		t.Fatalf("active plan = %#v", snapshot.Bundle.ActivePlan)
	}
	if len(snapshot.Bundle.SystemBlocks) == 0 || snapshot.Bundle.SystemBlocks[0].CapabilityKey != "capability.list_available" {
		t.Fatalf("system blocks = %#v", snapshot.Bundle.SystemBlocks)
	}
	if len(snapshot.Bundle.UserConstraints) == 0 || snapshot.Bundle.UserConstraints[0].Source != "user_profile" {
		t.Fatalf("user constraints = %#v", snapshot.Bundle.UserConstraints)
	}
	if !contextTestHasProtectedUnit(snapshot.SemanticUnits, "active_plan", "active_plan") {
		t.Fatalf("semantic units missing protected active plan: %#v", snapshot.SemanticUnits)
	}
	if !contextTestHasProtectedReason(snapshot.SemanticUnits, "previous_assistant_answer") {
		t.Fatalf("semantic units missing previous assistant protection: %#v", snapshot.SemanticUnits)
	}
}

func TestRefreshContextBundleAddsRuntimeObservationArtifactRefs(t *testing.T) {
	snapshot := ContextSnapshot{
		BudgetProfile: ContextBudgetProfileSubagentSearch,
		Bundle:        ContextBundle{BudgetProfile: ContextBudgetProfileSubagentSearch},
		Observations: []CapabilityObservation{
			{
				Capability:     "web.search",
				Status:         "succeeded",
				Summary:        "loaded web evidence",
				ObservationRef: "agent_observations/31",
				ArtifactRefs:   []string{"agent_artifact:41", "agent_run:9"},
			},
		},
	}

	snapshot = refreshContextSnapshotBundle(snapshot)

	if len(snapshot.Bundle.KeyObservations) != 1 {
		t.Fatalf("key observations = %#v", snapshot.Bundle.KeyObservations)
	}
	if snapshot.Bundle.KeyObservations[0].ObservationRef != "observation:31" {
		t.Fatalf("observation ref = %q", snapshot.Bundle.KeyObservations[0].ObservationRef)
	}
	if len(snapshot.Bundle.KeyArtifacts) != 1 {
		t.Fatalf("key artifacts = %#v", snapshot.Bundle.KeyArtifacts)
	}
	if strings.Join(snapshot.Bundle.KeyArtifacts[0].EvidenceRefs, ",") != "artifact:41" {
		t.Fatalf("artifact refs = %#v", snapshot.Bundle.KeyArtifacts[0].EvidenceRefs)
	}
}

func TestDefaultContextBuilderAddsCanonicalRefsToHistoryBlock(t *testing.T) {
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	builder := NewDefaultContextBuilder(DefaultContextBuilderOptions{
		ConversationMemory: fakeConversationMemoryProvider{
			memory: ConversationMemory{
				HistoryQueried: true,
				HistoryResults: []ContextMessage{
					{Role: domain.AgentTranscriptRoleUser, Content: "历史偏好", TranscriptEntryID: 123, TurnID: 1, CreatedAt: now.Add(-time.Hour)},
				},
				HistoryResultContent: "命中原文：历史偏好",
			},
		},
		Now: func() time.Time { return now },
	})

	snapshot, err := builder.Build(context.Background(), ContextBuildInput{
		UserID:      1,
		SessionID:   2,
		TurnID:      3,
		MessageText: "我之前说过什么偏好吗",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	var historyBlock ContextBlock
	for _, block := range snapshot.Blocks {
		if block.CapabilityKey == "conversation.query_history" {
			historyBlock = block
			break
		}
	}
	if strings.Join(historyBlock.EvidenceRefs, ",") != "transcript:123" {
		t.Fatalf("history evidence refs = %#v", historyBlock.EvidenceRefs)
	}
	if historyBlock.Source != "history_query_plan" {
		t.Fatalf("history block source = %q", historyBlock.Source)
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

func contextTestHasProtectedUnit(units []ContextSemanticUnit, id string, reason string) bool {
	for _, unit := range units {
		if unit.ID == id && unit.Protected && unit.Selected && unit.RetentionReason == reason {
			return true
		}
	}
	return false
}

func contextTestHasProtectedReason(units []ContextSemanticUnit, reason string) bool {
	for _, unit := range units {
		if unit.Protected && unit.Selected && unit.RetentionReason == reason {
			return true
		}
	}
	return false
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

func TestRecentConversationCandidateLimitIsDerivedFromTokenBudget(t *testing.T) {
	mainLimit := RecentConversationCandidateLimit(ContextBudgetProfileMainPlanning)
	searchLimit := RecentConversationCandidateLimit(ContextBudgetProfileSubagentSearch)
	if mainLimit <= searchLimit {
		t.Fatalf("main planning candidate limit = %d, search candidate limit = %d", mainLimit, searchLimit)
	}
	if searchLimit <= 12 {
		t.Fatalf("candidate limit should not preserve the old fixed 12 message strategy: %d", searchLimit)
	}
	if mainLimit > 240 {
		t.Fatalf("candidate limit should keep an engineering query cap: %d", mainLimit)
	}
}

func TestContextBudgetProfileForCapabilityScope(t *testing.T) {
	cases := []struct {
		name string
		keys []string
		want ContextBudgetProfile
	}{
		{name: "history recall", keys: []string{"conversation.query_history"}, want: ContextBudgetProfileSubagentHistoryRecall},
		{name: "search", keys: []string{"web.search"}, want: ContextBudgetProfileSubagentSearch},
		{name: "local source search", keys: []string{"feed.query_recent_items"}, want: ContextBudgetProfileSubagentSearch},
		{name: "analysis", keys: []string{"content.summarize_text"}, want: ContextBudgetProfileSubagentAnalysis},
		{name: "empty", keys: nil, want: ContextBudgetProfileSubagentAnalysis},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContextBudgetProfileForCapabilityScope(tt.keys); got != tt.want {
				t.Fatalf("profile = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeCanonicalRefSupportsLegacyEvidenceRefs(t *testing.T) {
	cases := map[string]string{
		"agent_transcript_entry:123": "transcript:123",
		"agent_observations/31":      "observation:31",
		"agent_artifact:41":          "artifact:41",
		"agent_plan_step:6":          "plan_step:6",
		"agent_run:12":               "run:12",
		"item:3595":                  "item:3595",
	}
	for input, want := range cases {
		if got := NormalizeCanonicalRef(input); got != want {
			t.Fatalf("NormalizeCanonicalRef(%q) = %q, want %q", input, got, want)
		}
	}
	if got := NormalizeCanonicalRef("web_search:https://example.com/path"); got != "web_search:https://example.com/path" {
		t.Fatalf("URL ref should be preserved when no stable numeric fact id exists: %q", got)
	}
}

func TestNormalizeCanonicalRefsDeduplicatesRefs(t *testing.T) {
	refs := NormalizeCanonicalRefs([]string{"agent_observation:31", "agent_observations/31", "artifact:2"})
	if strings.Join(refs, ",") != "observation:31,artifact:2" {
		t.Fatalf("refs = %#v", refs)
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
