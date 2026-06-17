package repository

import (
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestItemModelRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 17, 9, 0, 0, 0, time.UTC)
	item := domain.Item{
		ID:             3,
		SourceID:       2,
		Title:          "Item",
		URL:            "https://example.com/item",
		NormalizedURL:  "https://example.com/item",
		RawGUID:        "guid-1",
		ContentHash:    "hash",
		Summary:        "summary",
		ContentSnippet: "snippet",
		Author:         "author",
		PublishedAt:    &now,
		FetchedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	model := itemModelFromDomain(item)
	converted := itemModelToDomain(model)

	if converted.ID != item.ID {
		t.Fatalf("ID = %d, want %d", converted.ID, item.ID)
	}
	if converted.SourceID != item.SourceID {
		t.Fatalf("SourceID = %d, want %d", converted.SourceID, item.SourceID)
	}
	if converted.NormalizedURL != item.NormalizedURL {
		t.Fatalf("NormalizedURL = %q, want %q", converted.NormalizedURL, item.NormalizedURL)
	}
	if converted.RawGUID != item.RawGUID {
		t.Fatalf("RawGUID = %q, want %q", converted.RawGUID, item.RawGUID)
	}
	if converted.PublishedAt == nil || !converted.PublishedAt.Equal(now) {
		t.Fatalf("PublishedAt = %#v, want %s", converted.PublishedAt, now)
	}
}

func TestItemViewModelToDomainIncludesSourceAndState(t *testing.T) {
	now := time.Date(2026, 6, 17, 9, 0, 0, 0, time.UTC)
	model := itemViewModel{
		ID:          3,
		SourceID:    2,
		SourceName:  "Go Blog",
		Title:       "Item",
		URL:         "https://example.com/item",
		FetchedAt:   now,
		IsRead:      true,
		ReadAt:      &now,
		IsFavorite:  true,
		FavoritedAt: &now,
		IsHidden:    true,
		HiddenAt:    &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	item := itemViewModelToDomain(model)

	if item.SourceName != model.SourceName {
		t.Fatalf("SourceName = %q, want %q", item.SourceName, model.SourceName)
	}
	if !item.IsRead || !item.IsFavorite || !item.IsHidden {
		t.Fatalf("state flags = read:%v favorite:%v hidden:%v", item.IsRead, item.IsFavorite, item.IsHidden)
	}
	if item.ReadAt == nil || !item.ReadAt.Equal(now) {
		t.Fatalf("ReadAt = %#v, want %s", item.ReadAt, now)
	}
}
