package repository

import (
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestNotificationJobModelRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	finishedAt := now.Add(time.Second)
	job := domain.NotificationJob{
		ID:               1,
		UserID:           2,
		AlertCandidateID: 3,
		AlertRuleID:      4,
		AIAnalysisJobID:  5,
		SourceID:         6,
		ItemID:           7,
		Status:           domain.NotificationJobStatusSucceeded,
		Channel:          domain.NotificationChannelNtfy,
		PolicyDecision: domain.AlertPolicyDecision{
			Status:     domain.AlertPolicyDecisionStatusAllow,
			AutoNotify: true,
			Reasons:    []string{"policy allowed notification"},
			Channel:    "ntfy",
			Importance: 0.9,
			Confidence: 0.8,
		},
		Payload: domain.NotificationPayload{
			"title": "Important update",
		},
		RequestID:    "request-1",
		TraceID:      "trace-1",
		DedupeKey:    "notification:3",
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

	model := notificationJobModelFromDomain(job)
	converted := notificationJobModelToDomain(model)

	if converted.Status != job.Status {
		t.Fatalf("Status = %q, want %q", converted.Status, job.Status)
	}
	if converted.Channel != domain.NotificationChannelNtfy {
		t.Fatalf("Channel = %q, want ntfy", converted.Channel)
	}
	if !converted.PolicyDecision.AutoNotify {
		t.Fatal("AutoNotify = false, want true")
	}
	if converted.PolicyDecision.Reasons[0] != "policy allowed notification" {
		t.Fatalf("Reasons = %#v", converted.PolicyDecision.Reasons)
	}
	if converted.Payload["title"] != "Important update" {
		t.Fatalf("Payload title = %#v, want Important update", converted.Payload["title"])
	}
	if converted.FinishedAt == nil || !converted.FinishedAt.Equal(finishedAt) {
		t.Fatalf("FinishedAt = %#v, want %s", converted.FinishedAt, finishedAt)
	}
}

func TestNotificationDeliveryModelRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 30, 0, 0, time.UTC)
	responseStatus := 200
	delivery := domain.NotificationDelivery{
		ID:                1,
		NotificationJobID: 2,
		UserID:            3,
		Channel:           domain.NotificationChannelWeChatWork,
		Status:            domain.NotificationDeliveryStatusSucceeded,
		RequestID:         "request-2",
		TraceID:           "trace-2",
		ProviderMessageID: "provider-1",
		ResponseStatus:    &responseStatus,
		ResponseBody:      `{"ok":true}`,
		SentAt:            &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	model := notificationDeliveryModelFromDomain(delivery)
	converted := notificationDeliveryModelToDomain(model)

	if converted.NotificationJobID != delivery.NotificationJobID {
		t.Fatalf("NotificationJobID = %d, want %d", converted.NotificationJobID, delivery.NotificationJobID)
	}
	if converted.Channel != domain.NotificationChannelWeChatWork {
		t.Fatalf("Channel = %q, want wechat_work", converted.Channel)
	}
	if converted.ResponseStatus == nil || *converted.ResponseStatus != responseStatus {
		t.Fatalf("ResponseStatus = %#v, want %d", converted.ResponseStatus, responseStatus)
	}
	if converted.SentAt == nil || !converted.SentAt.Equal(now) {
		t.Fatalf("SentAt = %#v, want %s", converted.SentAt, now)
	}
}

func TestNotificationPayloadsAreCopied(t *testing.T) {
	job := domain.NotificationJob{
		PolicyDecision: domain.AlertPolicyDecision{
			Reasons: []string{"one"},
		},
		Payload: domain.NotificationPayload{
			"title": "one",
		},
	}

	model := notificationJobModelFromDomain(job)
	model.PolicyDecision.Reasons[0] = "changed"
	model.Payload["title"] = "changed"
	if job.PolicyDecision.Reasons[0] != "one" {
		t.Fatalf("source reasons were mutated: %#v", job.PolicyDecision.Reasons)
	}
	if job.Payload["title"] != "one" {
		t.Fatalf("source payload was mutated: %#v", job.Payload)
	}

	converted := notificationJobModelToDomain(model)
	converted.PolicyDecision.Reasons[0] = "converted"
	converted.Payload["title"] = "converted"
	if model.PolicyDecision.Reasons[0] != "changed" {
		t.Fatalf("model reasons were mutated: %#v", model.PolicyDecision.Reasons)
	}
	if model.Payload["title"] != "changed" {
		t.Fatalf("model payload was mutated: %#v", model.Payload)
	}
}

func TestNormalizeNotificationJobClaimInput(t *testing.T) {
	now := time.Date(2026, 6, 24, 13, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	input := normalizeNotificationJobClaimInput(domain.NotificationJobClaimInput{
		Now:      now,
		WorkerID: " worker-a ",
		Limit:    maxNotificationJobClaimLimit + 1,
	})

	if input.WorkerID != "worker-a" {
		t.Fatalf("WorkerID = %q, want worker-a", input.WorkerID)
	}
	if input.Limit != maxNotificationJobClaimLimit {
		t.Fatalf("Limit = %d, want %d", input.Limit, maxNotificationJobClaimLimit)
	}
	if input.Now.Location() != time.UTC {
		t.Fatalf("Now location = %s, want UTC", input.Now.Location())
	}

	defaulted := normalizeNotificationJobClaimInput(domain.NotificationJobClaimInput{})
	if defaulted.WorkerID != "unknown" {
		t.Fatalf("default WorkerID = %q, want unknown", defaulted.WorkerID)
	}
	if defaulted.Limit != defaultNotificationJobClaimLimit {
		t.Fatalf("default Limit = %d, want %d", defaulted.Limit, defaultNotificationJobClaimLimit)
	}
}

func TestNormalizeNotificationListOptions(t *testing.T) {
	jobOptions := normalizeNotificationJobListOptions(domain.NotificationJobListOptions{
		UserID: 1,
		Limit:  maxNotificationListLimit + 1,
		Offset: -1,
	})
	if jobOptions.Limit != maxNotificationListLimit {
		t.Fatalf("job Limit = %d, want %d", jobOptions.Limit, maxNotificationListLimit)
	}
	if jobOptions.Offset != 0 {
		t.Fatalf("job Offset = %d, want 0", jobOptions.Offset)
	}

	deliveryOptions := normalizeNotificationDeliveryListOptions(domain.NotificationDeliveryListOptions{
		UserID: 1,
		JobID:  2,
		Limit:  maxNotificationListLimit + 1,
		Offset: -1,
	})
	if deliveryOptions.Limit != maxNotificationListLimit {
		t.Fatalf("delivery Limit = %d, want %d", deliveryOptions.Limit, maxNotificationListLimit)
	}
	if deliveryOptions.Offset != 0 {
		t.Fatalf("delivery Offset = %d, want 0", deliveryOptions.Offset)
	}
}
