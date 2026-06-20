package service

import (
	"context"
	"fmt"
	"hash/fnv"
	"math/rand"
	"messagefeed/internal/domain"
	"messagefeed/internal/observability"
	"net/url"
	"sort"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const (
	defaultRecommendationLimit      = 30
	maxRecommendationLimit          = 60
	recommendationCatalogLimit      = 200
	maxRecommendationFetchSources   = 10
	minRecommendationFetchSources   = 6
	recommendationFetchConcurrency  = 8
	recommendationFetchTimeout      = 4 * time.Second
	maxRecommendationItemsPerSource = 8
)

type RecommendationService struct {
	catalogRepository SourceCatalogRepository
	feedFetcher       FeedFetcher
	now               func() time.Time
}

func NewRecommendationService(catalogRepository SourceCatalogRepository, feedFetcher FeedFetcher, options ...SourceServiceOption) *RecommendationService {
	sourceService := &SourceService{now: time.Now}
	for _, option := range options {
		option(sourceService)
	}
	now := sourceService.now
	if now == nil {
		now = time.Now
	}
	return &RecommendationService{
		catalogRepository: catalogRepository,
		feedFetcher:       feedFetcher,
		now:               now,
	}
}

type ListRecommendationsInput struct {
	UserID   int64
	SourceID int64
	Limit    int
	Offset   int
	Order    string
}

func (s *RecommendationService) ListRecommendations(ctx context.Context, input ListRecommendationsInput) (ListItemsResult, error) {
	ctx, span := observability.StartSpan(ctx, "service.recommendation.list",
		attribute.Int64("user.id", input.UserID),
		attribute.Int("limit", input.Limit),
		attribute.Int("offset", input.Offset),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.catalogRepository == nil || s.feedFetcher == nil {
		opErr = fmt.Errorf("recommendation service is not configured")
		return ListItemsResult{}, opErr
	}
	if input.UserID < 1 {
		opErr = fmt.Errorf("%w: user id must be positive", domain.ErrInvalidInput)
		return ListItemsResult{}, opErr
	}
	if input.Offset < 0 {
		opErr = fmt.Errorf("%w: offset must be non-negative", domain.ErrInvalidInput)
		return ListItemsResult{}, opErr
	}
	if input.SourceID < 0 {
		opErr = fmt.Errorf("%w: source_id must be non-negative", domain.ErrInvalidInput)
		return ListItemsResult{}, opErr
	}

	limit, err := normalizeRecommendationLimit(input.Limit)
	if err != nil {
		opErr = err
		return ListItemsResult{}, opErr
	}
	order := normalizeItemSortOrder(input.Order)

	var entries []domain.SourceCatalogEntry
	if input.SourceID > 0 {
		entries, err = s.catalogRepository.GetByIDs(ctx, []int64{input.SourceID})
		if err != nil {
			opErr = err
			return ListItemsResult{}, opErr
		}
	} else {
		var catalog domain.SourceCatalogListResult
		catalog, err = s.catalogRepository.List(ctx, domain.SourceCatalogListOptions{
			UserID: input.UserID,
			Limit:  recommendationCatalogLimit,
			Offset: 0,
		})
		if err != nil {
			opErr = err
			return ListItemsResult{}, opErr
		}
		entries = catalog.Entries
	}

	candidates := recommendationCandidates(entries)
	if len(candidates) == 0 {
		return ListItemsResult{Items: nil, Total: 0, Limit: limit, Offset: input.Offset}, nil
	}

	rng := rand.New(rand.NewSource(recommendationShuffleSeed(input.UserID, s.now())))
	rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	selected := candidates[:recommendationSourceLimit(limit, len(candidates))]
	sourceItems := s.fetchRecommendationItems(ctx, input.UserID, selected)
	items := sortedRecommendationItems(sourceItems, order)
	total := len(items)
	if input.Offset > 0 {
		if input.Offset >= len(items) {
			items = nil
		} else {
			items = items[input.Offset:]
		}
	}
	if len(items) > limit {
		items = items[:limit]
	}

	span.SetAttributes(
		attribute.Int("recommendation.catalog_candidates", len(candidates)),
		attribute.Int("recommendation.sources_selected", len(selected)),
		attribute.Int("recommendation.items", len(items)),
	)
	return ListItemsResult{
		Items:  items,
		Total:  int64(total),
		Limit:  limit,
		Offset: input.Offset,
	}, nil
}

type recommendationSourceItems struct {
	sourceIndex int
	items       []domain.Item
}

func (s *RecommendationService) fetchRecommendationItems(ctx context.Context, userID int64, entries []domain.SourceCatalogEntry) []recommendationSourceItems {
	results := make(chan recommendationSourceItems, len(entries))
	sem := make(chan struct{}, recommendationFetchConcurrency)
	var wg sync.WaitGroup

	for index, entry := range entries {
		wg.Add(1)
		go func(sourceIndex int, catalogEntry domain.SourceCatalogEntry) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			fetchCtx, cancel := context.WithTimeout(ctx, recommendationFetchTimeout)
			defer cancel()
			source := recommendationSourceFromCatalog(userID, catalogEntry)
			result, err := s.feedFetcher.Fetch(fetchCtx, source)
			if err != nil || len(result.Items) == 0 {
				return
			}
			items := normalizeRecommendationItems(catalogEntry, result.Items)
			if len(items) == 0 {
				return
			}
			results <- recommendationSourceItems{sourceIndex: sourceIndex, items: items}
		}(index, entry)
	}

	wg.Wait()
	close(results)

	sourceItems := make([]recommendationSourceItems, 0, len(entries))
	for result := range results {
		sourceItems = append(sourceItems, result)
	}
	sort.SliceStable(sourceItems, func(i, j int) bool {
		return sourceItems[i].sourceIndex < sourceItems[j].sourceIndex
	})
	return sourceItems
}

func normalizeRecommendationLimit(limit int) (int, error) {
	if limit == 0 {
		return defaultRecommendationLimit, nil
	}
	if limit < 0 {
		return 0, fmt.Errorf("%w: limit must be non-negative", domain.ErrInvalidInput)
	}
	if limit > maxRecommendationLimit {
		return maxRecommendationLimit, nil
	}
	return limit, nil
}

func recommendationCandidates(entries []domain.SourceCatalogEntry) []domain.SourceCatalogEntry {
	candidates := make([]domain.SourceCatalogEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.Official || entry.HealthStatus == domain.SourceCatalogHealthUnreachable {
			continue
		}
		if !fetchableFeedURL(entry.FeedURL) {
			continue
		}
		candidates = append(candidates, entry)
	}
	return candidates
}

func fetchableFeedURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func recommendationSourceLimit(itemLimit int, candidateCount int) int {
	sourceLimit := itemLimit/4 + 6
	if sourceLimit < minRecommendationFetchSources {
		sourceLimit = minRecommendationFetchSources
	}
	if sourceLimit > maxRecommendationFetchSources {
		sourceLimit = maxRecommendationFetchSources
	}
	if sourceLimit > candidateCount {
		sourceLimit = candidateCount
	}
	return sourceLimit
}

func recommendationSourceFromCatalog(userID int64, entry domain.SourceCatalogEntry) domain.Source {
	sourceType := entry.Type
	if sourceType == "" {
		sourceType = domain.SourceTypeRSS
	}
	return domain.Source{
		ID:                   entry.ID,
		UserID:               userID,
		Name:                 entry.Name,
		Type:                 sourceType,
		URL:                  entry.FeedURL,
		NormalizedURL:        entry.NormalizedURL,
		Status:               domain.SourceStatusActive,
		FetchIntervalSeconds: DefaultSourceFetchIntervalSeconds,
		Tags:                 append([]string(nil), entry.Tags...),
	}
}

func normalizeRecommendationItems(entry domain.SourceCatalogEntry, items []domain.Item) []domain.Item {
	normalized := make([]domain.Item, 0, len(items))
	for index, item := range items {
		item.ID = syntheticRecommendationItemID(entry.ID, item.NormalizedURL, index)
		item.SourceID = entry.ID
		item.SourceName = entry.Name
		if item.CreatedAt.IsZero() {
			item.CreatedAt = item.FetchedAt
		}
		if item.UpdatedAt.IsZero() {
			item.UpdatedAt = item.FetchedAt
		}
		normalized = append(normalized, item)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		return recommendationItemTime(normalized[i]).After(recommendationItemTime(normalized[j]))
	})
	if len(normalized) > maxRecommendationItemsPerSource {
		normalized = normalized[:maxRecommendationItemsPerSource]
	}
	return normalized
}

func interleaveRecommendationItems(sourceItems []recommendationSourceItems, limit int) []domain.Item {
	if limit <= 0 {
		return nil
	}
	items := make([]domain.Item, 0, limit)
	for cursor := 0; len(items) < limit; cursor++ {
		added := false
		for _, group := range sourceItems {
			if cursor >= len(group.items) {
				continue
			}
			items = append(items, group.items[cursor])
			added = true
			if len(items) >= limit {
				break
			}
		}
		if !added {
			break
		}
	}
	return items
}

func recommendationItemTime(item domain.Item) time.Time {
	if item.PublishedAt != nil {
		return *item.PublishedAt
	}
	return item.FetchedAt
}

func sortedRecommendationItems(sourceItems []recommendationSourceItems, order domain.ItemSortOrder) []domain.Item {
	items := make([]domain.Item, 0)
	for _, group := range sourceItems {
		items = append(items, group.items...)
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := recommendationItemTime(items[i])
		right := recommendationItemTime(items[j])
		if left.Equal(right) {
			if order == domain.ItemSortOrderAsc {
				return items[i].ID < items[j].ID
			}
			return items[i].ID > items[j].ID
		}
		if order == domain.ItemSortOrderAsc {
			return left.Before(right)
		}
		return left.After(right)
	})
	return items
}

func recommendationShuffleSeed(userID int64, now time.Time) int64 {
	hash := fnv.New64a()
	_, _ = fmt.Fprintf(hash, "%d:%s", userID, now.UTC().Format("2006-01-02"))
	return int64(hash.Sum64() & ((1 << 63) - 1))
}

func syntheticRecommendationItemID(sourceID int64, normalizedURL string, index int) int64 {
	hash := fnv.New64a()
	_, _ = fmt.Fprintf(hash, "%d:%s:%d", sourceID, normalizedURL, index)
	value := int64(hash.Sum64() & ((1 << 53) - 1))
	if value == 0 {
		return int64(index + 1)
	}
	return value
}
