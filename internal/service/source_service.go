package service

import (
	"context"
	"errors"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/metrics"
	"messagefeed/internal/observability"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const (
	DefaultSourceFetchIntervalSeconds = 3600
)

type SourceRepository interface {
	Create(ctx context.Context, source domain.Source) (domain.Source, error)
	ListByUser(ctx context.Context, userID int64) ([]domain.Source, error)
	GetByID(ctx context.Context, userID int64, id int64) (domain.Source, error)
	Update(ctx context.Context, source domain.Source) (domain.Source, error)
	UpdateFetchResult(ctx context.Context, source domain.Source) (domain.Source, error)
}

type ItemRepository interface {
	UpsertMany(ctx context.Context, items []domain.Item) (domain.ItemUpsertResult, error)
}

type FeedFetcher interface {
	Fetch(ctx context.Context, source domain.Source) (domain.FeedFetchResult, error)
}

type SourceService struct {
	repository     SourceRepository
	itemRepository ItemRepository
	feedFetcher    FeedFetcher
	now            func() time.Time
}

type SourceServiceOption func(*SourceService)

func WithItemRepository(repository ItemRepository) SourceServiceOption {
	return func(service *SourceService) {
		service.itemRepository = repository
	}
}

func WithFeedFetcher(feedFetcher FeedFetcher) SourceServiceOption {
	return func(service *SourceService) {
		service.feedFetcher = feedFetcher
	}
}

func WithNow(now func() time.Time) SourceServiceOption {
	return func(service *SourceService) {
		if now != nil {
			service.now = now
		}
	}
}

func NewSourceService(repository SourceRepository, options ...SourceServiceOption) *SourceService {
	service := &SourceService{
		repository: repository,
		now:        time.Now,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

type CreateSourceInput struct {
	UserID               int64
	Name                 string
	Type                 domain.SourceType
	URL                  string
	FetchIntervalSeconds int
	Tags                 []string
	Weight               int
}

type UpdateSourceInput struct {
	UserID               int64
	ID                   int64
	Name                 *string
	Type                 *domain.SourceType
	URL                  *string
	Status               *domain.SourceStatus
	FetchIntervalSeconds *int
	Tags                 *[]string
	Weight               *int
}

type FetchSourceInput struct {
	UserID int64
	ID     int64
}

type FetchSourceResult struct {
	Source       domain.Source
	ItemCount    int
	CreatedCount int
	UpdatedCount int
}

func (s *SourceService) CreateSource(ctx context.Context, input CreateSourceInput) (domain.Source, error) {
	ctx, span := observability.StartSpan(ctx, "service.source.create",
		attribute.Int64("user.id", input.UserID),
		attribute.String("source.url", input.URL),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.repository == nil {
		opErr = fmt.Errorf("source service is not configured")
		return domain.Source{}, opErr
	}

	sourceType := input.Type
	if sourceType == "" {
		sourceType = domain.SourceTypeRSS
	}
	if !sourceType.Valid() {
		opErr = fmt.Errorf("%w: unsupported source type", domain.ErrInvalidInput)
		return domain.Source{}, opErr
	}

	normalizedURL, host, err := NormalizeSourceURL(input.URL)
	if err != nil {
		opErr = err
		return domain.Source{}, opErr
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = host
	}

	fetchInterval := input.FetchIntervalSeconds
	if fetchInterval == 0 {
		fetchInterval = DefaultSourceFetchIntervalSeconds
	}
	if fetchInterval < 0 {
		opErr = fmt.Errorf("%w: fetch_interval_seconds must be non-negative", domain.ErrInvalidInput)
		return domain.Source{}, opErr
	}

	source := domain.Source{
		UserID:               input.UserID,
		Name:                 name,
		Type:                 sourceType,
		URL:                  strings.TrimSpace(input.URL),
		NormalizedURL:        normalizedURL,
		Status:               domain.SourceStatusActive,
		FetchIntervalSeconds: fetchInterval,
		Tags:                 normalizeTags(input.Tags),
		Weight:               input.Weight,
	}

	created, err := s.repository.Create(ctx, source)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			opErr = fmt.Errorf("%w: source already exists", domain.ErrConflict)
			return domain.Source{}, opErr
		}
		opErr = err
		return domain.Source{}, opErr
	}
	span.SetAttributes(attribute.Int64("source.id", created.ID))
	return created, nil
}

func (s *SourceService) ListSources(ctx context.Context, userID int64) ([]domain.Source, error) {
	ctx, span := observability.StartSpan(ctx, "service.source.list",
		attribute.Int64("user.id", userID),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.repository == nil {
		opErr = fmt.Errorf("source service is not configured")
		return nil, opErr
	}
	sources, err := s.repository.ListByUser(ctx, userID)
	if err != nil {
		opErr = err
		return nil, opErr
	}
	span.SetAttributes(attribute.Int("source.count", len(sources)))
	return sources, nil
}

func (s *SourceService) UpdateSource(ctx context.Context, input UpdateSourceInput) (domain.Source, error) {
	ctx, span := observability.StartSpan(ctx, "service.source.update",
		attribute.Int64("user.id", input.UserID),
		attribute.Int64("source.id", input.ID),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.repository == nil {
		opErr = fmt.Errorf("source service is not configured")
		return domain.Source{}, opErr
	}
	if input.ID < 1 {
		opErr = fmt.Errorf("%w: source id must be positive", domain.ErrInvalidInput)
		return domain.Source{}, opErr
	}

	source, err := s.repository.GetByID(ctx, input.UserID, input.ID)
	if err != nil {
		opErr = err
		return domain.Source{}, opErr
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			opErr = fmt.Errorf("%w: name must not be empty", domain.ErrInvalidInput)
			return domain.Source{}, opErr
		}
		source.Name = name
	}

	if input.Type != nil {
		if !input.Type.Valid() {
			opErr = fmt.Errorf("%w: unsupported source type", domain.ErrInvalidInput)
			return domain.Source{}, opErr
		}
		source.Type = *input.Type
	}

	if input.URL != nil {
		normalizedURL, _, err := NormalizeSourceURL(*input.URL)
		if err != nil {
			opErr = err
			return domain.Source{}, opErr
		}
		source.URL = strings.TrimSpace(*input.URL)
		source.NormalizedURL = normalizedURL
	}

	if input.Status != nil {
		if !input.Status.Valid() {
			opErr = fmt.Errorf("%w: unsupported source status", domain.ErrInvalidInput)
			return domain.Source{}, opErr
		}
		source.Status = *input.Status
		span.SetAttributes(attribute.String("source.status", string(source.Status)))
	}

	if input.FetchIntervalSeconds != nil {
		if *input.FetchIntervalSeconds < 0 {
			opErr = fmt.Errorf("%w: fetch_interval_seconds must be non-negative", domain.ErrInvalidInput)
			return domain.Source{}, opErr
		}
		source.FetchIntervalSeconds = *input.FetchIntervalSeconds
	}

	if input.Tags != nil {
		source.Tags = normalizeTags(*input.Tags)
	}
	if input.Weight != nil {
		source.Weight = *input.Weight
	}

	updated, err := s.repository.Update(ctx, source)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			opErr = fmt.Errorf("%w: source already exists", domain.ErrConflict)
			return domain.Source{}, opErr
		}
		opErr = err
		return domain.Source{}, opErr
	}
	return updated, nil
}

func (s *SourceService) TriggerFetch(ctx context.Context, input FetchSourceInput) (FetchSourceResult, error) {
	ctx, span := observability.StartSpan(ctx, "service.source.trigger_fetch",
		attribute.Int64("user.id", input.UserID),
		attribute.Int64("source.id", input.ID),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.repository == nil || s.itemRepository == nil || s.feedFetcher == nil {
		opErr = fmt.Errorf("source fetch service is not configured")
		return FetchSourceResult{}, opErr
	}
	if input.ID < 1 {
		opErr = fmt.Errorf("%w: source id must be positive", domain.ErrInvalidInput)
		return FetchSourceResult{}, opErr
	}

	source, err := s.repository.GetByID(ctx, input.UserID, input.ID)
	if err != nil {
		opErr = err
		return FetchSourceResult{}, opErr
	}
	sourceLabel := strconv.FormatInt(source.ID, 10)
	span.SetAttributes(
		attribute.String("source.name", source.Name),
		attribute.String("source.status", string(source.Status)),
	)
	if source.Status != domain.SourceStatusActive {
		opErr = fmt.Errorf("%w: inactive source cannot be fetched", domain.ErrInvalidInput)
		metrics.FeedFetchesTotal.WithLabelValues(sourceLabel, "invalid").Inc()
		return FetchSourceResult{}, opErr
	}

	startedAt := time.Now()
	fetchResult, err := s.feedFetcher.Fetch(ctx, source)
	durationMS := int(time.Since(startedAt).Milliseconds())
	if err != nil {
		_, _ = s.markFetchFailed(ctx, source, durationMS, err)
		metrics.FeedFetchesTotal.WithLabelValues(sourceLabel, "failed").Inc()
		metrics.FeedFetchDuration.Observe(time.Since(startedAt).Seconds())
		opErr = err
		return FetchSourceResult{}, opErr
	}

	upsertResult, err := s.itemRepository.UpsertMany(ctx, fetchResult.Items)
	if err != nil {
		_, _ = s.markFetchFailed(ctx, source, durationMS, err)
		metrics.FeedFetchesTotal.WithLabelValues(sourceLabel, "failed").Inc()
		metrics.FeedFetchDuration.Observe(time.Since(startedAt).Seconds())
		opErr = err
		return FetchSourceResult{}, opErr
	}

	itemCount := len(fetchResult.Items)
	metrics.FeedFetchesTotal.WithLabelValues(sourceLabel, "success").Inc()
	metrics.FeedFetchDuration.Observe(time.Since(startedAt).Seconds())
	metrics.FeedFetchItemsTotal.WithLabelValues(sourceLabel, "parsed").Add(float64(itemCount))
	metrics.FeedFetchItemsTotal.WithLabelValues(sourceLabel, "created").Add(float64(upsertResult.CreatedCount))
	metrics.FeedFetchItemsTotal.WithLabelValues(sourceLabel, "updated").Add(float64(upsertResult.UpdatedCount))
	span.SetAttributes(
		attribute.Int("feed.items", itemCount),
		attribute.Int("feed.items_created", upsertResult.CreatedCount),
		attribute.Int("feed.items_updated", upsertResult.UpdatedCount),
		attribute.Int("feed.duration_ms", durationMS),
	)

	updatedSource, err := s.markFetchSucceeded(ctx, source, durationMS, itemCount)
	if err != nil {
		opErr = err
		return FetchSourceResult{}, opErr
	}

	return FetchSourceResult{
		Source:       updatedSource,
		ItemCount:    itemCount,
		CreatedCount: upsertResult.CreatedCount,
		UpdatedCount: upsertResult.UpdatedCount,
	}, nil
}

func (s *SourceService) markFetchSucceeded(ctx context.Context, source domain.Source, durationMS int, itemCount int) (domain.Source, error) {
	fetchedAt := s.now().UTC()
	source.LastFetchedAt = &fetchedAt
	source.LastFetchStatus = domain.SourceLastFetchStatusSuccess
	source.LastFetchError = ""
	source.LastFetchDurationMS = &durationMS
	source.LastFetchItemCount = &itemCount
	return s.repository.UpdateFetchResult(ctx, source)
}

func (s *SourceService) markFetchFailed(ctx context.Context, source domain.Source, durationMS int, err error) (domain.Source, error) {
	fetchedAt := s.now().UTC()
	itemCount := 0
	source.LastFetchedAt = &fetchedAt
	source.LastFetchStatus = domain.SourceLastFetchStatusFailed
	source.LastFetchError = truncateError(err.Error(), 2000)
	source.LastFetchDurationMS = &durationMS
	source.LastFetchItemCount = &itemCount
	return s.repository.UpdateFetchResult(ctx, source)
}

func NormalizeSourceURL(rawURL string) (normalized string, host string, err error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", "", fmt.Errorf("%w: url must not be empty", domain.ErrInvalidInput)
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", "", fmt.Errorf("%w: invalid url", domain.ErrInvalidInput)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", fmt.Errorf("%w: url scheme must be http or https", domain.ErrInvalidInput)
	}
	if parsed.Host == "" {
		return "", "", fmt.Errorf("%w: url host is required", domain.ErrInvalidInput)
	}

	host = parsed.Hostname()
	if host == "" {
		return "", "", fmt.Errorf("%w: url host is required", domain.ErrInvalidInput)
	}
	if port := parsed.Port(); port != "" {
		if (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
			parsed.Host = host
		} else {
			parsed.Host = net.JoinHostPort(host, port)
		}
	}

	if parsed.Path == "/" {
		parsed.Path = ""
	}
	return parsed.String(), host, nil
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		value := strings.TrimSpace(tag)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func truncateError(value string, maxLength int) string {
	if maxLength < 1 || len(value) <= maxLength {
		return value
	}
	return value[:maxLength]
}
