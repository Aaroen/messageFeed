package service

import (
	"context"
	"errors"
	"messagefeed/internal/agent"
	"strings"
	"testing"
	"time"
)

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
