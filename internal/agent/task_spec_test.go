package agent

import (
	"strings"
	"testing"
)

func TestBuildTaskSpecDoesNotInferIntentFromKeywords(t *testing.T) {
	spec := BuildTaskSpec("搜索最新港股消息并分析")
	if spec.TaskType != TaskTypeQuestionAnswer {
		t.Fatalf("TaskType = %q", spec.TaskType)
	}
	if spec.Domain != TaskDomainGeneral {
		t.Fatalf("Domain = %q", spec.Domain)
	}
	if spec.Freshness != TaskFreshnessHistorical {
		t.Fatalf("Freshness = %q", spec.Freshness)
	}
	if spec.RequiresExternal || spec.NeedsAnalysis || spec.RequestsSearch() {
		t.Fatalf("spec should not infer search or analysis: %#v", spec)
	}
	if len(spec.QueryTerms) != 0 || len(spec.LowQualityTerms) != 0 || len(spec.RequiredTerms) != 0 {
		t.Fatalf("spec should not contain keyword-derived terms: %#v", spec)
	}
	if !strings.Contains(spec.PromptText(), "main_agent_plan_spec") {
		t.Fatalf("PromptText = %q", spec.PromptText())
	}
}
