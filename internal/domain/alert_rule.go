package domain

import "time"

type AlertRuleScope string

const (
	AlertRuleScopeSource   AlertRuleScope = "source"
	AlertRuleScopeCategory AlertRuleScope = "category"
	AlertRuleScopeTag      AlertRuleScope = "tag"
	AlertRuleScopeKeyword  AlertRuleScope = "keyword"
	AlertRuleScopeTicker   AlertRuleScope = "ticker"
	AlertRuleScopeGlobal   AlertRuleScope = "global"
)

func (s AlertRuleScope) Valid() bool {
	switch s {
	case AlertRuleScopeSource, AlertRuleScopeCategory, AlertRuleScopeTag, AlertRuleScopeKeyword, AlertRuleScopeTicker, AlertRuleScopeGlobal:
		return true
	default:
		return false
	}
}

type AlertCandidateStatus string

const (
	AlertCandidateStatusReady           AlertCandidateStatus = "ready"
	AlertCandidateStatusPendingAnalysis AlertCandidateStatus = "pending_analysis"
	AlertCandidateStatusSuppressed      AlertCandidateStatus = "suppressed"
)

func (s AlertCandidateStatus) Valid() bool {
	switch s {
	case AlertCandidateStatusReady, AlertCandidateStatusPendingAnalysis, AlertCandidateStatusSuppressed:
		return true
	default:
		return false
	}
}

type AlertRuleCondition struct {
	SourceIDs  []int64  `json:"source_ids,omitempty"`
	Categories []string `json:"categories,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Keywords   []string `json:"keywords,omitempty"`
	Tickers    []string `json:"tickers,omitempty"`
}

type AlertRule struct {
	ID              int64
	UserID          int64
	Name            string
	Scope           AlertRuleScope
	Condition       AlertRuleCondition
	MinImportance   float64
	AIRequired      bool
	CooldownSeconds int
	Channel         string
	Enabled         bool
	LastTriggeredAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type AlertCandidate struct {
	ID             int64
	UserID         int64
	RuleID         int64
	ItemEventID    int64
	SourceID       int64
	ItemID         int64
	Status         AlertCandidateStatus
	MatchedReasons []string
	DedupeKey      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
