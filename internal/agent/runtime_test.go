package agent

import (
	"context"
	"testing"
)

func TestP0CapabilityRegistryContainsReadOnlyCapabilities(t *testing.T) {
	registry := NewP0CapabilityRegistry()
	capability, ok := registry.Get("feed.query_recent_items")
	if !ok {
		t.Fatal("feed.query_recent_items was not registered")
	}
	if capability.Mutates {
		t.Fatal("feed.query_recent_items should be read-only")
	}
	if capability.Mode != CapabilityModeCore {
		t.Fatalf("mode = %q, want core", capability.Mode)
	}
}

func TestP0CapabilityRegistryContainsWebCapabilities(t *testing.T) {
	registry := NewP0CapabilityRegistry()
	for _, key := range []string{"web.search", "web.fetch_page", "web.extract_page", "repo.search", "repo.inspect_remote"} {
		capability, ok := registry.Get(key)
		if !ok {
			t.Fatalf("%s was not registered", key)
		}
		if !capability.ExternalAccess {
			t.Fatalf("%s should be marked as external access", key)
		}
		if capability.Mutates {
			t.Fatalf("%s should be read-only", key)
		}
	}
}

func TestP0CapabilityRegistryContainsMemoryFactRecall(t *testing.T) {
	registry := NewP0CapabilityRegistry()
	capability, ok := registry.Get("memory.fact_recall")
	if !ok {
		t.Fatal("memory.fact_recall was not registered")
	}
	if capability.Mutates {
		t.Fatal("memory.fact_recall should be read-only")
	}
	if capability.ExternalAccess {
		t.Fatal("memory.fact_recall should not be marked as external access")
	}
	if capability.DataDomain != "long_term_memory" {
		t.Fatalf("data domain = %q, want long_term_memory", capability.DataDomain)
	}
}

func TestP0CapabilityRegistryContainsScheduleTask(t *testing.T) {
	registry := NewP0CapabilityRegistry()
	capability, ok := registry.Get("agent.schedule_task")
	if !ok {
		t.Fatal("agent.schedule_task was not registered")
	}
	if !capability.Mutates || !capability.Schedulable {
		t.Fatalf("schedule task capability = %#v, want mutating schedulable", capability)
	}
	if capability.Risk != CapabilityRiskMedium {
		t.Fatalf("risk = %q, want medium", capability.Risk)
	}
}

func TestPlannerBuildFromSpecAllowsScheduleTaskToToolConfirmation(t *testing.T) {
	planner := NewPlanner(PlannerOptions{})
	output := planner.BuildFromSpec(context.Background(), PlanInput{
		UserID: 1,
		Goal:   "明天上午九点提醒我检查部署状态",
	}, PlanSpec{
		Intent:               "创建定时提醒",
		TaskType:             "scheduled_task",
		RequiredCapabilities: []string{"agent.schedule_task"},
		RequiresSubAgent:     true,
	})
	if output.Plan.Status != "approved" {
		t.Fatalf("plan status = %q, want approved", output.Plan.Status)
	}
	found := false
	for _, step := range output.Steps {
		if step.CapabilityKey == "agent.schedule_task" {
			found = true
		}
	}
	if !found {
		t.Fatalf("steps = %#v, want agent.schedule_task", output.Steps)
	}
}

func TestPolicyEngineAllowsReadOnlyAndPromptsMutatingCapability(t *testing.T) {
	engine := NewPolicyEngine()
	readOnly := Capability{Key: "feed.query_recent_items", Risk: CapabilityRiskLow}
	if decision := engine.Decide(context.Background(), PolicyInput{Capability: readOnly, UserID: 1}); decision.Decision != PolicyDecisionAllow {
		t.Fatalf("read-only decision = %q, want allow", decision.Decision)
	}

	mutating := Capability{Key: "source.subscribe", Risk: CapabilityRiskMedium, Mutates: true}
	if decision := engine.Decide(context.Background(), PolicyInput{Capability: mutating, UserID: 1}); decision.Decision != PolicyDecisionPrompt {
		t.Fatalf("mutating decision = %q, want prompt", decision.Decision)
	}

	toolConfirmed := Capability{
		Key:     "agent.schedule_task",
		Risk:    CapabilityRiskMedium,
		Mutates: true,
		InputSchema: map[string]any{"properties": map[string]any{
			"confirmed": map[string]any{"type": "boolean"},
		}},
	}
	if decision := engine.Decide(context.Background(), PolicyInput{Capability: toolConfirmed, UserID: 1}); decision.Decision != PolicyDecisionAllow {
		t.Fatalf("tool confirmed decision = %q, want allow", decision.Decision)
	}

	highRiskConfirmed := Capability{
		Key:     "source.edit",
		Risk:    CapabilityRiskHigh,
		Mutates: true,
		InputSchema: map[string]any{"properties": map[string]any{
			"confirmed": map[string]any{"type": "boolean"},
		}},
	}
	if decision := engine.Decide(context.Background(), PolicyInput{Capability: highRiskConfirmed, UserID: 1}); decision.Decision != PolicyDecisionPrompt {
		t.Fatalf("high risk confirmed decision = %q, want prompt", decision.Decision)
	}
}

func TestPolicyEngineForbidsMissingUser(t *testing.T) {
	engine := NewPolicyEngine()
	capability := Capability{Key: "feed.query_recent_items", Risk: CapabilityRiskLow}
	if decision := engine.Decide(context.Background(), PolicyInput{Capability: capability}); decision.Decision != PolicyDecisionForbidden {
		t.Fatalf("decision = %q, want forbidden", decision.Decision)
	}
}
