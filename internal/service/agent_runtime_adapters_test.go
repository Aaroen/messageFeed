package service

import (
	"context"
	"errors"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAgentFirecrawlMCPProvider(test *testing.T) {
	if os.Getenv("RUN_FIRECRAWL_MCP_TEST") != "1" {
		test.Skip("设置 RUN_FIRECRAWL_MCP_TEST=1 后执行真实联网验收")
	}
	provider := newFirecrawlMCP(os.Getenv("FIRECRAWL_API_KEY"))
	results, err := provider.search(context.Background(), "site:go.dev Go release notes", 1, true)
	if err != nil || len(results) != 1 || results[0].Body == "" || results[0].URL == "" {
		test.Fatalf("Firecrawl MCP 未返回来源与正文：%d 条，%v", len(results), err)
	}
	test.Logf("Firecrawl 已返回 %s 的正文，共 %d 字节，发布日期：%s", results[0].URL, len(results[0].Body), firstNonEmptyString(results[0].PublishedAt, "未知"))
}

func TestAgentRecentItemsRetainsBodyWithoutSummary(test *testing.T) {
	publishedAt := time.Date(2026, 11, 17, 8, 49, 51, 0, time.UTC)
	item := domain.Item{ID: 1, Title: "资讯正文验证", ContentSnippet: "正文记录事件变化、时间以及影响。", PublishedAt: &publishedAt}
	content := formatRecentItemsBlock([]domain.Item{item})
	if !strings.Contains(content, item.ContentSnippet) || !strings.Contains(content, publishedAt.Format(time.RFC3339)) {
		test.Fatal("缺少摘要的资讯未保留正文与发布时间")
	}
	item.Summary = item.ContentSnippet
	content = formatRecentItemsBlock([]domain.Item{item})
	if strings.Count(content, item.ContentSnippet) != 1 {
		test.Fatal("相同正文与摘要被重复写入上下文")
	}
	second := item
	second.ID, second.Title, second.ContentSnippet, second.Summary = 2, "另一条资讯正文验证", "另一项独立事件的正文。", ""
	blocks := agentItemContextBlocks([]domain.Item{item, second}, "feed.query_recent_items", publishedAt)
	result := capabilityExecuteResultToToolResult(agent.CapabilityExecuteResult{Blocks: blocks})
	if len(blocks) != 2 || len(result.Result.Content) != 2 || blocks[0].ItemCount != 1 || strings.Contains(result.Result.Content[0].Text, second.ContentSnippet) || !strings.Contains(result.Result.Content[1].Text, second.ContentSnippet) {
		test.Fatal("多条资讯未保留独立的正文与协议内容块")
	}
}

func TestAgentWebSearchRejectsUnauthorizedFutureQueryDate(t *testing.T) {
	now := time.Date(2026, 6, 29, 4, 0, 0, 0, time.UTC)
	fetchCalled := false
	executor := agentP0CapabilityExecutor{
		now: func() time.Time { return now },
		webFetcher: func(context.Context, string) ([]byte, string, int, string, error) {
			fetchCalled = true
			return nil, "", 0, "", errors.New("fetch should not be called")
		},
	}

	content, observation, count, err := executor.runWebSearch(
		context.Background(),
		"web.search",
		"亚太股市 2026年6月30日 早盘表现",
		5,
		"上午亚太资本市场表现如何，下午可能有什么走势",
	)
	if err != nil {
		t.Fatalf("runWebSearch() error = %v", err)
	}
	if fetchCalled {
		t.Fatal("web fetcher should not be called for unauthorized future query date")
	}
	if observation.Status != "failed" || count != 0 {
		t.Fatalf("observation = %#v count = %d", observation, count)
	}
	if !strings.Contains(content, "future_date") || !strings.Contains(content, "2026年6月30日") {
		t.Fatalf("content missing temporal failure details: %q", content)
	}
}

func TestAgentWebSearchFiltersFutureAndStaleEvidence(t *testing.T) {
	now := time.Date(2026, 6, 29, 4, 0, 0, 0, time.UTC)
	body := []byte(`<html><body>
<div class="result"><a class="result__a" href="https://news.example.com/20250630/future.html">港股收盘 2025-06-30</a><div class="result__snippet">旧年份结果</div></div>
<div class="result"><a class="result__a" href="https://news.example.com/20260630/future.html">港股收盘 2026-06-30</a><div class="result__snippet">未来结果</div></div>
<div class="result"><a class="result__a" href="https://news.example.com/20260629/current.html">港股午评 2026-06-29</a><div class="result__snippet">当前交易日结果</div></div>
</body></html>`)
	executor := agentP0CapabilityExecutor{
		now: func() time.Time { return now },
		webFetcher: func(_ context.Context, rawURL string) ([]byte, string, int, string, error) {
			return body, rawURL, 200, "text/html; charset=utf-8", nil
		},
	}

	content, observation, count, err := executor.runWebSearch(
		context.Background(),
		"web.search",
		"亚太股市 早盘表现",
		5,
		"上午亚太资本市场表现如何，下午可能有什么走势",
	)
	if err != nil {
		t.Fatalf("runWebSearch() error = %v", err)
	}
	if observation.Status != "succeeded" || count != 1 {
		t.Fatalf("observation = %#v count = %d", observation, count)
	}
	if strings.Contains(content, "20250630") || strings.Contains(content, "20260630") {
		t.Fatalf("content should not contain filtered evidence: %q", content)
	}
	if !strings.Contains(content, "20260629") || !strings.Contains(content, "future=1") || !strings.Contains(content, "stale=1") {
		t.Fatalf("content missing current result or filter summary: %q", content)
	}
}

func TestConversationHistoryFallsBackToFactRecallWhenTranscriptEmpty(t *testing.T) {
	now := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.factIndexes = []domain.AgentFactArchiveIndex{
		{
			ID:              1,
			CanonicalRef:    "transcript:101",
			FactType:        domain.AgentFactTypeTranscript,
			FactID:          101,
			UserID:          7,
			SessionID:       2,
			TurnID:          3,
			MemoryKind:      domain.AgentMemoryKindPreference,
			SummaryForIndex: "用户回复偏好",
			ContextualText:  "用户此前明确表示回复偏好是先给结论，再给依据。",
			IndexStatus:     domain.AgentFactIndexStatusReady,
			RiskLevel:       domain.AgentMemoryRiskLow,
			Importance:      85,
			Confidence:      0.92,
			UpdatedAt:       now,
			CreatedAt:       now,
		},
	}
	executor := agentP0CapabilityExecutor{
		repository:    repository,
		factRetriever: newAgentFactRetriever(repository, nil, "test-embedding", func() time.Time { return now }),
		now:           func() time.Time { return now },
	}

	result, err := executor.queryConversationHistory(context.Background(), agent.MCPCallToolInput{
		Capability:   agent.Capability{Key: "conversation.query_history"},
		UserID:       7,
		SessionID:    2,
		TurnID:       9,
		Message:      "我之前说过什么回复偏好",
		RawArguments: `{"mode":"search","query":"回复偏好","limit":8}`,
	})
	if err != nil {
		t.Fatalf("queryConversationHistory() error = %v", err)
	}
	if result.Observation.Status != "succeeded" {
		t.Fatalf("observation = %#v, want succeeded", result.Observation)
	}
	if !strings.Contains(result.TextContent(), "长期事实索引兜底") || !strings.Contains(result.TextContent(), "先给结论") {
		t.Fatalf("tool result missing fact recall fallback: %q", result.TextContent())
	}
	if result.Observation.Metadata["rag_fallback_status"] != "used" || result.Observation.Metadata["rag_hits_used"] != 1 {
		t.Fatalf("observation metadata = %#v", result.Observation.Metadata)
	}
	if !fakeTraceEventExists(repository.traceEvents, "agent_fact_recall", domain.AgentTraceEventDegraded) {
		t.Fatalf("trace events = %#v, want degraded agent_fact_recall trace", repository.traceEvents)
	}
}

func TestAgentWebExtractRejectsUnauthorizedFutureURLBeforeFetch(t *testing.T) {
	now := time.Date(2026, 6, 29, 4, 0, 0, 0, time.UTC)
	fetchCalled := false
	executor := agentP0CapabilityExecutor{
		now: func() time.Time { return now },
		webFetcher: func(context.Context, string) ([]byte, string, int, string, error) {
			fetchCalled = true
			return nil, "", 0, "", errors.New("fetch should not be called")
		},
	}

	result, err := executor.webExtractPage(context.Background(), agent.MCPCallToolInput{
		Capability:   agent.Capability{Key: "web.extract_page"},
		Message:      "上午亚太资本市场表现如何",
		RawArguments: `{"url":"https://news.qq.com/rain/a/20260630A06KX900"}`,
	})
	if err != nil {
		t.Fatalf("webExtractPage() error = %v", err)
	}
	if fetchCalled {
		t.Fatal("web fetcher should not be called for unauthorized future url")
	}
	if !result.Result.IsError || result.Observation.Status != "empty" {
		t.Fatalf("result = %#v", result)
	}
	if !strings.Contains(result.TextContent(), "future_date") {
		t.Fatalf("tool result missing temporal failure: %q", result.TextContent())
	}
}
