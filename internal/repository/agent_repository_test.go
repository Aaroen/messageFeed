package repository

import (
	"messagefeed/internal/domain"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAgentRecallEventModelMapsQueryTextColumn(t *testing.T) {
	field, ok := reflect.TypeOf(agentRecallEventModel{}).FieldByName("Query")
	if !ok {
		t.Fatal("Query field is missing")
	}
	if !strings.Contains(string(field.Tag), "column:query_text") {
		t.Fatalf("Query gorm tag = %q, want column:query_text", field.Tag)
	}
}

func TestAgentJSONSliceClonesKeepEmptyArray(t *testing.T) {
	if got := cloneStringSlice(nil); got == nil || len(got) != 0 {
		t.Fatalf("cloneStringSlice(nil) = %#v, want non-nil empty slice", got)
	}
	stepModel := agentPlanStepModelFromDomain(domain.AgentPlanStep{
		Status:          domain.AgentPlanStepStatusPending,
		CapabilityScope: []string{"web.search"},
		ArtifactRefs:    []string{},
		RetryMetadata:   domain.AgentJSON{},
	})
	if stepModel.ArtifactRefs == nil || len(stepModel.ArtifactRefs) != 0 {
		t.Fatalf("ArtifactRefs = %#v, want non-nil empty slice", stepModel.ArtifactRefs)
	}
}

func TestAgentRunUpdateColumnsPersistPlanScopeFields(t *testing.T) {
	for _, required := range []string{"TaskPacket", "CapabilityScope", "ContextBudget"} {
		if !agentRepositoryStringSliceContains(agentRunUpdateColumns, required) {
			t.Fatalf("agentRunUpdateColumns missing %q: %#v", required, agentRunUpdateColumns)
		}
	}
}

func TestAgentAuditLogModelUsesNullOptionalRefs(t *testing.T) {
	model := agentAuditLogModelFromDomain(domain.AgentAuditLog{
		UserID:    1,
		EventType: "agent.test",
		Status:    "review",
		Metadata:  domain.AgentJSON{},
	})
	if model.SessionID != nil || model.TurnID != nil {
		t.Fatalf("optional refs should be nil: session=%#v turn=%#v", model.SessionID, model.TurnID)
	}
	sessionID := int64(2)
	turnID := int64(3)
	converted := agentAuditLogModelToDomain(agentAuditLogModel{SessionID: &sessionID, TurnID: &turnID})
	if converted.SessionID != 2 || converted.TurnID != 3 {
		t.Fatalf("converted refs = %#v", converted)
	}
}

func TestAgentContextMemoryMigrationDefinesArchiveAndRecallTables(t *testing.T) {
	content, err := os.ReadFile("../../migrations/000019_add_agent_context_memory.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	text := string(content)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS agent_transcript_archive_index",
		"CREATE TABLE IF NOT EXISTS agent_recall_events",
		"query_text TEXT NOT NULL DEFAULT ''",
		"keywords_json JSONB NOT NULL DEFAULT '[]'::jsonb",
		"update_updated_at_column()",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestAgentSessionManagementMigrationDefinesActiveSessionAndContextState(t *testing.T) {
	content, err := os.ReadFile("../../migrations/000020_add_agent_session_management.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	text := string(content)
	for _, required := range []string{
		"active_agent_session_id BIGINT REFERENCES agent_sessions(id) ON DELETE SET NULL",
		"context_initialized_at TIMESTAMP WITH TIME ZONE",
		"context_version BIGINT NOT NULL DEFAULT 0",
		"transcript_count_indexed BIGINT NOT NULL DEFAULT 0",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestTranscriptMemoryClassificationRuleV2(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		kind       domain.AgentMemoryKind
		importance int
	}{
		{name: "decision", content: "最终决定就用 Go 实现 agent 框架", kind: domain.AgentMemoryKindDecision, importance: 80},
		{name: "task", content: "提醒我明天检查部署状态", kind: domain.AgentMemoryKindTask, importance: 70},
		{name: "preference", content: "我的偏好是短回复，不要写太多", kind: domain.AgentMemoryKindPreference, importance: 75},
		{name: "fact", content: "我的用户名是 aroen，时区是 Asia/Shanghai", kind: domain.AgentMemoryKindFact, importance: 55},
		{name: "casual", content: "你好，随便聊聊", kind: domain.AgentMemoryKindCasual, importance: 20},
		{name: "unknown", content: "  ", kind: domain.AgentMemoryKindUnknown, importance: 0},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyTranscriptMemoryKind(tt.content); got != tt.kind {
				t.Fatalf("kind = %q, want %q", got, tt.kind)
			}
			if got := transcriptImportance(tt.content); got != tt.importance {
				t.Fatalf("importance = %d, want %d", got, tt.importance)
			}
		})
	}
}

func TestTranscriptClassificationMetadataIncludesReclassifyFields(t *testing.T) {
	classification := classifyTranscriptMemory("我决定以后优先关注 Go")
	metadata := transcriptClassificationMetadata(classification, true)
	if metadata["classification_strategy"] != "rule_v2" {
		t.Fatalf("strategy = %#v", metadata["classification_strategy"])
	}
	if metadata["classification_version"] != 2 {
		t.Fatalf("version = %#v", metadata["classification_version"])
	}
	if metadata["llm_classifier_status"] != "not_requested" {
		t.Fatalf("llm classifier status = %#v", metadata["llm_classifier_status"])
	}
	if metadata["background_reclassify"] != true {
		t.Fatalf("background reclassify = %#v", metadata["background_reclassify"])
	}
	if metadata["rebuild"] != true {
		t.Fatalf("rebuild = %#v", metadata["rebuild"])
	}
}

func TestAgentFactSearchFragmentsExtractsRecallTokens(t *testing.T) {
	fragments := agentFactSearchFragments([]string{
		"RAG-E2E-4096-20260630 internal/service/agent_fact_retrieval.go 先给结论再说明依据",
	})
	for _, want := range []string{
		"RAG-E2E-4096-20260630",
		"internal/service/agent_fact_retrieval.go",
		"先给结论再说明依据",
	} {
		if !agentRepositoryStringSliceContains(fragments, want) {
			t.Fatalf("fragments = %#v, want %q", fragments, want)
		}
	}
}

func TestSafeTextPrefixKeepsValidUTF8(t *testing.T) {
	if got := safeTextPrefix("你好世界", 3); got != "你好世" || !utf8.ValidString(got) {
		t.Fatalf("safeTextPrefix() = %q valid=%v, want valid prefix", got, utf8.ValidString(got))
	}
	invalid := string([]byte{'o', 'k', 0xe4})
	if got := safeTextPrefix(invalid, 10); got != "ok" || !utf8.ValidString(got) {
		t.Fatalf("safeTextPrefix(invalid) = %q valid=%v, want cleaned valid string", got, utf8.ValidString(got))
	}
}

func agentRepositoryStringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
