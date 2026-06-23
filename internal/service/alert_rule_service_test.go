package service

import (
	"context"
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestAlertRuleServiceProcessItemEventCreatesReadyCandidateForKeywordRule(t *testing.T) {
	now := time.Date(2026, 6, 23, 20, 0, 0, 0, time.UTC)
	ruleStore := &fakeAlertRuleStore{
		rules: []domain.AlertRule{
			{
				ID:      10,
				UserID:  1,
				Name:    "AI infra",
				Scope:   domain.AlertRuleScopeKeyword,
				Enabled: true,
				Condition: domain.AlertRuleCondition{
					Keywords: []string{"OpenAI"},
				},
			},
		},
	}
	candidateStore := &fakeAlertCandidateStore{}
	service := NewAlertRuleService(
		ruleStore,
		candidateStore,
		WithAlertRuleNow(func() time.Time { return now }),
	)

	result, err := service.ProcessItemEvent(context.Background(), ProcessItemEventInput{
		Event: domain.ItemEvent{
			ID:        20,
			UserID:    1,
			SourceID:  30,
			ItemID:    40,
			EventType: domain.ItemEventTypeItemCreated,
			Payload: domain.ItemEventPayload{
				"title": "OpenAI releases a PostgreSQL connector",
				"url":   "https://example.com/openai-postgresql",
			},
		},
	})
	if err != nil {
		t.Fatalf("ProcessItemEvent returned error: %v", err)
	}

	if result.RuleCount != 1 || result.CandidateCount != 1 {
		t.Fatalf("result = %#v, want one rule and one candidate", result)
	}
	if got, want := len(candidateStore.candidates), 1; got != want {
		t.Fatalf("candidate count = %d, want %d", got, want)
	}
	candidate := candidateStore.candidates[0]
	if candidate.Status != domain.AlertCandidateStatusReady {
		t.Fatalf("Status = %q, want %q", candidate.Status, domain.AlertCandidateStatusReady)
	}
	if candidate.DedupeKey != "alert_candidate:10:40" {
		t.Fatalf("DedupeKey = %q, want alert_candidate:10:40", candidate.DedupeKey)
	}
	if got, want := candidate.MatchedReasons[0], "keyword matched: openai"; got != want {
		t.Fatalf("MatchedReasons[0] = %q, want %q", got, want)
	}
}

func TestAlertRuleServiceProcessItemEventCreatesPendingAnalysisCandidateForAIRule(t *testing.T) {
	now := time.Date(2026, 6, 23, 20, 30, 0, 0, time.UTC)
	ruleStore := &fakeAlertRuleStore{
		rules: []domain.AlertRule{
			{
				ID:         11,
				UserID:     1,
				Name:       "Important source",
				Scope:      domain.AlertRuleScopeSource,
				AIRequired: true,
				Enabled:    true,
				Condition: domain.AlertRuleCondition{
					SourceIDs: []int64{31},
				},
			},
		},
	}
	candidateStore := &fakeAlertCandidateStore{}
	service := NewAlertRuleService(
		ruleStore,
		candidateStore,
		WithAlertRuleNow(func() time.Time { return now }),
	)

	result, err := service.ProcessItemEvent(context.Background(), ProcessItemEventInput{
		Event: domain.ItemEvent{
			ID:        21,
			UserID:    1,
			SourceID:  31,
			ItemID:    41,
			EventType: domain.ItemEventTypeItemCreated,
		},
	})
	if err != nil {
		t.Fatalf("ProcessItemEvent returned error: %v", err)
	}

	if result.CandidateCount != 1 {
		t.Fatalf("CandidateCount = %d, want 1", result.CandidateCount)
	}
	if got, want := candidateStore.candidates[0].Status, domain.AlertCandidateStatusPendingAnalysis; got != want {
		t.Fatalf("Status = %q, want %q", got, want)
	}
	if got, want := candidateStore.candidates[0].MatchedReasons[0], "source matched"; got != want {
		t.Fatalf("MatchedReasons[0] = %q, want %q", got, want)
	}
}

func TestAlertRuleServiceProcessItemEventSkipsRuleInCooldown(t *testing.T) {
	now := time.Date(2026, 6, 23, 21, 0, 0, 0, time.UTC)
	lastTriggeredAt := now.Add(-time.Minute)
	ruleStore := &fakeAlertRuleStore{
		rules: []domain.AlertRule{
			{
				ID:              12,
				UserID:          1,
				Name:            "Cooldown",
				Scope:           domain.AlertRuleScopeKeyword,
				Condition:       domain.AlertRuleCondition{Keywords: []string{"PostgreSQL"}},
				CooldownSeconds: 3600,
				Enabled:         true,
				LastTriggeredAt: &lastTriggeredAt,
			},
		},
	}
	candidateStore := &fakeAlertCandidateStore{}
	service := NewAlertRuleService(
		ruleStore,
		candidateStore,
		WithAlertRuleNow(func() time.Time { return now }),
	)

	result, err := service.ProcessItemEvent(context.Background(), ProcessItemEventInput{
		Event: domain.ItemEvent{
			ID:        22,
			UserID:    1,
			SourceID:  32,
			ItemID:    42,
			EventType: domain.ItemEventTypeItemCreated,
			Payload:   domain.ItemEventPayload{"title": "PostgreSQL release notes"},
		},
	})
	if err != nil {
		t.Fatalf("ProcessItemEvent returned error: %v", err)
	}

	if result.RuleCount != 1 || result.CandidateCount != 0 {
		t.Fatalf("result = %#v, want one rule and zero candidates", result)
	}
	if got, want := len(candidateStore.candidates), 0; got != want {
		t.Fatalf("candidate count = %d, want %d", got, want)
	}
}

func TestAlertRuleServiceProcessItemEventIgnoresDuplicateCandidate(t *testing.T) {
	ruleStore := &fakeAlertRuleStore{
		rules: []domain.AlertRule{
			{
				ID:        13,
				UserID:    1,
				Name:      "Duplicate",
				Scope:     domain.AlertRuleScopeGlobal,
				Enabled:   true,
				Condition: domain.AlertRuleCondition{},
			},
		},
	}
	candidateStore := &fakeAlertCandidateStore{err: domain.ErrConflict}
	service := NewAlertRuleService(ruleStore, candidateStore)

	result, err := service.ProcessItemEvent(context.Background(), ProcessItemEventInput{
		Event: domain.ItemEvent{
			ID:        23,
			UserID:    1,
			SourceID:  33,
			ItemID:    43,
			EventType: domain.ItemEventTypeItemCreated,
		},
	})
	if err != nil {
		t.Fatalf("ProcessItemEvent returned error: %v", err)
	}

	if result.RuleCount != 1 || result.CandidateCount != 0 {
		t.Fatalf("result = %#v, want one rule and zero created candidates", result)
	}
	if got, want := len(candidateStore.candidates), 0; got != want {
		t.Fatalf("candidate count = %d, want %d", got, want)
	}
}

func TestAlertRuleServiceProcessItemEventIgnoresNonItemCreatedEvent(t *testing.T) {
	ruleStore := &fakeAlertRuleStore{
		rules: []domain.AlertRule{
			{ID: 14, UserID: 1, Scope: domain.AlertRuleScopeGlobal, Enabled: true},
		},
	}
	candidateStore := &fakeAlertCandidateStore{}
	service := NewAlertRuleService(ruleStore, candidateStore)

	result, err := service.ProcessItemEvent(context.Background(), ProcessItemEventInput{
		Event: domain.ItemEvent{
			ID:        24,
			UserID:    1,
			SourceID:  33,
			EventType: domain.ItemEventTypeSourceFetchFailed,
		},
	})
	if err != nil {
		t.Fatalf("ProcessItemEvent returned error: %v", err)
	}

	if result.RuleCount != 0 || result.CandidateCount != 0 {
		t.Fatalf("result = %#v, want zero result", result)
	}
	if ruleStore.listCalls != 0 {
		t.Fatalf("ListEnabledByUser calls = %d, want 0", ruleStore.listCalls)
	}
	if got, want := len(candidateStore.candidates), 0; got != want {
		t.Fatalf("candidate count = %d, want %d", got, want)
	}
}

func TestStringSlicePayloadValueSupportsStringAndAnySlices(t *testing.T) {
	payload := domain.ItemEventPayload{
		"tags":    []string{"database", "ai"},
		"tickers": []any{"AAPL", 123, "MSFT"},
	}

	tags := stringSlicePayloadValue(payload, "tags")
	if got, want := len(tags), 2; got != want {
		t.Fatalf("tags length = %d, want %d", got, want)
	}
	if tags[0] != "database" || tags[1] != "ai" {
		t.Fatalf("tags = %#v, want database and ai", tags)
	}

	tickers := stringSlicePayloadValue(payload, "tickers")
	if got, want := len(tickers), 2; got != want {
		t.Fatalf("tickers length = %d, want %d", got, want)
	}
	if tickers[0] != "AAPL" || tickers[1] != "MSFT" {
		t.Fatalf("tickers = %#v, want AAPL and MSFT", tickers)
	}
}

type fakeAlertRuleStore struct {
	rules     []domain.AlertRule
	listCalls int
}

func (s *fakeAlertRuleStore) ListEnabledByUser(_ context.Context, userID int64) ([]domain.AlertRule, error) {
	s.listCalls++
	rules := make([]domain.AlertRule, 0, len(s.rules))
	for _, rule := range s.rules {
		if rule.UserID == userID && rule.Enabled {
			rules = append(rules, rule)
		}
	}
	return rules, nil
}

type fakeAlertCandidateStore struct {
	candidates []domain.AlertCandidate
	err        error
}

func (s *fakeAlertCandidateStore) Create(_ context.Context, candidate domain.AlertCandidate) (domain.AlertCandidate, error) {
	if s.err != nil {
		return domain.AlertCandidate{}, s.err
	}
	candidate.ID = int64(len(s.candidates) + 1)
	s.candidates = append(s.candidates, candidate)
	return candidate, nil
}
