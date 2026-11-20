package repository

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestFeedVisibilityAndCursorPostgres(test *testing.T) {
	databaseURL := os.Getenv("FEED_TEST_DATABASE_URL")
	if databaseURL == "" {
		test.Skip("设置 FEED_TEST_DATABASE_URL 后执行 PostgreSQL 验收")
	}
	database, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		test.Fatal(err)
	}
	connection, err := database.DB()
	if err != nil {
		test.Fatal(err)
	}
	defer connection.Close()
	transaction := database.Begin()
	if transaction.Error != nil {
		test.Fatal(transaction.Error)
	}
	defer transaction.Rollback()
	stamp := time.Now().UnixNano()
	var ownerID, memberID, publicSource, privateSource, memberSource, catalogID int64
	for role, id := range map[string]*int64{"owner": &ownerID, "user": &memberID} {
		if err := transaction.Raw("INSERT INTO users (username, role) VALUES (?, ?) RETURNING id", fmt.Sprintf("feed-%s-%d", role, stamp), role).Scan(id).Error; err != nil {
			test.Fatal(err)
		}
	}
	publicURL := fmt.Sprintf("https://example.org/feed/%d", stamp)
	if err := transaction.Raw(`INSERT INTO source_catalog_entries (source_key, name, feed_url, normalized_url, official, license_status)
		VALUES (?, 'Public', ?, ?, true, 'allowed') RETURNING id`, publicURL, publicURL, publicURL).Scan(&catalogID).Error; err != nil {
		test.Fatal(err)
	}
	for _, source := range []struct {
		id    *int64
		owner int64
		url   string
	}{
		{&publicSource, ownerID, publicURL}, {&privateSource, ownerID, publicURL + "/private"}, {&memberSource, memberID, publicURL},
	} {
		if err := transaction.Raw("INSERT INTO sources (user_id, name, url, normalized_url) VALUES (?, 'Feed', ?, ?) RETURNING id", source.owner, source.url, source.url).Scan(source.id).Error; err != nil {
			test.Fatal(err)
		}
	}
	repo := NewItemRepository(transaction)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	items := []domain.Item{
		{SourceID: publicSource, Title: "First", URL: publicURL + "/1", NormalizedURL: publicURL + "/1", PublishedAt: &now, CreatedAt: now},
		{SourceID: publicSource, Title: "Second", URL: publicURL + "/2", NormalizedURL: publicURL + "/2", SourceUpdatedAt: &now, CreatedAt: now},
		{SourceID: publicSource, Title: "Internal", URL: "messagefeed://ai/agent_operation_log/internal", NormalizedURL: "messagefeed://ai/agent_operation_log/internal", CreatedAt: now},
		{SourceID: privateSource, Title: "Private", URL: publicURL + "/3", NormalizedURL: publicURL + "/3", CreatedAt: now},
		{SourceID: memberSource, Title: "Member", URL: publicURL + "/4", NormalizedURL: publicURL + "/4", CreatedAt: now},
	}
	created, err := repo.UpsertMany(ctx, items)
	if err != nil {
		test.Fatal(err)
	}
	for _, item := range created.CreatedItems[2:] {
		if _, err := repo.GetByIDPublic(ctx, item.ID); err != domain.ErrNotFound {
			test.Fatalf("public detail for %s returned %v", item.Title, err)
		}
	}
	first, err := repo.ListPublic(ctx, domain.ItemListOptions{SourceID: publicSource, Limit: 1, Cursor: &domain.ItemCursor{Snapshot: now}})
	if err != nil || first.Total != 2 || len(first.Items) != 1 || first.Items[0].CatalogID != catalogID {
		test.Fatalf("first page: %+v, %v", first, err)
	}
	newItem := domain.Item{SourceID: publicSource, Title: "New", URL: publicURL + "/5", NormalizedURL: publicURL + "/5", CreatedAt: now.Add(time.Second)}
	if _, err := repo.UpsertMany(ctx, []domain.Item{newItem}); err != nil {
		test.Fatal(err)
	}
	last := first.Items[0]
	second, err := repo.ListPublic(ctx, domain.ItemListOptions{SourceID: publicSource, Limit: 1, Cursor: &domain.ItemCursor{Snapshot: now, ID: last.ID, Time: last.FeedOrderTime()}})
	if err != nil || len(second.Items) != 1 || second.Items[0].Title != "First" {
		test.Fatalf("second page: %+v, %v", second, err)
	}
}

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

func TestGlobalItemListFiltersAgentOperationLogs(t *testing.T) {
	if got, want := hiddenAgentOperationLogURLPattern, "messagefeed://ai/agent_operation_log/%"; got != want {
		t.Fatalf("hidden agent operation log URL pattern = %q, want %q", got, want)
	}
}
