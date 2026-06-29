package agent

import (
	"fmt"
	"messagefeed/internal/domain"
	"strings"
)

type ContextBudgetProfile string

const (
	ContextBudgetProfileMainPlanning          ContextBudgetProfile = "main_planning"
	ContextBudgetProfileMainEvaluation        ContextBudgetProfile = "main_evaluation"
	ContextBudgetProfileSubagentSearch        ContextBudgetProfile = "subagent_search"
	ContextBudgetProfileSubagentHistoryRecall ContextBudgetProfile = "subagent_history_recall"
	ContextBudgetProfileSubagentAnalysis      ContextBudgetProfile = "subagent_analysis"
	ContextBudgetProfileFinalSynthesis        ContextBudgetProfile = "final_synthesis"
)

type ContextBudgetSpec struct {
	Profile              ContextBudgetProfile
	TotalTokens          int
	RecentMessagesTokens int
	StableMemoryTokens   int
	HistoryRecallTokens  int
	EvidenceTokens       int
	PlanTokens           int
	OutputReserveTokens  int
	SafetyMarginTokens   int
}

type ContextBundle struct {
	BudgetProfile   ContextBudgetProfile `json:"budget_profile"`
	SystemBlocks    []ContextBlock       `json:"system_blocks,omitempty"`
	RecentMessages  []ContextMessage     `json:"recent_messages,omitempty"`
	CurrentMessage  *ContextMessage      `json:"current_message,omitempty"`
	ActiveGoal      string               `json:"active_goal,omitempty"`
	ActivePlan      *ContextBlock        `json:"active_plan,omitempty"`
	KeyObservations []CapabilityObservation
	KeyArtifacts    []ContextBlock        `json:"key_artifacts,omitempty"`
	UserConstraints []ContextBlock        `json:"user_constraints,omitempty"`
	MemoryBlocks    []ContextBlock        `json:"memory_blocks,omitempty"`
	SemanticUnits   []ContextSemanticUnit `json:"semantic_units,omitempty"`
	BudgetReport    ContextBudgetReport   `json:"budget_report"`
}

type ContextSemanticUnitType string

const (
	ContextSemanticUnitMessage        ContextSemanticUnitType = "message"
	ContextSemanticUnitMessagePair    ContextSemanticUnitType = "message_pair"
	ContextSemanticUnitContextBlock   ContextSemanticUnitType = "context_block"
	ContextSemanticUnitHistoryRecall  ContextSemanticUnitType = "history_recall"
	ContextSemanticUnitObservation    ContextSemanticUnitType = "observation"
	ContextSemanticUnitArtifact       ContextSemanticUnitType = "artifact"
	ContextSemanticUnitPlan           ContextSemanticUnitType = "plan"
	ContextSemanticUnitUserConstraint ContextSemanticUnitType = "user_constraint"
)

type ContextSemanticUnit struct {
	ID              string                  `json:"id"`
	Type            ContextSemanticUnitType `json:"type"`
	Source          string                  `json:"source,omitempty"`
	Content         string                  `json:"content,omitempty"`
	Messages        []ContextMessage        `json:"messages,omitempty"`
	BlockRefs       []ContextBlockRef       `json:"block_refs,omitempty"`
	EvidenceRefs    []ContextEvidenceRef    `json:"evidence_refs,omitempty"`
	CanonicalRef    string                  `json:"canonical_ref,omitempty"`
	TokenEstimate   int                     `json:"token_estimate"`
	Protected       bool                    `json:"protected"`
	Selected        bool                    `json:"selected"`
	Projected       bool                    `json:"projected"`
	RetentionReason string                  `json:"retention_reason,omitempty"`
	OmittedReason   string                  `json:"omitted_reason,omitempty"`
}

type ContextBlockRef struct {
	Name          string `json:"name,omitempty"`
	CapabilityKey string `json:"capability_key,omitempty"`
	CanonicalRef  string `json:"canonical_ref,omitempty"`
}

type ContextEvidenceRef struct {
	Ref          string `json:"ref,omitempty"`
	CanonicalRef string `json:"canonical_ref,omitempty"`
	Source       string `json:"source,omitempty"`
}

type ContextBudgetReport struct {
	Profile              ContextBudgetProfile     `json:"profile"`
	TotalBudgetTokens    int                      `json:"total_budget_tokens"`
	RecentMessagesTokens int                      `json:"recent_messages_tokens"`
	OutputReserveTokens  int                      `json:"output_reserve_tokens"`
	SafetyMarginTokens   int                      `json:"safety_margin_tokens"`
	AvailableInputTokens int                      `json:"available_input_tokens"`
	UsedTokens           int                      `json:"used_tokens"`
	SelectedUnitCount    int                      `json:"selected_unit_count"`
	SkippedUnitCount     int                      `json:"skipped_unit_count"`
	ProtectedUnitCount   int                      `json:"protected_unit_count"`
	OversizedUnitCount   int                      `json:"oversized_unit_count"`
	SelectedMessageCount int                      `json:"selected_message_count"`
	SkippedMessageCount  int                      `json:"skipped_message_count"`
	Units                []ContextBudgetUnitTrace `json:"units,omitempty"`
}

type ContextBudgetUnitTrace struct {
	UnitID          string                  `json:"unit_id"`
	UnitType        ContextSemanticUnitType `json:"unit_type"`
	TokenEstimate   int                     `json:"token_estimate"`
	Selected        bool                    `json:"selected"`
	Protected       bool                    `json:"protected"`
	Projected       bool                    `json:"projected"`
	RetentionReason string                  `json:"retention_reason,omitempty"`
	OmittedReason   string                  `json:"omitted_reason,omitempty"`
}

func NormalizeContextBudgetProfile(profile ContextBudgetProfile) ContextBudgetProfile {
	switch profile {
	case ContextBudgetProfileMainPlanning,
		ContextBudgetProfileMainEvaluation,
		ContextBudgetProfileSubagentSearch,
		ContextBudgetProfileSubagentHistoryRecall,
		ContextBudgetProfileSubagentAnalysis,
		ContextBudgetProfileFinalSynthesis:
		return profile
	default:
		return ContextBudgetProfileSubagentAnalysis
	}
}

func ContextBudgetForProfile(profile ContextBudgetProfile) ContextBudgetSpec {
	profile = NormalizeContextBudgetProfile(profile)
	switch profile {
	case ContextBudgetProfileMainPlanning:
		return ContextBudgetSpec{
			Profile:              profile,
			TotalTokens:          64000,
			RecentMessagesTokens: 32000,
			StableMemoryTokens:   6000,
			HistoryRecallTokens:  8000,
			EvidenceTokens:       10000,
			PlanTokens:           5000,
			OutputReserveTokens:  7000,
			SafetyMarginTokens:   3000,
		}
	case ContextBudgetProfileMainEvaluation:
		return ContextBudgetSpec{
			Profile:              profile,
			TotalTokens:          64000,
			RecentMessagesTokens: 12000,
			StableMemoryTokens:   4000,
			HistoryRecallTokens:  8000,
			EvidenceTokens:       28000,
			PlanTokens:           6000,
			OutputReserveTokens:  6000,
			SafetyMarginTokens:   3000,
		}
	case ContextBudgetProfileSubagentSearch:
		return ContextBudgetSpec{
			Profile:              profile,
			TotalTokens:          64000,
			RecentMessagesTokens: 4000,
			StableMemoryTokens:   2000,
			HistoryRecallTokens:  0,
			EvidenceTokens:       44000,
			PlanTokens:           4000,
			OutputReserveTokens:  8000,
			SafetyMarginTokens:   2000,
		}
	case ContextBudgetProfileSubagentHistoryRecall:
		return ContextBudgetSpec{
			Profile:              profile,
			TotalTokens:          64000,
			RecentMessagesTokens: 8000,
			StableMemoryTokens:   2000,
			HistoryRecallTokens:  36000,
			EvidenceTokens:       8000,
			PlanTokens:           3000,
			OutputReserveTokens:  6000,
			SafetyMarginTokens:   2000,
		}
	case ContextBudgetProfileFinalSynthesis:
		return ContextBudgetSpec{
			Profile:              profile,
			TotalTokens:          64000,
			RecentMessagesTokens: 24000,
			StableMemoryTokens:   4000,
			HistoryRecallTokens:  6000,
			EvidenceTokens:       28000,
			PlanTokens:           4000,
			OutputReserveTokens:  8000,
			SafetyMarginTokens:   3000,
		}
	default:
		return ContextBudgetSpec{
			Profile:              ContextBudgetProfileSubagentAnalysis,
			TotalTokens:          64000,
			RecentMessagesTokens: 6000,
			StableMemoryTokens:   4000,
			HistoryRecallTokens:  4000,
			EvidenceTokens:       42000,
			PlanTokens:           4000,
			OutputReserveTokens:  8000,
			SafetyMarginTokens:   3000,
		}
	}
}

func RecentConversationCandidateLimit(profile ContextBudgetProfile) int {
	spec := ContextBudgetForProfile(profile)
	return recentConversationCandidateLimitForTokenBudget(spec.RecentMessagesTokens)
}

func ContextBudgetProfileForCapabilityScope(capabilityKeys []string) ContextBudgetProfile {
	if len(capabilityKeys) == 0 {
		return ContextBudgetProfileSubagentAnalysis
	}
	hasHistory := false
	hasSearch := false
	hasAnalysis := false
	for _, key := range capabilityKeys {
		switch strings.TrimSpace(key) {
		case "conversation.query_history":
			hasHistory = true
		case "feed.query_recent_items", "source.query_latest_items", "web.search", "web.fetch_page", "web.extract_page", "repo.search", "repo.inspect_remote":
			hasSearch = true
		default:
			hasAnalysis = true
		}
	}
	if hasHistory && !hasSearch && !hasAnalysis {
		return ContextBudgetProfileSubagentHistoryRecall
	}
	if hasSearch {
		return ContextBudgetProfileSubagentSearch
	}
	return ContextBudgetProfileSubagentAnalysis
}

func CompleteContextBudgetReport(report ContextBudgetReport, spec ContextBudgetSpec) ContextBudgetReport {
	report.Profile = spec.Profile
	report.TotalBudgetTokens = spec.TotalTokens
	report.RecentMessagesTokens = spec.RecentMessagesTokens
	report.OutputReserveTokens = spec.OutputReserveTokens
	report.SafetyMarginTokens = spec.SafetyMarginTokens
	if report.AvailableInputTokens == 0 {
		report.AvailableInputTokens = spec.TotalTokens - spec.OutputReserveTokens - spec.SafetyMarginTokens
	}
	return report
}

func recentConversationCandidateLimitForTokenBudget(tokenBudget int) int {
	if tokenBudget <= 0 {
		return 32
	}
	limit := tokenBudget / 80
	if limit < 32 {
		return 32
	}
	if limit > 240 {
		return 240
	}
	return limit
}

func BuildConversationSemanticUnits(messages []ContextMessage) []ContextSemanticUnit {
	units := make([]ContextSemanticUnit, 0, len(messages))
	for index := 0; index < len(messages); index++ {
		message := messages[index]
		if !contextMessageModelVisible(message) {
			continue
		}
		if message.Role == domain.AgentTranscriptRoleUser && index+1 < len(messages) {
			next := messages[index+1]
			if contextMessageModelVisible(next) && next.Role == domain.AgentTranscriptRoleAssistant && sameConversationTurn(message, next) {
				unitMessages := []ContextMessage{message, next}
				units = append(units, contextMessageUnit(ContextSemanticUnitMessagePair, unitMessages, "recent_conversation"))
				index++
				continue
			}
		}
		units = append(units, contextMessageUnit(ContextSemanticUnitMessage, []ContextMessage{message}, "recent_conversation"))
	}
	return markLatestAssistantUnitProtected(units)
}

func SelectSemanticUnitsByTokenBudget(units []ContextSemanticUnit, budgetTokens int, profile ContextBudgetProfile) ([]ContextSemanticUnit, ContextBudgetReport) {
	profile = NormalizeContextBudgetProfile(profile)
	if budgetTokens < 0 {
		budgetTokens = 0
	}
	output := make([]ContextSemanticUnit, len(units))
	copy(output, units)
	report := ContextBudgetReport{
		Profile:              profile,
		RecentMessagesTokens: budgetTokens,
		AvailableInputTokens: budgetTokens,
		Units:                make([]ContextBudgetUnitTrace, 0, len(output)),
	}
	selected := make(map[int]struct{}, len(output))
	used := 0
	for index := range output {
		ensureSemanticUnitTokenEstimate(&output[index])
		if output[index].Protected {
			report.ProtectedUnitCount++
			if used+output[index].TokenEstimate <= budgetTokens {
				output[index].Selected = true
				output[index].RetentionReason = "protected_unit"
				selected[index] = struct{}{}
				used += output[index].TokenEstimate
				continue
			}
			output[index].OmittedReason = "protected_unit_exceeds_budget_requires_projection"
			output[index].Projected = true
			report.OversizedUnitCount++
		}
	}
	for index := len(output) - 1; index >= 0; index-- {
		if _, ok := selected[index]; ok {
			continue
		}
		if output[index].Protected {
			continue
		}
		ensureSemanticUnitTokenEstimate(&output[index])
		if output[index].TokenEstimate > budgetTokens {
			output[index].OmittedReason = "semantic_unit_exceeds_budget_requires_projection"
			output[index].Projected = true
			report.OversizedUnitCount++
			continue
		}
		if used+output[index].TokenEstimate > budgetTokens {
			output[index].OmittedReason = "recent_message_budget_exhausted"
			continue
		}
		output[index].Selected = true
		output[index].RetentionReason = "fits_recent_message_budget"
		selected[index] = struct{}{}
		used += output[index].TokenEstimate
	}
	report.UsedTokens = used
	for _, unit := range output {
		if unit.Selected {
			report.SelectedUnitCount++
			report.SelectedMessageCount += len(unit.Messages)
		} else {
			report.SkippedUnitCount++
			report.SkippedMessageCount += len(unit.Messages)
		}
		report.Units = append(report.Units, ContextBudgetUnitTrace{
			UnitID:          unit.ID,
			UnitType:        unit.Type,
			TokenEstimate:   unit.TokenEstimate,
			Selected:        unit.Selected,
			Protected:       unit.Protected,
			Projected:       unit.Projected,
			RetentionReason: unit.RetentionReason,
			OmittedReason:   unit.OmittedReason,
		})
	}
	return output, report
}

func SelectedMessagesFromSemanticUnits(units []ContextSemanticUnit) []ContextMessage {
	messages := make([]ContextMessage, 0, len(units))
	seenEntries := map[int64]struct{}{}
	for _, unit := range units {
		if !unit.Selected {
			continue
		}
		for _, message := range unit.Messages {
			if message.TranscriptEntryID > 0 {
				if _, ok := seenEntries[message.TranscriptEntryID]; ok {
					continue
				}
				seenEntries[message.TranscriptEntryID] = struct{}{}
			}
			messages = append(messages, message)
		}
	}
	return messages
}

func NewCurrentMessageSemanticUnit(content string) ContextSemanticUnit {
	content = strings.TrimSpace(content)
	unit := ContextSemanticUnit{
		ID:              "current_user_message",
		Type:            ContextSemanticUnitMessage,
		Source:          "current_turn",
		Content:         content,
		Protected:       true,
		Selected:        true,
		RetentionReason: "current_user_message",
	}
	ensureSemanticUnitTokenEstimate(&unit)
	return unit
}

func estimateContextTokenCount(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	return (len([]rune(value)) + 3) / 4
}

func contextMessageUnit(unitType ContextSemanticUnitType, messages []ContextMessage, source string) ContextSemanticUnit {
	unit := ContextSemanticUnit{
		ID:       semanticUnitID(unitType, messages),
		Type:     unitType,
		Source:   source,
		Messages: append([]ContextMessage(nil), messages...),
		Content:  FormatContextMessages(messages),
	}
	ensureSemanticUnitTokenEstimate(&unit)
	return unit
}

func semanticUnitID(unitType ContextSemanticUnitType, messages []ContextMessage) string {
	if len(messages) == 0 {
		return string(unitType)
	}
	first := messages[0]
	last := messages[len(messages)-1]
	if first.TranscriptEntryID > 0 || last.TranscriptEntryID > 0 {
		return fmt.Sprintf("%s:transcript:%d-%d", unitType, first.TranscriptEntryID, last.TranscriptEntryID)
	}
	if first.TurnID > 0 || last.TurnID > 0 {
		return fmt.Sprintf("%s:turn:%d-%d", unitType, first.TurnID, last.TurnID)
	}
	hash := textHash(FormatContextMessages(messages))
	if len(hash) > 12 {
		hash = hash[:12]
	}
	return fmt.Sprintf("%s:content:%s", unitType, hash)
}

func ensureSemanticUnitTokenEstimate(unit *ContextSemanticUnit) {
	if unit == nil || unit.TokenEstimate > 0 {
		return
	}
	content := strings.TrimSpace(unit.Content)
	if content == "" && len(unit.Messages) > 0 {
		content = FormatContextMessages(unit.Messages)
		unit.Content = content
	}
	unit.TokenEstimate = estimateContextTokenCount(content)
}

func contextMessageModelVisible(message ContextMessage) bool {
	if strings.TrimSpace(message.Content) == "" {
		return false
	}
	return message.Role == domain.AgentTranscriptRoleUser || message.Role == domain.AgentTranscriptRoleAssistant
}

func sameConversationTurn(a ContextMessage, b ContextMessage) bool {
	if a.TurnID == 0 || b.TurnID == 0 {
		return true
	}
	return a.TurnID == b.TurnID
}

func markLatestAssistantUnitProtected(units []ContextSemanticUnit) []ContextSemanticUnit {
	output := make([]ContextSemanticUnit, len(units))
	copy(output, units)
	for index := len(output) - 1; index >= 0; index-- {
		for _, message := range output[index].Messages {
			if message.Role == domain.AgentTranscriptRoleAssistant {
				output[index].Protected = true
				output[index].RetentionReason = "previous_assistant_answer"
				return output
			}
		}
	}
	return output
}
