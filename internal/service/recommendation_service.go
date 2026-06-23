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
	recommendationFetchConcurrency  = 10
	recommendationFetchTimeout      = 2 * time.Second
	recommendationCacheTTL          = 5 * time.Minute
	recommendationRefreshTimeout    = 6 * time.Second
	maxRecommendationItemsPerSource = 8
)

type RecommendationService struct {
	catalogRepository SourceCatalogRepository
	feedFetcher       FeedFetcher
	sourceRepository  RecommendationSourceRepository
	itemRepository    RecommendationItemRepository
	now               func() time.Time
	cacheMu           sync.Mutex
	cache             map[recommendationCacheKey]recommendationCacheEntry
	refreshing        map[recommendationCacheKey]struct{}
}

type RecommendationSourceRepository interface {
	ListByUser(ctx context.Context, userID int64) ([]domain.Source, error)
}

type RecommendationItemRepository interface {
	ListByUser(ctx context.Context, options domain.ItemListOptions) (domain.ItemListResult, error)
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
		cache:             make(map[recommendationCacheKey]recommendationCacheEntry),
		refreshing:        make(map[recommendationCacheKey]struct{}),
	}
}

func (s *RecommendationService) SetLocalHistoryRepositories(sourceRepository RecommendationSourceRepository, itemRepository RecommendationItemRepository) {
	if s == nil {
		return
	}
	s.sourceRepository = sourceRepository
	s.itemRepository = itemRepository
}

type ListRecommendationsInput struct {
	UserID   int64
	SourceID int64
	Limit    int
	Offset   int
	Order    string
	Refresh  bool
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
		if len(entries) > 0 {
			localResult, resolved, err := s.listSubscribedSourceHistory(ctx, input, entries[0], limit, order)
			if err != nil {
				opErr = err
				return ListItemsResult{}, opErr
			}
			if resolved {
				span.SetAttributes(
					attribute.Bool("recommendation.local_history", true),
					attribute.Int64("recommendation.catalog_source_id", input.SourceID),
					attribute.Int("recommendation.items", len(localResult.Items)),
				)
				return localResult, nil
			}
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

	now := s.now()
	shuffleSeed := recommendationShuffleSeed(input.UserID, now)
	if input.Refresh {
		shuffleSeed = recommendationRefreshShuffleSeed(input.UserID, input.SourceID, now)
	}
	rng := rand.New(rand.NewSource(shuffleSeed))
	rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	selected := candidates[:recommendationSourceLimit(maxRecommendationLimit, len(candidates))]
	cacheKey := recommendationCacheKey{
		userID:   input.UserID,
		sourceID: input.SourceID,
		order:    order,
		day:      recommendationCacheDay(now),
	}
	cachedItems, cachedOK := s.getRecommendationCache(cacheKey, now)
	if cachedOK && !input.Refresh {
		items := paginateRecommendationItems(cachedItems, limit, input.Offset)
		span.SetAttributes(
			attribute.Bool("recommendation.cache_hit", true),
			attribute.Bool("recommendation.refresh_requested", input.Refresh),
			attribute.Int("recommendation.catalog_candidates", len(candidates)),
			attribute.Int("recommendation.sources_selected", len(selected)),
			attribute.Int("recommendation.items", len(items)),
		)
		return ListItemsResult{
			Items:  items,
			Total:  int64(len(cachedItems)),
			Limit:  limit,
			Offset: input.Offset,
		}, nil
	}

	maxItemsPerSource := maxRecommendationItemsPerSource
	if input.SourceID > 0 {
		maxItemsPerSource = 0
	}
	items := s.buildRecommendationItems(ctx, input.UserID, selected, order, maxItemsPerSource)
	usedCacheFallback := false
	if len(items) == 0 && input.Refresh && cachedOK {
		items = cachedItems
		usedCacheFallback = true
	} else {
		s.setRecommendationCache(cacheKey, items, now)
	}
	pagedItems := paginateRecommendationItems(items, limit, input.Offset)
	span.SetAttributes(
		attribute.Bool("recommendation.cache_hit", usedCacheFallback),
		attribute.Bool("recommendation.refresh_requested", input.Refresh),
		attribute.Int("recommendation.catalog_candidates", len(candidates)),
		attribute.Int("recommendation.sources_selected", len(selected)),
		attribute.Int("recommendation.items", len(pagedItems)),
	)
	return ListItemsResult{
		Items:  pagedItems,
		Total:  int64(len(items)),
		Limit:  limit,
		Offset: input.Offset,
	}, nil
}

func (s *RecommendationService) listSubscribedSourceHistory(
	ctx context.Context,
	input ListRecommendationsInput,
	entry domain.SourceCatalogEntry,
	limit int,
	order domain.ItemSortOrder,
) (ListItemsResult, bool, error) {
	if s.sourceRepository == nil || s.itemRepository == nil {
		return ListItemsResult{}, false, nil
	}
	if entry.NormalizedURL == "" {
		return ListItemsResult{}, false, nil
	}

	sources, err := s.sourceRepository.ListByUser(ctx, input.UserID)
	if err != nil {
		return ListItemsResult{}, false, err
	}
	for _, source := range sources {
		if source.UserID != input.UserID || source.NormalizedURL != entry.NormalizedURL {
			continue
		}
		result, err := s.itemRepository.ListByUser(ctx, domain.ItemListOptions{
			UserID:    input.UserID,
			SourceID:  source.ID,
			Limit:     limit,
			Offset:    input.Offset,
			SortOrder: order,
		})
		if err != nil {
			return ListItemsResult{}, false, err
		}
		return ListItemsResult{
			Items:  result.Items,
			Total:  result.Total,
			Limit:  result.Limit,
			Offset: result.Offset,
		}, true, nil
	}
	return ListItemsResult{}, false, nil
}

type recommendationSourceItems struct {
	sourceIndex int
	items       []domain.Item
}

type recommendationCacheKey struct {
	userID   int64
	sourceID int64
	order    domain.ItemSortOrder
	day      string
}

type recommendationCacheEntry struct {
	items    []domain.Item
	storedAt time.Time
}

func (s *RecommendationService) fetchRecommendationItems(ctx context.Context, userID int64, entries []domain.SourceCatalogEntry, maxItemsPerSource int) []recommendationSourceItems {
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
			items := normalizeRecommendationItems(catalogEntry, result.Items, maxItemsPerSource)
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

func normalizeRecommendationItems(entry domain.SourceCatalogEntry, items []domain.Item, maxItems int) []domain.Item {
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
	if maxItems > 0 && len(normalized) > maxItems {
		normalized = normalized[:maxItems]
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

func (s *RecommendationService) buildRecommendationItems(ctx context.Context, userID int64, entries []domain.SourceCatalogEntry, order domain.ItemSortOrder, maxItemsPerSource int) []domain.Item {
	sourceItems := s.fetchRecommendationItems(ctx, userID, entries, maxItemsPerSource)
	return sortedRecommendationItems(sourceItems, order)
}

func (s *RecommendationService) getRecommendationCache(key recommendationCacheKey, now time.Time) ([]domain.Item, bool) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	entry, ok := s.cache[key]
	if !ok || now.Sub(entry.storedAt) > recommendationCacheTTL {
		if ok {
			delete(s.cache, key)
		}
		return nil, false
	}
	return cloneRecommendationItems(entry.items), true
}

func (s *RecommendationService) setRecommendationCache(key recommendationCacheKey, items []domain.Item, now time.Time) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if s.cache == nil {
		s.cache = make(map[recommendationCacheKey]recommendationCacheEntry)
	}
	s.cache[key] = recommendationCacheEntry{
		items:    cloneRecommendationItems(items),
		storedAt: now,
	}
}

func (s *RecommendationService) refreshRecommendationCacheAsync(key recommendationCacheKey, userID int64, entries []domain.SourceCatalogEntry, order domain.ItemSortOrder) {
	if !s.beginRecommendationRefresh(key) {
		return
	}
	refreshEntries := append([]domain.SourceCatalogEntry(nil), entries...)
	go func() {
		defer s.endRecommendationRefresh(key)
		ctx, cancel := context.WithTimeout(context.Background(), recommendationRefreshTimeout)
		defer cancel()
		items := s.buildRecommendationItems(ctx, userID, refreshEntries, order, maxRecommendationItemsPerSource)
		if len(items) == 0 {
			return
		}
		s.setRecommendationCache(key, items, s.now())
	}()
}

func (s *RecommendationService) beginRecommendationRefresh(key recommendationCacheKey) bool {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if s.refreshing == nil {
		s.refreshing = make(map[recommendationCacheKey]struct{})
	}
	if _, ok := s.refreshing[key]; ok {
		return false
	}
	s.refreshing[key] = struct{}{}
	return true
}

func (s *RecommendationService) endRecommendationRefresh(key recommendationCacheKey) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	delete(s.refreshing, key)
}

func paginateRecommendationItems(items []domain.Item, limit int, offset int) []domain.Item {
	if offset >= len(items) {
		return nil
	}
	paged := items[offset:]
	if len(paged) > limit {
		paged = paged[:limit]
	}
	return cloneRecommendationItems(paged)
}

func cloneRecommendationItems(items []domain.Item) []domain.Item {
	return append([]domain.Item(nil), items...)
}

func recommendationCacheDay(now time.Time) string {
	return now.UTC().Format("2006-01-02")
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

func recommendationRefreshShuffleSeed(userID int64, sourceID int64, now time.Time) int64 {
	hash := fnv.New64a()
	_, _ = fmt.Fprintf(hash, "%d:%d:%d", userID, sourceID, now.UTC().UnixNano())
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
