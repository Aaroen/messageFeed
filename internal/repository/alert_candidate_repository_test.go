package repository

import (
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestAlertCandidateModelRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 23, 19, 30, 0, 0, time.UTC)
	candidate := domain.AlertCandidate{
		ID:             1,
		UserID:         2,
		RuleID:         3,
		ItemEventID:    4,
		SourceID:       5,
		ItemID:         6,
		Status:         domain.AlertCandidateStatusPendingAnalysis,
		MatchedReasons: []string{"keyword matched: openai"},
		DedupeKey:      "alert_candidate:3:6",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	model := alertCandidateModelFromDomain(candidate)
	converted := alertCandidateModelToDomain(model)

	if converted.Status != candidate.Status {
		t.Fatalf("Status = %q, want %q", converted.Status, candidate.Status)
	}
	if converted.DedupeKey != candidate.DedupeKey {
		t.Fatalf("DedupeKey = %q, want %q", converted.DedupeKey, candidate.DedupeKey)
	}
	if converted.MatchedReasons[0] != candidate.MatchedReasons[0] {
		t.Fatalf("MatchedReasons = %#v", converted.MatchedReasons)
	}
}

func TestAlertCandidateReasonsAreCopied(t *testing.T) {
	candidate := domain.AlertCandidate{MatchedReasons: []string{"one"}}

	model := alertCandidateModelFromDomain(candidate)
	model.MatchedReasons[0] = "changed"
	if candidate.MatchedReasons[0] != "one" {
		t.Fatalf("source reasons were mutated: %#v", candidate.MatchedReasons)
	}

	converted := alertCandidateModelToDomain(model)
	converted.MatchedReasons[0] = "converted"
	if model.MatchedReasons[0] != "changed" {
		t.Fatalf("model reasons were mutated: %#v", model.MatchedReasons)
	}
}
