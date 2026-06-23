package repository

import (
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestAIAnalysisJobModelRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 24, 9, 0, 0, 0, time.UTC)
	finishedAt := now.Add(2 * time.Second)
	job := domain.AIAnalysisJob{
		ID:               1,
		UserID:           2,
		AlertCandidateID: 3,
		SourceID:         4,
		ItemID:           5,
		Status:           domain.AIAnalysisJobStatusSucceeded,
		Input: domain.AIAnalysisJobInput{
			"title": "OpenAI infrastructure update",
		},
		Result: domain.AIAnalysisResult{
			ShouldNotify:   true,
			Importance:     0.86,
			MatchedReasons: []string{"source is high priority", "matches keyword"},
			Summary:        "OpenAI published an infrastructure update.",
			RiskLevel:      domain.AIAnalysisRiskLevelLow,
			Confidence:     0.78,
		},
		ScheduledAt:  now,
		StartedAt:    &now,
		FinishedAt:   &finishedAt,
		AttemptCount: 1,
		MaxAttempts:  3,
		LockedBy:     "worker-a",
		LockedAt:     &now,
		CreatedAt:    now,
		UpdatedAt:    finishedAt,
	}

	model := aiAnalysisJobModelFromDomain(job)
	converted := aiAnalysisJobModelToDomain(model)

	if converted.Status != job.Status {
		t.Fatalf("Status = %q, want %q", converted.Status, job.Status)
	}
	if converted.Input["title"] != job.Input["title"] {
		t.Fatalf("Input title = %#v, want %#v", converted.Input["title"], job.Input["title"])
	}
	if !converted.Result.ShouldNotify {
		t.Fatal("ShouldNotify = false, want true")
	}
	if converted.Result.RiskLevel != domain.AIAnalysisRiskLevelLow {
		t.Fatalf("RiskLevel = %q, want low", converted.Result.RiskLevel)
	}
	if got, want := converted.Result.MatchedReasons[1], "matches keyword"; got != want {
		t.Fatalf("MatchedReasons[1] = %q, want %q", got, want)
	}
	if converted.FinishedAt == nil || !converted.FinishedAt.Equal(finishedAt) {
		t.Fatalf("FinishedAt = %#v, want %s", converted.FinishedAt, finishedAt)
	}
}

func TestAIAnalysisJobPayloadsAreCopied(t *testing.T) {
	job := domain.AIAnalysisJob{
		Input: domain.AIAnalysisJobInput{
			"title": "one",
		},
		Result: domain.AIAnalysisResult{
			MatchedReasons: []string{"one"},
		},
	}

	model := aiAnalysisJobModelFromDomain(job)
	model.Input["title"] = "changed"
	model.Result.MatchedReasons[0] = "changed"
	if job.Input["title"] != "one" {
		t.Fatalf("source input was mutated: %#v", job.Input)
	}
	if job.Result.MatchedReasons[0] != "one" {
		t.Fatalf("source result was mutated: %#v", job.Result.MatchedReasons)
	}

	converted := aiAnalysisJobModelToDomain(model)
	converted.Input["title"] = "converted"
	converted.Result.MatchedReasons[0] = "converted"
	if model.Input["title"] != "changed" {
		t.Fatalf("model input was mutated: %#v", model.Input)
	}
	if model.Result.MatchedReasons[0] != "changed" {
		t.Fatalf("model result was mutated: %#v", model.Result.MatchedReasons)
	}
}

func TestNormalizeAIAnalysisJobClaimInput(t *testing.T) {
	now := time.Date(2026, 6, 24, 10, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	input := normalizeAIAnalysisJobClaimInput(domain.AIAnalysisJobClaimInput{
		Now:      now,
		WorkerID: " worker-a ",
		Limit:    maxAIAnalysisJobClaimLimit + 1,
	})

	if input.WorkerID != "worker-a" {
		t.Fatalf("WorkerID = %q, want worker-a", input.WorkerID)
	}
	if input.Limit != maxAIAnalysisJobClaimLimit {
		t.Fatalf("Limit = %d, want %d", input.Limit, maxAIAnalysisJobClaimLimit)
	}
	if input.Now.Location() != time.UTC {
		t.Fatalf("Now location = %s, want UTC", input.Now.Location())
	}

	defaulted := normalizeAIAnalysisJobClaimInput(domain.AIAnalysisJobClaimInput{})
	if defaulted.WorkerID != "unknown" {
		t.Fatalf("default WorkerID = %q, want unknown", defaulted.WorkerID)
	}
	if defaulted.Limit != defaultAIAnalysisJobClaimLimit {
		t.Fatalf("default Limit = %d, want %d", defaulted.Limit, defaultAIAnalysisJobClaimLimit)
	}
}

func TestNormalizeAIAnalysisJobListOptions(t *testing.T) {
	options := normalizeAIAnalysisJobListOptions(domain.AIAnalysisJobListOptions{
		UserID: 1,
		Limit:  maxAIAnalysisJobListLimit + 1,
		Offset: -1,
	})

	if options.Limit != maxAIAnalysisJobListLimit {
		t.Fatalf("Limit = %d, want %d", options.Limit, maxAIAnalysisJobListLimit)
	}
	if options.Offset != 0 {
		t.Fatalf("Offset = %d, want 0", options.Offset)
	}

	defaulted := normalizeAIAnalysisJobListOptions(domain.AIAnalysisJobListOptions{UserID: 1})
	if defaulted.Limit != defaultAIAnalysisJobListLimit {
		t.Fatalf("default Limit = %d, want %d", defaulted.Limit, defaultAIAnalysisJobListLimit)
	}
}
