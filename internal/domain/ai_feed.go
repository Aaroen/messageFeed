package domain

type AIFeedEntryKind string

const (
	AIFeedEntryKindDailySummary      AIFeedEntryKind = "daily_summary"
	AIFeedEntryKindAlertExplanation  AIFeedEntryKind = "alert_explanation"
	AIFeedEntryKindSourceHealth      AIFeedEntryKind = "source_health"
	AIFeedEntryKindAgentOperationLog AIFeedEntryKind = "agent_operation_log"
)

func (k AIFeedEntryKind) Valid() bool {
	switch k {
	case AIFeedEntryKindDailySummary, AIFeedEntryKindAlertExplanation, AIFeedEntryKindSourceHealth, AIFeedEntryKindAgentOperationLog:
		return true
	default:
		return false
	}
}
