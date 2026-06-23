package service

import (
	"context"
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestAlertPolicyEngineAllowsReadyCandidateWithoutAI(t *testing.T) {
	engine := NewAlertPolicyEngine()
	rule := alertPolicyTestRule()
	rule.AIRequired = false
	candidate := alertPolicyTestCandidate(rule)

	decision, err := engine.Evaluate(context.Background(), EvaluateAlertPolicyInput{
		Rule:      rule,
		Candidate: candidate,
	})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}

	if decision.Status != domain.AlertPolicyDecisionStatusAllow {
		t.Fatalf("Status = %q, want allow", decision.Status)
	}
	if !decision.AutoNotify {
		t.Fatal("AutoNotify = false, want true")
	}
	if decision.Channel != "ntfy" {
		t.Fatalf("Channel = %q, want ntfy", decision.Channel)
	}
}

func TestAlertPolicyEngineWaitsForRequiredAIAnalysis(t *testing.T) {
	engine := NewAlertPolicyEngine()
	rule := alertPolicyTestRule()
	rule.AIRequired = true
	candidate := alertPolicyTestCandidate(rule)
	candidate.Status = domain.AlertCandidateStatusPendingAnalysis

	decision, err := engine.Evaluate(context.Background(), EvaluateAlertPolicyInput{
		Rule:      rule,
		Candidate: candidate,
	})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}

	if decision.Status != domain.AlertPolicyDecisionStatusPendingAnalysis {
		t.Fatalf("Status = %q, want pending_analysis", decision.Status)
	}
	if decision.AutoNotify {
		t.Fatal("AutoNotify = true, want false")
	}
}

func TestAlertPolicyEngineSuppressesBelowImportanceThreshold(t *testing.T) {
	engine := NewAlertPolicyEngine()
	rule := alertPolicyTestRule()
	rule.AIRequired = true
	rule.MinImportance = 0.8
	candidate := alertPolicyTestCandidate(rule)
	analysis := domain.AIAnalysisResult{
		ShouldNotify:   true,
		Importance:     0.7,
		Confidence:     0.9,
		RiskLevel:      domain.AIAnalysisRiskLevelLow,
		MatchedReasons: []string{"keyword matched"},
	}

	decision, err := engine.Evaluate(context.Background(), EvaluateAlertPolicyInput{
		Rule:      rule,
		Candidate: candidate,
		Analysis:  &analysis,
	})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}

	if decision.Status != domain.AlertPolicyDecisionStatusSuppressed {
		t.Fatalf("Status = %q, want suppressed", decision.Status)
	}
	if decision.AutoNotify {
		t.Fatal("AutoNotify = true, want false")
	}
	if got, want := decision.Reasons[len(decision.Reasons)-1], "importance below rule threshold"; got != want {
		t.Fatalf("last reason = %q, want %q", got, want)
	}
}

func TestAlertPolicyEngineRequiresConfirmationBelowConfidenceThreshold(t *testing.T) {
	engine := NewAlertPolicyEngine(WithAlertPolicyMinConfidence(0.75))
	rule := alertPolicyTestRule()
	rule.AIRequired = true
	candidate := alertPolicyTestCandidate(rule)
	analysis := domain.AIAnalysisResult{
		ShouldNotify: true,
		Importance:   0.9,
		Confidence:   0.6,
		RiskLevel:    domain.AIAnalysisRiskLevelLow,
	}

	decision, err := engine.Evaluate(context.Background(), EvaluateAlertPolicyInput{
		Rule:      rule,
		Candidate: candidate,
		Analysis:  &analysis,
	})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}

	if decision.Status != domain.AlertPolicyDecisionStatusRequiresConfirmation {
		t.Fatalf("Status = %q, want requires_confirmation", decision.Status)
	}
	if !decision.RequiresConfirmation {
		t.Fatal("RequiresConfirmation = false, want true")
	}
	if decision.AutoNotify {
		t.Fatal("AutoNotify = true, want false")
	}
}

func TestAlertPolicyEngineRequiresConfirmationForHighRiskAnalysis(t *testing.T) {
	engine := NewAlertPolicyEngine()
	rule := alertPolicyTestRule()
	rule.AIRequired = true
	candidate := alertPolicyTestCandidate(rule)
	analysis := domain.AIAnalysisResult{
		ShouldNotify: true,
		Importance:   0.9,
		Confidence:   0.9,
		RiskLevel:    domain.AIAnalysisRiskLevelHigh,
	}

	decision, err := engine.Evaluate(context.Background(), EvaluateAlertPolicyInput{
		Rule:      rule,
		Candidate: candidate,
		Analysis:  &analysis,
	})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}

	if decision.Status != domain.AlertPolicyDecisionStatusRequiresConfirmation {
		t.Fatalf("Status = %q, want requires_confirmation", decision.Status)
	}
	if got, want := decision.Reasons[len(decision.Reasons)-1], "high risk analysis requires confirmation"; got != want {
		t.Fatalf("last reason = %q, want %q", got, want)
	}
}

func TestAlertPolicyEngineSuppressesCooldownAndDuplicateNotifications(t *testing.T) {
	now := time.Date(2026, 6, 24, 11, 0, 0, 0, time.UTC)
	lastTriggeredAt := now.Add(-time.Minute)
	engine := NewAlertPolicyEngine(WithAlertPolicyNow(func() time.Time { return now }))
	rule := alertPolicyTestRule()
	rule.CooldownSeconds = 3600
	rule.LastTriggeredAt = &lastTriggeredAt
	candidate := alertPolicyTestCandidate(rule)

	decision, err := engine.Evaluate(context.Background(), EvaluateAlertPolicyInput{
		Rule:      rule,
		Candidate: candidate,
		Now:       now,
	})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if decision.Status != domain.AlertPolicyDecisionStatusSuppressed {
		t.Fatalf("cooldown Status = %q, want suppressed", decision.Status)
	}
	if got, want := decision.Reasons[0], "rule cooldown active"; got != want {
		t.Fatalf("cooldown reason = %q, want %q", got, want)
	}

	rule.LastTriggeredAt = nil
	decision, err = engine.Evaluate(context.Background(), EvaluateAlertPolicyInput{
		Rule:            rule,
		Candidate:       candidate,
		AlreadyNotified: true,
		Now:             now,
	})
	if err != nil {
		t.Fatalf("duplicate Evaluate returned error: %v", err)
	}
	if got, want := decision.Reasons[0], "duplicate notification"; got != want {
		t.Fatalf("duplicate reason = %q, want %q", got, want)
	}
}

func alertPolicyTestRule() domain.AlertRule {
	return domain.AlertRule{
		ID:      10,
		UserID:  1,
		Name:    "Policy",
		Scope:   domain.AlertRuleScopeKeyword,
		Channel: "ntfy",
		Enabled: true,
	}
}

func alertPolicyTestCandidate(rule domain.AlertRule) domain.AlertCandidate {
	return domain.AlertCandidate{
		ID:        20,
		UserID:    rule.UserID,
		RuleID:    rule.ID,
		ItemID:    30,
		Status:    domain.AlertCandidateStatusReady,
		DedupeKey: "alert_candidate:10:30",
	}
}
