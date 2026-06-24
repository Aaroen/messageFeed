package repository

import (
	"os"
	"reflect"
	"strings"
	"testing"
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
