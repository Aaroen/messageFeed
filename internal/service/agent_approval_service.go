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
	GetByID(ctx context.Context, userID int64, approvalID int64) (domain.AgentApproval, error)
	Decide(ctx context.Context, userID int64, tokenHash string, status domain.AgentApprovalStatus, now time.Time) (domain.AgentApproval, error)
	DecideByID(ctx context.Context, userID int64, approvalID int64, status domain.AgentApprovalStatus, now time.Time) (domain.AgentApproval, error)
	UpdateAgentPlanStatusForApproval(ctx context.Context, userID int64, planID int64, status domain.AgentPlanStatus, now time.Time) error
	CreateAuditLog(ctx context.Context, log domain.AgentAuditLog) (domain.AgentAuditLog, error)
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

func (s *AgentApprovalService) DecideByID(ctx context.Context, userID int64, approvalID int64, input AgentApprovalDecisionInput) (AgentApprovalDetail, error) {
	if s == nil || s.store == nil {
		return AgentApprovalDetail{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_approval_unavailable", "agent approval service is unavailable", "service.agent_approval.decide_by_id", false, nil)
	}
	if userID < 1 || approvalID < 1 {
		return AgentApprovalDetail{}, fmt.Errorf("%w: user id and approval id are required", domain.ErrInvalidInput)
	}
	decision := domain.AgentApprovalDecision(strings.TrimSpace(input.Decision))
	if !decision.Valid() {
		return AgentApprovalDetail{}, fmt.Errorf("%w: unsupported approval decision", domain.ErrInvalidInput)
	}
	status := domain.AgentApprovalStatusApproved
	if decision == domain.AgentApprovalDecisionReject {
		status = domain.AgentApprovalStatusRejected
	}
	approval, err := s.store.DecideByID(ctx, userID, approvalID, status, s.now().UTC())
	if err != nil {
		if domain.ClassifyError(err) == domain.ErrorKindNotFound {
			return AgentApprovalDetail{}, domain.NewAppError(domain.ErrorKindConflict, "agent_approval_not_pending", "approval is not pending or has expired", "service.agent_approval.decide_by_id", false, err)
		}
		return AgentApprovalDetail{}, err
	}
	if err := s.applyPlanDecision(ctx, approval, status); err != nil {
		return AgentApprovalDetail{}, err
	}
	s.recordDecisionAudit(ctx, approval, status)
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
	if approval.PlanID != nil && *approval.PlanID > 0 {
		if err := s.applyPlanDecision(ctx, approval, status); err != nil {
			opErr = err
			return AgentApprovalDetail{}, err
		}
	}
	s.recordDecisionAudit(ctx, approval, status)
	span.SetAttributes(attribute.Int64("agent.approval_id", approval.ID))
	return s.detail(approval), nil
}

func (s *AgentApprovalService) applyPlanDecision(ctx context.Context, approval domain.AgentApproval, status domain.AgentApprovalStatus) error {
	if approval.PlanID == nil || *approval.PlanID < 1 {
		return nil
	}
	planStatus := domain.AgentPlanStatusApproved
	if status == domain.AgentApprovalStatusRejected {
		planStatus = domain.AgentPlanStatusRejected
	}
	return s.store.UpdateAgentPlanStatusForApproval(ctx, approval.UserID, *approval.PlanID, planStatus, s.now().UTC())
}

func (s *AgentApprovalService) recordDecisionAudit(ctx context.Context, approval domain.AgentApproval, status domain.AgentApprovalStatus) {
	planID := int64(0)
	if approval.PlanID != nil {
		planID = *approval.PlanID
	}
	_, _ = s.store.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    approval.UserID,
		EventType: "agent.approval_decided",
		Status:    string(status),
		Message:   "agent approval decision recorded",
		Metadata: domain.AgentJSON{
			"approval_id": approval.ID,
			"plan_id":     planID,
			"channel":     approval.Channel,
		},
		RequestID: approval.RequestID,
		TraceID:   approval.TraceID,
		CreatedAt: s.now().UTC(),
	})
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
