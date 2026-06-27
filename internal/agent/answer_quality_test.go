package agent

import (
	"strings"
	"testing"
)

func TestEvaluateEvidenceQualityRequiresEnoughRelevantEvidence(t *testing.T) {
	spec := BuildTaskSpec("搜索最新港股消息并分析")
	result := EvaluateEvidenceQuality(spec, []EvidenceScoreInput{
		{
			Title:       "港股收评：恒生指数下跌，科技股走弱",
			Source:      "财联社",
			Summary:     "南向资金净卖出。",
			PublishedAt: "2026-06-26T13:00:00Z",
		},
	})
	if result.Passed {
		t.Fatalf("quality should fail with one item: %#v", result)
	}
	if !strings.Contains(result.Reply, "证据数量不足") {
		t.Fatalf("reply = %q", result.Reply)
	}
}

func TestEvaluateAnswerQualityRejectsUnsupportedDirection(t *testing.T) {
	spec := BuildTaskSpec("搜索最新港股消息并分析")
	evidence := []EvidenceScoreInput{
		{
			Title:       "港股收评：恒生指数上涨",
			Source:      "财联社",
			Summary:     "恒生科技指数反弹，南向资金净买入。",
			PublishedAt: "2026-06-26T13:00:00Z",
		},
		{
			Title:       "港股科技股走强",
			Source:      "AASTOCKS",
			Summary:     "腾讯、美团等权重股上涨。",
			PublishedAt: "2026-06-26T13:30:00Z",
		},
	}
	result := EvaluateAnswerQuality(spec, "结论：当前消息面偏弱，港股下跌压力较大。", evidence)
	if result.Passed {
		t.Fatalf("quality should reject unsupported negative answer: %#v", result)
	}
	if !strings.Contains(result.Reply, "不足以支撑偏弱") {
		t.Fatalf("reply = %q", result.Reply)
	}
}

func TestEvaluateAnswerQualityPassesSupportedAnswer(t *testing.T) {
	spec := BuildTaskSpec("搜索最新港股消息并分析")
	evidence := []EvidenceScoreInput{
		{
			Title:       "港股收评：恒生指数下跌，科技股走弱",
			Source:      "财联社",
			Summary:     "南向资金净卖出。",
			PublishedAt: "2026-06-26T13:00:00Z",
		},
		{
			Title:       "港股午后跌幅扩大",
			Source:      "AASTOCKS",
			Summary:     "恒生科技指数走弱，成交额放大。",
			PublishedAt: "2026-06-26T13:30:00Z",
		},
	}
	result := EvaluateAnswerQuality(spec, "结论：当前消息面偏谨慎，短线需要关注科技股走弱。", evidence)
	if !result.Passed {
		t.Fatalf("quality should pass supported answer: %#v", result)
	}
}
