package service

import (
	"context"
	"errors"
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestAIFeedServicePublishEntryCreatesInternalSourceAndItem(t *testing.T) {
	now := time.Date(2026, 6, 24, 14, 0, 0, 0, time.UTC)
	sourceRepository := &fakeAIFeedSourceRepository{}
	itemRepository := &fakeAIFeedItemRepository{}
	service := NewAIFeedService(
		sourceRepository,
		itemRepository,
		WithAIFeedNow(func() time.Time { return now }),
	)

	result, err := service.PublishEntry(context.Background(), PublishAIFeedEntryInput{
		UserID:    1,
		Kind:      domain.AIFeedEntryKindAlertExplanation,
		Title:     "Important reminder",
		Summary:   "A short explanation.",
		Content:   "The rule and AI analysis support this reminder.",
		DedupeKey: "alert-candidate-10",
	})
	if err != nil {
		t.Fatalf("PublishEntry returned error: %v", err)
	}

	if result.Source.Type != domain.SourceTypeInternal {
		t.Fatalf("source type = %q, want internal", result.Source.Type)
	}
	if result.Source.NormalizedURL != aiFeedSourceURL {
		t.Fatalf("source normalized URL = %q, want %q", result.Source.NormalizedURL, aiFeedSourceURL)
	}
	if got, want := len(itemRepository.items), 1; got != want {
		t.Fatalf("items length = %d, want %d", got, want)
	}
	item := itemRepository.items[0]
	if item.SourceID != result.Source.ID {
		t.Fatalf("item SourceID = %d, want %d", item.SourceID, result.Source.ID)
	}
	if item.URL != "messagefeed://ai/alert_explanation/alert-candidate-10" {
		t.Fatalf("item URL = %q", item.URL)
	}
	if item.Author != aiFeedSourceName {
		t.Fatalf("item Author = %q, want %q", item.Author, aiFeedSourceName)
	}
	if item.PublishedAt == nil || !item.PublishedAt.Equal(now) {
		t.Fatalf("PublishedAt = %#v, want %s", item.PublishedAt, now)
	}
}

func TestAIFeedServicePublishEntryReusesExistingInternalSource(t *testing.T) {
	now := time.Date(2026, 6, 24, 14, 30, 0, 0, time.UTC)
	sourceRepository := &fakeAIFeedSourceRepository{
		sources: []domain.Source{
			{
				ID:            7,
				UserID:        1,
				Name:          aiFeedSourceName,
				Type:          domain.SourceTypeInternal,
				URL:           aiFeedSourceURL,
				NormalizedURL: aiFeedSourceURL,
				Status:        domain.SourceStatusActive,
			},
		},
	}
	itemRepository := &fakeAIFeedItemRepository{}
	service := NewAIFeedService(
		sourceRepository,
		itemRepository,
		WithAIFeedNow(func() time.Time { return now }),
	)

	result, err := service.PublishEntry(context.Background(), PublishAIFeedEntryInput{
		UserID:    1,
		Kind:      domain.AIFeedEntryKindDailySummary,
		Title:     "Daily summary",
		DedupeKey: "daily-2026-06-24",
	})
	if err != nil {
		t.Fatalf("PublishEntry returned error: %v", err)
	}

	if result.Source.ID != 7 {
		t.Fatalf("source ID = %d, want 7", result.Source.ID)
	}
	if sourceRepository.createCalls != 0 {
		t.Fatalf("Create calls = %d, want 0", sourceRepository.createCalls)
	}
}

func TestAIFeedServicePublishEntryReloadsSourceAfterCreateConflict(t *testing.T) {
	now := time.Date(2026, 6, 24, 15, 0, 0, 0, time.UTC)
	sourceRepository := &fakeAIFeedSourceRepository{
		createErr: domain.ErrConflict,
		afterConflictSources: []domain.Source{
			{
				ID:            8,
				UserID:        1,
				Name:          aiFeedSourceName,
				Type:          domain.SourceTypeInternal,
				URL:           aiFeedSourceURL,
				NormalizedURL: aiFeedSourceURL,
				Status:        domain.SourceStatusActive,
			},
		},
	}
	itemRepository := &fakeAIFeedItemRepository{}
	service := NewAIFeedService(
		sourceRepository,
		itemRepository,
		WithAIFeedNow(func() time.Time { return now }),
	)

	result, err := service.PublishEntry(context.Background(), PublishAIFeedEntryInput{
		UserID:    1,
		Kind:      domain.AIFeedEntryKindSourceHealth,
		Title:     "Source health report",
		DedupeKey: "source-health-1",
	})
	if err != nil {
		t.Fatalf("PublishEntry returned error: %v", err)
	}

	if result.Source.ID != 8 {
		t.Fatalf("source ID = %d, want 8", result.Source.ID)
	}
	if sourceRepository.listCalls != 2 {
		t.Fatalf("ListByUser calls = %d, want 2", sourceRepository.listCalls)
	}
}

func TestAIFeedServicePublishEntryValidatesInput(t *testing.T) {
	service := NewAIFeedService(&fakeAIFeedSourceRepository{}, &fakeAIFeedItemRepository{})

	_, err := service.PublishEntry(context.Background(), PublishAIFeedEntryInput{
		UserID:    1,
		Kind:      domain.AIFeedEntryKindAgentOperationLog,
		Title:     " ",
		DedupeKey: "operation-1",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want invalid input", err)
	}
}

type fakeAIFeedSourceRepository struct {
	sources              []domain.Source
	afterConflictSources []domain.Source
	createErr            error
	createCalls          int
	listCalls            int
	nextID               int64
}

func (r *fakeAIFeedSourceRepository) Create(_ context.Context, source domain.Source) (domain.Source, error) {
	r.createCalls++
	if r.createErr != nil {
		return domain.Source{}, r.createErr
	}
	if r.nextID == 0 {
		r.nextID = int64(len(r.sources) + 1)
	}
	source.ID = r.nextID
	r.nextID++
	r.sources = append(r.sources, source)
	return source, nil
}

func (r *fakeAIFeedSourceRepository) ListByUser(_ context.Context, userID int64) ([]domain.Source, error) {
	r.listCalls++
	sources := r.sources
	if r.createCalls > 0 && len(r.afterConflictSources) > 0 {
		sources = r.afterConflictSources
	}
	result := make([]domain.Source, 0, len(sources))
	for _, source := range sources {
		if source.UserID == userID {
			result = append(result, source)
		}
	}
	return result, nil
}

type fakeAIFeedItemRepository struct {
	items []domain.Item
}

func (r *fakeAIFeedItemRepository) UpsertMany(_ context.Context, items []domain.Item) (domain.ItemUpsertResult, error) {
	result := domain.ItemUpsertResult{TotalCount: len(items)}
	for _, item := range items {
		item.ID = int64(len(r.items) + 1)
		r.items = append(r.items, item)
		result.CreatedCount++
		result.CreatedItems = append(result.CreatedItems, item)
	}
	return result, nil
}
