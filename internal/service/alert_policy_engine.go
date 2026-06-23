package service

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/observability"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const defaultAlertPolicyMinConfidence = 0.5

type AlertPolicyEngine struct {
	minConfidence                  float64
	requireConfirmationForHighRisk bool
	now                            func() time.Time
}

type AlertPolicyEngineOption func(*AlertPolicyEngine)

func WithAlertPolicyMinConfidence(minConfidence float64) AlertPolicyEngineOption {
	return func(engine *AlertPolicyEngine) {
		if minConfidence >= 0 && minConfidence <= 1 {
			engine.minConfidence = minConfidence
		}
	}
}

func WithAlertPolicyHighRiskConfirmation(enabled bool) AlertPolicyEngineOption {
	return func(engine *AlertPolicyEngine) {
		engine.requireConfirmationForHighRisk = enabled
	}
}

func WithAlertPolicyNow(now func() time.Time) AlertPolicyEngineOption {
	return func(engine *AlertPolicyEngine) {
		if now != nil {
			engine.now = now
		}
	}
}

func NewAlertPolicyEngine(options ...AlertPolicyEngineOption) *AlertPolicyEngine {
	engine := &AlertPolicyEngine{
		minConfidence:                  defaultAlertPolicyMinConfidence,
		requireConfirmationForHighRisk: true,
		now:                            time.Now,
	}
	for _, option := range options {
		option(engine)
	}
	return engine
}

type EvaluateAlertPolicyInput struct {
	Rule            domain.AlertRule
	Candidate       domain.AlertCandidate
	Analysis        *domain.AIAnalysisResult
	AlreadyNotified bool
	Now             time.Time
}

func (e *AlertPolicyEngine) Evaluate(ctx context.Context, input EvaluateAlertPolicyInput) (domain.AlertPolicyDecision, error) {
	ctx, span := observability.StartSpan(ctx, "service.alert_policy.evaluate",
		attribute.Int64("user.id", input.Candidate.UserID),
		attribute.Int64("alert_rule.id", input.Rule.ID),
		attribute.Int64("alert_candidate.id", input.Candidate.ID),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if e == nil {
		opErr = fmt.Errorf("alert policy engine is not configured")
		return domain.AlertPolicyDecision{}, opErr
	}
	if input.Rule.ID < 1 {
		opErr = fmt.Errorf("%w: alert rule id must be positive", domain.ErrInvalidInput)
		return domain.AlertPolicyDecision{}, opErr
	}
	if input.Candidate.ID < 1 {
		opErr = fmt.Errorf("%w: alert candidate id must be positive", domain.ErrInvalidInput)
		return domain.AlertPolicyDecision{}, opErr
	}
	if input.Rule.UserID != input.Candidate.UserID || input.Rule.ID != input.Candidate.RuleID {
		opErr = fmt.Errorf("%w: alert rule and candidate mismatch", domain.ErrInvalidInput)
		return domain.AlertPolicyDecision{}, opErr
	}

	now := input.Now
	if now.IsZero() {
		now = e.now().UTC()
	} else {
		now = now.UTC()
	}

	decision := domain.AlertPolicyDecision{
		Status:  domain.AlertPolicyDecisionStatusAllow,
		Channel: input.Rule.Channel,
	}
	if !input.Rule.Enabled {
		return suppressAlertPolicyDecision(decision, "rule disabled"), nil
	}
	if input.Candidate.Status == domain.AlertCandidateStatusSuppressed {
		return suppressAlertPolicyDecision(decision, "candidate suppressed"), nil
	}
	if input.AlreadyNotified {
		return suppressAlertPolicyDecision(decision, "duplicate notification"), nil
	}
	if alertRuleInCooldown(input.Rule, now) {
		return suppressAlertPolicyDecision(decision, "rule cooldown active"), nil
	}
	if input.Rule.AIRequired && input.Analysis == nil {
		decision.Status = domain.AlertPolicyDecisionStatusPendingAnalysis
		decision.Reasons = append(decision.Reasons, "ai analysis required")
		return decision, nil
	}
	if input.Analysis != nil {
		decision.Importance = input.Analysis.Importance
		decision.Confidence = input.Analysis.Confidence
		decision.Reasons = append(decision.Reasons, input.Analysis.MatchedReasons...)
		if !input.Analysis.ShouldNotify {
			return suppressAlertPolicyDecision(decision, "ai declined notification"), nil
		}
		if input.Analysis.Importance < input.Rule.MinImportance {
			return suppressAlertPolicyDecision(decision, "importance below rule threshold"), nil
		}
		if input.Analysis.Confidence < e.minConfidence {
			decision.Status = domain.AlertPolicyDecisionStatusRequiresConfirmation
			decision.RequiresConfirmation = true
			decision.Reasons = append(decision.Reasons, "confidence below auto notification threshold")
			return decision, nil
		}
		if e.requireConfirmationForHighRisk && input.Analysis.RiskLevel == domain.AIAnalysisRiskLevelHigh {
			decision.Status = domain.AlertPolicyDecisionStatusRequiresConfirmation
			decision.RequiresConfirmation = true
			decision.Reasons = append(decision.Reasons, "high risk analysis requires confirmation")
			return decision, nil
		}
	}

	decision.AutoNotify = true
	if len(decision.Reasons) == 0 {
		decision.Reasons = append(decision.Reasons, "policy allowed notification")
	}
	span.SetAttributes(
		attribute.String("alert_policy.status", string(decision.Status)),
		attribute.Bool("alert_policy.auto_notify", decision.AutoNotify),
	)
	return decision, nil
}

func alertRuleInCooldown(rule domain.AlertRule, now time.Time) bool {
	return rule.CooldownSeconds > 0 && rule.LastTriggeredAt != nil && now.Sub(*rule.LastTriggeredAt) < time.Duration(rule.CooldownSeconds)*time.Second
}

func suppressAlertPolicyDecision(decision domain.AlertPolicyDecision, reason string) domain.AlertPolicyDecision {
	decision.Status = domain.AlertPolicyDecisionStatusSuppressed
	decision.AutoNotify = false
	decision.RequiresConfirmation = false
	decision.Reasons = append(decision.Reasons, reason)
	return decision
}
