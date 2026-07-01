package service

import (
	"context"
	"messagefeed/internal/domain"
	"messagefeed/internal/metrics"
	"messagefeed/internal/observability"
	"strings"
	"time"
)

type agentTraceEventStore interface {
	CreateAgentTraceEvent(ctx context.Context, event domain.AgentTraceEvent) (domain.AgentTraceEvent, error)
}

func (s *AgentConversationService) recordAgentTraceEvent(ctx context.Context, event domain.AgentTraceEvent) {
	if s == nil || s.repository == nil {
		return
	}
	store, ok := any(s.repository).(agentTraceEventStore)
	if !ok {
		return
	}
	event = s.prepareAgentTraceEvent(ctx, event)
	if !event.EventKind.Valid() || !event.Status.Valid() {
		return
	}
	metrics.AgentTraceEventsTotal.WithLabelValues(string(event.EventKind), string(event.Status)).Inc()
	if event.DurationMS > 0 {
		metrics.AgentTraceEventDuration.WithLabelValues(string(event.EventKind), string(event.Status)).Observe(float64(event.DurationMS) / 1000)
	}
	_, _ = store.CreateAgentTraceEvent(ctx, event)
}

func (s *AgentConversationService) prepareAgentTraceEvent(ctx context.Context, event domain.AgentTraceEvent) domain.AgentTraceEvent {
	now := s.now().UTC()
	event.RequestID = strings.TrimSpace(event.RequestID)
	if event.RequestID == "" {
		event.RequestID = observability.RequestID(ctx)
	}
	event.TraceID = strings.TrimSpace(event.TraceID)
	if event.TraceID == "" {
		event.TraceID = observability.TraceID(ctx)
	}
	event.SpanID = strings.TrimSpace(event.SpanID)
	if event.SpanID == "" {
		event.SpanID = observability.SpanID(ctx)
	}
	event.EventName = strings.TrimSpace(event.EventName)
	event.ModelKey = strings.TrimSpace(event.ModelKey)
	event.CapabilityKey = strings.TrimSpace(event.CapabilityKey)
	event.ToolName = strings.TrimSpace(event.ToolName)
	event.JobID = strings.TrimSpace(event.JobID)
	event.InputSummary = safeSummary(event.InputSummary, 1000)
	event.OutputSummary = safeSummary(event.OutputSummary, 1000)
	event.ErrorCode = strings.TrimSpace(event.ErrorCode)
	event.ErrorMessage = safeSummary(event.ErrorMessage, 1000)
	if event.StartedAt.IsZero() {
		event.StartedAt = now
	}
	if event.FinishedAt != nil && event.DurationMS == 0 {
		event.DurationMS = event.FinishedAt.Sub(event.StartedAt).Milliseconds()
		if event.DurationMS < 0 {
			event.DurationMS = 0
		}
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}
	if event.Metadata == nil {
		event.Metadata = domain.AgentJSON{}
	}
	return event
}

func agentTraceStatusFromError(err error) domain.AgentTraceEventStatus {
	if err != nil {
		return domain.AgentTraceEventFailed
	}
	return domain.AgentTraceEventSucceeded
}

func agentTraceFinish(startedAt time.Time, now func() time.Time) (*time.Time, int64) {
	if now == nil {
		now = time.Now
	}
	finishedAt := now().UTC()
	durationMS := finishedAt.Sub(startedAt).Milliseconds()
	if durationMS < 0 {
		durationMS = 0
	}
	return &finishedAt, durationMS
}

func boolTraceLabel(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func (s *AgentConversationService) recordAgentApprovalTraceEvent(ctx context.Context, input ReceiveWeChatWorkAppMessageInput, account domain.ExternalAccount, session domain.AgentSession, turn domain.AgentTurn, run domain.AgentRun, plan domain.AgentPlan, decision string, status domain.AgentTraceEventStatus) {
	decision = strings.TrimSpace(decision)
	if decision == "" {
		decision = "unknown"
	}
	riskLevel := strings.TrimSpace(plan.RiskLevel)
	if riskLevel == "" {
		riskLevel = "unknown"
	}
	metrics.AgentApprovalsTotal.WithLabelValues(decision, riskLevel).Inc()
	now := s.now().UTC()
	s.recordAgentTraceEvent(ctx, domain.AgentTraceEvent{
		RequestID:  input.RequestID,
		TraceID:    input.TraceID,
		UserID:     account.UserID,
		SessionID:  session.ID,
		TurnID:     turn.ID,
		PlanID:     plan.ID,
		RunID:      run.ID,
		EventKind:  domain.AgentTraceEventApproval,
		EventName:  "plan_governance",
		Status:     status,
		StartedAt:  now,
		FinishedAt: &now,
		ModelKey:   run.ModelKey,
		Metadata: domain.AgentJSON{
			"decision":            decision,
			"risk_level":          riskLevel,
			"plan_status":         string(plan.Status),
			"policy_decision":     plan.PolicyDecision,
			"policy_reason":       plan.PolicyReason,
			"confirmation_policy": plan.ConfirmationPolicy,
		},
	})
}

func (s *AgentConversationService) recordAgentNotificationTraceEvent(ctx context.Context, input ReceiveWeChatWorkAppMessageInput, account domain.ExternalAccount, session domain.AgentSession, turn domain.AgentTurn, plan domain.AgentPlan, eventName string, status domain.AgentTraceEventStatus, sendCount int, err error) {
	now := s.now().UTC()
	event := domain.AgentTraceEvent{
		RequestID:  input.RequestID,
		TraceID:    input.TraceID,
		UserID:     account.UserID,
		SessionID:  session.ID,
		TurnID:     turn.ID,
		PlanID:     plan.ID,
		EventKind:  domain.AgentTraceEventNotification,
		EventName:  eventName,
		Status:     status,
		StartedAt:  now,
		FinishedAt: &now,
		Metadata: domain.AgentJSON{
			"provider":   input.Provider,
			"msg_type":   input.MsgType,
			"send_count": sendCount,
		},
	}
	if err != nil {
		event.ErrorCode = "agent_notification_failed"
		event.ErrorMessage = err.Error()
	}
	s.recordAgentTraceEvent(ctx, event)
}
