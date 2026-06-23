package repository

import (
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestItemEventModelRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	processedAt := now.Add(time.Second)
	event := domain.ItemEvent{
		ID:           5,
		UserID:       1,
		SourceID:     2,
		ItemID:       3,
		EventType:    domain.ItemEventTypeItemCreated,
		Status:       domain.ItemEventStatusProcessed,
		Payload:      domain.ItemEventPayload{"title": "Item", "importance": 0.8},
		DedupeKey:    "item.created:3",
		AvailableAt:  now,
		ProcessedAt:  &processedAt,
		AttemptCount: 1,
		CreatedAt:    now,
		UpdatedAt:    processedAt,
	}

	model := itemEventModelFromDomain(event)
	converted := itemEventModelToDomain(model)

	if converted.ID != event.ID {
		t.Fatalf("ID = %d, want %d", converted.ID, event.ID)
	}
	if converted.EventType != event.EventType {
		t.Fatalf("EventType = %q, want %q", converted.EventType, event.EventType)
	}
	if converted.Status != event.Status {
		t.Fatalf("Status = %q, want %q", converted.Status, event.Status)
	}
	if converted.DedupeKey != event.DedupeKey {
		t.Fatalf("DedupeKey = %q, want %q", converted.DedupeKey, event.DedupeKey)
	}
	if converted.Payload["title"] != "Item" {
		t.Fatalf("Payload title = %#v, want Item", converted.Payload["title"])
	}
	if converted.ProcessedAt == nil || !converted.ProcessedAt.Equal(processedAt) {
		t.Fatalf("ProcessedAt = %#v, want %s", converted.ProcessedAt, processedAt)
	}
}

func TestItemEventPayloadIsCopied(t *testing.T) {
	event := domain.ItemEvent{
		Payload: domain.ItemEventPayload{"title": "Item"},
	}

	model := itemEventModelFromDomain(event)
	model.Payload["title"] = "Changed"
	if event.Payload["title"] != "Item" {
		t.Fatalf("source payload was mutated: %#v", event.Payload)
	}

	converted := itemEventModelToDomain(model)
	converted.Payload["title"] = "Converted"
	if model.Payload["title"] != "Changed" {
		t.Fatalf("model payload was mutated: %#v", model.Payload)
	}
}

func TestNormalizeItemEventClaimInput(t *testing.T) {
	now := time.Date(2026, 6, 23, 13, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	input := normalizeItemEventClaimInput(domain.ItemEventClaimInput{
		Now:   now,
		Limit: maxItemEventClaimLimit + 1,
	})

	if input.Limit != maxItemEventClaimLimit {
		t.Fatalf("Limit = %d, want %d", input.Limit, maxItemEventClaimLimit)
	}
	if input.Now.Location() != time.UTC {
		t.Fatalf("Now location = %s, want UTC", input.Now.Location())
	}

	defaulted := normalizeItemEventClaimInput(domain.ItemEventClaimInput{})
	if defaulted.Limit != defaultItemEventClaimLimit {
		t.Fatalf("default Limit = %d, want %d", defaulted.Limit, defaultItemEventClaimLimit)
	}
	if defaulted.Now.IsZero() {
		t.Fatal("default Now is zero")
	}
}
