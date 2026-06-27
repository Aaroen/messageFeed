package agent

import (
	"strconv"
	"strings"
)

type AnswerQualityResult struct {
	Passed  bool
	Summary string
	Reply   string
}

func EvaluateEvidenceQuality(spec TaskSpec, evidence []EvidenceScoreInput) AnswerQualityResult {
	if !spec.RequestsSearch() {
		return AnswerQualityResult{Passed: true, Summary: "quality gate not required"}
	}
	if len(evidence) == 0 {
		return AnswerQualityResult{
			Passed:  false,
			Summary: "no relevant evidence",
			Reply:   buildInsufficientEvidenceReply(spec, evidence, "未检索到足够可靠的相关证据，暂不形成分析判断。"),
		}
	}
	minimum := spec.MinimumEvidenceCount
	if minimum < 1 {
		minimum = 1
	}
	if len(evidence) < minimum {
		return AnswerQualityResult{
			Passed:  false,
			Summary: "insufficient relevant evidence",
			Reply:   buildInsufficientEvidenceReply(spec, evidence, "当前相关证据数量不足，暂不形成完整分析判断。"),
		}
	}
	return AnswerQualityResult{Passed: true, Summary: "evidence quality gate passed"}
}

func EvaluateAnswerQuality(spec TaskSpec, reply string, evidence []EvidenceScoreInput) AnswerQualityResult {
	evidenceQuality := EvaluateEvidenceQuality(spec, evidence)
	if !evidenceQuality.Passed {
		return evidenceQuality
	}
	if !spec.RequestsSearch() {
		return AnswerQualityResult{Passed: true, Summary: "answer quality gate not required"}
	}
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return AnswerQualityResult{
			Passed:  false,
			Summary: "empty answer",
			Reply:   buildInsufficientEvidenceReply(spec, evidence, "当前没有生成可用内容，暂不形成分析判断。"),
		}
	}
	lowerReply := strings.ToLower(reply)
	if countContainedTerms(lowerReply, spec.LowQualityTerms) > 0 && spec.TaskType == TaskTypeNewsAnalysis {
		return AnswerQualityResult{
			Passed:  false,
			Summary: "answer contains low quality terms",
			Reply:   buildInsufficientEvidenceReply(spec, evidence, "当前回答混入与任务不匹配的内容，暂不形成分析判断。"),
		}
	}
	evidenceText := strings.ToLower(joinEvidenceText(evidence))
	if containsEvidenceAny(lowerReply, negativeMarketTerms()) && !containsEvidenceAny(evidenceText, negativeMarketTerms()) {
		return AnswerQualityResult{
			Passed:  false,
			Summary: "negative conclusion is not supported by evidence",
			Reply:   buildInsufficientEvidenceReply(spec, evidence, "当前证据不足以支撑偏弱或下跌判断，暂不形成该方向结论。"),
		}
	}
	if containsEvidenceAny(lowerReply, positiveMarketTerms()) && !containsEvidenceAny(evidenceText, positiveMarketTerms()) {
		return AnswerQualityResult{
			Passed:  false,
			Summary: "positive conclusion is not supported by evidence",
			Reply:   buildInsufficientEvidenceReply(spec, evidence, "当前证据不足以支撑走强或反弹判断，暂不形成该方向结论。"),
		}
	}
	return AnswerQualityResult{Passed: true, Summary: "answer quality gate passed"}
}

func buildInsufficientEvidenceReply(spec TaskSpec, evidence []EvidenceScoreInput, conclusion string) string {
	var builder strings.Builder
	builder.WriteString("结论：")
	builder.WriteString(strings.TrimSpace(conclusion))
	if len(evidence) > 0 {
		limit := len(evidence)
		if limit > 3 {
			limit = 3
		}
		builder.WriteString("\n\n依据：")
		for index := 0; index < limit; index++ {
			item := evidence[index]
			builder.WriteString("\n")
			builder.WriteString(strconv.Itoa(index + 1))
			builder.WriteString(". ")
			builder.WriteString(strings.TrimSpace(item.Title))
			if strings.TrimSpace(item.Source) != "" {
				builder.WriteString("（")
				builder.WriteString(strings.TrimSpace(item.Source))
				builder.WriteString("）")
			}
			if strings.TrimSpace(item.PublishedAt) != "" {
				builder.WriteString("，时间：")
				builder.WriteString(strings.TrimSpace(item.PublishedAt))
			}
			if strings.TrimSpace(item.Summary) != "" {
				builder.WriteString("\n   ")
				builder.WriteString(trimRunes(item.Summary, 180))
			}
		}
	} else {
		builder.WriteString("\n\n依据：未获得可用于回答该问题的合格事实来源。")
	}
	builder.WriteString("\n\n分析过程：")
	switch spec.Domain {
	case TaskDomainFinance:
		builder.WriteString("已排除与问题不匹配的开户、教程、课程或交易流程类页面；需要新闻、行情、资金流向或公告等事实交叉验证后，才能形成市场方向判断。")
	default:
		builder.WriteString("当前可用证据与任务规格不充分匹配，因此只给出证据不足结论，不补充未被来源支持的判断。")
	}
	return builder.String()
}

func joinEvidenceText(evidence []EvidenceScoreInput) string {
	var builder strings.Builder
	for _, item := range evidence {
		builder.WriteString(item.Title)
		builder.WriteString(" ")
		builder.WriteString(item.Source)
		builder.WriteString(" ")
		builder.WriteString(item.Summary)
		builder.WriteString(" ")
		builder.WriteString(item.PublishedAt)
		builder.WriteString(" ")
	}
	return builder.String()
}

func negativeMarketTerms() []string {
	return []string{"跌", "下跌", "下挫", "低开", "重挫", "新低", "净卖出", "走弱", "偏弱", "谨慎"}
}

func positiveMarketTerms() []string {
	return []string{"涨", "上涨", "走强", "反弹", "净买入", "创新高", "修复"}
}
