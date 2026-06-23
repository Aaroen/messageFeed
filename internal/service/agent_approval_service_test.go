package service

import (
	"context"
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestAgentApprovalServiceGetAndApprove(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	store := fakeAgentApprovalStore{
		approval: domain.AgentApproval{
			ID:                9,
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

type fakeAgentApprovalStore struct {
	approval domain.AgentApproval
}

func (s *fakeAgentApprovalStore) GetByTokenHash(ctx context.Context, userID int64, tokenHash string) (domain.AgentApproval, error) {
	if s.approval.UserID != userID || s.approval.ApprovalTokenHash != tokenHash {
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
