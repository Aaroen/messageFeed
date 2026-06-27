package agent

import "strings"

type AnswerQualityResult struct {
	Passed  bool
	Summary string
	Reply   string
}

// EvaluateEvidenceQuality 只检查证据数量，不再根据领域词表或市场方向替模型做判断。
func EvaluateEvidenceQuality(spec TaskSpec, evidence []EvidenceScoreInput) AnswerQualityResult {
	if !spec.RequestsSearch() {
		return AnswerQualityResult{Passed: true, Summary: "quality gate not required"}
	}
	minimum := spec.MinimumEvidenceCount
	if minimum < 1 {
		minimum = 1
	}
	if len(evidence) < minimum {
		return AnswerQualityResult{
			Passed:  false,
			Summary: "insufficient evidence count",
		}
	}
	return AnswerQualityResult{Passed: true, Summary: "evidence count gate passed"}
}

// EvaluateAnswerQuality 只检查模型是否返回非空内容，事实和方向一致性由模型基于工具观察自行完成。
func EvaluateAnswerQuality(spec TaskSpec, reply string, evidence []EvidenceScoreInput) AnswerQualityResult {
	if quality := EvaluateEvidenceQuality(spec, evidence); !quality.Passed {
		return quality
	}
	if strings.TrimSpace(reply) == "" {
		return AnswerQualityResult{
			Passed:  false,
			Summary: "empty answer",
		}
	}
	return AnswerQualityResult{Passed: true, Summary: "answer quality gate passed"}
}
