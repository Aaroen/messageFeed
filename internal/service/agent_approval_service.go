package service

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/observability"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

type AgentApprovalStore interface {
	GetByTokenHash(ctx context.Context, userID int64, tokenHash string) (domain.AgentApproval, error)
	Decide(ctx context.Context, userID int64, tokenHash string, status domain.AgentApprovalStatus, now time.Time) (domain.AgentApproval, error)
}

type AgentApprovalService struct {
	store AgentApprovalStore
	now   func() time.Time
}

type AgentApprovalServiceOption func(*AgentApprovalService)

func WithAgentApprovalNow(now func() time.Time) AgentApprovalServiceOption {
	return func(service *AgentApprovalService) {
		if now != nil {
			service.now = now
		}
	}
}

func NewAgentApprovalService(store AgentApprovalStore, options ...AgentApprovalServiceOption) *AgentApprovalService {
	service := &AgentApprovalService{
		store: store,
		now:   time.Now,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

type AgentApprovalDetail struct {
	ID        int64            `json:"id"`
	PlanID    *int64           `json:"plan_id,omitempty"`
	Status    string           `json:"status"`
	Channel   string           `json:"channel"`
	ExpiresAt string           `json:"expires_at"`
	DecidedAt string           `json:"decided_at,omitempty"`
	Metadata  domain.AgentJSON `json:"metadata"`
}

type AgentApprovalDecisionInput struct {
	Decision string
}

func (s *AgentApprovalService) Get(ctx context.Context, userID int64, token string) (AgentApprovalDetail, error) {
	if s == nil || s.store == nil {
		return AgentApprovalDetail{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_approval_unavailable", "agent approval service is unavailable", "service.agent_approval.get", false, nil)
	}
	if userID < 1 || strings.TrimSpace(token) == "" {
		return AgentApprovalDetail{}, fmt.Errorf("%w: user id and token are required", domain.ErrInvalidInput)
	}
	ctx, span := observability.StartSpan(ctx, "service.agent_approval.get")
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	approval, err := s.store.GetByTokenHash(ctx, userID, hashSecret(token))
	if err != nil {
		opErr = err
		return AgentApprovalDetail{}, err
	}
	span.SetAttributes(attribute.Int64("agent.approval_id", approval.ID))
	return s.detail(approval), nil
}

func (s *AgentApprovalService) Decide(ctx context.Context, userID int64, token string, input AgentApprovalDecisionInput) (AgentApprovalDetail, error) {
	if s == nil || s.store == nil {
		return AgentApprovalDetail{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_approval_unavailable", "agent approval service is unavailable", "service.agent_approval.decide", false, nil)
	}
	if userID < 1 || strings.TrimSpace(token) == "" {
		return AgentApprovalDetail{}, fmt.Errorf("%w: user id and token are required", domain.ErrInvalidInput)
	}
	decision := domain.AgentApprovalDecision(strings.TrimSpace(input.Decision))
	if !decision.Valid() {
		return AgentApprovalDetail{}, fmt.Errorf("%w: unsupported approval decision", domain.ErrInvalidInput)
	}
	ctx, span := observability.StartSpan(ctx, "service.agent_approval.decide",
		attribute.String("agent.approval_decision", string(decision)),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	status := domain.AgentApprovalStatusApproved
	if decision == domain.AgentApprovalDecisionReject {
		status = domain.AgentApprovalStatusRejected
	}
	approval, err := s.store.Decide(ctx, userID, hashSecret(token), status, s.now().UTC())
	if err != nil {
		if domain.ClassifyError(err) == domain.ErrorKindNotFound {
			opErr = domain.NewAppError(domain.ErrorKindConflict, "agent_approval_not_pending", "approval is not pending or has expired", "service.agent_approval.decide", false, err)
			return AgentApprovalDetail{}, opErr
		}
		opErr = err
		return AgentApprovalDetail{}, err
	}
	span.SetAttributes(attribute.Int64("agent.approval_id", approval.ID))
	return s.detail(approval), nil
}

func (s *AgentApprovalService) detail(approval domain.AgentApproval) AgentApprovalDetail {
	status := approval.Status
	if status == domain.AgentApprovalStatusPending && !approval.ExpiresAt.IsZero() && !s.now().UTC().Before(approval.ExpiresAt.UTC()) {
		status = domain.AgentApprovalStatusExpired
	}
	detail := AgentApprovalDetail{
		ID:        approval.ID,
		PlanID:    approval.PlanID,
		Status:    string(status),
		Channel:   approval.Channel,
		ExpiresAt: approval.ExpiresAt.UTC().Format(time.RFC3339),
		Metadata:  cloneApprovalMetadata(approval.Metadata),
	}
	if approval.DecidedAt != nil {
		detail.DecidedAt = approval.DecidedAt.UTC().Format(time.RFC3339)
	}
	return detail
}

func cloneApprovalMetadata(input domain.AgentJSON) domain.AgentJSON {
	if input == nil {
		return domain.AgentJSON{}
	}
	output := make(domain.AgentJSON, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
