package handler

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"messagefeed/internal/domain"
	"messagefeed/internal/service"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type sourceService interface {
	CreateSource(ctx context.Context, input service.CreateSourceInput) (domain.Source, error)
	ListSources(ctx context.Context, userID int64) ([]domain.Source, error)
	ListSourceCatalog(ctx context.Context, input service.ListSourceCatalogInput) (service.ListSourceCatalogResult, error)
	ImportCatalogSources(ctx context.Context, input service.ImportCatalogSourcesInput) (service.ImportSourceResult, error)
	ImportURLSources(ctx context.Context, input service.ImportURLSourcesInput) (service.ImportSourceResult, error)
	ListSourceImportJobs(ctx context.Context, input service.ListSourceImportJobsInput) (service.ListSourceImportJobsResult, error)
	UpdateSource(ctx context.Context, input service.UpdateSourceInput) (domain.Source, error)
	TriggerFetch(ctx context.Context, input service.FetchSourceInput) (service.FetchSourceResult, error)
	FetchActiveSources(ctx context.Context, input service.FetchActiveSourcesInput) (service.FetchSourcesResult, error)
}

type sourceHandler struct {
	service sourceService
}

func registerSourceRoutes(router *gin.RouterGroup, service sourceService) {
	handler := sourceHandler{service: service}
	router.POST("/sources", handler.createSource)
	router.GET("/sources", handler.listSources)
	router.PATCH("/sources/:id", handler.updateSource)
	router.POST("/sources/:id/fetch", handler.fetchSource)
	router.POST("/source-fetches", handler.fetchActiveSources)
	router.GET("/source-catalogs", handler.listSourceCatalog)
	router.GET("/source-catalogs/search", handler.searchSourceCatalog)
	router.POST("/sources/import/catalog", handler.importCatalogSources)
	router.POST("/sources/import/urls", handler.importURLSources)
	router.POST("/sources/import/opml", handler.importOPMLSources)
	router.GET("/sources/import-jobs", handler.listSourceImportJobs)
}

type createSourceRequest struct {
	Name                 string   `json:"name"`
	Type                 string   `json:"type"`
	URL                  string   `json:"url" binding:"required"`
	FetchIntervalSeconds int      `json:"fetch_interval_seconds"`
	Tags                 []string `json:"tags"`
	Weight               int      `json:"weight"`
}

type updateSourceRequest struct {
	Name                 *string   `json:"name"`
	Type                 *string   `json:"type"`
	URL                  *string   `json:"url"`
	Status               *string   `json:"status"`
	FetchIntervalSeconds *int      `json:"fetch_interval_seconds"`
	Tags                 *[]string `json:"tags"`
	Weight               *int      `json:"weight"`
}

type sourceResponse struct {
	ID                   int64      `json:"id"`
	UserID               int64      `json:"user_id"`
	Name                 string     `json:"name"`
	Type                 string     `json:"type"`
	URL                  string     `json:"url"`
	NormalizedURL        string     `json:"normalized_url"`
	Status               string     `json:"status"`
	FetchIntervalSeconds int        `json:"fetch_interval_seconds"`
	Tags                 []string   `json:"tags"`
	Weight               int        `json:"weight"`
	LastFetchedAt        *time.Time `json:"last_fetched_at,omitempty"`
	LastFetchStatus      string     `json:"last_fetch_status,omitempty"`
	LastFetchError       string     `json:"last_fetch_error,omitempty"`
	LastFetchDurationMS  *int       `json:"last_fetch_duration_ms,omitempty"`
	LastFetchItemCount   *int       `json:"last_fetch_item_count,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type fetchSourceResponse struct {
	Source       sourceResponse `json:"source"`
	ItemCount    int            `json:"item_count"`
	CreatedCount int            `json:"created_count"`
	UpdatedCount int            `json:"updated_count"`
}

type fetchSourcesResponse struct {
	RequestedCount int                        `json:"requested_count"`
	SuccessCount   int                        `json:"success_count"`
	FailureCount   int                        `json:"failure_count"`
	Sources        []sourceResponse           `json:"sources"`
	Errors         []fetchSourceErrorResponse `json:"errors"`
}

type fetchSourceErrorResponse struct {
	SourceID   int64  `json:"source_id"`
	SourceName string `json:"source_name"`
	Message    string `json:"message"`
}

type sourceCatalogListResponse struct {
	Entries []sourceCatalogResponse `json:"entries"`
	Total   int64                   `json:"total"`
	Limit   int                     `json:"limit"`
	Offset  int                     `json:"offset"`
}

type sourceCatalogResponse struct {
	ID             int64      `json:"id"`
	SourceKey      string     `json:"source_key"`
	Name           string     `json:"name"`
	SiteURL        string     `json:"site_url,omitempty"`
	FeedURL        string     `json:"feed_url"`
	NormalizedURL  string     `json:"normalized_url"`
	Type           string     `json:"type"`
	Category       string     `json:"category"`
	Tags           []string   `json:"tags"`
	Language       string     `json:"language"`
	Country        string     `json:"country,omitempty"`
	Official       bool       `json:"official"`
	SourceOrigin   string     `json:"source_origin"`
	HealthStatus   string     `json:"health_status"`
	LastCheckedAt  *time.Time `json:"last_checked_at,omitempty"`
	LastCheckError string     `json:"last_check_error,omitempty"`
	Subscribed     bool       `json:"subscribed"`
	SourceID       int64      `json:"source_id,omitempty"`
	SourceStatus   string     `json:"source_status,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type importCatalogRequest struct {
	CatalogIDs []int64 `json:"catalog_ids"`
}

type importURLsRequest struct {
	URLs []string `json:"urls"`
}

type opmlDocument struct {
	Outlines []opmlOutline `xml:"body>outline"`
}

type opmlOutline struct {
	XMLURL   string        `xml:"xmlUrl,attr"`
	HTMLURL  string        `xml:"htmlUrl,attr"`
	Title    string        `xml:"title,attr"`
	Text     string        `xml:"text,attr"`
	Outlines []opmlOutline `xml:"outline"`
}

type importSourceErrorResponse struct {
	Reference string `json:"reference"`
	Message   string `json:"message"`
}

type importSourceResponse struct {
	RequestedCount int                         `json:"requested_count"`
	SuccessCount   int                         `json:"success_count"`
	FailureCount   int                         `json:"failure_count"`
	Sources        []sourceResponse            `json:"sources"`
	Errors         []importSourceErrorResponse `json:"errors"`
	ImportJob      *sourceImportJobResponse    `json:"import_job,omitempty"`
}

type sourceImportJobResponse struct {
	ID             int64                       `json:"id"`
	UserID         int64                       `json:"user_id"`
	ImportType     string                      `json:"import_type"`
	Status         string                      `json:"status"`
	RequestedCount int                         `json:"requested_count"`
	SuccessCount   int                         `json:"success_count"`
	FailureCount   int                         `json:"failure_count"`
	ErrorDetails   []importSourceErrorResponse `json:"error_details"`
	CreatedAt      time.Time                   `json:"created_at"`
	UpdatedAt      time.Time                   `json:"updated_at"`
}

type sourceImportJobListResponse struct {
	Jobs   []sourceImportJobResponse `json:"jobs"`
	Total  int64                     `json:"total"`
	Limit  int                       `json:"limit"`
	Offset int                       `json:"offset"`
}

func (h sourceHandler) createSource(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "source service unavailable")
		return
	}

	var request createSourceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}

	source, err := h.service.CreateSource(c.Request.Context(), service.CreateSourceInput{
		UserID:               currentUserID(c),
		Name:                 request.Name,
		Type:                 domain.SourceType(request.Type),
		URL:                  request.URL,
		FetchIntervalSeconds: request.FetchIntervalSeconds,
		Tags:                 request.Tags,
		Weight:               request.Weight,
	})
	if err != nil {
		writeSourceError(c, err)
		return
	}

	Created(c, sourceResponseFromDomain(source))
}

func (h sourceHandler) listSources(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "source service unavailable")
		return
	}

	sources, err := h.service.ListSources(c.Request.Context(), currentUserID(c))
	if err != nil {
		writeSourceError(c, err)
		return
	}

	response := make([]sourceResponse, 0, len(sources))
	for _, source := range sources {
		response = append(response, sourceResponseFromDomain(source))
	}
	Success(c, response)
}

func (h sourceHandler) updateSource(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "source service unavailable")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid source id")
		return
	}

	var request updateSourceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}

	var sourceType *domain.SourceType
	if request.Type != nil {
		value := domain.SourceType(*request.Type)
		sourceType = &value
	}

	var sourceStatus *domain.SourceStatus
	if request.Status != nil {
		value := domain.SourceStatus(*request.Status)
		sourceStatus = &value
	}

	source, err := h.service.UpdateSource(c.Request.Context(), service.UpdateSourceInput{
		UserID:               currentUserID(c),
		ID:                   id,
		Name:                 request.Name,
		Type:                 sourceType,
		URL:                  request.URL,
		Status:               sourceStatus,
		FetchIntervalSeconds: request.FetchIntervalSeconds,
		Tags:                 request.Tags,
		Weight:               request.Weight,
	})
	if err != nil {
		writeSourceError(c, err)
		return
	}

	Success(c, sourceResponseFromDomain(source))
}

func (h sourceHandler) fetchSource(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "source service unavailable")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid source id")
		return
	}

	result, err := h.service.TriggerFetch(c.Request.Context(), service.FetchSourceInput{
		UserID: currentUserID(c),
		ID:     id,
	})
	if err != nil {
		writeSourceError(c, err)
		return
	}

	Success(c, fetchSourceResponse{
		Source:       sourceResponseFromDomain(result.Source),
		ItemCount:    result.ItemCount,
		CreatedCount: result.CreatedCount,
		UpdatedCount: result.UpdatedCount,
	})
}

func (h sourceHandler) fetchActiveSources(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "source service unavailable")
		return
	}

	result, err := h.service.FetchActiveSources(c.Request.Context(), service.FetchActiveSourcesInput{
		UserID: currentUserID(c),
	})
	if err != nil {
		writeSourceError(c, err)
		return
	}
	response := fetchSourcesResponse{
		RequestedCount: result.RequestedCount,
		SuccessCount:   result.SuccessCount,
		FailureCount:   result.FailureCount,
		Sources:        make([]sourceResponse, 0, len(result.Sources)),
		Errors:         make([]fetchSourceErrorResponse, 0, len(result.Errors)),
	}
	for _, source := range result.Sources {
		response.Sources = append(response.Sources, sourceResponseFromDomain(source))
	}
	for _, item := range result.Errors {
		response.Errors = append(response.Errors, fetchSourceErrorResponse{
			SourceID:   item.SourceID,
			SourceName: item.SourceName,
			Message:    item.Message,
		})
	}

	Success(c, response)
}

func (h sourceHandler) listSourceCatalog(c *gin.Context) {
	h.listSourceCatalogWithQuery(c, "")
}

func (h sourceHandler) searchSourceCatalog(c *gin.Context) {
	h.listSourceCatalogWithQuery(c, c.Query("q"))
}

func (h sourceHandler) listSourceCatalogWithQuery(c *gin.Context, query string) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "source catalog service unavailable")
		return
	}

	limit, err := optionalIntQuery(c, "limit")
	if err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid limit")
		return
	}
	offset, err := optionalIntQuery(c, "offset")
	if err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid offset")
		return
	}

	result, err := h.service.ListSourceCatalog(c.Request.Context(), service.ListSourceCatalogInput{
		UserID:   currentUserID(c),
		Category: c.Query("category"),
		Query:    query,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		writeSourceError(c, err)
		return
	}

	entries := make([]sourceCatalogResponse, 0, len(result.Entries))
	for _, entry := range result.Entries {
		entries = append(entries, sourceCatalogResponseFromDomain(entry))
	}
	Success(c, sourceCatalogListResponse{
		Entries: entries,
		Total:   result.Total,
		Limit:   result.Limit,
		Offset:  result.Offset,
	})
}

func (h sourceHandler) importCatalogSources(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "source service unavailable")
		return
	}
	var request importCatalogRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.ImportCatalogSources(c.Request.Context(), service.ImportCatalogSourcesInput{
		UserID:     currentUserID(c),
		CatalogIDs: request.CatalogIDs,
	})
	if err != nil {
		writeSourceError(c, err)
		return
	}
	Success(c, importSourceResponseFromDomain(result))
}

func (h sourceHandler) importURLSources(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "source service unavailable")
		return
	}
	var request importURLsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.ImportURLSources(c.Request.Context(), service.ImportURLSourcesInput{
		UserID:     currentUserID(c),
		URLs:       request.URLs,
		ImportType: domain.SourceImportTypeURLs,
	})
	if err != nil {
		writeSourceError(c, err)
		return
	}
	Success(c, importSourceResponseFromDomain(result))
}

func (h sourceHandler) importOPMLSources(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "source service unavailable")
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "opml file is required")
		return
	}
	defer file.Close()

	body, err := io.ReadAll(io.LimitReader(file, 2*1024*1024+1))
	if err != nil || len(body) > 2*1024*1024 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid opml file")
		return
	}

	urls, err := parseOPMLFeedURLs(body)
	if err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid opml content")
		return
	}
	result, err := h.service.ImportURLSources(c.Request.Context(), service.ImportURLSourcesInput{
		UserID:     currentUserID(c),
		URLs:       urls,
		ImportType: domain.SourceImportTypeOPML,
	})
	if err != nil {
		writeSourceError(c, err)
		return
	}
	Success(c, importSourceResponseFromDomain(result))
}

func (h sourceHandler) listSourceImportJobs(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "source service unavailable")
		return
	}

	limit, err := optionalIntQuery(c, "limit")
	if err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid limit")
		return
	}
	offset, err := optionalIntQuery(c, "offset")
	if err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid offset")
		return
	}

	result, err := h.service.ListSourceImportJobs(c.Request.Context(), service.ListSourceImportJobsInput{
		UserID: currentUserID(c),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeSourceError(c, err)
		return
	}

	jobs := make([]sourceImportJobResponse, 0, len(result.Jobs))
	for _, job := range result.Jobs {
		jobs = append(jobs, sourceImportJobResponseValueFromDomain(job))
	}
	Success(c, sourceImportJobListResponse{
		Jobs:   jobs,
		Total:  result.Total,
		Limit:  result.Limit,
		Offset: result.Offset,
	})
}

func writeSourceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		RenderError(c, err, "invalid source input")
	case errors.Is(err, domain.ErrNotFound):
		RenderError(c, err, "source not found")
	case errors.Is(err, domain.ErrConflict):
		RenderError(c, err, "source already exists")
	default:
		RenderError(c, err, "source operation failed")
	}
}

func sourceResponseFromDomain(source domain.Source) sourceResponse {
	return sourceResponse{
		ID:                   source.ID,
		UserID:               source.UserID,
		Name:                 source.Name,
		Type:                 string(source.Type),
		URL:                  source.URL,
		NormalizedURL:        source.NormalizedURL,
		Status:               string(source.Status),
		FetchIntervalSeconds: source.FetchIntervalSeconds,
		Tags:                 append([]string(nil), source.Tags...),
		Weight:               source.Weight,
		LastFetchedAt:        source.LastFetchedAt,
		LastFetchStatus:      source.LastFetchStatus,
		LastFetchError:       source.LastFetchError,
		LastFetchDurationMS:  source.LastFetchDurationMS,
		LastFetchItemCount:   source.LastFetchItemCount,
		CreatedAt:            source.CreatedAt,
		UpdatedAt:            source.UpdatedAt,
	}
}

func sourceCatalogResponseFromDomain(entry domain.SourceCatalogEntry) sourceCatalogResponse {
	return sourceCatalogResponse{
		ID:             entry.ID,
		SourceKey:      entry.SourceKey,
		Name:           entry.Name,
		SiteURL:        entry.SiteURL,
		FeedURL:        entry.FeedURL,
		NormalizedURL:  entry.NormalizedURL,
		Type:           string(entry.Type),
		Category:       entry.Category,
		Tags:           append([]string(nil), entry.Tags...),
		Language:       entry.Language,
		Country:        entry.Country,
		Official:       entry.Official,
		SourceOrigin:   entry.SourceOrigin,
		HealthStatus:   string(entry.HealthStatus),
		LastCheckedAt:  entry.LastCheckedAt,
		LastCheckError: entry.LastCheckError,
		Subscribed:     entry.Subscribed,
		SourceID:       entry.SourceID,
		SourceStatus:   string(entry.SourceStatus),
		CreatedAt:      entry.CreatedAt,
		UpdatedAt:      entry.UpdatedAt,
	}
}

func importSourceResponseFromDomain(result service.ImportSourceResult) importSourceResponse {
	sources := make([]sourceResponse, 0, len(result.Sources))
	for _, source := range result.Sources {
		sources = append(sources, sourceResponseFromDomain(source))
	}
	errors := make([]importSourceErrorResponse, 0, len(result.Errors))
	for _, item := range result.Errors {
		errors = append(errors, importSourceErrorResponse{Reference: item.Reference, Message: item.Message})
	}
	response := importSourceResponse{
		RequestedCount: result.RequestedCount,
		SuccessCount:   result.SuccessCount,
		FailureCount:   result.FailureCount,
		Sources:        sources,
		Errors:         errors,
	}
	if result.ImportJob != nil {
		response.ImportJob = sourceImportJobResponseFromDomain(*result.ImportJob)
	}
	return response
}

func sourceImportJobResponseFromDomain(job domain.SourceImportJob) *sourceImportJobResponse {
	response := sourceImportJobResponseValueFromDomain(job)
	return &response
}

func sourceImportJobResponseValueFromDomain(job domain.SourceImportJob) sourceImportJobResponse {
	errors := make([]importSourceErrorResponse, 0, len(job.ErrorDetails))
	for _, item := range job.ErrorDetails {
		errors = append(errors, importSourceErrorResponse{Reference: item.Reference, Message: item.Message})
	}
	return sourceImportJobResponse{
		ID:             job.ID,
		UserID:         job.UserID,
		ImportType:     string(job.ImportType),
		Status:         string(job.Status),
		RequestedCount: job.RequestedCount,
		SuccessCount:   job.SuccessCount,
		FailureCount:   job.FailureCount,
		ErrorDetails:   errors,
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      job.UpdatedAt,
	}
}

func parseOPMLFeedURLs(body []byte) ([]string, error) {
	var document opmlDocument
	if err := xml.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	var urls []string
	for _, outline := range document.Outlines {
		urls = collectOPMLOutlineURLs(urls, outline)
	}
	return urls, nil
}

func collectOPMLOutlineURLs(urls []string, outline opmlOutline) []string {
	if value := strings.TrimSpace(outline.XMLURL); value != "" {
		urls = append(urls, value)
	}
	for _, child := range outline.Outlines {
		urls = collectOPMLOutlineURLs(urls, child)
	}
	return urls
}
