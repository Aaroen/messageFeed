package service

import (
	"context"
	"encoding/json"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"messagefeed/internal/repository"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAgentWorkspacePublicActivity(test *testing.T) {
	run := AgentRunResponse{
		ID: 3, ParentRunID: 2, TurnID: 1, Role: "executor", Status: "succeeded",
		TaskPacket: domain.AgentJSON{"goal": "整理公开资料", "tool_arguments": "内部参数"},
		ContextTraces: []AgentRunContextTraceResponse{{Content: domain.AgentJSON{"messages": "内部上下文"}}},
		Artifacts: []AgentArtifactResponse{{Summary: `{"tool_arguments":"内部参数"}`}, {Summary: "已整理三项资料。"}},
	}
	activity := PublicAgentActivity(run)
	if activity.Kind != "subagent" || activity.Title != "整理公开资料" || len(activity.Results) != 1 || activity.Results[0] != "已整理三项资料。" {
		test.Fatalf("活动内容不正确：%+v", activity)
	}
	encoded, err := json.Marshal(activity)
	if err != nil {
		test.Fatal(err)
	}
	for _, field := range []string{"task_packet", "tool_arguments", "context_traces", "内部上下文"} {
		if strings.Contains(string(encoded), field) {
			test.Fatalf("活动响应包含内部字段：%s", field)
		}
	}
}

func TestAgentToolRoutesStayInMainProcess(test *testing.T) {
	route := guardAgentTaskRoute(agentTaskRouteClassification{
		TaskType: agentTaskRouteRAGAnswer, Confidence: 0.95, RequiresSubAgent: true,
	})
	if route.RequiresSubAgent || agentTaskRoutePlanSpec(route, "读取历史").RequiresSubAgent {
		test.Fatal("历史查询不应委派子代理")
	}
	route = guardAgentTaskRoute(agentTaskRouteClassification{
		TaskType: agentTaskRouteDeepTask, Confidence: 0.95,
		NeedsTools: true, CandidateCapabilities: []string{"conversation.query_history"}, EstimatedLatencyClass: "normal",
	})
	if !agentTaskRouteShouldUseLightPlan(route) {
		test.Fatal("单次查询应使用主流程轻量计划")
	}
	spec := agentTaskRoutePlanSpec(route, "读取历史并转换格式")
	if spec.RequiresSubAgent || spec.DirectAnswerAllowed || len(spec.RequiredCapabilities) != 1 || spec.RequiredCapabilities[0] != "conversation.query_history" {
		test.Fatal("轻量计划未保留实际工具范围")
	}
	route.RequiresSubAgent = true
	if agentTaskRouteShouldUseLightPlan(route) {
		test.Fatal("独立任务委派必须经过完整规划校验")
	}
	route.RequiresSubAgent = false
	route.CandidateCapabilities = []string{"web.search"}
	if agentTaskRouteShouldUseLightPlan(route) {
		test.Fatal("外部事实查询应保留完整证据规划")
	}
	if fallbackAgentTaskRoute("分类失败").RequiresSubAgent {
		test.Fatal("分类失败不应扩大子代理派发范围")
	}
}

func TestAgentWorkspaceDatabaseRead(test *testing.T) {
	databaseURL := os.Getenv("AGENT_WORKSPACE_DATABASE_URL")
	if databaseURL == "" {
		test.Skip("设置 AGENT_WORKSPACE_DATABASE_URL 后执行真实会话读取验收")
	}
	sessionID, err := strconv.ParseInt(os.Getenv("AGENT_WORKSPACE_SESSION_ID"), 10, 64)
	if err != nil || sessionID < 1 {
		test.Fatal("必须提供真实会话编号")
	}
	userID, err := strconv.ParseInt(os.Getenv("AGENT_WORKSPACE_USER_ID"), 10, 64)
	if err != nil || userID < 1 {
		test.Fatal("必须提供真实用户编号")
	}
	database, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		test.Fatal(err)
	}
	connection, err := database.DB()
	if err != nil {
		test.Fatal(err)
	}
	defer connection.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	store := repository.NewAgentRepository(database)
	service := NewAgentSessionService(store)
	auth := CurrentAuth{Authenticated: true, User: domain.User{ID: userID}}
	workspace, err := service.GetWorkspace(ctx, auth, sessionID, 0)
	if err != nil {
		test.Fatal(err)
	}
	if workspace.Session.ID != sessionID || len(workspace.Turns) == 0 {
		test.Fatal("会话缺少历史回合")
	}
	memory, err := (agentConversationMemoryProvider{repository: store}).BuildConversationMemory(ctx, agent.ContextBuildInput{
		UserID: userID, SessionID: sessionID, TurnID: workspace.Turns[len(workspace.Turns)-1].ID + 1,
		CapabilityKeys: []string{"conversation.query_history"}, HistoryQueryPlan: agent.PlanHistoryQueryPlan{Mode: "earliest", Limit: 1},
	})
	if err != nil || memory.HistoryQueried || len(memory.HistoryResults) != 0 || len(memory.Messages) == 0 {
		test.Fatalf("显式历史工具执行前发生了上下文预查询：%v", err)
	}
	if _, err := service.GetWorkspaceEvents(ctx, auth, sessionID, workspace.Cursor); err != nil {
		test.Fatal(err)
	}
	state, err := service.GetWorkspaceState(ctx, auth, sessionID)
	if err != nil || len(state.Items) != 0 || len(state.Turns) != len(workspace.Turns) {
		test.Fatalf("会话状态读取异常：%v", err)
	}
	if _, err := service.GetWorkspace(ctx, CurrentAuth{Authenticated: true, User: domain.User{ID: userID + 1000000}}, sessionID, 0); err == nil {
		test.Fatal("会话允许其他用户读取")
	}
}

func TestAgentBatchEvidenceDatabaseRoundTrip(test *testing.T) {
	databaseURL := os.Getenv("AGENT_WORKSPACE_DATABASE_URL")
	if databaseURL == "" {
		test.Skip("设置 AGENT_WORKSPACE_DATABASE_URL 后验证多条资讯的独立证据")
	}
	userID, err := strconv.ParseInt(os.Getenv("AGENT_WORKSPACE_USER_ID"), 10, 64)
	if err != nil || userID < 1 {
		test.Fatal("必须提供真实用户编号")
	}
	database, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		test.Fatal(err)
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
		test.Fatal(transaction.Error)
	}
	defer transaction.Rollback()
	var turn domain.AgentTurn
	if err := transaction.Table("agent_turns").Where("user_id = ?", userID).Order("id DESC").First(&turn).Error; err != nil {
		test.Fatal(err)
	}
	var items []domain.Item
	if err := transaction.Table("items").Order("id DESC").Limit(3).Find(&items).Error; err != nil || len(items) != 3 {
		test.Fatalf("读取真实资讯失败：%v", err)
	}
	store := repository.NewAgentRepository(transaction)
	manager := agent.NewRunManager(agent.RunManagerOptions{Store: store})
	run, err := manager.CreateExecutorRun(ctx, agent.CreateRunInput{SessionID: turn.SessionID, TurnID: turn.ID, TaskPacket: domain.AgentJSON{"capability_key": "feed.query_recent_items"}})
	if err != nil {
		test.Fatal(err)
	}
	blocks := agentItemContextBlocks(items, "feed.query_recent_items", time.Now())
	bodies := make([]string, 0, len(blocks))
	for _, block := range blocks {
		bodies = append(bodies, block.Content)
	}
	executor := agentRunRecordingExecutor{runManager: manager}
	if _, err := executor.recordExecutorResult(ctx, run, "feed.query_recent_items", "读取最近资讯", agent.CapabilityObservation{Status: "succeeded"}, contextBlocksContent(blocks), nil, bodies...); err != nil {
		test.Fatal(err)
	}
	detail, err := store.GetAgentRunDetail(ctx, userID, run.ID)
	if err != nil || len(detail.Artifacts) != len(items) {
		test.Fatalf("资讯未被保存为独立证据：%v", err)
	}
	for index, artifact := range detail.Artifacts {
		if artifact.Content != bodies[index] || artifact.ContentHash != agentAnswerContentHash(bodies[index]) {
			test.Fatal("独立证据的正文或校验值不完整")
		}
	}
}

func TestAgentStreamItemsReconcileCompleteContent(test *testing.T) {
	events := []domain.AgentStreamEvent{
		{RunID: 1, TurnID: 2, Kind: "text", ItemID: "message", Delta: "片段", Metadata: domain.AgentJSON{"stage": "generation"}},
		{RunID: 1, TurnID: 2, Kind: "text", ItemID: "message", Delta: "完整回答", Metadata: domain.AgentJSON{"replace": true, "phase": "final_answer"}},
		{RunID: 3, TurnID: 4, Kind: "text", ItemID: "message", Delta: "另一个回合", Metadata: domain.AgentJSON{}},
	}
	items := agentStreamItems(events)
	if len(items) != 2 || items[0].Content != "完整回答" || items[0].Stage != "generation" || items[0].Phase != "final_answer" || items[1].Content != "另一个回合" {
		test.Fatalf("回合事件隔离或内容收敛失败：%+v", items)
	}
}

func TestPublicTraceExcludesOpaqueProtocolState(test *testing.T) {
	content := domain.AgentJSON{"content": "公开回答", "response_items": []any{map[string]any{"encrypted_content": "private-state"}}, "request": map[string]any{"messages": []any{map[string]any{"Content": "用户消息", "ResponseItems": []any{"private-state"}}}}}
	public := agentPublicTraceContent(content)
	if _, exists := public["response_items"]; exists {
		test.Fatal("公开详情包含不透明协议内容")
	}
	message := public["request"].(map[string]any)["messages"].([]any)[0].(map[string]any)
	if _, exists := message["ResponseItems"]; exists {
		test.Fatal("消息快照包含不透明协议内容")
	}
	if public["content"] != "公开回答" || content["response_items"] == nil {
		test.Fatal("公开内容或原始记录发生变化")
	}
}
