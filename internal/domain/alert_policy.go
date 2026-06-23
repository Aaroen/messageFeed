package domain

type AlertPolicyDecisionStatus string

const (
	AlertPolicyDecisionStatusAllow                AlertPolicyDecisionStatus = "allow"
	AlertPolicyDecisionStatusPendingAnalysis      AlertPolicyDecisionStatus = "pending_analysis"
	AlertPolicyDecisionStatusRequiresConfirmation AlertPolicyDecisionStatus = "requires_confirmation"
	AlertPolicyDecisionStatusSuppressed           AlertPolicyDecisionStatus = "suppressed"
)

func (s AlertPolicyDecisionStatus) Valid() bool {
	switch s {
	case AlertPolicyDecisionStatusAllow, AlertPolicyDecisionStatusPendingAnalysis, AlertPolicyDecisionStatusRequiresConfirmation, AlertPolicyDecisionStatusSuppressed:
		return true
	default:
		return false
	}
}

type AlertPolicyDecision struct {
	Status               AlertPolicyDecisionStatus
	AutoNotify           bool
	RequiresConfirmation bool
	Reasons              []string
	Channel              string
	Importance           float64
	Confidence           float64
}
