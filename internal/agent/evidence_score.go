package agent

import (
	"net/url"
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

func ScoreEvidence(spec TaskSpec, input EvidenceScoreInput) EvidenceScore {
	text := evidenceSearchText(input)
	score := 0.0
	reasons := make([]string, 0, 6)

	queryHits := countContainedTerms(text, spec.QueryTerms)
	if queryHits > 0 {
		score += 0.25 + minEvidenceFloat(float64(queryHits)*0.08, 0.24)
		reasons = append(reasons, "query_term_match")
	}
	requiredHits := countContainedTerms(text, spec.RequiredTerms)
	if len(spec.RequiredTerms) > 0 && requiredHits > 0 {
		score += 0.22 + minEvidenceFloat(float64(requiredHits)*0.06, 0.18)
		reasons = append(reasons, "required_term_match")
	}
	preferredHits := countContainedTerms(text, spec.PreferredTerms)
	if preferredHits > 0 {
		score += minEvidenceFloat(float64(preferredHits)*0.05, 0.2)
		reasons = append(reasons, "preferred_term_match")
	}
	if strings.TrimSpace(input.PublishedAt) != "" {
		score += 0.12
		reasons = append(reasons, "has_published_at")
	} else if spec.Freshness == TaskFreshnessRealtime && containsEvidenceAny(text, []string{"今日", "今天", "最新", "实时", "盘中", "收盘", "早盘", "午盘"}) {
		score += 0.08
		reasons = append(reasons, "freshness_text_match")
	}
	if trustedEvidenceSource(spec, input) {
		score += 0.12
		reasons = append(reasons, "trusted_source")
	}
	lowQualityHits := countContainedTerms(text, spec.LowQualityTerms)
	if lowQualityHits > 0 {
		score -= minEvidenceFloat(0.25+float64(lowQualityHits)*0.16, 0.75)
		reasons = append(reasons, "low_quality_term")
	}

	if score < 0 {
		score = 0
	}
	threshold := evidenceRelevanceThreshold(spec)
	relevant := score >= threshold
	if spec.RequestsSearch() && len(spec.QueryTerms) > 0 && queryHits == 0 && requiredHits == 0 {
		relevant = false
		reasons = append(reasons, "missing_query_or_required_term")
	}
	if spec.TaskType == TaskTypeNewsAnalysis && lowQualityHits >= 2 {
		relevant = false
		reasons = append(reasons, "too_many_low_quality_terms")
	}
	return EvidenceScore{
		Score:      roundEvidenceScore(score),
		Relevant:   relevant,
		LowQuality: lowQualityHits > 0,
		Reasons:    appendUniqueStrings(nil, reasons...),
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

func evidenceSearchText(input EvidenceScoreInput) string {
	return strings.ToLower(strings.Join([]string{
		input.Title,
		input.Source,
		input.Summary,
		input.URL,
		input.PublishedAt,
	}, " "))
}

func countContainedTerms(text string, terms []string) int {
	count := 0
	seen := map[string]struct{}{}
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		if strings.Contains(text, term) {
			count++
		}
	}
	return count
}

func trustedEvidenceSource(spec TaskSpec, input EvidenceScoreInput) bool {
	text := evidenceSearchText(input)
	switch spec.Domain {
	case TaskDomainFinance:
		if containsEvidenceAny(text, []string{"财经", "证券", "财联社", "经济", "金融", "交易所", "港交所", "aastocks", "etnet", "hkex", "reuters", "bloomberg", "yahoo finance", "investing"}) {
			return true
		}
		host := evidenceHost(input.URL)
		return containsEvidenceAny(host, []string{
			"hkexnews.hk",
			"hkex.com.hk",
			"aastocks.com",
			"etnet.com.hk",
			"eastmoney.com",
			"finance.sina.com",
			"finance.ifeng.com",
			"yahoo.com",
			"investing.com",
			"reuters.com",
			"bloomberg.com",
		})
	case TaskDomainTech:
		return containsEvidenceAny(text, []string{"github.com", "go.dev", "developer", "docs", "release", "官方"})
	default:
		return false
	}
}

func evidenceHost(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
}

func evidenceRelevanceThreshold(spec TaskSpec) float64 {
	switch spec.TaskType {
	case TaskTypeNewsAnalysis:
		return 0.42
	case TaskTypeSearch:
		return 0.34
	default:
		return 0.25
	}
}

func containsEvidenceAny(value string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(value, strings.ToLower(strings.TrimSpace(term))) {
			return true
		}
	}
	return false
}

func minEvidenceFloat(left float64, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

func roundEvidenceScore(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}
