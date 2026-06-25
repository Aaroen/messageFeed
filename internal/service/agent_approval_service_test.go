package service

import (
	"context"
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestAgentApprovalServiceGetAndApprove(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	planID := int64(22)
	store := fakeAgentApprovalStore{
		approval: domain.AgentApproval{
			ID:                9,
			PlanID:            &planID,
			UserID:            1,
			ApprovalTokenHash: hashSecret("token"),
			Channel:           "web",
			Status:            domain.AgentApprovalStatusPending,
			ExpiresAt:         now.Add(time.Hour),
			Metadata:          domain.AgentJSON{"summary": "调整通知配置"},
		},
	}
	service := NewAgentApprovalService(&store, WithAgentApprovalNow(func() time.Time { return now }))

	detail, err := service.Get(context.Background(), 1, "token")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if detail.Status != "pending" {
		t.Fatalf("Status = %q, want pending", detail.Status)
	}

	decided, err := service.Decide(context.Background(), 1, "token", AgentApprovalDecisionInput{Decision: "approve"})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if decided.Status != "approved" {
		t.Fatalf("Decide status = %q, want approved", decided.Status)
	}
	if store.planID != planID || store.plan != domain.AgentPlanStatusApproved {
		t.Fatalf("plan status = (%d, %q), want (%d, approved)", store.planID, store.plan, planID)
	}
}

func TestAgentApprovalServiceMarksExpiredDetail(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	store := fakeAgentApprovalStore{
		approval: domain.AgentApproval{
			ID:                10,
			UserID:            1,
			ApprovalTokenHash: hashSecret("expired"),
			Channel:           "web",
			Status:            domain.AgentApprovalStatusPending,
			ExpiresAt:         now.Add(-time.Minute),
		},
	}
	service := NewAgentApprovalService(&store, WithAgentApprovalNow(func() time.Time { return now }))

	detail, err := service.Get(context.Background(), 1, "expired")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if detail.Status != "expired" {
		t.Fatalf("Status = %q, want expired", detail.Status)
	}
}

func TestAgentApprovalServiceDecideByIDRecordsAudit(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	planID := int64(22)
	store := fakeAgentApprovalStore{
		approval: domain.AgentApproval{
			ID:        9,
			PlanID:    &planID,
			UserID:    1,
			Channel:   "web",
			Status:    domain.AgentApprovalStatusPending,
			ExpiresAt: now.Add(time.Hour),
			RequestID: "request-1",
			TraceID:   "trace-1",
		},
	}
	service := NewAgentApprovalService(&store, WithAgentApprovalNow(func() time.Time { return now }))

	decided, err := service.DecideByID(context.Background(), 1, 9, AgentApprovalDecisionInput{Decision: "reject"})
	if err != nil {
		t.Fatalf("DecideByID() error = %v", err)
	}
	if decided.Status != "rejected" || store.plan != domain.AgentPlanStatusRejected {
		t.Fatalf("decision = %#v, plan = %q", decided, store.plan)
	}
	if len(store.audits) != 1 || store.audits[0].EventType != "agent.approval_decided" || store.audits[0].Status != "rejected" {
		t.Fatalf("audits = %#v", store.audits)
	}
}

type fakeAgentApprovalStore struct {
	approval domain.AgentApproval
	planID   int64
	plan     domain.AgentPlanStatus
	audits   []domain.AgentAuditLog
}

func (s *fakeAgentApprovalStore) GetByTokenHash(ctx context.Context, userID int64, tokenHash string) (domain.AgentApproval, error) {
	if s.approval.UserID != userID || s.approval.ApprovalTokenHash != tokenHash {
		return domain.AgentApproval{}, domain.ErrNotFound
	}
	return s.approval, nil
}

func (s *fakeAgentApprovalStore) GetByID(ctx context.Context, userID int64, approvalID int64) (domain.AgentApproval, error) {
	if s.approval.UserID != userID || s.approval.ID != approvalID {
		return domain.AgentApproval{}, domain.ErrNotFound
	}
	return s.approval, nil
}

func (s *fakeAgentApprovalStore) Decide(ctx context.Context, userID int64, tokenHash string, status domain.AgentApprovalStatus, now time.Time) (domain.AgentApproval, error) {
	if s.approval.UserID != userID ||
		s.approval.ApprovalTokenHash != tokenHash ||
		s.approval.Status != domain.AgentApprovalStatusPending ||
		!now.Before(s.approval.ExpiresAt) {
		return domain.AgentApproval{}, domain.ErrNotFound
	}
	s.approval.Status = status
	s.approval.DecidedAt = &now
	return s.approval, nil
}

func (s *fakeAgentApprovalStore) DecideByID(ctx context.Context, userID int64, approvalID int64, status domain.AgentApprovalStatus, now time.Time) (domain.AgentApproval, error) {
	if s.approval.UserID != userID ||
		s.approval.ID != approvalID ||
		s.approval.Status != domain.AgentApprovalStatusPending ||
		!now.Before(s.approval.ExpiresAt) {
		return domain.AgentApproval{}, domain.ErrNotFound
	}
	s.approval.Status = status
	s.approval.DecidedAt = &now
	return s.approval, nil
}

func (s *fakeAgentApprovalStore) UpdateAgentPlanStatusForApproval(_ context.Context, userID int64, planID int64, status domain.AgentPlanStatus, _ time.Time) error {
	if s.approval.UserID != userID || s.approval.PlanID == nil || *s.approval.PlanID != planID {
		return domain.ErrNotFound
	}
	s.planID = planID
	s.plan = status
	return nil
}

func (s *fakeAgentApprovalStore) CreateAuditLog(_ context.Context, log domain.AgentAuditLog) (domain.AgentAuditLog, error) {
	log.ID = int64(len(s.audits) + 1)
	s.audits = append(s.audits, log)
	return log, nil
}
