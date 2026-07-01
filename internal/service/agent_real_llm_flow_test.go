package service

import (
	"context"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"messagefeed/internal/domain"
	"messagefeed/internal/llm"

	"github.com/prometheus/client_golang/prometheus"
)

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
	if !fakeContextTraceContains(repository.contextTraces, "main_agent_plan_spec_valid") {
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
