package agent

import (
	"strconv"
	"strings"
)

type CanonicalFactRef struct {
	FactType     string
	FactID       int64
	CanonicalRef string
	LegacyRef    string
}

func NormalizeCanonicalRef(ref string) string {
	parsed, ok := ParseCanonicalFactRef(ref)
	if !ok {
		return strings.TrimSpace(ref)
	}
	return parsed.CanonicalRef
}

func ParseCanonicalFactRef(ref string) (CanonicalFactRef, bool) {
	legacy := strings.TrimSpace(ref)
	if legacy == "" {
		return CanonicalFactRef{}, false
	}
	refType, refID, ok := splitFactRef(legacy)
	if !ok {
		return CanonicalFactRef{}, false
	}
	refType = canonicalFactType(refType)
	if refType == "" || refID <= 0 {
		return CanonicalFactRef{}, false
	}
	canonical := refType + ":" + strconv.FormatInt(refID, 10)
	return CanonicalFactRef{
		FactType:     refType,
		FactID:       refID,
		CanonicalRef: canonical,
		LegacyRef:    legacy,
	}, true
}

func NormalizeCanonicalRefs(refs []string) []string {
	if len(refs) == 0 {
		return nil
	}
	output := make([]string, 0, len(refs))
	seen := map[string]struct{}{}
	for _, ref := range refs {
		normalized := NormalizeCanonicalRef(ref)
		if strings.TrimSpace(normalized) == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		output = append(output, normalized)
	}
	return output
}

func splitFactRef(ref string) (string, int64, bool) {
	separator := strings.LastIndex(ref, ":")
	if separator < 0 {
		separator = strings.LastIndex(ref, "/")
	}
	if separator <= 0 || separator >= len(ref)-1 {
		return "", 0, false
	}
	id, err := strconv.ParseInt(strings.TrimSpace(ref[separator+1:]), 10, 64)
	if err != nil {
		return "", 0, false
	}
	return strings.TrimSpace(ref[:separator]), id, true
}

func canonicalFactType(value string) string {
	switch strings.TrimSpace(value) {
	case "transcript", "agent_transcript_entry", "agent_transcript_entries":
		return "transcript"
	case "turn", "agent_turn", "agent_turns":
		return "turn"
	case "plan", "agent_plan", "agent_plans":
		return "plan"
	case "plan_step", "agent_plan_step", "agent_plan_steps":
		return "plan_step"
	case "run", "agent_run", "agent_runs":
		return "run"
	case "observation", "agent_observation", "agent_observations":
		return "observation"
	case "artifact", "agent_artifact", "agent_artifacts":
		return "artifact"
	case "item", "items":
		return "item"
	case "web_snapshot", "web_snapshots":
		return "web_snapshot"
	default:
		return ""
	}
}
