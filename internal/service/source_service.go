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
	"sync"
	"time"
	"unicode/utf8"

	"go.opentelemetry.io/otel/attribute"
)

const (
	DefaultSourceFetchIntervalSeconds = 3600
	fetchActiveSourcesConcurrency     = 4
)

type SourceRepository interface {
	Create(ctx context.Context, source domain.Source) (domain.Source, error)
	ListByUser(ctx context.Context, userID int64) ([]domain.Source, error)
	GetByID(ctx context.Context, userID int64, id int64) (domain.Source, error)
	Update(ctx context.Context, source domain.Source) (domain.Source, error)
	UpdateFetchResult(ctx context.Context, source domain.Source) (domain.Source, error)
}

type SourceCatalogRepository interface {
	List(ctx context.Context, options domain.SourceCatalogListOptions) (domain.SourceCatalogListResult, error)
	GetByIDs(ctx context.Context, ids []int64) ([]domain.SourceCatalogEntry, error)
}

type ItemRepository interface {
	UpsertMany(ctx context.Context, items []domain.Item) (domain.ItemUpsertResult, error)
}

type SourceImportJobRepository interface {
	Create(ctx context.Context, job domain.SourceImportJob) (domain.SourceImportJob, error)
	ListByUser(ctx context.Context, options domain.SourceImportJobListOptions) (domain.SourceImportJobListResult, error)
}

type SourceFetchJobQueueRepository interface {
	CreateJob(ctx context.Context, job domain.SourceFetchJob) (domain.SourceFetchJob, error)
	ListJobsByIDs(ctx context.Context, options domain.SourceFetchJobListByIDsOptions) ([]domain.SourceFetchJob, error)
	ListAttemptsByJob(ctx context.Context, options domain.SourceFetchAttemptListOptions) (domain.SourceFetchAttemptListResult, error)
}

type FeedFetcher interface {
	Fetch(ctx context.Context, source domain.Source) (domain.FeedFetchResult, error)
}

type SourceService struct {
	repository        SourceRepository
	catalogRepository SourceCatalogRepository
	importJobRepo     SourceImportJobRepository
	fetchJobRepo      SourceFetchJobQueueRepository
	itemRepository    ItemRepository
	feedFetcher       FeedFetcher
	now               func() time.Time
}

type SourceServiceOption func(*SourceService)

func WithItemRepository(repository ItemRepository) SourceServiceOption {
	return func(service *SourceService) {
		service.itemRepository = repository
	}
}

func WithSourceCatalogRepository(repository SourceCatalogRepository) SourceServiceOption {
	return func(service *SourceService) {
		service.catalogRepository = repository
	}
}

func WithSourceImportJobRepository(repository SourceImportJobRepository) SourceServiceOption {
	return func(service *SourceService) {
		service.importJobRepo = repository
	}
}

func WithSourceFetchJobRepository(repository SourceFetchJobQueueRepository) SourceServiceOption {
	return func(service *SourceService) {
		service.fetchJobRepo = repository
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

type FetchActiveSourcesInput struct {
	UserID int64
}

type SourceFetchStatusInput struct {
	UserID int64
	JobIDs []int64
}

type ListSourceCatalogInput struct {
	UserID   int64
	Category string
	Query    string
	Limit    int
	Offset   int
}

type ListSourceCatalogResult struct {
	Entries []domain.SourceCatalogEntry
	Total   int64
	Limit   int
	Offset  int
}

type ImportCatalogSourcesInput struct {
	UserID     int64
	CatalogIDs []int64
}

type ImportURLSourcesInput struct {
	UserID     int64
	URLs       []string
	ImportType domain.SourceImportType
}

type ListSourceImportJobsInput struct {
	UserID int64
	Limit  int
	Offset int
}

type ImportSourceResult struct {
	RequestedCount int
	SuccessCount   int
	FailureCount   int
	Sources        []domain.Source
	Errors         []ImportSourceError
	ImportJob      *domain.SourceImportJob
}

type ImportSourceError struct {
	Reference string `json:"reference"`
	Message   string `json:"message"`
}

type FetchSourceResult struct {
	Source       domain.Source
	ItemCount    int
	CreatedCount int
	UpdatedCount int
}

type FetchSourceError struct {
	SourceID   int64
	SourceName string
	Message    string
}

type FetchSourcesResult struct {
	RequestedCount int
	SuccessCount   int
	FailureCount   int
	Async          bool
	QueuedCount    int
	JobIDs         []int64
	Sources        []domain.Source
	Errors         []FetchSourceError
}

type SourceFetchStatusResult struct {
	RequestedCount     int
	CompletedCount     int
	QueuedCount        int
	RunningCount       int
	SuccessCount       int
	FailureCount       int
	CreatedCount       int
	UpdatedCount       int
	UpdatedSourceCount int
	Done               bool
	Sources            []domain.Source
	Errors             []FetchSourceError
}

type ListSourceImportJobsResult struct {
	Jobs   []domain.SourceImportJob
	Total  int64
	Limit  int
	Offset int
}

type fetchSourceAttempt struct {
	source domain.Source
	result FetchSourceResult
	err    error
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

func (s *SourceService) FetchActiveSources(ctx context.Context, input FetchActiveSourcesInput) (FetchSourcesResult, error) {
	ctx, span := observability.StartSpan(ctx, "service.source.fetch_active",
		attribute.Int64("user.id", input.UserID),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if input.UserID < 1 {
		opErr = fmt.Errorf("%w: user id must be positive", domain.ErrInvalidInput)
		return FetchSourcesResult{}, opErr
	}

	sources, err := s.ListSources(ctx, input.UserID)
	if err != nil {
		opErr = err
		return FetchSourcesResult{}, opErr
	}

	activeSources := make([]domain.Source, 0, len(sources))
	for _, source := range sources {
		if source.Status == domain.SourceStatusActive {
			activeSources = append(activeSources, source)
		}
	}

	if s.fetchJobRepo != nil {
		result, err := s.enqueueActiveSourceFetchJobs(ctx, activeSources)
		if err != nil {
			opErr = err
			return FetchSourcesResult{}, opErr
		}
		span.SetAttributes(
			attribute.Int("source.fetch_requested", result.RequestedCount),
			attribute.Int("source.fetch_queued", result.QueuedCount),
			attribute.Bool("source.fetch_async", true),
		)
		return result, nil
	}

	attempts := s.fetchSources(ctx, input.UserID, activeSources)
	result := FetchSourcesResult{
		RequestedCount: len(activeSources),
		Sources:        make([]domain.Source, 0, len(attempts)),
		Errors:         make([]FetchSourceError, 0),
	}
	for _, attempt := range attempts {
		if attempt.err != nil {
			result.FailureCount++
			result.Errors = append(result.Errors, FetchSourceError{
				SourceID:   attempt.source.ID,
				SourceName: attempt.source.Name,
				Message:    sourceImportErrorMessage(attempt.err),
			})
			continue
		}
		result.SuccessCount++
		result.Sources = append(result.Sources, attempt.result.Source)
	}
	span.SetAttributes(
		attribute.Int("source.fetch_requested", result.RequestedCount),
		attribute.Int("source.fetch_success", result.SuccessCount),
		attribute.Int("source.fetch_failed", result.FailureCount),
	)
	return result, nil
}

func (s *SourceService) GetFetchStatus(ctx context.Context, input SourceFetchStatusInput) (SourceFetchStatusResult, error) {
	ctx, span := observability.StartSpan(ctx, "service.source.fetch_status",
		attribute.Int64("user.id", input.UserID),
		attribute.Int("source_fetch_job.requested", len(input.JobIDs)),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.repository == nil || s.fetchJobRepo == nil {
		opErr = fmt.Errorf("source fetch status service is not configured")
		return SourceFetchStatusResult{}, opErr
	}
	if input.UserID < 1 {
		opErr = fmt.Errorf("%w: user id must be positive", domain.ErrInvalidInput)
		return SourceFetchStatusResult{}, opErr
	}
	jobIDs := uniquePositiveIDs(input.JobIDs)
	if len(jobIDs) == 0 {
		opErr = fmt.Errorf("%w: job_ids must not be empty", domain.ErrInvalidInput)
		return SourceFetchStatusResult{}, opErr
	}

	jobs, err := s.fetchJobRepo.ListJobsByIDs(ctx, domain.SourceFetchJobListByIDsOptions{
		UserID: input.UserID,
		IDs:    jobIDs,
	})
	if err != nil {
		opErr = err
		return SourceFetchStatusResult{}, opErr
	}
	sources, err := s.repository.ListByUser(ctx, input.UserID)
	if err != nil {
		opErr = err
		return SourceFetchStatusResult{}, opErr
	}
	sourcesByID := make(map[int64]domain.Source, len(sources))
	for _, source := range sources {
		sourcesByID[source.ID] = source
	}

	result := SourceFetchStatusResult{
		RequestedCount: len(jobs),
		Sources:        make([]domain.Source, 0),
		Errors:         make([]FetchSourceError, 0),
	}
	for _, job := range jobs {
		switch job.Status {
		case domain.SourceFetchJobStatusQueued:
			result.QueuedCount++
		case domain.SourceFetchJobStatusRunning:
			result.RunningCount++
		case domain.SourceFetchJobStatusSucceeded:
			result.SuccessCount++
			result.CompletedCount++
		case domain.SourceFetchJobStatusFailed, domain.SourceFetchJobStatusCanceled:
			result.FailureCount++
			result.CompletedCount++
			source := sourcesByID[job.SourceID]
			result.Errors = append(result.Errors, FetchSourceError{
				SourceID:   job.SourceID,
				SourceName: source.Name,
				Message:    job.LastError,
			})
		}

		attempt, ok, err := s.latestFetchAttempt(ctx, input.UserID, job.ID)
		if err != nil {
			opErr = err
			return SourceFetchStatusResult{}, opErr
		}
		if !ok || attempt.Status != domain.SourceFetchAttemptStatusSucceeded {
			continue
		}
		result.CreatedCount += attempt.CreatedCount
		result.UpdatedCount += attempt.UpdatedCount
		if attempt.CreatedCount > 0 {
			result.UpdatedSourceCount++
			if source, ok := sourcesByID[job.SourceID]; ok {
				result.Sources = append(result.Sources, source)
			}
		}
	}
	result.Done = result.RequestedCount > 0 && result.CompletedCount >= result.RequestedCount
	span.SetAttributes(
		attribute.Int("source_fetch_job.completed", result.CompletedCount),
		attribute.Int("source_fetch_job.success", result.SuccessCount),
		attribute.Int("source_fetch_job.failed", result.FailureCount),
		attribute.Int("source_fetch_job.created_items", result.CreatedCount),
	)
	return result, nil
}

func (s *SourceService) enqueueActiveSourceFetchJobs(ctx context.Context, sources []domain.Source) (FetchSourcesResult, error) {
	now := s.now().UTC()
	result := FetchSourcesResult{
		RequestedCount: len(sources),
		Async:          true,
		JobIDs:         make([]int64, 0, len(sources)),
		Sources:        make([]domain.Source, 0, len(sources)),
		Errors:         make([]FetchSourceError, 0),
	}
	for _, source := range sources {
		job, err := s.fetchJobRepo.CreateJob(ctx, domain.SourceFetchJob{
			UserID:      source.UserID,
			SourceID:    source.ID,
			Status:      domain.SourceFetchJobStatusQueued,
			Trigger:     domain.SourceFetchTriggerManual,
			ScheduledAt: now,
			MaxAttempts: 1,
			Priority:    source.FetchPriority,
		})
		if err != nil {
			result.FailureCount++
			result.Errors = append(result.Errors, FetchSourceError{
				SourceID:   source.ID,
				SourceName: source.Name,
				Message:    sourceImportErrorMessage(err),
			})
			continue
		}
		result.QueuedCount++
		result.JobIDs = append(result.JobIDs, job.ID)
		result.Sources = append(result.Sources, source)
	}
	return result, nil
}

func (s *SourceService) latestFetchAttempt(ctx context.Context, userID int64, jobID int64) (domain.SourceFetchAttempt, bool, error) {
	result, err := s.fetchJobRepo.ListAttemptsByJob(ctx, domain.SourceFetchAttemptListOptions{
		UserID: userID,
		JobID:  jobID,
		Limit:  100,
		Offset: 0,
	})
	if err != nil {
		return domain.SourceFetchAttempt{}, false, err
	}
	if len(result.Attempts) == 0 {
		return domain.SourceFetchAttempt{}, false, nil
	}
	latest := result.Attempts[0]
	for _, attempt := range result.Attempts[1:] {
		if attempt.AttemptNumber > latest.AttemptNumber || (attempt.AttemptNumber == latest.AttemptNumber && attempt.ID > latest.ID) {
			latest = attempt
		}
	}
	return latest, true, nil
}

func (s *SourceService) fetchSources(ctx context.Context, userID int64, sources []domain.Source) []fetchSourceAttempt {
	if len(sources) == 0 {
		return nil
	}

	sem := make(chan struct{}, fetchActiveSourcesConcurrency)
	results := make(chan fetchSourceAttempt, len(sources))
	var wg sync.WaitGroup
	for _, source := range sources {
		wg.Add(1)
		go func(source domain.Source) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results <- fetchSourceAttempt{source: source, err: ctx.Err()}
				return
			}

			result, err := s.TriggerFetch(ctx, FetchSourceInput{
				UserID: userID,
				ID:     source.ID,
			})
			results <- fetchSourceAttempt{source: source, result: result, err: err}
		}(source)
	}
	wg.Wait()
	close(results)

	attempts := make([]fetchSourceAttempt, 0, len(sources))
	for result := range results {
		attempts = append(attempts, result)
	}
	return attempts
}

func (s *SourceService) ListSourceCatalog(ctx context.Context, input ListSourceCatalogInput) (ListSourceCatalogResult, error) {
	ctx, span := observability.StartSpan(ctx, "service.source_catalog.list",
		attribute.Int64("user.id", input.UserID),
		attribute.String("source_catalog.category", input.Category),
		attribute.String("source_catalog.query", input.Query),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.catalogRepository == nil {
		opErr = fmt.Errorf("source catalog service is not configured")
		return ListSourceCatalogResult{}, opErr
	}
	result, err := s.catalogRepository.List(ctx, domain.SourceCatalogListOptions{
		UserID:   input.UserID,
		Category: strings.TrimSpace(input.Category),
		Query:    strings.TrimSpace(input.Query),
		Limit:    input.Limit,
		Offset:   input.Offset,
	})
	if err != nil {
		opErr = err
		return ListSourceCatalogResult{}, opErr
	}
	span.SetAttributes(attribute.Int("source_catalog.count", len(result.Entries)))
	return ListSourceCatalogResult{
		Entries: result.Entries,
		Total:   result.Total,
		Limit:   result.Limit,
		Offset:  result.Offset,
	}, nil
}

func (s *SourceService) ImportCatalogSources(ctx context.Context, input ImportCatalogSourcesInput) (ImportSourceResult, error) {
	ctx, span := observability.StartSpan(ctx, "service.source.import_catalog",
		attribute.Int64("user.id", input.UserID),
		attribute.Int("source_catalog.requested", len(input.CatalogIDs)),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.catalogRepository == nil {
		opErr = fmt.Errorf("source catalog service is not configured")
		return ImportSourceResult{}, opErr
	}
	if input.UserID < 1 {
		opErr = fmt.Errorf("%w: user id must be positive", domain.ErrInvalidInput)
		return ImportSourceResult{}, opErr
	}
	ids := uniquePositiveIDs(input.CatalogIDs)
	if len(ids) == 0 {
		opErr = fmt.Errorf("%w: catalog_ids must not be empty", domain.ErrInvalidInput)
		return ImportSourceResult{}, opErr
	}

	entries, err := s.catalogRepository.GetByIDs(ctx, ids)
	if err != nil {
		opErr = err
		return ImportSourceResult{}, opErr
	}
	byID := make(map[int64]domain.SourceCatalogEntry, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = entry
	}

	result := ImportSourceResult{RequestedCount: len(ids)}
	for _, id := range ids {
		entry, ok := byID[id]
		if !ok {
			result.Errors = append(result.Errors, ImportSourceError{Reference: strconv.FormatInt(id, 10), Message: "catalog entry not found"})
			result.FailureCount++
			continue
		}
		source, err := s.createOrReactivateSource(ctx, CreateSourceInput{
			UserID:               input.UserID,
			Name:                 entry.Name,
			Type:                 entry.Type,
			URL:                  entry.FeedURL,
			FetchIntervalSeconds: DefaultSourceFetchIntervalSeconds,
			Tags:                 entry.Tags,
			Weight:               0,
		})
		if err != nil {
			result.Errors = append(result.Errors, ImportSourceError{Reference: entry.Name, Message: sourceImportErrorMessage(err)})
			result.FailureCount++
			continue
		}
		result.Sources = append(result.Sources, source)
		result.SuccessCount++
	}
	if err := s.recordImportJob(ctx, input.UserID, domain.SourceImportTypeCatalog, &result); err != nil {
		opErr = err
		return ImportSourceResult{}, opErr
	}
	span.SetAttributes(attribute.Int("source.imported", result.SuccessCount), attribute.Int("source.failed", result.FailureCount))
	return result, nil
}

func (s *SourceService) ImportURLSources(ctx context.Context, input ImportURLSourcesInput) (ImportSourceResult, error) {
	ctx, span := observability.StartSpan(ctx, "service.source.import_urls",
		attribute.Int64("user.id", input.UserID),
		attribute.Int("source.requested", len(input.URLs)),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if input.UserID < 1 {
		opErr = fmt.Errorf("%w: user id must be positive", domain.ErrInvalidInput)
		return ImportSourceResult{}, opErr
	}
	importType := input.ImportType
	if importType == "" {
		importType = domain.SourceImportTypeURLs
	}
	if !importType.Valid() {
		opErr = fmt.Errorf("%w: unsupported import type", domain.ErrInvalidInput)
		return ImportSourceResult{}, opErr
	}
	urls := uniqueNonEmptyStrings(input.URLs)
	if len(urls) == 0 {
		opErr = fmt.Errorf("%w: urls must not be empty", domain.ErrInvalidInput)
		return ImportSourceResult{}, opErr
	}

	result := ImportSourceResult{RequestedCount: len(urls)}
	for _, rawURL := range urls {
		source, err := s.createOrReactivateSource(ctx, CreateSourceInput{
			UserID: input.UserID,
			URL:    rawURL,
		})
		if err != nil {
			result.Errors = append(result.Errors, ImportSourceError{Reference: rawURL, Message: sourceImportErrorMessage(err)})
			result.FailureCount++
			continue
		}
		result.Sources = append(result.Sources, source)
		result.SuccessCount++
	}
	if err := s.recordImportJob(ctx, input.UserID, importType, &result); err != nil {
		opErr = err
		return ImportSourceResult{}, opErr
	}
	span.SetAttributes(attribute.Int("source.imported", result.SuccessCount), attribute.Int("source.failed", result.FailureCount))
	return result, nil
}

func (s *SourceService) ListSourceImportJobs(ctx context.Context, input ListSourceImportJobsInput) (ListSourceImportJobsResult, error) {
	ctx, span := observability.StartSpan(ctx, "service.source_import_job.list",
		attribute.Int64("user.id", input.UserID),
		attribute.Int("limit", input.Limit),
		attribute.Int("offset", input.Offset),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.importJobRepo == nil {
		opErr = fmt.Errorf("source import job service is not configured")
		return ListSourceImportJobsResult{}, opErr
	}
	options, err := normalizeSourceImportJobListOptions(input)
	if err != nil {
		opErr = err
		return ListSourceImportJobsResult{}, opErr
	}

	result, err := s.importJobRepo.ListByUser(ctx, options)
	if err != nil {
		opErr = err
		return ListSourceImportJobsResult{}, opErr
	}
	span.SetAttributes(
		attribute.Int("source_import_job.count", len(result.Jobs)),
		attribute.Int64("source_import_job.total", result.Total),
		attribute.Int("limit.normalized", result.Limit),
		attribute.Int("offset.normalized", result.Offset),
	)
	return ListSourceImportJobsResult{
		Jobs:   result.Jobs,
		Total:  result.Total,
		Limit:  result.Limit,
		Offset: result.Offset,
	}, nil
}

func normalizeSourceImportJobListOptions(input ListSourceImportJobsInput) (domain.SourceImportJobListOptions, error) {
	if input.UserID < 1 {
		return domain.SourceImportJobListOptions{}, fmt.Errorf("%w: user id must be positive", domain.ErrInvalidInput)
	}
	if input.Offset < 0 {
		return domain.SourceImportJobListOptions{}, fmt.Errorf("%w: offset must be non-negative", domain.ErrInvalidInput)
	}

	limit := input.Limit
	if limit == 0 {
		limit = domain.DefaultSourceImportJobListLimit
	}
	if limit < 0 {
		return domain.SourceImportJobListOptions{}, fmt.Errorf("%w: limit must be non-negative", domain.ErrInvalidInput)
	}
	if limit > domain.MaxSourceImportJobListLimit {
		limit = domain.MaxSourceImportJobListLimit
	}

	return domain.SourceImportJobListOptions{
		UserID: input.UserID,
		Limit:  limit,
		Offset: input.Offset,
	}, nil
}

func (s *SourceService) recordImportJob(ctx context.Context, userID int64, importType domain.SourceImportType, result *ImportSourceResult) error {
	if s == nil || s.importJobRepo == nil || result == nil {
		return nil
	}
	status := domain.SourceImportStatusCompleted
	if result.FailureCount > 0 && result.SuccessCount > 0 {
		status = domain.SourceImportStatusPartial
	}
	if result.FailureCount > 0 && result.SuccessCount == 0 {
		status = domain.SourceImportStatusFailed
	}

	errorDetails := make([]domain.SourceImportJobError, 0, len(result.Errors))
	for _, item := range result.Errors {
		errorDetails = append(errorDetails, domain.SourceImportJobError{
			Reference: item.Reference,
			Message:   item.Message,
		})
	}
	job, err := s.importJobRepo.Create(ctx, domain.SourceImportJob{
		UserID:         userID,
		ImportType:     importType,
		Status:         status,
		RequestedCount: result.RequestedCount,
		SuccessCount:   result.SuccessCount,
		FailureCount:   result.FailureCount,
		ErrorDetails:   errorDetails,
	})
	if err != nil {
		return fmt.Errorf("record source import job: %w", err)
	}
	result.ImportJob = &job
	return nil
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

func (s *SourceService) createOrReactivateSource(ctx context.Context, input CreateSourceInput) (domain.Source, error) {
	source, err := s.CreateSource(ctx, input)
	if err == nil {
		return source, nil
	}
	if !errors.Is(err, domain.ErrConflict) {
		return domain.Source{}, err
	}

	normalizedURL, _, normalizeErr := NormalizeSourceURL(input.URL)
	if normalizeErr != nil {
		return domain.Source{}, normalizeErr
	}
	sources, listErr := s.ListSources(ctx, input.UserID)
	if listErr != nil {
		return domain.Source{}, listErr
	}
	for _, existing := range sources {
		if existing.NormalizedURL != normalizedURL {
			continue
		}
		if existing.Status == domain.SourceStatusActive {
			return existing, nil
		}
		status := domain.SourceStatusActive
		return s.UpdateSource(ctx, UpdateSourceInput{
			UserID: input.UserID,
			ID:     existing.ID,
			Status: &status,
		})
	}
	return domain.Source{}, err
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

func uniquePositiveIDs(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	normalized := make([]int64, 0, len(values))
	for _, value := range values {
		if value < 1 {
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

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, rawValue := range values {
		value := strings.TrimSpace(rawValue)
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

func sourceImportErrorMessage(err error) string {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return "invalid source input"
	case errors.Is(err, domain.ErrConflict):
		return "source already exists"
	case errors.Is(err, domain.ErrNotFound):
		return "source not found"
	default:
		return err.Error()
	}
}

func truncateError(value string, maxLength int) string {
	value = strings.ToValidUTF8(value, "")
	if maxLength < 1 || len(value) <= maxLength {
		return value
	}
	end := 0
	for end < len(value) {
		_, size := utf8.DecodeRuneInString(value[end:])
		if size == 0 || end+size > maxLength {
			break
		}
		end += size
	}
	return value[:end]
}
