package agent

import (
	"context"
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestPlannerBuildsPermissionBudgetAndQualityMetadata(t *testing.T) {
	now := time.Date(2026, 6, 25, 15, 0, 0, 0, time.UTC)
	planner := NewPlanner(PlannerOptions{Now: func() time.Time { return now }})

	output := planner.Build(context.Background(), PlanInput{
		UserID:    1,
		SessionID: 2,
		TurnID:    3,
		Goal:      "联网搜索最新 AI 新闻并明天上午九点汇报",
	})

	if output.Plan.ID != 0 {
		t.Fatalf("unexpected persisted plan id = %d", output.Plan.ID)
	}
	permission := testAgentJSONMap(output.Plan.Metadata["permission_governance"])
	if permission == nil || permission["has_external_access"] != true || permission["requires_confirmation"] != true {
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

func TestPlannerSelectsSourceLatestForNamedSourceQuery(t *testing.T) {
	planner := NewPlanner(PlannerOptions{})
	output := planner.Build(context.Background(), PlanInput{
		UserID: 1,
		Goal:   "Go 官方博客最近有什么",
	})
	if !testPlanHasStep(output.Steps, "source.query_latest_items") {
		t.Fatalf("steps = %#v, want source.query_latest_items", output.Steps)
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

func testAgentJSONMap(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	if typed, ok := value.(domain.AgentJSON); ok {
		return map[string]any(typed)
	}
	return nil
}
