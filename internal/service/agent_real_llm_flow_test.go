package service

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
)

// TestAgentConversationServiceRealLLMFullFlowContracts 使用 .env 中的真实 LLM 配置验证主闭环模型契约。
// 默认跳过，只有显式设置 RUN_REAL_LLM_TESTS=1 时才会访问外部模型，避免常规单测依赖网络和账号余额。
func TestAgentConversationServiceRealLLMFullFlowContracts(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM_TESTS") != "1" {
		t.Skip("set RUN_REAL_LLM_TESTS=1 to run real LLM full-flow contract tests")
	}
	client := realAgentFlowLLMClient(t)
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)

	// 历史查询覆盖：主 Agent 规划、子 Agent 工具调用、工具观察、最终回答四个模型相关环节。
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
		assertRealAgentFlowPlanContainsCapability(t, result.Plan, "conversation.query_history")
		assertRealAgentFlowObservation(t, repository, "conversation.query_history")
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
		HTTPClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	})
	if err != nil {
		t.Fatalf("create real llm client: %v", err)
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
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
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
		t.Fatalf("turn status = %q, error = %q, reply = %q", result.Turn.Status, result.Turn.ErrorMessage, result.Reply)
	}
	if result.Plan.Status != domain.AgentPlanStatusCompleted {
		t.Fatalf("plan status = %q, error = %q, reply = %q", result.Plan.Status, result.Plan.ErrorMessage, result.Reply)
	}
	if strings.TrimSpace(result.Reply) == "" {
		t.Fatal("reply is empty")
	}
	if !fakeContextTraceContains(repository.contextTraces, "main_agent_plan_spec_valid") {
		t.Fatalf("main agent planning trace is missing: %#v", repository.contextTraces)
	}
	if result.Plan.Metadata["main_agent_plan"] == nil {
		t.Fatalf("main_agent_plan metadata is missing: %#v", result.Plan.Metadata)
	}
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
