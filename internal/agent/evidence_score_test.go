package agent

import "testing"

func TestScoreEvidenceUsesStructuralCompleteness(t *testing.T) {
	spec := BuildTaskSpec("搜索最新港股消息并分析")
	score := ScoreEvidence(spec, EvidenceScoreInput{
		Title:       "港股收评：恒生指数下跌，科技股走弱",
		Source:      "财联社",
		Summary:     "南向资金净卖出，腾讯、美团等科技股承压。",
		URL:         "https://www.cls.cn/detail/example",
		PublishedAt: "Fri, 26 Jun 2026 13:00:00 GMT",
	})
	if !score.Relevant || score.Score <= 0.9 {
		t.Fatalf("score = %#v", score)
	}
}

func TestFilterAndRankEvidenceKeepsStructuredCandidates(t *testing.T) {
	spec := BuildTaskSpec("搜索最新港股消息并分析")
	inputs := []EvidenceScoreInput{
		{Title: "只有标题"},
		{Title: "港股午评：恒指震荡", Source: "AASTOCKS", Summary: "市场波动。", URL: "https://example.com/a", PublishedAt: "2026-06-26T13:00:00Z"},
	}
	filtered := FilterAndRankEvidence(spec, inputs)
	if len(filtered) != 2 {
		t.Fatalf("filtered = %#v", filtered)
	}
	if filtered[0].Source != "AASTOCKS" {
		t.Fatalf("unexpected rank = %#v", filtered)
	}
}
