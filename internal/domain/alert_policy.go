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
	Status               AlertPolicyDecisionStatus `json:"status"`
	AutoNotify           bool                      `json:"auto_notify"`
	RequiresConfirmation bool                      `json:"requires_confirmation"`
	Reasons              []string                  `json:"reasons"`
	Channel              string                    `json:"channel"`
	Importance           float64                   `json:"importance"`
	Confidence           float64                   `json:"confidence"`
}
