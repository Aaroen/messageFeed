package agent

import (
	"sort"
	"strings"
)

type EvidenceScoreInput struct {
	Title       string
	Source      string
	Summary     string
	URL         string
	PublishedAt string
}

type EvidenceScore struct {
	Score      float64
	Relevant   bool
	LowQuality bool
	Reasons    []string
}

// ScoreEvidence 只评估证据结构完整性，不再使用领域或市场方向关键词。
func ScoreEvidence(_ TaskSpec, input EvidenceScoreInput) EvidenceScore {
	score := 0.0
	reasons := make([]string, 0, 4)
	if strings.TrimSpace(input.Title) != "" {
		score += 0.3
		reasons = append(reasons, "has_title")
	}
	if strings.TrimSpace(input.URL) != "" {
		score += 0.25
		reasons = append(reasons, "has_url")
	}
	if strings.TrimSpace(input.Source) != "" {
		score += 0.15
		reasons = append(reasons, "has_source")
	}
	if strings.TrimSpace(input.Summary) != "" {
		score += 0.2
		reasons = append(reasons, "has_summary")
	}
	if strings.TrimSpace(input.PublishedAt) != "" {
		score += 0.1
		reasons = append(reasons, "has_published_at")
	}
	return EvidenceScore{
		Score:    roundEvidenceScore(score),
		Relevant: score >= 0.3,
		Reasons:  appendUniqueStrings(nil, reasons...),
	}
}

func FilterAndRankEvidence(spec TaskSpec, inputs []EvidenceScoreInput) []EvidenceScoreInput {
	type scoredEvidence struct {
		input EvidenceScoreInput
		score EvidenceScore
		index int
	}
	scored := make([]scoredEvidence, 0, len(inputs))
	for index, input := range inputs {
		score := ScoreEvidence(spec, input)
		if !score.Relevant {
			continue
		}
		scored = append(scored, scoredEvidence{input: input, score: score, index: index})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score.Score == scored[j].score.Score {
			return scored[i].index < scored[j].index
		}
		return scored[i].score.Score > scored[j].score.Score
	})
	filtered := make([]EvidenceScoreInput, 0, len(scored))
	for _, item := range scored {
		filtered = append(filtered, item.input)
	}
	return filtered
}

func roundEvidenceScore(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}
