package repository

import (
	"messagefeed/internal/domain"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAgentScheduleEvalMigrationDefinesRequiredTables(t *testing.T) {
	content, err := os.ReadFile("../../migrations/000025_add_agent_scheduled_tasks_eval.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	text := string(content)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS agent_scheduled_tasks",
		"CREATE TABLE IF NOT EXISTS agent_eval_cases",
		"CREATE TABLE IF NOT EXISTS agent_eval_runs",
		"CREATE TABLE IF NOT EXISTS agent_eval_results",
		"allowed_capabilities_json JSONB NOT NULL DEFAULT '[]'::jsonb",
		"CONSTRAINT chk_agent_scheduled_tasks_status",
		"CONSTRAINT chk_agent_eval_results_score",
		"FOR EACH ROW",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestNormalizeAgentScheduledTaskDefaults(t *testing.T) {
	task := normalizeAgentScheduledTask(domain.AgentScheduledTask{
		UserID: 1,
		Goal:   "  run daily digest  ",
	})
	if task.Status != domain.AgentScheduledTaskStatusQueued {
		t.Fatalf("status = %q, want queued", task.Status)
	}
	if task.TaskType != "agent_task" {
		t.Fatalf("task type = %q, want agent_task", task.TaskType)
	}
	if task.FreshnessPolicy != "latest_at_run" {
		t.Fatalf("freshness = %q, want latest_at_run", task.FreshnessPolicy)
	}
	if task.MaxAttempts != 3 {
		t.Fatalf("max attempts = %d, want 3", task.MaxAttempts)
	}
	if task.ScheduledAt.IsZero() {
		t.Fatal("scheduled at should be defaulted")
	}
	if task.AllowedCapabilities == nil || task.ModelPolicy == nil || task.FailurePolicy == nil || task.Payload == nil {
		t.Fatal("json fields and capability slice should be initialized")
	}
}

func TestAgentPlanStepRetryMigrationDefinesMetadata(t *testing.T) {
	content, err := os.ReadFile("../../migrations/000026_add_agent_plan_step_retry.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	text := string(content)
	for _, required := range []string{
		"retry_count INTEGER NOT NULL DEFAULT 0",
		"max_retries INTEGER NOT NULL DEFAULT 1",
		"retry_metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb",
		"chk_agent_plan_steps_retry",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestNormalizeAgentScheduledTaskClaimInput(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
	input := normalizeAgentScheduledTaskClaimInput(domain.AgentScheduledTaskClaimInput{
		Now:   now,
		Limit: 1000,
	})
	if input.WorkerID != "agent-scheduler" {
		t.Fatalf("worker = %q, want agent-scheduler", input.WorkerID)
	}
	if input.Limit != maxAgentScheduledTaskClaimLimit {
		t.Fatalf("limit = %d, want %d", input.Limit, maxAgentScheduledTaskClaimLimit)
	}
	if input.Now.Location() != time.UTC {
		t.Fatalf("now location = %v, want UTC", input.Now.Location())
	}
}

func TestNormalizeAgentEvalResultClampsScoreAndDefaultsStatus(t *testing.T) {
	result := normalizeAgentEvalResult(domain.AgentEvalResult{Score: 3})
	if result.Status != domain.AgentEvalResultStatusSkipped {
		t.Fatalf("status = %q, want skipped", result.Status)
	}
	if result.Score != 1 {
		t.Fatalf("score = %v, want 1", result.Score)
	}
	if result.Input == nil || result.Metrics == nil || result.EvidenceRefs == nil {
		t.Fatal("json fields and refs should be initialized")
	}
}
