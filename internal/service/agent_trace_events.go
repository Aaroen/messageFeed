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
	metadata := domain.AgentJSON{
		"decision":            decision,
		"risk_level":          riskLevel,
		"plan_status":         string(plan.Status),
		"policy_decision":     plan.PolicyDecision,
		"policy_reason":       plan.PolicyReason,
		"confirmation_policy": plan.ConfirmationPolicy,
	}
	if policy := agentPlanWebFallbackPolicy(plan); policy != nil {
		metadata["web_fallback_policy"] = policy
	}
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
		Metadata:   metadata,
	})
}

func agentPlanWebFallbackPolicy(plan domain.AgentPlan) domain.AgentJSON {
	mainPlan := metadataMap(plan.Metadata, "main_agent_plan")
	if mainPlan == nil {
		return nil
	}
	plannerMetadata := agentTraceNestedMetadataMap(mainPlan, "metadata")
	if plannerMetadata == nil {
		return nil
	}
	policy := agentTraceNestedMetadataMap(plannerMetadata, "web_fallback_policy")
	if policy == nil {
		return nil
	}
	cloned := make(domain.AgentJSON, len(policy))
	for key, value := range policy {
		cloned[key] = value
	}
	return cloned
}

func agentTraceNestedMetadataMap(metadata map[string]any, key string) map[string]any {
	if metadata == nil {
		return nil
	}
	raw := metadata[key]
	if typed, ok := raw.(map[string]any); ok {
		return typed
	}
	if typed, ok := raw.(domain.AgentJSON); ok {
		return map[string]any(typed)
	}
	return nil
}

func (s *AgentConversationService) recordAgentNotificationTraceEvent(ctx context.Context, input ReceiveWeChatWorkAppMessageInput, account domain.ExternalAccount, session domain.AgentSession, turn domain.AgentTurn, plan domain.AgentPlan, eventName string, status domain.AgentTraceEventStatus, sendCount int, err error, extraMetadata ...domain.AgentJSON) {
	now := s.now().UTC()
	metadata := domain.AgentJSON{
		"provider":   input.Provider,
		"msg_type":   input.MsgType,
		"send_count": sendCount,
	}
	for _, extra := range extraMetadata {
		for key, value := range extra {
			metadata[key] = value
		}
	}
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
		Metadata:   metadata,
	}
	if err != nil {
		event.ErrorCode = "agent_notification_failed"
		event.ErrorMessage = err.Error()
	}
	s.recordAgentTraceEvent(ctx, event)
}
