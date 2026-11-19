package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"messagefeed/internal/agent"
	"messagefeed/internal/config"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"messagefeed/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAgentSelectedModelStructuredContract(test *testing.T) {
	baseURL := os.Getenv("AGENT_CONTRACT_BASE_URL")
	if baseURL == "" {
		test.Skip("未配置低频模型验收地址")
	}
	model := os.Getenv("AGENT_CONTRACT_MODEL")
	if model == "" {
		test.Fatal("未配置模型验收标识")
	}
	client, err := llm.NewOpenAICompatibleClient(llm.OpenAICompatibleConfig{
		Provider: "openai_compatible", BaseURL: baseURL, Model: model, APIKey: os.Getenv("AGENT_CONTRACT_API_KEY"),
		ProtocolMode: "responses", Timeout: 30 * time.Second, MaxAttempts: 1,
	})
	if err != nil {
		test.Fatal(err)
	}
	service := &AgentConversationService{llmClient: client}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	streamed := false
	response, decision, err := service.requestFollowupDecision(ctx, llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: agentFollowupIntentSystemPrompt()},
			{Role: "user", Content: `{"user_message":"请停止执行刚才的任务","active_plan":{"found":true,"goal":"查阅公开资讯"},"allowed_intents":["stop"],"required_schema":{"intent":"stop","confidence":0.95,"reason":"用户要求停止"}}`},
		},
		OutputSchema: agentFollowupSchema,
		OnEvent: func(event llm.StreamEvent) error {
			streamed = streamed || event.Kind == "text" && event.Delta != "" && !event.Replace
			return nil
		},
	})
	if err != nil {
		test.Fatal(err)
	}
	if decision.Intent != agentFollowupIntentStop || response.Model != model || !streamed {
		test.Fatalf("所选模型的流式决策不符合契约：intent=%s model=%s streamed=%t", decision.Intent, response.Model, streamed)
	}
}

func TestAgentRealDatabaseAndProviderAcceptance(test *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("AGENT_RUNTIME_DATABASE_URL"))
	if databaseURL == "" || os.Getenv("RUN_REAL_LLM_TESTS") != "1" {
		test.Skip("设置 AGENT_RUNTIME_DATABASE_URL 与 RUN_REAL_LLM_TESTS=1 后执行真实数据库和模型验收")
	}
	userID, err := strconv.ParseInt(os.Getenv("AGENT_RUNTIME_USER_ID"), 10, 64)
	if err != nil || userID <= 0 || os.Getenv("AGENT_RUNTIME_CONFIG_SECRET") == "" {
		test.Fatal("必须提供真实用户编号和生产配置加密凭据")
	}
	database, err := gorm.Open(postgres.New(postgres.Config{DSN: databaseURL, PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		test.Fatalf("连接真实数据库失败：%T", err)
	}
	connection, err := database.DB()
	if err != nil {
		test.Fatal(err)
	}
	defer connection.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	transaction := database.WithContext(ctx).Begin()
	if transaction.Error != nil {
		test.Fatalf("建立验收事务失败：%T", transaction.Error)
	}
	defer transaction.Rollback()
	var sourceSchema string
	if err := transaction.Raw("SELECT current_schema()").Scan(&sourceSchema).Error; err != nil {
		test.Fatal(err)
	}
	var tables []string
	if err := transaction.Raw("SELECT tablename FROM pg_tables WHERE schemaname = ?", sourceSchema).Scan(&tables).Error; err != nil {
		test.Fatal(err)
	}
	schema := fmt.Sprintf("agent_acceptance_%d", time.Now().UnixNano())
	if err := transaction.Exec("CREATE SCHEMA " + pgx.Identifier{schema}.Sanitize()).Error; err != nil {
		test.Fatalf("建立隔离验收 schema 失败：%T", err)
	}
	for _, table := range tables {
		statement := "CREATE TABLE " + pgx.Identifier{schema, table}.Sanitize() + " (LIKE " + pgx.Identifier{sourceSchema, table}.Sanitize() + " INCLUDING ALL)"
		if err := transaction.Exec(statement).Error; err != nil {
			test.Fatalf("建立真实数据库验收表失败：%T", err)
		}
	}
	for _, table := range []string{"users", "agent_llm_provider_configs"} {
		column := "user_id"
		if table == "users" {
			column = "id"
		}
		statement := "INSERT INTO " + pgx.Identifier{schema, table}.Sanitize() + " SELECT * FROM " + pgx.Identifier{sourceSchema, table}.Sanitize() + " WHERE " + column + " = ?"
		if err := transaction.Exec(statement, userID).Error; err != nil {
			test.Fatalf("复制验收用户和模型配置失败：%T", err)
		}
	}
	if err := transaction.Exec("SET LOCAL search_path TO " + pgx.Identifier{schema}.Sanitize() + ", " + pgx.Identifier{sourceSchema}.Sanitize()).Error; err != nil {
		test.Fatal(err)
	}
	for _, name := range []string{"000040_expand_agent_artifact_content.up.sql", "000041_expand_agent_turn_delivery.up.sql", "000042_expand_agent_stream_events.up.sql", "000043_expand_agent_llm_models.up.sql", "000044_expand_guest_chat_history.up.sql", "000045_expand_agent_model_display_name.up.sql", "000046_expand_agent_model_icon.up.sql", "000047_expand_agent_turn_effects.up.sql", "000048_expand_agent_workflows.up.sql"} {
		migration, err := os.ReadFile("../../migrations/" + name)
		if err != nil {
			test.Fatal(err)
		}
		if err := transaction.Exec(string(migration)).Error; err != nil {
			test.Fatalf("在隔离 schema 应用迁移失败：%T", err)
		}
	}
	store := repository.NewAgentRepository(transaction)
	runtime := NewAgentLLMRuntime(store, WithAgentLLMRuntimeSecret(os.Getenv("AGENT_RUNTIME_CONFIG_SECRET")), WithAgentLLMRuntimeRunManager(agent.NewRunManager(agent.RunManagerOptions{Store: store})))
	selection := verifyRealAgentModelManagement(test, ctx, store, runtime, userID)
	authService := NewAuthService(repository.NewAuthRepository(transaction), config.Defaults())
	service := NewAgentConversationService(store, WithAgentConversationLLM(runtime), WithAgentConversationUserContextProvider(authService), WithAgentConversationInlineProcessing(true))
	auth := CurrentAuth{Authenticated: true, User: domain.User{ID: userID}}
	first, err := service.ReceiveWebAgentTask(ctx, auth, ReceiveWebAgentTaskInput{Message: "本次为上下文自动验收。我们约定的代号为 ABC。请仅回复这三个大写英文字母，不要附加解释。", Channel: "assistant", ModelConfigID: selection.ConfigID, Model: selection.Model})
	if err != nil || first.Turn.Status != "succeeded" || first.Reply != "ABC" || first.Plan.Status != "completed" {
		test.Fatalf("普通回答验收失败：turn=%s，plan=%s，reply=%q，error=%v", first.Turn.Status, first.Plan.Status, first.Reply, err)
	}
	test.Log("普通回答与最终格式验收通过")
	inbound, err := store.GetInboundMessage(ctx, userID, first.Turn.InboundMessageID)
	if err != nil {
		test.Fatal(err)
	}
	restored, err := agentSelectionFromInbound(ctx, inbound)
	if err != nil || restored.Value(agentModelSelectionKey{}) != selection {
		test.Fatalf("持久化回合模型选择恢复失败：%v", err)
	}
	followup, err := service.ReceiveWebAgentTask(ctx, auth, ReceiveWebAgentTaskInput{SessionID: first.Session.ID, Message: "对于上一轮已完成任务，刚才约定的代号是什么？请回答其小写形式，仅回复三个小写英文字母，不要附加解释。", Channel: "assistant", ModelConfigID: selection.ConfigID, Model: selection.Model})
	if err != nil || followup.Turn.Status != "succeeded" || followup.Reply != "abc" || followup.Plan.ID != first.Plan.ID {
		runs, _ := store.ListAgentRunsByTurn(ctx, userID, followup.Turn.ID)
		for _, run := range runs {
			detail, _ := store.GetAgentRunDetail(ctx, userID, run.ID)
			for _, trace := range detail.ContextTraces {
				if trace.TraceKind == "llm_response" {
					test.Logf("追问模型阶段=%v，回答=%v，错误=%v", trace.Content["stage"], trace.Content["content"], trace.Content["error"])
				}
			}
		}
		var transcript []struct{ MetadataJSON string }
		transaction.Raw("SELECT metadata_json FROM agent_transcript_entries WHERE turn_id = ? AND role = 'assistant'", followup.Turn.ID).Scan(&transcript)
		test.Logf("追问验证结果：%v", transcript)
		test.Fatalf("追问上下文与当前格式验收失败：turn=%s，reply=%q，error=%v", followup.Turn.Status, followup.Reply, err)
	}
	test.Log("追问原始上下文与当前格式验收通过")
	history, err := store.QueryTranscriptEntries(ctx, domain.AgentTranscriptQueryOptions{UserID: userID, SessionID: first.Session.ID, Mode: "search", Keyword: "约定代号 ABC", BeforeTurnID: followup.Turn.ID + 1, Limit: 20})
	if err != nil || len(history) == 0 {
		test.Fatalf("多关键词历史查询没有命中已有原文：%v", err)
	}
	hasOriginal := false
	for _, entry := range history {
		if entry.TurnID == first.Turn.ID && entry.Role == domain.AgentTranscriptRoleUser {
			hasOriginal = true
		}
	}
	if !hasOriginal {
		test.Fatal("多关键词历史查询缺少约定代号的原始消息")
	}
	for _, keyword := range []string{"%", "_"} {
		entries, err := store.QueryTranscriptEntries(ctx, domain.AgentTranscriptQueryOptions{UserID: userID, SessionID: first.Session.ID, Mode: "search", Keyword: keyword, Limit: 20})
		if err != nil || len(entries) != 0 {
			test.Fatalf("历史查询没有转义通配符：%v", err)
		}
	}
	otherHistory, err := store.QueryTranscriptEntries(ctx, domain.AgentTranscriptQueryOptions{UserID: userID + 1000000, SessionID: first.Session.ID, Mode: "search", Keyword: "ABC", Limit: 20})
	if err != nil || len(otherHistory) != 0 {
		test.Fatal("历史关键词查询没有隔离用户")
	}
	third, err := service.ReceiveWebAgentTask(ctx, auth, ReceiveWebAgentTaskInput{SessionID: first.Session.ID, Message: "新建独立任务：必须调用 conversation.query_history 读取本会话中我们约定的代号，并将查到的代号转换为小写，最终仅回复三个小写英文字母。", Channel: "assistant", ModelConfigID: selection.ConfigID, Model: selection.Model})
	if err != nil || third.Turn.Status != "succeeded" || third.Reply != "abc" || third.Plan.ID == first.Plan.ID {
		runs, _ := store.ListAgentRunsByTurn(ctx, userID, third.Turn.ID)
		for _, run := range runs {
			detail, _ := store.GetAgentRunDetail(ctx, userID, run.ID)
			for _, trace := range detail.ContextTraces {
				if trace.TraceKind == "llm_response" {
					test.Logf("工具任务模型阶段=%v，回答=%v，错误=%v", trace.Content["stage"], trace.Content["content"], trace.Content["error"])
				}
			}
			for _, artifact := range detail.Artifacts {
				test.Logf("工具产物编号=%d，完整正文=%s", artifact.ID, artifact.Content)
			}
		}
		test.Fatalf("真实工具与证据验收失败：turn=%s，plan=%s，reply=%q，error=%v", third.Turn.Status, third.Plan.Status, third.Reply, err)
	}
	stages := map[string]int{}
	artifactCount := 0
	historyToolCount := 0
	for _, turnID := range []int64{first.Turn.ID, followup.Turn.ID, third.Turn.ID} {
		runs, err := store.ListAgentRunsByTurn(ctx, userID, turnID)
		if err != nil {
			test.Fatal(err)
		}
		for _, run := range runs {
			detail, err := store.GetAgentRunDetail(ctx, userID, run.ID)
			if err != nil {
				test.Fatal(err)
			}
			for _, artifact := range detail.Artifacts {
				if artifact.Content == "" || artifact.ContentHash != fmt.Sprintf("%x", sha256.Sum256([]byte(artifact.Content))) {
					test.Fatal("真实工具产物缺少完整正文或有效散列")
				}
				artifactCount++
			}
			for _, observation := range detail.Observations {
				if observation.CapabilityKey == "conversation.query_history" && observation.Status == "succeeded" && run.Status == domain.AgentRunStatusSucceeded && len(detail.Artifacts) > 0 {
					historyToolCount++
				}
			}
			stage := metadataString(run.TaskPacket, "stage")
			if stage == "" || run.Role != domain.AgentRunRoleExecutor {
				continue
			}
			stages[stage]++
			requestCount, responseCount := 0, 0
			for _, trace := range detail.ContextTraces {
				if trace.TraceKind == "llm_request" {
					requestCount++
					configuration := metadataMap(trace.Content, "model_configuration")
					if metadataString(configuration, "model") != selection.Model || metadataNumber(configuration, "config_id") != int(selection.ConfigID) {
						test.Fatal("内部模型调用未保留用户选择")
					}
					if metadataMap(trace.Content, "request") == nil || trace.ContentHash == "" || trace.PromptVersion == "" {
						test.Fatal("实际模型请求缺少完整快照")
					}
				}
				if trace.TraceKind == "llm_response" {
					responseCount++
					if metadataString(trace.Content, "response_id") == "" || metadataString(trace.Content, "protocol") == "" || metadataString(trace.Content, "finish_reason") == "" || trace.Content["usage"] == nil {
						test.Fatalf("模型响应元数据缺失：stage=%s", stage)
					}
				}
			}
			if requestCount != 1 || responseCount != 1 || run.Status != domain.AgentRunStatusSucceeded {
				test.Fatalf("实际模型调用与快照不一致：stage=%s，request=%d，response=%d，status=%s，error=%s", stage, requestCount, responseCount, run.Status, run.ErrorMessage)
			}
		}
	}
	if stages["generation"] == 0 || stages["followup_answer"] == 0 || stages["evaluation"] != 0 || artifactCount == 0 || historyToolCount == 0 {
		test.Fatalf("实际验收路径覆盖不足：stages=%v，artifacts=%d", stages, artifactCount)
	}
	var auditFacts int64
	if err := transaction.Raw("SELECT count(*) FROM agent_fact_archive_index f JOIN agent_run_context_traces t ON t.id = f.fact_id WHERE f.fact_type = 'run_trace' AND t.trace_kind IN ('llm_request','llm_response')").Scan(&auditFacts).Error; err != nil {
		test.Fatal(err)
	}
	if auditFacts != 0 {
		test.Fatal("实际模型审计快照进入了事实召回索引")
	}
	workspace, err := NewAgentSessionService(store).GetWorkspace(ctx, auth, first.Session.ID, 0)
	if err != nil || len(workspace.Turns) != 3 || len(workspace.Items) == 0 || workspace.Cursor < 1 {
		test.Fatalf("真实会话和持久化增量读取失败：%v", err)
	}
	for _, turn := range workspace.Turns {
		if turn.DeliveryStatus != "not_required" || turn.OutputText == "" {
			test.Fatal("网页助理回合缺少正文或渠道状态异常")
		}
	}
	var reportAudits int64
	if err := transaction.Raw("SELECT count(*) FROM agent_audit_logs WHERE session_id = ? AND event_type IN ('wechat_work.reply_sent', 'agent.web_task_final_report_skipped')", first.Session.ID).Scan(&reportAudits).Error; err != nil {
		test.Fatal(err)
	}
	if reportAudits != 0 {
		test.Fatal("网页助理消息进入了渠道报告发送流程")
	}
	test.Logf("真实数据库与默认模型验收通过：普通回答=%q，追问=%q，工具回答=%q，模型阶段=%v，完整产物=%d。", first.Reply, followup.Reply, third.Reply, stages, artifactCount)
}

func TestAgentModelIconDatabaseRoundTrip(test *testing.T) {
	databaseURL := os.Getenv("AGENT_WORKSPACE_DATABASE_URL")
	if databaseURL == "" {
		test.Skip("设置 AGENT_WORKSPACE_DATABASE_URL 后验证模型图标持久化")
	}
	database, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		test.Fatalf("连接数据库失败：%T", err)
	}
	connection, err := database.DB()
	if err != nil {
		test.Fatal(err)
	}
	defer connection.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	transaction := database.WithContext(ctx).Begin()
	if transaction.Error != nil {
		test.Fatal(transaction.Error)
	}
	defer transaction.Rollback()
	for _, table := range []string{"agent_llm_provider_configs", "agent_llm_models"} {
		statement := "CREATE TEMP TABLE " + table + " (LIKE " + table + " INCLUDING ALL)"
		if err := transaction.Exec(statement).Error; err != nil {
			test.Fatal(err)
		}
	}
	migration, err := os.ReadFile("../../migrations/000046_expand_agent_model_icon.up.sql")
	if err != nil {
		test.Fatal(err)
	}
	if err := transaction.Exec(string(migration)).Error; err != nil {
		test.Fatal(err)
	}
	var ownerID int64
	if err := transaction.Table("users").Select("id").Where("role = ? AND status = ?", domain.UserRoleOwner, domain.UserStatusActive).Order("id").Limit(1).Scan(&ownerID).Error; err != nil || ownerID == 0 {
		test.Fatalf("没有可用管理员用于验收：%v", err)
	}
	const configID, modelID = 1, "google/models/gemini-3.5-flash-lite"
	if err := transaction.Exec("INSERT INTO agent_llm_provider_configs (id, user_id, name, provider, model, api_key_ciphertext) VALUES (?, ?, ?, ?, ?, '')", configID, ownerID, "图标验收", "openai-compatible", modelID).Error; err != nil {
		test.Fatal(err)
	}
	service := NewAgentLLMConfigService(repository.NewAgentRepository(transaction))
	owner := CurrentAuth{Authenticated: true, User: domain.User{ID: ownerID, Role: domain.UserRoleOwner}}
	input := AgentProviderModelsInput{Models: []string{modelID}, GuestModels: []string{modelID}, DisplayNames: map[string]string{modelID: "图标验收模型"}}
	for _, icons := range []map[string]string{{modelID: "unknown"}, {"disabled-model": "brain"}} {
		input.Icons = icons
		if err := service.SaveModels(ctx, owner, configID, input); !errors.Is(err, domain.ErrInvalidInput) {
			test.Fatalf("无效模型图标未被拒绝：%v", err)
		}
	}
	for _, icon := range []string{"lightning", "brain", ""} {
		input.Icons = map[string]string{modelID: icon}
		if err := service.SaveModels(ctx, owner, configID, input); err != nil {
			test.Fatal(err)
		}
		input.Icons = nil
		if err := service.SaveModels(ctx, owner, configID, input); err != nil {
			test.Fatal(err)
		}
		models, err := service.GuestModels(ctx)
		if err != nil || len(models) != 1 || models[0].Icon != icon || models[0].DisplayName != input.DisplayNames[modelID] {
			test.Fatalf("模型图标或显示名称未完整保存：%v", err)
		}
	}
}

func verifyRealAgentModelManagement(test *testing.T, ctx context.Context, store *repository.AgentRepository, runtime *AgentLLMRuntime, userID int64) domain.AgentModelSelection {
	test.Helper()
	configuration, err := store.GetDefaultAgentLLMProviderConfig(ctx, userID)
	if err != nil {
		test.Fatal(err)
	}
	expectedModel := strings.TrimSpace(os.Getenv("AGENT_RUNTIME_MODEL"))
	if expectedModel == "" || configuration.Model != expectedModel {
		test.Fatal("必须通过 AGENT_RUNTIME_MODEL 指定需要验收的当前默认模型")
	}
	defaultID := configuration.ID
	configuration.ID = 0
	configuration.Name = "真实验收供应商"
	configuration.IsDefault = false
	configuration, err = store.CreateAgentLLMProviderConfig(ctx, configuration)
	if err != nil || configuration.ID == defaultID {
		test.Fatalf("创建独立供应商配置失败：%v", err)
	}
	selection, err := runtime.ValidateModelSelection(ctx, userID, configuration.ID, configuration.Model, "")
	if err != nil {
		test.Fatal(err)
	}
	modelsService := NewAgentLLMConfigService(store, WithAgentLLMConfigSecret(os.Getenv("AGENT_RUNTIME_CONFIG_SECRET")))
	owner := CurrentAuth{Authenticated: true, User: domain.User{ID: userID, Role: domain.UserRoleOwner}}
	catalog, err := modelsService.DiscoverModels(ctx, owner, configuration.ID)
	if err != nil || len(catalog.Models) == 0 {
		test.Fatalf("读取真实供应商模型目录失败：%v", err)
	}
	input := AgentProviderModelsInput{Models: []string{selection.Model}, GuestModels: []string{selection.Model}}
	for _, model := range catalog.Models {
		if model.ID < selection.Model {
			input.Models = append(input.Models, model.ID)
			break
		}
	}
	member := owner
	member.User.Role = domain.UserRoleUser
	if err := modelsService.SaveModels(ctx, member, configuration.ID, input); domain.ClassifyError(err) != domain.ErrorKindForbidden {
		test.Fatal("普通用户能够开放游客模型")
	}
	if _, err := modelsService.DiscoverModels(ctx, CurrentAuth{Authenticated: true, User: domain.User{ID: userID + 1000000}}, configuration.ID); !errors.Is(err, domain.ErrNotFound) {
		test.Fatal("其他用户能够读取供应商模型")
	}
	if err := modelsService.SaveModels(ctx, owner, configuration.ID, input); err != nil {
		test.Fatal(err)
	}
	selectable, err := modelsService.SelectableModels(ctx, owner)
	if err != nil {
		test.Fatal(err)
	}
	for _, model := range selectable {
		if model.ConfigID == selection.ConfigID {
			if model.Model != selection.Model {
				test.Fatal("供应商默认模型没有优先排列")
			}
			break
		}
	}
	guestModels, err := modelsService.GuestModels(ctx)
	if err != nil || len(guestModels) != 1 || guestModels[0].Model != selection.Model {
		test.Fatalf("游客模型目录没有遵循管理员授权：%v", err)
	}
	if _, err := runtime.ValidateModelSelection(ctx, userID+1000000, selection.ConfigID, selection.Model, ""); !errors.Is(err, domain.ErrNotFound) {
		test.Fatal("模型选择没有隔离用户配置")
	}
	if _, err := runtime.ValidateModelSelection(ctx, userID, selection.ConfigID, "unavailable-model", ""); !errors.Is(err, domain.ErrInvalidInput) {
		test.Fatal("未启用模型没有被拒绝")
	}
	if _, err := store.GetGuestAgentModelConfig(ctx, selection.ConfigID, "unavailable-model"); !errors.Is(err, domain.ErrNotFound) {
		test.Fatal("游客能够请求未授权模型")
	}
	if err := store.SetAgentLLMModels(ctx, userID, selection.ConfigID, nil); !errors.Is(err, domain.ErrInvalidInput) {
		test.Fatal("默认模型停用校验没有生效")
	}
	publicEvents := 0
	response, err := modelsService.GuestChat(ctx, "192.0.2.10", AgentGuestChatInput{ConfigID: selection.ConfigID, Model: selection.Model, Messages: []AgentGuestMessage{{Role: "user", Content: "请计算 21 加 21，说明计算结果。"}}}, func(event llm.StreamEvent) error { publicEvents++; return nil })
	if err != nil || (!strings.Contains(response.Content, "42") && !strings.Contains(response.Content, "四十二")) || response.Model == "" || publicEvents == 0 {
		test.Fatalf("真实游客模型流式对话失败：events=%d，model=%s，content=%q，error=%v", publicEvents, response.Model, response.Content, err)
	}
	for index := 0; index < 20; index++ {
		if _, err := store.ReserveGuestAgentRequest(ctx, "quota-acceptance-guest", domain.AgentGuestTurn{ConfigID: selection.ConfigID, Model: selection.Model, InputText: "游客额度验收"}); err != nil {
			test.Fatal(err)
		}
	}
	if _, err := store.ReserveGuestAgentRequest(ctx, "quota-acceptance-guest", domain.AgentGuestTurn{ConfigID: selection.ConfigID, Model: selection.Model, InputText: "游客额度验收"}); !errors.Is(err, domain.ErrRateLimited) {
		test.Fatal("游客每日调用额度没有生效")
	}
	input.GuestModels = nil
	if err := modelsService.SaveModels(ctx, owner, selection.ConfigID, input); err != nil {
		test.Fatal(err)
	}
	if _, err := store.GetGuestAgentModelConfig(ctx, selection.ConfigID, selection.Model); !errors.Is(err, domain.ErrNotFound) {
		test.Fatal("管理员取消授权后游客仍能使用模型")
	}
	test.Logf("真实模型目录与授权验收通过：%d 个上游模型，游客流式事件=%d，用户隔离、停用校验与每日额度通过", len(catalog.Models), publicEvents)
	return selection
}

func TestAgentConversationContextFromDatabase(test *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("AGENT_CONTEXT_DATABASE_URL"))
	if databaseURL == "" {
		test.Skip("设置 AGENT_CONTEXT_DATABASE_URL 后执行真实数据库上下文回归")
	}
	identifiers := make(map[string]int64, 3)
	for _, name := range []string{"AGENT_CONTEXT_USER_ID", "AGENT_CONTEXT_PLAN_ID", "AGENT_CONTEXT_TURN_ID"} {
		value, err := strconv.ParseInt(strings.TrimSpace(os.Getenv(name)), 10, 64)
		if err != nil || value <= 0 {
			test.Fatalf("%s 必须设置为正整数", name)
		}
		identifiers[name] = value
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	database, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		test.Fatalf("打开真实数据库失败：%T", err)
	}
	sqlDatabase, err := database.DB()
	if err != nil {
		test.Fatalf("取得数据库连接失败：%T", err)
	}
	defer sqlDatabase.Close()
	transaction := database.WithContext(ctx).Begin(&sql.TxOptions{ReadOnly: true})
	if transaction.Error != nil {
		test.Fatalf("建立只读事务失败：%T", transaction.Error)
	}
	defer transaction.Rollback()
	store := repository.NewAgentRepository(transaction)
	userID := identifiers["AGENT_CONTEXT_USER_ID"]
	plan, err := store.GetAgentPlan(ctx, userID, identifiers["AGENT_CONTEXT_PLAN_ID"])
	if err != nil {
		test.Fatalf("读取原任务计划失败：%T", err)
	}
	turn, err := store.GetAgentTurn(ctx, userID, identifiers["AGENT_CONTEXT_TURN_ID"])
	if err != nil {
		test.Fatalf("读取追问调用失败：%T", err)
	}
	session, err := store.GetAgentSession(ctx, userID, turn.SessionID)
	if err != nil {
		test.Fatalf("读取会话失败：%T", err)
	}
	account := domain.ExternalAccount{ID: session.ExternalAccountID, UserID: userID}
	input := ReceiveWeChatWorkAppMessageInput{Provider: session.Provider, MsgType: "text", TextContent: turn.InputText}
	service := NewAgentConversationService(store, WithAgentConversationNow(func() time.Time { return turn.CreatedAt }))
	payload, err := service.multiTurnFollowupAnswerPayload(ctx, account, session, turn, input, plan, turn.InputText)
	if err != nil {
		test.Fatalf("构建真实追问上下文失败：%T", err)
	}
	sourceEntries, err := store.ListRecentTranscriptEntries(ctx, domain.AgentTranscriptListOptions{
		SessionID:    session.ID,
		UserID:       userID,
		BeforeTurnID: plan.TurnID + 1,
		Roles:        []domain.AgentTranscriptRole{domain.AgentTranscriptRoleAssistant},
		Limit:        1,
	})
	if err != nil || len(sourceEntries) != 1 || sourceEntries[0].TurnID != plan.TurnID || strings.TrimSpace(sourceEntries[0].Content) == "" {
		test.Fatal("原任务未取得完整 assistant 回答")
	}
	containsEntry := func(messages []domain.AgentJSON, entry domain.AgentTranscriptEntry) bool {
		for _, message := range messages {
			if message["transcript_entry_id"] == entry.ID && message["turn_id"] == entry.TurnID && message["role"] == string(entry.Role) && message["content"] == strings.TrimSpace(entry.Content) {
				return true
			}
		}
		return false
	}
	sourceResult, ok := payload["source_result"].([]domain.AgentJSON)
	if !ok || !containsEntry(sourceResult, sourceEntries[0]) {
		test.Fatal("source_result 未保留原任务完整 assistant 回答")
	}
	recentEntries, err := store.ListRecentTranscriptEntries(ctx, domain.AgentTranscriptListOptions{
		SessionID:    session.ID,
		UserID:       userID,
		BeforeTurnID: turn.ID,
		Roles:        []domain.AgentTranscriptRole{domain.AgentTranscriptRoleAssistant},
		Limit:        1,
	})
	if err != nil || len(recentEntries) != 1 || strings.TrimSpace(recentEntries[0].Content) == "" {
		test.Fatal("未取得紧邻追问之前的 assistant 回答")
	}
	projection, ok := payload["context_projection"].(domain.AgentJSON)
	if !ok {
		test.Fatal("追问请求缺少 context_projection")
	}
	recentMessages, ok := projection["recent_messages"].([]domain.AgentJSON)
	if !ok || !containsEntry(recentMessages, recentEntries[0]) {
		test.Fatal("追问上下文未保留紧邻上一轮的完整回答")
	}
	if payload["user_message"] != strings.TrimSpace(turn.InputText) {
		test.Fatal("追问请求未保留完整当前消息")
	}
	currentMessage, ok := projection["current_message"].(domain.AgentJSON)
	if !ok || currentMessage["content"] != strings.TrimSpace(turn.InputText) || currentMessage["role"] != string(domain.AgentTranscriptRoleUser) {
		test.Fatal("上下文投影未保留完整当前用户消息")
	}
	facts := agentPlanArtifactFacts(userID, plan)
	evidence, ok := payload["evidence"].([]domain.AgentFactSource)
	if !ok || len(facts) > 0 && len(evidence) == 0 {
		test.Fatal("原任务的 artifact 引用未解析为证据")
	}
	for _, source := range evidence {
		parsed, valid := agent.ParseCanonicalFactRef(source.CanonicalRef)
		if !valid || parsed.FactType != string(domain.AgentFactTypeArtifact) || parsed.FactID != source.FactID || strings.TrimSpace(source.Summary) == "" {
			test.Fatal("artifact 证据缺少有效引用或独立摘要")
		}
		if source.Content == "" && (source.Metadata["content_status"] != "missing" || source.Metadata["content_missing_reason"] == "") {
			test.Fatal("历史 artifact 未明确报告正文缺失")
		}
		if source.Content != "" && source.Metadata["content_status"] != "available" {
			test.Fatal("完整正文未明确标记为可用")
		}
		if source.UserID != userID || source.SessionID != plan.SessionID || source.TurnID != plan.TurnID {
			test.Fatal("artifact 证据的用户、会话或调用范围不正确")
		}
	}
	planningProjection, err := service.mainAgentPlanningContextProjection(ctx, account, session, turn, input)
	if err != nil {
		test.Fatalf("构建真实规划上下文失败：%T", err)
	}
	planningMessages, ok := planningProjection["recent_messages"].([]domain.AgentJSON)
	if !ok || !containsEntry(planningMessages, recentEntries[0]) {
		test.Fatal("规划上下文未保留最近真实对话")
	}
	test.Logf("数据库上下文验证完成：原任务消息=%d，近期消息=%d，产物证据=%d，规划消息=%d", len(sourceResult), len(recentMessages), len(evidence), len(planningMessages))
	if os.Getenv("RUN_REAL_LLM_TESTS") != "1" {
		return
	}
	request, err := multiTurnFollowupAnswerRequest(payload)
	if err != nil {
		test.Fatalf("构建真实模型请求失败：%T", err)
	}
	response, err := realAgentFlowLLMClient(test).Chat(ctx, request)
	if err != nil {
		test.Fatalf("真实模型请求失败：%T", err)
	}
	reply := strings.TrimSpace(response.Content)
	if reply == "" {
		test.Fatal("真实模型返回空回答")
	}
	if expected := strings.TrimSpace(os.Getenv("AGENT_CONTEXT_EXPECTED_REPLIES")); expected != "" {
		allowedReplies := strings.Split(expected, ",")
		expectedLength := len([]rune(strings.TrimSpace(allowedReplies[0])))
		matched := false
		for _, allowed := range allowedReplies {
			allowed = strings.TrimSpace(allowed)
			if allowed == "" || len([]rune(allowed)) != expectedLength {
				test.Fatal("AGENT_CONTEXT_EXPECTED_REPLIES 必须包含长度相同的非空允许值")
			}
			if reply == allowed {
				matched = true
			}
		}
		if len([]rune(reply)) != expectedLength || !matched {
			test.Fatal("真实模型回答未满足指定长度与允许值集合")
		}
	}
	test.Logf("真实模型验证完成：model=%s，回答字符数=%d", response.Model, len([]rune(reply)))
}

// TestAgentConversationServiceRealLLMFullFlowContracts 使用 .env 中的真实 LLM 配置验证主闭环模型契约。
// 默认跳过，只有显式设置 RUN_REAL_LLM_TESTS=1 时才会访问外部模型，避免常规单测依赖网络和账号余额。
func TestAgentConversationServiceRealLLMFullFlowContracts(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM_TESTS") != "1" {
		t.Skip("set RUN_REAL_LLM_TESTS=1 to run real LLM full-flow contract tests")
	}
	client := realAgentFlowLLMClient(t)
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)

	// 历史上下文覆盖：主 Agent 可通过近期上下文投影直接回答，也可授权 conversation.query_history 工具补充证据。
	t.Run("conversation_history", func(t *testing.T) {
		repository := newFakeAgentConversationRepository()
		repository.session = realAgentFlowSession(now)
		repository.transcripts = []domain.AgentTranscriptEntry{
			{
				ID:        21,
				SessionID: repository.session.ID,
				TurnID:    1,
				UserID:    1,
				Role:      domain.AgentTranscriptRoleUser,
				Content:   "我的长期偏好是关注 Go、AI 基础设施和港股市场。",
				CreatedAt: now.Add(-24 * time.Hour),
			},
		}
		repository.nextID = 200
		service := realAgentFlowService(repository, client, now)

		result := realAgentFlowReceive(t, service, "real-history", "请根据历史聊天原文查一下我的长期偏好是什么")
		assertRealAgentFlowCompleted(t, result, repository)
		if !strings.Contains(result.Reply, "Go") && !strings.Contains(result.Reply, "AI") {
			t.Fatalf("history reply does not cite stored preference: %q", result.Reply)
		}
	})

	// 联网搜索覆盖：主 Agent 授权 web.search，子 Agent 发起工具调用，fake fetcher 返回稳定网页候选。
	t.Run("web_search", func(t *testing.T) {
		repository := newFakeAgentConversationRepository()
		service := realAgentFlowService(repository, client, now)

		result := realAgentFlowReceive(t, service, "real-search", "搜索最新港股消息并分析")
		assertRealAgentFlowCompleted(t, result, repository)
		assertRealAgentFlowPlanContainsCapability(t, result.Plan, "web.search")
		assertRealAgentFlowObservation(t, repository, "web.search")
		if !strings.Contains(result.Reply, "港股") && !strings.Contains(result.Reply, "恒指") {
			t.Fatalf("search reply does not include supplied market facts: %q", result.Reply)
		}
	})

	// 定时确认覆盖：主 Agent 授权定时工具，子 Agent 必须通过 confirmed 参数进入工具级确认检查点。
	t.Run("schedule_requires_confirmation", func(t *testing.T) {
		repository := newFakeAgentConversationRepository()
		scheduledStore := &fakeAgentScheduleEvalRepository{}
		service := realAgentFlowService(repository, client, now,
			WithAgentConversationScheduledTaskStore(scheduledStore),
		)

		result := realAgentFlowReceive(t, service, "real-schedule", "明天上午9点提醒我检查部署状态")
		assertRealAgentFlowCompleted(t, result, repository)
		if !realAgentFlowPlanHasAnyCapability(result.Plan, "agent.schedule_task", "agent.schedule_message") {
			t.Fatalf("schedule plan scopes = %#v, want schedule capability", result.Plan.AllowedScopes)
		}
		if len(scheduledStore.tasks) != 0 {
			t.Fatalf("schedule task should not be created before confirmation: %#v", scheduledStore.tasks)
		}
		if !strings.Contains(result.Reply, "确认") && !strings.Contains(result.Reply, "需要") {
			t.Fatalf("schedule reply should ask for confirmation: %q", result.Reply)
		}
	})
}

func TestAgentConversationServiceRealMemoryRAGE2EObservability(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM_TESTS") != "1" {
		t.Skip("set RUN_REAL_LLM_TESTS=1 to run real LLM RAG observability tests")
	}
	llmClient := realAgentFlowLLMClient(t)
	embeddingClient := realAgentFlowEmbeddingClient(t)
	embeddingModel := strings.TrimSpace(os.Getenv("EMBEDDING_MODEL"))
	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.nextID = 500
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationNow(func() time.Time { return now }),
	)
	baseline := newRealAgentLatencyBaseline()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	err := baseline.measure("memory_capture_ms", func() error {
		service.captureMemoryCandidateFromTranscript(ctx, domain.AgentTranscriptEntry{
			ID:        501,
			SessionID: 601,
			TurnID:    701,
			UserID:    1,
			Role:      domain.AgentTranscriptRoleUser,
			Content:   "长期偏好：以后所有回答默认先给结论，然后列风险和验证步骤。这是稳定偏好，请写入长期记忆。",
			CreatedAt: now,
		})
		return nil
	})
	if err != nil {
		t.Fatalf("memory capture failed: %v", err)
	}
	if len(repository.memoryCandidates) == 0 || len(repository.memoryBlocks) == 0 || len(repository.memoryChunks) == 0 {
		t.Fatalf("memory capture incomplete: candidates=%#v blocks=%#v chunks=%#v trace_events=%#v", repository.memoryCandidates, repository.memoryBlocks, repository.memoryChunks, repository.traceEvents)
	}
	if !fakeTraceEventExists(repository.traceEvents, "memory_classification", domain.AgentTraceEventSucceeded) {
		t.Fatalf("memory classification trace missing: %#v", repository.traceEvents)
	}
	if len(repository.factIndexJobs) == 0 {
		t.Fatalf("embedding job was not enqueued for memory chunk: chunks=%#v", repository.memoryChunks)
	}

	worker := NewAgentEmbeddingWorkerService(repository, embeddingClient, embeddingModel, func() time.Time { return now })
	var workerResult AgentEmbeddingWorkerResult
	err = baseline.measure("embedding_worker_ms", func() error {
		result, runErr := worker.RunOnce(ctx, RunAgentEmbeddingWorkerOnceInput{WorkerID: "real-rag-e2e", Limit: 5})
		workerResult = result
		return runErr
	})
	if err != nil {
		t.Fatalf("embedding worker failed: %v", err)
	}
	if workerResult.SucceededCount == 0 || len(repository.embeddingTraces) == 0 {
		t.Fatalf("embedding result=%#v traces=%#v", workerResult, repository.embeddingTraces)
	}
	embeddingTrace := latestSucceededEmbeddingTrace(repository.embeddingTraces)
	if embeddingTrace.EmbeddingDimension <= 0 {
		t.Fatalf("embedding trace did not record dimension: %#v", repository.embeddingTraces)
	}
	if !fakeTraceEventExists(repository.traceEvents, "embed_fact_index", domain.AgentTraceEventSucceeded) {
		t.Fatalf("embedding worker trace event missing: %#v", repository.traceEvents)
	}

	retriever := newAgentFactRetriever(repository, embeddingClient, embeddingModel, func() time.Time { return now })
	var recallResult domain.AgentFactRecallResult
	err = baseline.measure("hybrid_recall_ms", func() error {
		result, recallErr := retriever.Recall(ctx, domain.AgentFactRecallPlan{
			Mode:            domain.AgentFactRecallModeHybrid,
			Query:           "我之前说过回答格式偏好吗？",
			UserID:          1,
			SessionID:       601,
			TurnID:          702,
			FactTypes:       []domain.AgentFactType{domain.AgentFactTypeMemoryChunk},
			MemoryKinds:     []domain.AgentMemoryKind{domain.AgentMemoryKindPreference},
			Limit:           5,
			NeedsSourceFact: true,
			MaxRiskLevel:    domain.AgentMemoryRiskMedium,
			EmbeddingModel:  embeddingModel,
		})
		recallResult = result
		return recallErr
	})
	if err != nil {
		t.Fatalf("hybrid recall failed: %v", err)
	}
	if len(recallResult.Hits) == 0 || recallResult.Diagnostics.QueryEmbeddingStatus != "ready" || recallResult.Diagnostics.QueryEmbeddingDimension <= 0 {
		t.Fatalf("recall result=%#v diagnostics=%#v", recallResult.Hits, recallResult.Diagnostics)
	}
	if !realAgentRecallHasSource(recallResult.Hits, "vector") {
		t.Fatalf("recall hits do not include vector source: %#v", recallResult.Hits)
	}
	if len(repository.recallTraces) == 0 || repository.recallTraces[len(repository.recallTraces)-1].EmbeddingStatus != "ready" {
		t.Fatalf("recall traces=%#v", repository.recallTraces)
	}
	if !fakeTraceEventExists(repository.traceEvents, "agent_fact_recall", domain.AgentTraceEventSucceeded) {
		t.Fatalf("recall trace event missing: %#v", repository.traceEvents)
	}
	for _, metricName := range []string{
		"messagefeed_agent_recall_requests_total",
		"messagefeed_agent_embedding_requests_total",
		"messagefeed_agent_embedding_jobs_total",
	} {
		if !realAgentMetricHasSample(t, metricName) {
			t.Fatalf("metric %s has no sample", metricName)
		}
	}
	baseline.log(t)
	t.Logf("real RAG dimensions: batch_embedding=%d query_embedding=%d hit_sources=%v", embeddingTrace.EmbeddingDimension, recallResult.Diagnostics.QueryEmbeddingDimension, recallResult.Diagnostics.VectorCandidateCount)
}

// realAgentFlowLLMClient 从当前进程环境创建真实 OpenAI-compatible 客户端。
// 调用命令负责先 source .env；测试本身不读取或打印任何密钥值。
func realAgentFlowLLMClient(t *testing.T) *llm.OpenAICompatibleClient {
	t.Helper()
	for _, key := range []string{"LLM_API_KEY", "LLM_MODEL"} {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			t.Fatalf("%s is required for RUN_REAL_LLM_TESTS=1", key)
		}
	}
	client, err := llm.NewOpenAICompatibleClient(llm.OpenAICompatibleConfig{
		Provider: os.Getenv("LLM_PROVIDER"),
		BaseURL:  os.Getenv("LLM_BASE_URL"),
		APIKey:   os.Getenv("LLM_API_KEY"),
		Model:    os.Getenv("LLM_MODEL"),
	})
	if err != nil {
		t.Fatalf("create real llm client: %v", err)
	}
	return client
}

func realAgentFlowEmbeddingClient(t *testing.T) *llm.OpenAICompatibleEmbeddingClient {
	t.Helper()
	model := strings.TrimSpace(os.Getenv("EMBEDDING_MODEL"))
	if model == "" {
		t.Fatal("EMBEDDING_MODEL is required for RUN_REAL_LLM_TESTS=1")
	}
	apiKey := firstNonEmptyString(os.Getenv("EMBEDDING_API_KEY"), os.Getenv("LLM_API_KEY"))
	baseURL := firstNonEmptyString(os.Getenv("EMBEDDING_BASE_URL"), os.Getenv("LLM_BASE_URL"))
	provider := firstNonEmptyString(os.Getenv("EMBEDDING_PROVIDER"), os.Getenv("LLM_PROVIDER"), "openai_compatible")
	if strings.TrimSpace(apiKey) == "" {
		t.Fatal("EMBEDDING_API_KEY or LLM_API_KEY is required for RUN_REAL_LLM_TESTS=1")
	}
	if strings.TrimSpace(baseURL) == "" {
		t.Fatal("EMBEDDING_BASE_URL or LLM_BASE_URL is required for RUN_REAL_LLM_TESTS=1")
	}
	client, err := llm.NewOpenAICompatibleEmbeddingClient(llm.OpenAICompatibleEmbeddingConfig{
		Provider: provider,
		BaseURL:  baseURL,
		APIKey:   apiKey,
		Model:    model,
	})
	if err != nil {
		t.Fatalf("create real embedding client: %v", err)
	}
	return client
}

// realAgentFlowService 构造只替换 LLM 为真实模型的闭环服务。
// 其他外部依赖仍使用测试替身，保证测试只验证模型契约和编排行为。
func realAgentFlowService(repository *fakeAgentConversationRepository, client *llm.OpenAICompatibleClient, now time.Time, extra ...AgentConversationServiceOption) *AgentConversationService {
	options := []AgentConversationServiceOption{
		WithAgentConversationLLM(client),
		WithAgentConversationSender(&fakeAgentConversationSender{}),
		WithAgentConversationExternalAccountResolver(&fakeAgentExternalAccountResolver{account: testAgentExternalAccount(now)}),
		WithAgentConversationUserContextProvider(&fakeAgentUserContextProvider{}),
		WithAgentConversationWebFetcher(func(_ context.Context, rawURL string) ([]byte, string, int, string, error) {
			if strings.Contains(rawURL, "duckduckgo.com") {
				return []byte(`<html><body><div class="anomaly-modal">challenge</div></body></html>`), rawURL, 202, "text/html; charset=utf-8", nil
			}
			body := `<html><body><ol><li class="b_algo"><h2><a href="https://example.com/hk-market-20260627">港股最新消息：2026年6月27日恒指震荡，科技股分化</a></h2><div class="b_caption"><p>2026年6月27日港股市场关注恒指、科技股成交和南向资金变化。</p></div></li></ol></body></html>`
			return []byte(body), rawURL, 200, "text/html; charset=utf-8", nil
		}),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationInlineProcessing(true),
	}
	options = append(options, extra...)
	return NewAgentConversationService(repository, options...)
}

// realAgentFlowSession 固定测试会话，便于历史查询工具读到预置 transcript。
func realAgentFlowSession(now time.Time) domain.AgentSession {
	return domain.AgentSession{
		ID:                100,
		UserID:            1,
		ExternalAccountID: 10,
		Provider:          domain.AgentProviderWeChatWorkApp,
		ChannelSessionKey: "corp-a:1000002:zhangsan",
		Status:            domain.AgentSessionStatusActive,
		StartedAt:         now.Add(-48 * time.Hour),
		LastActiveAt:      now.Add(-time.Hour),
	}
}

// realAgentFlowReceive 执行一次完整企微文本消息闭环，并给真实模型调用设置总超时。
func realAgentFlowReceive(t *testing.T, service *AgentConversationService, messageID string, text string) ReceiveWeChatWorkAppMessageResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	result, err := service.ReceiveWeChatWorkAppMessage(ctx, ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: messageID,
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       text,
		RequestID:         messageID + "-request",
		TraceID:           messageID + "-trace",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	return result
}

// assertRealAgentFlowCompleted 校验主闭环完成、规划 JSON 有效且最终回复非空。
func assertRealAgentFlowCompleted(t *testing.T, result ReceiveWeChatWorkAppMessageResult, repository *fakeAgentConversationRepository) {
	t.Helper()
	if result.Turn.Status != domain.AgentTurnStatusSucceeded {
		t.Logf("plan scopes=%#v steps=%#v observations=%#v traces=%s", result.Plan.AllowedScopes, result.Plan.Steps, repository.observations, realAgentFlowTraceKinds(repository.contextTraces))
		t.Fatalf("turn status = %q, error = %q, reply = %q", result.Turn.Status, result.Turn.ErrorMessage, result.Reply)
	}
	if result.Plan.Status != domain.AgentPlanStatusCompleted {
		t.Logf("plan scopes=%#v steps=%#v observations=%#v traces=%s", result.Plan.AllowedScopes, result.Plan.Steps, repository.observations, realAgentFlowTraceKinds(repository.contextTraces))
		t.Fatalf("plan status = %q, error = %q, reply = %q", result.Plan.Status, result.Plan.ErrorMessage, result.Reply)
	}
	if strings.TrimSpace(result.Reply) == "" {
		t.Fatal("reply is empty")
	}
	route := metadataMap(result.Plan.Metadata, "task_route")
	lightPlan, _ := route["light_plan"].(bool)
	if !lightPlan && !fakeContextTraceContains(repository.contextTraces, "main_agent_plan_spec_valid") {
		t.Fatalf("main agent planning trace is missing: %#v", repository.contextTraces)
	}
	if result.Plan.Metadata["main_agent_plan"] == nil {
		t.Fatalf("main_agent_plan metadata is missing: %#v", result.Plan.Metadata)
	}
}

func realAgentFlowTraceKinds(traces []domain.AgentRunContextTrace) string {
	kinds := make([]string, 0, len(traces))
	for _, trace := range traces {
		if strings.TrimSpace(trace.TraceKind) == "" {
			continue
		}
		kinds = append(kinds, trace.TraceKind)
	}
	return strings.Join(kinds, ",")
}

// assertRealAgentFlowPlanContainsCapability 校验主 Agent 输出的子 Agent 授权范围包含指定能力。
func assertRealAgentFlowPlanContainsCapability(t *testing.T, plan domain.AgentPlan, capability string) {
	t.Helper()
	if !realAgentFlowPlanHasAnyCapability(plan, capability) {
		t.Fatalf("plan scopes = %#v steps = %#v, want %s", plan.AllowedScopes, plan.Steps, capability)
	}
}

// realAgentFlowPlanHasAnyCapability 同时检查 allowed scopes 和 plan steps，适配不同计划状态下的存储字段。
func realAgentFlowPlanHasAnyCapability(plan domain.AgentPlan, capabilities ...string) bool {
	for _, expected := range capabilities {
		for _, scope := range plan.AllowedScopes {
			if scope == expected {
				return true
			}
		}
		for _, step := range plan.Steps {
			if step.CapabilityKey == expected {
				return true
			}
		}
	}
	return false
}

// assertRealAgentFlowObservation 校验子 Agent 实际执行了模型规划授权的工具。
func assertRealAgentFlowObservation(t *testing.T, repository *fakeAgentConversationRepository, capability string) {
	t.Helper()
	if fakeObservationContains(repository.observations, capability, "succeeded") {
		return
	}
	t.Fatalf("observation for %s is missing or not succeeded: %#v", capability, repository.observations)
}

type realAgentLatencyBaseline struct {
	values map[string][]int64
}

func newRealAgentLatencyBaseline() *realAgentLatencyBaseline {
	return &realAgentLatencyBaseline{values: map[string][]int64{}}
}

func (b *realAgentLatencyBaseline) measure(name string, fn func() error) error {
	startedAt := time.Now()
	err := fn()
	b.values[name] = append(b.values[name], time.Since(startedAt).Milliseconds())
	return err
}

func (b *realAgentLatencyBaseline) log(t *testing.T) {
	t.Helper()
	names := make([]string, 0, len(b.values))
	for name := range b.values {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		values := append([]int64(nil), b.values[name]...)
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		t.Logf("latency baseline %s count=%d p50=%d p95=%d values=%v", name, len(values), percentileLatency(values, 0.50), percentileLatency(values, 0.95), values)
	}
}

func percentileLatency(values []int64, percentile float64) int64 {
	if len(values) == 0 {
		return 0
	}
	if percentile <= 0 {
		return values[0]
	}
	index := int(float64(len(values)-1) * percentile)
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

func latestSucceededEmbeddingTrace(traces []domain.AgentEmbeddingTrace) domain.AgentEmbeddingTrace {
	for index := len(traces) - 1; index >= 0; index-- {
		if traces[index].Status == domain.AgentEmbeddingTraceSucceeded {
			return traces[index]
		}
	}
	return domain.AgentEmbeddingTrace{}
}

func realAgentRecallHasSource(hits []domain.AgentFactRecallHit, source string) bool {
	for _, hit := range hits {
		for _, candidate := range hit.HitSources {
			if candidate == source {
				return true
			}
		}
	}
	return false
}

func realAgentMetricHasSample(t *testing.T, name string) bool {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather prometheus metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() == name && len(family.GetMetric()) > 0 {
			return true
		}
	}
	return false
}
