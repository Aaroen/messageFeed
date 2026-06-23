package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/observability"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const (
	aiFeedSourceName          = "messageFeed AI"
	aiFeedSourceURL           = "messagefeed://ai/internal"
	aiFeedContentSnippetLimit = 2000
)

type AIFeedSourceRepository interface {
	Create(ctx context.Context, source domain.Source) (domain.Source, error)
	ListByUser(ctx context.Context, userID int64) ([]domain.Source, error)
}

type AIFeedItemRepository interface {
	UpsertMany(ctx context.Context, items []domain.Item) (domain.ItemUpsertResult, error)
}

type AIFeedService struct {
	sourceRepository AIFeedSourceRepository
	itemRepository   AIFeedItemRepository
	now              func() time.Time
}

type AIFeedServiceOption func(*AIFeedService)

func WithAIFeedNow(now func() time.Time) AIFeedServiceOption {
	return func(service *AIFeedService) {
		if now != nil {
			service.now = now
		}
	}
}

func NewAIFeedService(sourceRepository AIFeedSourceRepository, itemRepository AIFeedItemRepository, options ...AIFeedServiceOption) *AIFeedService {
	service := &AIFeedService{
		sourceRepository: sourceRepository,
		itemRepository:   itemRepository,
		now:              time.Now,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

type PublishAIFeedEntryInput struct {
	UserID      int64
	Kind        domain.AIFeedEntryKind
	Title       string
	Summary     string
	Content     string
	DedupeKey   string
	PublishedAt time.Time
}

type PublishAIFeedEntryResult struct {
	Source       domain.Source
	UpsertResult domain.ItemUpsertResult
}

func (s *AIFeedService) PublishEntry(ctx context.Context, input PublishAIFeedEntryInput) (PublishAIFeedEntryResult, error) {
	ctx, span := observability.StartSpan(ctx, "service.ai_feed.publish_entry",
		attribute.Int64("user.id", input.UserID),
		attribute.String("ai_feed.kind", string(input.Kind)),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.sourceRepository == nil || s.itemRepository == nil {
		opErr = fmt.Errorf("ai feed service is not configured")
		return PublishAIFeedEntryResult{}, opErr
	}
	input = normalizeAIFeedEntryInput(input)
	if err := validateAIFeedEntryInput(input); err != nil {
		opErr = err
		return PublishAIFeedEntryResult{}, opErr
	}

	source, err := s.ensureAIFeedSource(ctx, input.UserID)
	if err != nil {
		opErr = err
		return PublishAIFeedEntryResult{}, opErr
	}

	now := s.now().UTC()
	publishedAt := input.PublishedAt
	if publishedAt.IsZero() {
		publishedAt = now
	} else {
		publishedAt = publishedAt.UTC()
	}
	item := domain.Item{
		SourceID:       source.ID,
		SourceName:     source.Name,
		Title:          input.Title,
		URL:            aiFeedEntryURL(input.Kind, input.DedupeKey),
		NormalizedURL:  aiFeedEntryURL(input.Kind, input.DedupeKey),
		RawGUID:        aiFeedEntryRawGUID(input.Kind, input.DedupeKey),
		ContentHash:    aiFeedEntryContentHash(input),
		Summary:        input.Summary,
		ContentSnippet: truncateError(input.Content, aiFeedContentSnippetLimit),
		Author:         aiFeedSourceName,
		PublishedAt:    &publishedAt,
		FetchedAt:      now,
	}
	upsertResult, err := s.itemRepository.UpsertMany(ctx, []domain.Item{item})
	if err != nil {
		opErr = err
		return PublishAIFeedEntryResult{}, opErr
	}

	span.SetAttributes(
		attribute.Int("ai_feed.items_created", upsertResult.CreatedCount),
		attribute.Int("ai_feed.items_updated", upsertResult.UpdatedCount),
	)
	return PublishAIFeedEntryResult{
		Source:       source,
		UpsertResult: upsertResult,
	}, nil
}

func (s *AIFeedService) ensureAIFeedSource(ctx context.Context, userID int64) (domain.Source, error) {
	source, found, err := s.findAIFeedSource(ctx, userID)
	if err != nil {
		return domain.Source{}, err
	}
	if found {
		return source, nil
	}

	source, err = s.sourceRepository.Create(ctx, domain.Source{
		UserID:               userID,
		Name:                 aiFeedSourceName,
		Type:                 domain.SourceTypeInternal,
		URL:                  aiFeedSourceURL,
		NormalizedURL:        aiFeedSourceURL,
		Status:               domain.SourceStatusActive,
		FetchIntervalSeconds: 0,
		Tags:                 []string{"ai", "internal"},
		Weight:               100,
	})
	if err == nil {
		return source, nil
	}
	if !errors.Is(err, domain.ErrConflict) {
		return domain.Source{}, err
	}

	source, found, err = s.findAIFeedSource(ctx, userID)
	if err != nil {
		return domain.Source{}, err
	}
	if !found {
		return domain.Source{}, domain.ErrConflict
	}
	return source, nil
}

func (s *AIFeedService) findAIFeedSource(ctx context.Context, userID int64) (domain.Source, bool, error) {
	sources, err := s.sourceRepository.ListByUser(ctx, userID)
	if err != nil {
		return domain.Source{}, false, err
	}
	for _, source := range sources {
		if source.UserID == userID && source.Type == domain.SourceTypeInternal && source.NormalizedURL == aiFeedSourceURL {
			return source, true, nil
		}
	}
	return domain.Source{}, false, nil
}

func normalizeAIFeedEntryInput(input PublishAIFeedEntryInput) PublishAIFeedEntryInput {
	input.Title = strings.TrimSpace(input.Title)
	input.Summary = strings.TrimSpace(input.Summary)
	input.Content = strings.TrimSpace(input.Content)
	input.DedupeKey = strings.TrimSpace(input.DedupeKey)
	return input
}

func validateAIFeedEntryInput(input PublishAIFeedEntryInput) error {
	if input.UserID < 1 {
		return fmt.Errorf("%w: user id must be positive", domain.ErrInvalidInput)
	}
	if !input.Kind.Valid() {
		return fmt.Errorf("%w: ai feed entry kind is invalid", domain.ErrInvalidInput)
	}
	if input.Title == "" {
		return fmt.Errorf("%w: title must not be empty", domain.ErrInvalidInput)
	}
	if input.DedupeKey == "" {
		return fmt.Errorf("%w: dedupe key must not be empty", domain.ErrInvalidInput)
	}
	return nil
}

func aiFeedEntryURL(kind domain.AIFeedEntryKind, dedupeKey string) string {
	return "messagefeed://ai/" + string(kind) + "/" + url.PathEscape(dedupeKey)
}

func aiFeedEntryRawGUID(kind domain.AIFeedEntryKind, dedupeKey string) string {
	return string(kind) + ":" + dedupeKey
}

func aiFeedEntryContentHash(input PublishAIFeedEntryInput) string {
	hash := sha256.Sum256([]byte(string(input.Kind) + "\n" + input.Title + "\n" + input.Summary + "\n" + input.Content))
	return hex.EncodeToString(hash[:])
}
