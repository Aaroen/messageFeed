package repository

import (
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestAlertRuleModelRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 23, 19, 0, 0, 0, time.UTC)
	rule := domain.AlertRule{
		ID:              1,
		UserID:          2,
		Name:            "AI infra",
		Scope:           domain.AlertRuleScopeKeyword,
		Condition:       domain.AlertRuleCondition{Keywords: []string{"OpenAI", "PostgreSQL"}},
		MinImportance:   0.7,
		AIRequired:      true,
		CooldownSeconds: 3600,
		Channel:         "ntfy",
		Enabled:         true,
		LastTriggeredAt: &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	model := alertRuleModelFromDomain(rule)
	converted := alertRuleModelToDomain(model)

	if converted.Scope != rule.Scope {
		t.Fatalf("Scope = %q, want %q", converted.Scope, rule.Scope)
	}
	if converted.Condition.Keywords[0] != "OpenAI" {
		t.Fatalf("Keywords = %#v", converted.Condition.Keywords)
	}
	if !converted.AIRequired {
		t.Fatal("AIRequired = false, want true")
	}
	if converted.LastTriggeredAt == nil || !converted.LastTriggeredAt.Equal(now) {
		t.Fatalf("LastTriggeredAt = %#v, want %s", converted.LastTriggeredAt, now)
	}
}

func TestAlertRuleConditionIsCopied(t *testing.T) {
	rule := domain.AlertRule{Condition: domain.AlertRuleCondition{Keywords: []string{"one"}}}

	model := alertRuleModelFromDomain(rule)
	model.Condition.Keywords[0] = "changed"
	if rule.Condition.Keywords[0] != "one" {
		t.Fatalf("source condition was mutated: %#v", rule.Condition)
	}

	converted := alertRuleModelToDomain(model)
	converted.Condition.Keywords[0] = "converted"
	if model.Condition.Keywords[0] != "changed" {
		t.Fatalf("model condition was mutated: %#v", model.Condition)
	}
}
