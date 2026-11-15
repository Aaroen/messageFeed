package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"messagefeed/internal/domain"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAgentTurnCompletionDatabaseTransaction(test *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("AGENT_COMPLETION_DATABASE_URL"))
	if databaseURL == "" {
		test.Skip("设置 AGENT_COMPLETION_DATABASE_URL 后执行真实数据库事务验收")
	}
	turnID, err := strconv.ParseInt(os.Getenv("AGENT_COMPLETION_TURN_ID"), 10, 64)
	if err != nil || turnID <= 0 {
		test.Fatal("AGENT_COMPLETION_TURN_ID 必须为真实调用编号")
	}
	database, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		test.Fatalf("连接真实数据库失败：%T", err)
	}
	connection, err := database.DB()
	if err != nil {
		test.Fatal(err)
	}
	defer connection.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	transaction := database.WithContext(ctx).Begin()
	if transaction.Error != nil {
		test.Fatalf("开始真实数据库事务失败：%T", transaction.Error)
	}
	defer transaction.Rollback()
	for _, name := range []string{"000040_add_agent_artifact_content.up.sql", "000041_add_agent_turn_delivery.up.sql"} {
		migration, readErr := os.ReadFile("../../migrations/" + name)
		if readErr != nil {
			test.Fatal(readErr)
		}
		if err := transaction.Exec(string(migration)).Error; err != nil {
			test.Fatalf("事务内应用迁移失败：%T", err)
		}
	}
	var original agentTurnModel
	if err := transaction.Where("id = ?", turnID).First(&original).Error; err != nil {
		test.Fatalf("读取验收参照调用失败：%T", err)
	}
	store := NewAgentRepository(transaction)
	originalInbound, err := store.GetInboundMessage(ctx, original.UserID, original.InboundMessageID)
	if err != nil {
		test.Fatal(err)
	}
	now := time.Now().UTC()
	createCompletion := func(suffix string) domain.AgentTurnCompletion {
		test.Helper()
		inbound := originalInbound
		inbound.ID = 0
		inbound.ProviderMessageID = fmt.Sprintf("completion-acceptance-%d-%s", now.UnixNano(), suffix)
		inbound.Status = domain.AgentInboundMessageStatusReceived
		inbound.CreatedAt, inbound.UpdatedAt = now, now
		inbound, _, err = store.CreateInboundMessage(ctx, inbound)
		if err != nil {
			test.Fatal(err)
		}
		lease := now.Add(time.Minute)
		turn, createErr := store.CreateTurn(ctx, domain.AgentTurn{UserID: original.UserID, SessionID: original.SessionID, InboundMessageID: inbound.ID, Status: domain.AgentTurnStatusRunning, InputText: "验证事务提交", LockedBy: "completion-acceptance", LockedAt: &now, LeaseUntil: &lease, StartedAt: now})
		if createErr != nil {
			test.Fatal(createErr)
		}
		run, createErr := store.CreateAgentRun(ctx, domain.AgentRun{SessionID: turn.SessionID, TurnID: turn.ID, Role: domain.AgentRunRoleController, Status: domain.AgentRunStatusRunning, TaskPacket: domain.AgentJSON{"user_id": turn.UserID}, StartedAt: now})
		if createErr != nil {
			test.Fatal(createErr)
		}
		plan, createErr := store.CreateAgentPlan(ctx, domain.AgentPlan{UserID: turn.UserID, SessionID: turn.SessionID, TurnID: turn.ID, ControllerRunID: run.ID, Goal: "验证事务提交", Status: domain.AgentPlanStatusExecuting}, []domain.AgentPlanStep{{StepOrder: 1, Status: domain.AgentPlanStepStatusExecuting, CapabilityKey: "conversation.query_history", Title: "验证提交状态"}})
		if createErr != nil {
			test.Fatal(createErr)
		}
		turn.Status, turn.OutputText, turn.DeliveryStatus = domain.AgentTurnStatusSucceeded, "事务验证完成。", "pending"
		plan.Status = domain.AgentPlanStatusCompleted
		plan.Steps[0].Status, plan.Steps[0].CompletedAt = domain.AgentPlanStepStatusCompleted, &now
		return domain.AgentTurnCompletion{Turn: turn, Plan: plan, ControllerRunID: run.ID, WorkerID: "completion-acceptance", CompletedAt: now}
	}
	assertUnchanged := func(completion domain.AgentTurnCompletion) {
		test.Helper()
		turn, getErr := store.GetAgentTurn(ctx, completion.Turn.UserID, completion.Turn.ID)
		if getErr != nil || turn.Status != domain.AgentTurnStatusRunning || turn.FinishedAt != nil || turn.OutputText != "" {
			test.Fatalf("失败事务提交了调用结果：%#v，%v", turn, getErr)
		}
		plan, getErr := store.GetAgentPlan(ctx, turn.UserID, completion.Plan.ID)
		if getErr != nil || plan.Status != domain.AgentPlanStatusExecuting || plan.Steps[0].Status != domain.AgentPlanStepStatusExecuting {
			test.Fatalf("失败事务改变了计划状态：%#v，%v", plan, getErr)
		}
		var count int64
		if err := transaction.Model(&agentTranscriptEntryModel{}).Where("turn_id = ? AND role = ?", turn.ID, domain.AgentTranscriptRoleAssistant).Count(&count).Error; err != nil || count != 0 {
			test.Fatalf("失败事务留下了回答记录：%d，%v", count, err)
		}
		var run agentRunModel
		if err := transaction.Where("id = ?", completion.ControllerRunID).First(&run).Error; err != nil || run.Status != string(domain.AgentRunStatusRunning) {
			test.Fatalf("失败事务改变了 controller：%#v，%v", run, err)
		}
	}
	broken := createCompletion("broken")
	broken.Plan.Steps[0].ID = 9223372036854775807
	if _, err := store.CommitAgentTurnResult(ctx, broken); !errors.Is(err, domain.ErrNotFound) {
		test.Fatalf("步骤写入错误未传播：%v", err)
	}
	assertUnchanged(broken)
	unauthorized := createCompletion("ownership")
	unauthorized.WorkerID = "different-worker"
	if _, err := store.CommitAgentTurnResult(ctx, unauthorized); err == nil {
		test.Fatal("失去所有权后仍然允许完成调用")
	}
	assertUnchanged(unauthorized)
	canceled := createCompletion("canceled")
	if _, err := store.RequestAgentTurnCancel(ctx, canceled.Turn.UserID, canceled.Turn.ID, "验收取消", now); err != nil {
		test.Fatal(err)
	}
	if _, err := store.CommitAgentTurnResult(ctx, canceled); err == nil {
		test.Fatal("取消请求未阻止成功提交")
	}
	assertUnchanged(canceled)
	completed := createCompletion("completed")
	result, err := store.CommitAgentTurnResult(ctx, completed)
	if err != nil || result.Turn.Status != domain.AgentTurnStatusSucceeded || result.Plan.Status != domain.AgentPlanStatusCompleted || result.InboundMessage.Status != domain.AgentInboundMessageStatusSucceeded {
		test.Fatalf("成功事务的终态不一致：%#v，%v", result, err)
	}
	var count int64
	if err := transaction.Model(&agentTranscriptEntryModel{}).Where("turn_id = ? AND role = ?", result.Turn.ID, domain.AgentTranscriptRoleAssistant).Count(&count).Error; err != nil || count != 1 {
		test.Fatalf("成功事务未且仅持久化一次回答：%d，%v", count, err)
	}
	if _, err := store.CommitAgentTurnResult(ctx, completed); err == nil {
		test.Fatal("重复提交未被拒绝")
	}
	delivered, err := store.UpdateAgentTurnDelivery(ctx, completed.Turn.UserID, completed.Turn.ID, "failed", "验收投递错误", now)
	if err != nil || delivered.Status != domain.AgentTurnStatusSucceeded || delivered.DeliveryStatus != "failed" {
		test.Fatalf("投递状态影响了生成业务状态：%#v，%v", delivered, err)
	}
	test.Log("真实数据库事务已验证：成功提交、重复提交拒绝、错误整体撤销、所有权检查、取消检查、独立投递状态。")
}

func TestNormalizeAgentArtifactPreservesContentAndValidatesHash(test *testing.T) {
	content := "  \n" + strings.Repeat("完整工具正文\n", 600) + "\n  "
	digest := sha256.Sum256([]byte(content))
	expectedHash := hex.EncodeToString(digest[:])
	for _, suppliedHash := range []string{"", expectedHash, strings.ToUpper(expectedHash)} {
		artifact, err := normalizeAgentArtifact(domain.AgentArtifact{Content: content, Summary: "独立摘要", ContentHash: suppliedHash})
		if err != nil || artifact.Content != content || artifact.Summary != "独立摘要" || artifact.ContentHash != expectedHash {
			test.Fatal("完整正文、独立摘要或正文散列未被保留")
		}
		converted := agentArtifactModelToDomain(agentArtifactModelFromDomain(artifact))
		if converted.Content != content || converted.Summary != artifact.Summary || converted.ContentHash != expectedHash {
			test.Fatal("产物模型往返改变了完整正文或校验值")
		}
	}
	if _, err := normalizeAgentArtifact(domain.AgentArtifact{Content: content, ContentHash: strings.Repeat("0", 64)}); err == nil {
		test.Fatal("不匹配的正文散列未被拒绝")
	}
	legacy, err := normalizeAgentArtifact(domain.AgentArtifact{Summary: "历史摘要", ContentHash: strings.Repeat("1", 64)})
	if err != nil || legacy.Content != "" || legacy.Summary != "历史摘要" {
		test.Fatal("历史摘要被当作完整正文")
	}
}

func TestAgentArtifactFactIndexUsesFullContentAndMarksLegacySummary(test *testing.T) {
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	builder := newAgentFactIndexBuilder(func() time.Time { return now })
	scope := agentFactRunScope{UserID: 7, SessionID: 47, TurnID: 111}
	content := " \n" + strings.Repeat("产物原始正文", 500) + "\n "
	artifact, err := normalizeAgentArtifact(domain.AgentArtifact{ID: 320, RunID: 367, Content: content, Summary: "独立摘要", CreatedAt: now})
	if err != nil {
		test.Fatal("构造正文产物失败")
	}
	fact, ok := builder.BuildArtifact(scope, artifact)
	if !ok || fact.ContextualText != content || fact.SummaryForIndex != artifact.Summary || fact.Metadata["content_hash"] != artifact.ContentHash || fact.Metadata["content_status"] != "available" {
		test.Fatal("事实索引未使用完整正文及其校验值")
	}
	if fact.UserID != scope.UserID || fact.SessionID != scope.SessionID || fact.TurnID != scope.TurnID {
		test.Fatal("事实索引未保留用户、会话和调用范围")
	}
	legacy, ok := builder.BuildArtifact(scope, domain.AgentArtifact{ID: 321, RunID: 367, Summary: "历史摘要", ContentRef: "agent_run:367:capability_output"})
	if !ok || legacy.ContextualText != "" || legacy.SummaryForIndex != "历史摘要" || legacy.Metadata["content_status"] != "missing" || legacy.Metadata["index_source"] != "summary_only" {
		test.Fatal("历史索引未明确区分摘要与缺失正文")
	}
}

func TestAgentArtifactChunkProjectionPreservesSourceBody(test *testing.T) {
	fact := domain.AgentFactArchiveIndex{CanonicalRef: "artifact:320#chunk:1", Metadata: domain.AgentJSON{"chunk_text": "索引摘要片段"}}
	for _, content := range []string{"完整工具正文", ""} {
		source := domain.AgentFactSource{FactType: domain.AgentFactTypeArtifact, Content: content, Summary: "独立摘要"}
		projected := applyFactChunkProjection(fact, source)
		if projected.Content != content || projected.Summary != source.Summary || projected.Metadata["chunk_ref"] != fact.CanonicalRef {
			test.Fatal("索引片段覆盖了产物正文或摘要")
		}
	}
}

func TestAgentArtifactContentMigrationMatchesHelm(test *testing.T) {
	for _, suffix := range []string{"up.sql", "down.sql"} {
		name := "000040_add_agent_artifact_content." + suffix
		migration, err := os.ReadFile("../../migrations/" + name)
		if err != nil {
			test.Fatalf("读取正文迁移失败：%v", err)
		}
		helm, err := os.ReadFile("../../deploy/helm/messagefeed/files/migrations/" + name)
		if err != nil {
			test.Fatalf("读取 Helm 正文迁移失败：%v", err)
		}
		if string(migration) != string(helm) {
			test.Fatal("仓库与 Helm 的正文迁移不一致")
		}
		if suffix == "up.sql" && !strings.Contains(string(migration), "ADD COLUMN IF NOT EXISTS content TEXT NOT NULL DEFAULT ''") {
			test.Fatal("正文迁移未保留历史摘要记录的兼容性")
		}
	}
}

func TestAgentArtifactDatabaseRoundTrip(test *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("AGENT_ARTIFACT_DATABASE_URL"))
	if databaseURL == "" {
		test.Skip("设置 AGENT_ARTIFACT_DATABASE_URL 后执行真实数据库产物回归")
	}
	runID, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("AGENT_ARTIFACT_RUN_ID")), 10, 64)
	if err != nil || runID <= 0 {
		test.Fatal("AGENT_ARTIFACT_RUN_ID 必须设置为正整数")
	}
	database, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		test.Fatalf("打开真实数据库失败：%T", err)
	}
	sqlDatabase, err := database.DB()
	if err != nil {
		test.Fatalf("取得数据库连接失败：%T", err)
	}
	defer sqlDatabase.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	transaction := database.WithContext(ctx).Begin()
	if transaction.Error != nil {
		test.Fatalf("建立数据库事务失败：%T", transaction.Error)
	}
	defer transaction.Rollback()
	store := NewAgentRepository(transaction)
	scope, err := store.agentRunScope(ctx, runID)
	if err != nil || scope.UserID <= 0 {
		test.Fatal("指定运行记录不存在有效用户范围")
	}
	content := " \n" + strings.Repeat("验证完整工具正文\n", 600) + "\n "
	artifact, err := store.CreateAgentArtifact(ctx, domain.AgentArtifact{RunID: runID, ArtifactType: "capability_output", Content: content, Summary: "独立产物摘要", CreatedAt: time.Now().UTC()})
	if err != nil {
		test.Fatalf("持久化产物正文失败：%T", err)
	}
	digest := sha256.Sum256([]byte(content))
	if artifact.Content != content || artifact.ContentHash != hex.EncodeToString(digest[:]) {
		test.Fatal("持久化返回值未保留完整正文与散列")
	}
	fact := domain.AgentFactArchiveIndex{CanonicalRef: "artifact:" + strconv.FormatInt(artifact.ID, 10), FactType: domain.AgentFactTypeArtifact, FactID: artifact.ID, UserID: scope.UserID, SessionID: scope.SessionID, TurnID: scope.TurnID}
	sources, err := store.ResolveAgentFactSources(ctx, scope.UserID, []domain.AgentFactArchiveIndex{fact})
	if err != nil || len(sources) != 1 || sources[0].Content != content || sources[0].Summary != artifact.Summary || sources[0].Metadata["content_status"] != "available" || sources[0].Metadata["content_hash"] != artifact.ContentHash {
		test.Fatal("根据产物引用未解析到真实完整正文")
	}
	if sources[0].UserID != scope.UserID || sources[0].SessionID != scope.SessionID || sources[0].TurnID != scope.TurnID {
		test.Fatal("产物解析未返回真实用户、会话和调用范围")
	}
	for _, mismatched := range []domain.AgentFactArchiveIndex{
		{CanonicalRef: fact.CanonicalRef, FactType: fact.FactType, FactID: fact.FactID, UserID: scope.UserID + 1, SessionID: scope.SessionID, TurnID: scope.TurnID},
		{CanonicalRef: fact.CanonicalRef, FactType: fact.FactType, FactID: fact.FactID, UserID: scope.UserID, SessionID: scope.SessionID + 1, TurnID: scope.TurnID},
		{CanonicalRef: fact.CanonicalRef, FactType: fact.FactType, FactID: fact.FactID, UserID: scope.UserID, SessionID: scope.SessionID, TurnID: scope.TurnID + 1},
	} {
		resolved, resolveErr := store.ResolveAgentFactSources(ctx, mismatched.UserID, []domain.AgentFactArchiveIndex{mismatched})
		if resolveErr != nil || len(resolved) != 0 {
			test.Fatal("产物正文越过了用户、会话或调用范围")
		}
	}
	legacy, err := store.CreateAgentArtifact(ctx, domain.AgentArtifact{RunID: runID, ArtifactType: "capability_output", Summary: "历史独立摘要", CreatedAt: time.Now().UTC()})
	if err != nil {
		test.Fatalf("持久化历史兼容产物失败：%T", err)
	}
	fact.CanonicalRef = "artifact:" + strconv.FormatInt(legacy.ID, 10)
	fact.FactID = legacy.ID
	sources, err = store.ResolveAgentFactSources(ctx, scope.UserID, []domain.AgentFactArchiveIndex{fact})
	if err != nil || len(sources) != 1 || sources[0].Content != "" || sources[0].Summary != legacy.Summary || sources[0].Metadata["content_status"] != "missing" || sources[0].Metadata["content_missing_reason"] == "" {
		test.Fatal("历史产物未明确保留正文缺失状态")
	}
}

func TestAgentRecallEventModelMapsQueryTextColumn(t *testing.T) {
	field, ok := reflect.TypeOf(agentRecallEventModel{}).FieldByName("Query")
	if !ok {
		t.Fatal("Query field is missing")
	}
	if !strings.Contains(string(field.Tag), "column:query_text") {
		t.Fatalf("Query gorm tag = %q, want column:query_text", field.Tag)
	}
}

func TestAgentRecallTraceModelMapsFullTextColumns(t *testing.T) {
	modelType := reflect.TypeOf(agentRecallTraceModel{})
	for _, tt := range []struct {
		field string
		tag   string
	}{
		{field: "FullTextAttempted", tag: "column:fulltext_attempted"},
		{field: "FullTextCount", tag: "column:fulltext_count"},
		{field: "FullTextMS", tag: "column:fulltext_ms"},
	} {
		field, ok := modelType.FieldByName(tt.field)
		if !ok {
			t.Fatalf("%s field is missing", tt.field)
		}
		if !strings.Contains(string(field.Tag), tt.tag) {
			t.Fatalf("%s gorm tag = %q, want %s", tt.field, field.Tag, tt.tag)
		}
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

func TestRepositoryJSONBExprMarshalsStringSlice(t *testing.T) {
	expr, err := repositoryJSONBExpr([]string{"比赛", "赛程"})
	if err != nil {
		t.Fatalf("repositoryJSONBExpr() error = %v", err)
	}
	if expr.SQL != "?::jsonb" {
		t.Fatalf("expr.SQL = %q, want jsonb cast", expr.SQL)
	}
	if len(expr.Vars) != 1 || expr.Vars[0] != `["比赛","赛程"]` {
		t.Fatalf("expr.Vars = %#v", expr.Vars)
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

func TestAgentTraceEventsMigrationDefinesWaterfallTable(t *testing.T) {
	content, err := os.ReadFile("../../migrations/000035_add_agent_trace_events.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	text := string(content)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS agent_trace_events",
		"request_id TEXT NOT NULL DEFAULT ''",
		"event_kind VARCHAR(48) NOT NULL",
		"'subagent_dispatch'",
		"'tool_execution'",
		"'degraded'",
		"metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb",
		"idx_agent_trace_events_request",
		"idx_agent_trace_events_run",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestAgentRecallEmbeddingTraceMigrationDefinesDiagnosticsTables(t *testing.T) {
	content, err := os.ReadFile("../../migrations/000036_add_agent_recall_embedding_traces.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	text := string(content)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS agent_recall_traces",
		"fulltext_ms BIGINT NOT NULL DEFAULT 0",
		"embedding_status VARCHAR(32) NOT NULL DEFAULT ''",
		"vector_candidate_count INTEGER NOT NULL DEFAULT 0",
		"fallback_reason TEXT NOT NULL DEFAULT ''",
		"CREATE TABLE IF NOT EXISTS agent_embedding_traces",
		"canonical_ref VARCHAR(128) NOT NULL DEFAULT ''",
		"idx_agent_recall_traces_turn",
		"idx_agent_embedding_traces_ref",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestAgentMemoryTopicsChunksMigrationDefinesChunkFactType(t *testing.T) {
	content, err := os.ReadFile("../../migrations/000037_add_agent_memory_topics_chunks.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	text := string(content)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS agent_memory_topics",
		"CREATE TABLE IF NOT EXISTS agent_memory_chunks",
		"'memory_chunk'",
		"embedding_status VARCHAR(16) NOT NULL DEFAULT 'pending'",
		"consolidation_reason VARCHAR(32) NOT NULL DEFAULT 'unknown'",
		"uq_agent_memory_chunks_content_hash",
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
