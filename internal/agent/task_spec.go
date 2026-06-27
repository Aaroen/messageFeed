package agent

import "strings"

const (
	TaskTypeQuestionAnswer = "question_answer"
	TaskTypeSearch         = "search"
	TaskTypeNewsAnalysis   = "news_analysis"
	TaskTypeProjectStatus  = "project_status"

	TaskDomainGeneral = "general"
	TaskDomainFinance = "finance"
	TaskDomainTech    = "technology"
	TaskDomainProject = "project"

	TaskFreshnessRealtime   = "realtime"
	TaskFreshnessRecent     = "recent"
	TaskFreshnessHistorical = "historical"
)

// TaskSpec 是旧质量门的兼容结构。
// 主路径不再由该结构根据用户文本判断领域、时效或搜索意图；这些语义均来自主 Agent 的模型 PlanSpec。
type TaskSpec struct {
	RawText              string
	TaskType             string
	Domain               string
	Freshness            string
	RequiresExternal     bool
	NeedsAnalysis        bool
	QueryTerms           []string
	RequiredTerms        []string
	PreferredTerms       []string
	EvidenceTypes        []string
	ExcludedTerms        []string
	LowQualityTerms      []string
	MinimumEvidenceCount int
}

// BuildTaskSpec 只做兼容初始化，不再包含任何面向具体领域或意图的关键词规则。
func BuildTaskSpec(message string) TaskSpec {
	return TaskSpec{
		RawText:              strings.TrimSpace(message),
		TaskType:             TaskTypeQuestionAnswer,
		Domain:               TaskDomainGeneral,
		Freshness:            TaskFreshnessHistorical,
		MinimumEvidenceCount: 1,
	}
}

func (s TaskSpec) RequestsSearch() bool {
	return s.RequiresExternal || s.TaskType == TaskTypeSearch || s.TaskType == TaskTypeNewsAnalysis
}

func (s TaskSpec) PromptText() string {
	parts := []string{
		"任务规格来源=main_agent_plan_spec",
		"领域判断=模型生成",
		"能力选择=模型生成并由后端校验",
	}
	if strings.TrimSpace(s.RawText) != "" {
		parts = append(parts, "原始消息已提供")
	}
	return strings.Join(parts, "；")
}

func appendUniqueStrings(values []string, additions ...string) []string {
	seen := map[string]struct{}{}
	deduped := make([]string, 0, len(values)+len(additions))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		deduped = append(deduped, value)
	}
	for _, value := range additions {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		deduped = append(deduped, value)
	}
	return deduped
}
