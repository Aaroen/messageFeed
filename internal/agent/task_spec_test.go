package agent

import (
	"strings"
	"testing"
)

func TestBuildTaskSpecForFinanceNewsAnalysis(t *testing.T) {
	spec := BuildTaskSpec("搜索最新港股消息并分析")
	if spec.TaskType != TaskTypeNewsAnalysis {
		t.Fatalf("TaskType = %q", spec.TaskType)
	}
	if spec.Domain != TaskDomainFinance {
		t.Fatalf("Domain = %q", spec.Domain)
	}
	if spec.Freshness != TaskFreshnessRealtime {
		t.Fatalf("Freshness = %q", spec.Freshness)
	}
	if !spec.RequiresExternal || !spec.NeedsAnalysis {
		t.Fatalf("external/analysis = %v/%v", spec.RequiresExternal, spec.NeedsAnalysis)
	}
	if !containsTestString(spec.QueryTerms, "港股") {
		t.Fatalf("QueryTerms = %#v", spec.QueryTerms)
	}
	if !containsTestString(spec.RequiredTerms, "恒生") || !containsTestString(spec.LowQualityTerms, "开户") {
		t.Fatalf("finance defaults missing: required=%#v low_quality=%#v", spec.RequiredTerms, spec.LowQualityTerms)
	}
	if !strings.Contains(spec.PromptText(), "证据类型=财经新闻") {
		t.Fatalf("PromptText = %q", spec.PromptText())
	}
}

func TestBuildTaskSpecForProjectStatus(t *testing.T) {
	spec := BuildTaskSpec("汇报当前项目实现进度")
	if spec.TaskType != TaskTypeProjectStatus {
		t.Fatalf("TaskType = %q", spec.TaskType)
	}
	if spec.Domain != TaskDomainProject {
		t.Fatalf("Domain = %q", spec.Domain)
	}
	if spec.RequiresExternal {
		t.Fatalf("project status should not require external retrieval")
	}
}

func containsTestString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
