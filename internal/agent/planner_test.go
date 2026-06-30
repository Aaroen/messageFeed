package agent

import (
	"context"
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestPlannerBuildFromSpecBuildsPermissionBudgetAndQualityMetadata(t *testing.T) {
	now := time.Date(2026, 6, 25, 15, 0, 0, 0, time.UTC)
	planner := NewPlanner(PlannerOptions{Now: func() time.Time { return now }})

	output := planner.BuildFromSpec(context.Background(), PlanInput{
		UserID:    1,
		SessionID: 2,
		TurnID:    3,
		Goal:      "联网搜索最新 AI 新闻并明天上午九点汇报",
	}, PlanSpec{
		Intent:               "搜索最新 AI 新闻并在指定时间汇报",
		TaskType:             "scheduled_research",
		RequiredCapabilities: []string{"web.search", "agent.schedule_task"},
		RequiresSubAgent:     true,
		EvidenceRequirements: []PlanEvidenceRequirement{
			{EvidenceType: "news", Summary: "需要可引用的新闻来源", MinimumCount: 2, Required: true},
		},
	})

	if output.Plan.ID != 0 {
		t.Fatalf("unexpected persisted plan id = %d", output.Plan.ID)
	}
	permission := testAgentJSONMap(output.Plan.Metadata["permission_governance"])
	if permission == nil || permission["has_external_access"] != true || permission["requires_confirmation"] != false {
		t.Fatalf("permission governance = %#v", output.Plan.Metadata["permission_governance"])
	}
	budget := testAgentJSONMap(output.Plan.Metadata["budget_governance"])
	if budget == nil || budget["status"] != "within_budget" {
		t.Fatalf("budget governance = %#v", output.Plan.Metadata["budget_governance"])
	}
	quality := testAgentJSONMap(output.Plan.Metadata["planner_quality"])
	if quality == nil || quality["status"] != "passed" {
		t.Fatalf("planner quality = %#v", output.Plan.Metadata["planner_quality"])
	}
	if len(output.Steps) == 0 || output.Steps[0].RetryMetadata["permission"] == nil {
		t.Fatalf("steps missing permission metadata: %#v", output.Steps)
	}
}

func TestPlannerBuildUsesFallbackLocalContextOnly(t *testing.T) {
	planner := NewPlanner(PlannerOptions{})
	output := planner.Build(context.Background(), PlanInput{
		UserID: 1,
		Goal:   "Go 官方博客最近有什么",
	})
	if len(output.Steps) != 1 || output.Steps[0].CapabilityKey != "feed.query_recent_items" {
		t.Fatalf("steps = %#v, want fallback feed.query_recent_items only", output.Steps)
	}
	if output.Plan.Metadata["planner"] != "fallback-local-context-v1" {
		t.Fatalf("planner metadata = %#v", output.Plan.Metadata["planner"])
	}
}

func TestPlannerBuildFromSpecCreatesStructuredSubAgentPlan(t *testing.T) {
	now := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)
	planner := NewPlanner(PlannerOptions{Now: func() time.Time { return now }})

	output := planner.BuildFromSpec(context.Background(), PlanInput{
		UserID:    1,
		SessionID: 2,
		TurnID:    3,
		Goal:      "搜索最新港股消息并分析",
	}, PlanSpec{
		Intent:           "搜索最新港股消息并形成简要分析",
		TaskType:         "news_analysis",
		Complexity:       PlanningComplexityComplex,
		RequiresSubAgent: true,
		Subtasks: []PlanSubtask{
			{
				Title:           "检索港股消息",
				Prompt:          "检索最新港股市场消息，优先返回新闻、行情、资金流向和公告来源。",
				ContextSummary:  "用户需要事实依据和最终分析，不需要执行层治理说明。",
				CapabilityKeys:  []string{"web.search", "web.fetch_page"},
				ExpectedOutput:  "相关来源、发布时间、摘要和可支撑分析的事实。",
				FailureStrategy: "搜索为空时调整查询并返回失败观察。",
				MaxRetries:      2,
			},
		},
		EvidenceRequirements: []PlanEvidenceRequirement{
			{EvidenceType: "market_news", Summary: "至少两条相关新闻或行情来源", MinimumCount: 2, Freshness: "latest", Required: true},
		},
		MaxIterations:          1,
		FinalAnswerConstraints: []string{"只输出结论、依据和分析过程"},
	})

	if output.Plan.Metadata["planner"] != "main-agent-structured-v1" {
		t.Fatalf("planner metadata = %#v", output.Plan.Metadata)
	}
	if output.Plan.Metadata["execution_mode"] != "subagent_execution" {
		t.Fatalf("execution mode = %#v", output.Plan.Metadata["execution_mode"])
	}
	if output.Plan.Status != domain.AgentPlanStatusApproved {
		t.Fatalf("plan status = %q", output.Plan.Status)
	}
	if !testPlanHasStep(output.Steps, "web.search") || !testPlanHasStep(output.Steps, "web.fetch_page") {
		t.Fatalf("steps = %#v, want web.search and web.fetch_page", output.Steps)
	}
	step := testPlanStep(output.Steps, "web.search")
	subAgent := testAgentJSONMap(step.RetryMetadata["sub_agent"])
	if subAgent == nil || subAgent["prompt"] == "" || subAgent["execution_mode"] != "subagent_execution" {
		t.Fatalf("sub agent metadata = %#v", step.RetryMetadata["sub_agent"])
	}
	mainPlan := testAgentJSONMap(output.Plan.Metadata["main_agent_plan"])
	if mainPlan == nil || mainPlan["complexity"] != string(PlanningComplexityComplex) {
		t.Fatalf("main agent plan metadata = %#v", output.Plan.Metadata["main_agent_plan"])
	}
}

func TestPlannerBuildFromSpecAllowsDirectAnswerWithoutToolSteps(t *testing.T) {
	planner := NewPlanner(PlannerOptions{})

	output := planner.BuildFromSpec(context.Background(), PlanInput{
		UserID: 1,
		Goal:   "说明一下当前项目 agent 闭环是什么",
	}, PlanSpec{
		Intent:              "解释当前项目 agent 闭环",
		TaskType:            "question_answer",
		Complexity:          PlanningComplexitySimple,
		DirectAnswerAllowed: true,
	})

	if output.Plan.Status != domain.AgentPlanStatusApproved {
		t.Fatalf("plan status = %q", output.Plan.Status)
	}
	if len(output.Steps) != 0 {
		t.Fatalf("steps = %#v, want no tool steps", output.Steps)
	}
	if output.Plan.Metadata["execution_mode"] != "direct_answer" {
		t.Fatalf("execution mode = %#v", output.Plan.Metadata["execution_mode"])
	}
	quality := testAgentJSONMap(output.Plan.Metadata["planner_quality"])
	if quality == nil || quality["status"] != "passed" {
		t.Fatalf("planner quality = %#v", output.Plan.Metadata["planner_quality"])
	}
}

func TestPlannerBuildFromSpecAddsHistoryRecallCapability(t *testing.T) {
	planner := NewPlanner(PlannerOptions{})

	output := planner.BuildFromSpec(context.Background(), PlanInput{
		UserID: 1,
		Goal:   "我之前说过的回复偏好是什么",
	}, PlanSpec{
		Intent:              "查询用户此前表达的回复偏好",
		TaskType:            "history_recall",
		NeedsHistoryRecall:  true,
		DirectAnswerAllowed: false,
	})

	if !testPlanHasStep(output.Steps, "conversation.query_history") {
		t.Fatalf("steps = %#v, want conversation.query_history", output.Steps)
	}
	if !testStringSliceContains(output.Plan.AllowedScopes, "conversation.query_history") {
		t.Fatalf("allowed scopes = %#v, want conversation.query_history", output.Plan.AllowedScopes)
	}
	mainPlan := testAgentJSONMap(output.Plan.Metadata["main_agent_plan"])
	if mainPlan == nil || mainPlan["needs_history_recall"] != true {
		t.Fatalf("main agent plan metadata = %#v", output.Plan.Metadata["main_agent_plan"])
	}
	historyPlan := testAgentJSONMap(mainPlan["history_query_plan"])
	if historyPlan == nil || historyPlan["mode"] != "search" || historyPlan["query"] == "" {
		t.Fatalf("history query plan = %#v", historyPlan)
	}
}

func testPlanHasStep(steps []domain.AgentPlanStep, capabilityKey string) bool {
	for _, step := range steps {
		if step.CapabilityKey == capabilityKey {
			return true
		}
	}
	return false
}

func testPlanStep(steps []domain.AgentPlanStep, capabilityKey string) domain.AgentPlanStep {
	for _, step := range steps {
		if step.CapabilityKey == capabilityKey {
			return step
		}
	}
	return domain.AgentPlanStep{}
}

func testStringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func testAgentJSONMap(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	if typed, ok := value.(domain.AgentJSON); ok {
		return map[string]any(typed)
	}
	return nil
}
