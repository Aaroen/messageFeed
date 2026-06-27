package agent

import "testing"

func TestEvaluateEvidenceQualityChecksMinimumCountOnly(t *testing.T) {
	spec := BuildTaskSpec("搜索最新港股消息并分析")
	spec.RequiresExternal = true
	spec.MinimumEvidenceCount = 2
	result := EvaluateEvidenceQuality(spec, []EvidenceScoreInput{{Title: "一条来源"}})
	if result.Passed {
		t.Fatalf("quality should fail with one item: %#v", result)
	}
	if result.Reply != "" {
		t.Fatalf("reply should be model-generated outside quality gate, got %q", result.Reply)
	}
}

func TestEvaluateAnswerQualityDoesNotCheckMarketDirectionKeywords(t *testing.T) {
	spec := BuildTaskSpec("搜索最新港股消息并分析")
	spec.RequiresExternal = true
	spec.MinimumEvidenceCount = 1
	evidence := []EvidenceScoreInput{{Title: "结构化来源", URL: "https://example.com"}}
	result := EvaluateAnswerQuality(spec, "结论由模型基于证据生成。", evidence)
	if !result.Passed {
		t.Fatalf("quality should pass non-empty model answer: %#v", result)
	}
}
