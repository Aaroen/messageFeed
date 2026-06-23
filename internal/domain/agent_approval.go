package domain

import "time"

type AgentApprovalStatus string

const (
	AgentApprovalStatusPending  AgentApprovalStatus = "pending"
	AgentApprovalStatusApproved AgentApprovalStatus = "approved"
	AgentApprovalStatusRejected AgentApprovalStatus = "rejected"
	AgentApprovalStatusExpired  AgentApprovalStatus = "expired"
)

func (s AgentApprovalStatus) Valid() bool {
	switch s {
	case AgentApprovalStatusPending, AgentApprovalStatusApproved, AgentApprovalStatusRejected, AgentApprovalStatusExpired:
		return true
	default:
		return false
	}
}

type AgentApprovalDecision string

const (
	AgentApprovalDecisionApprove AgentApprovalDecision = "approve"
	AgentApprovalDecisionReject  AgentApprovalDecision = "reject"
)

func (d AgentApprovalDecision) Valid() bool {
	switch d {
	case AgentApprovalDecisionApprove, AgentApprovalDecisionReject:
		return true
	default:
		return false
	}
}

type AgentApproval struct {
	ID                int64
	PlanID            *int64
	UserID            int64
	ExternalAccountID *int64
	ApprovalTokenHash string
	Channel           string
	Status            AgentApprovalStatus
	ExpiresAt         time.Time
	DecidedAt         *time.Time
	RequestID         string
	TraceID           string
	Metadata          AgentJSON
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
