package agent

import "testing"

func TestScoreEvidenceRejectsLowQualityFinanceTutorial(t *testing.T) {
	spec := BuildTaskSpec("搜索最新港股消息并分析")
	score := ScoreEvidence(spec, EvidenceScoreInput{
		Title:   "港美股交易怎么操作？新手完整交易流程讲解",
		Source:  "湾区阿瑟",
		Summary: "大陆居民如何开户，讲解港股和美股交易规则、账户开通和零基础教程。",
		URL:     "https://bayase.com/hk-us-stock-guide",
	})
	if score.Relevant {
		t.Fatalf("tutorial should be rejected: %#v", score)
	}
	if !score.LowQuality {
		t.Fatalf("expected low quality marker: %#v", score)
	}
}

func TestScoreEvidenceAcceptsFinanceNews(t *testing.T) {
	spec := BuildTaskSpec("搜索最新港股消息并分析")
	score := ScoreEvidence(spec, EvidenceScoreInput{
		Title:       "港股收评：恒生指数下跌，科技股走弱",
		Source:      "财联社",
		Summary:     "南向资金净卖出，腾讯、美团等科技股承压。",
		URL:         "https://www.cls.cn/detail/example",
		PublishedAt: "Fri, 26 Jun 2026 13:00:00 GMT",
	})
	if !score.Relevant {
		t.Fatalf("finance news should be relevant: %#v", score)
	}
}

func TestFilterAndRankEvidenceDropsIrrelevantItems(t *testing.T) {
	spec := BuildTaskSpec("搜索最新港股消息并分析")
	inputs := []EvidenceScoreInput{
		{Title: "港美股零基础成长 0 到 1 执行路径", Source: "湾区阿瑟", Summary: "开户、课程、交易流程。"},
		{Title: "港股午评：恒指震荡，恒生科技走弱", Source: "AASTOCKS", Summary: "南向资金净卖出。", PublishedAt: "2026-06-26T13:00:00Z"},
	}
	filtered := FilterAndRankEvidence(spec, inputs)
	if len(filtered) != 1 {
		t.Fatalf("filtered = %#v", filtered)
	}
	if filtered[0].Source != "AASTOCKS" {
		t.Fatalf("unexpected item = %#v", filtered[0])
	}
}
