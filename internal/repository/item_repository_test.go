package repository

import (
	"messagefeed/internal/domain"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
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

func TestItemModelFromDomainSanitizesInvalidUTF8(t *testing.T) {
	invalidSuffix := string([]byte{0xe6, 0xb5})
	item := domain.Item{
		SourceID:       2,
		Title:          "标题" + invalidSuffix,
		URL:            "https://example.com/item" + invalidSuffix,
		NormalizedURL:  "https://example.com/item" + invalidSuffix,
		RawGUID:        "guid" + invalidSuffix,
		ContentHash:    "hash" + invalidSuffix,
		Summary:        "summary" + invalidSuffix,
		ContentSnippet: "snippet" + invalidSuffix,
		Author:         "author" + invalidSuffix,
		FetchedAt:      time.Date(2026, 6, 17, 9, 0, 0, 0, time.UTC),
	}

	model := itemModelFromDomain(item)
	values := map[string]string{
		"title":           model.Title,
		"url":             model.URL,
		"normalized_url":  model.NormalizedURL,
		"raw_guid":        model.RawGUID,
		"content_hash":    model.ContentHash,
		"summary":         model.Summary,
		"content_snippet": model.ContentSnippet,
		"author":          model.Author,
	}
	for name, value := range values {
		if !utf8.ValidString(value) {
			t.Fatalf("%s contains invalid UTF-8: %q", name, value)
		}
	}
	if model.Title != "标题" {
		t.Fatalf("Title = %q, want sanitized title", model.Title)
	}
	if model.RawGUID != "guid" {
		t.Fatalf("RawGUID = %q, want sanitized guid", model.RawGUID)
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

func TestAppendItemUpsertResultTracksCreatedAndUpdatedItems(t *testing.T) {
	result := domain.ItemUpsertResult{TotalCount: 2}

	appendItemUpsertResult(&result, domain.Item{ID: 1, Title: "Created"}, true)
	appendItemUpsertResult(&result, domain.Item{ID: 2, Title: "Updated"}, false)

	if result.CreatedCount != 1 {
		t.Fatalf("CreatedCount = %d, want 1", result.CreatedCount)
	}
	if result.UpdatedCount != 1 {
		t.Fatalf("UpdatedCount = %d, want 1", result.UpdatedCount)
	}
	if got, want := len(result.CreatedItems), 1; got != want {
		t.Fatalf("CreatedItems length = %d, want %d", got, want)
	}
	if got, want := len(result.UpdatedItems), 1; got != want {
		t.Fatalf("UpdatedItems length = %d, want %d", got, want)
	}
	if result.CreatedItems[0].ID != 1 {
		t.Fatalf("CreatedItems[0].ID = %d, want 1", result.CreatedItems[0].ID)
	}
	if result.UpdatedItems[0].ID != 2 {
		t.Fatalf("UpdatedItems[0].ID = %d, want 2", result.UpdatedItems[0].ID)
	}
}

func TestItemSourceFiltersKeepGlobalTimelineActiveOnly(t *testing.T) {
	if !strings.Contains(activeSourceStatusFilter, "sources.status") {
		t.Fatalf("active source filter = %q, want sources.status predicate", activeSourceStatusFilter)
	}
	if got, want := string(domain.SourceStatusActive), "active"; got != want {
		t.Fatalf("active source status = %q, want %q", got, want)
	}
}

func TestItemSourceSpecificListAllowsInactiveSources(t *testing.T) {
	if itemSourceFilterRequiresActive(domain.ItemListOptions{SourceID: 7}) {
		t.Fatal("source-specific item list should not require active source status")
	}
	if !itemSourceFilterRequiresActive(domain.ItemListOptions{}) {
		t.Fatal("global item list should require active source status")
	}
}
