package handler

import (
	"context"
	"errors"
	"messagefeed/internal/domain"
	"messagefeed/internal/service"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const defaultUserID int64 = 1

type sourceService interface {
	CreateSource(ctx context.Context, input service.CreateSourceInput) (domain.Source, error)
	ListSources(ctx context.Context, userID int64) ([]domain.Source, error)
	UpdateSource(ctx context.Context, input service.UpdateSourceInput) (domain.Source, error)
	TriggerFetch(ctx context.Context, input service.FetchSourceInput) (service.FetchSourceResult, error)
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
		UserID:               defaultUserID,
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

	sources, err := h.service.ListSources(c.Request.Context(), defaultUserID)
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
		UserID:               defaultUserID,
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
		UserID: defaultUserID,
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

func writeSourceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid source input")
	case errors.Is(err, domain.ErrNotFound):
		Error(c, http.StatusNotFound, http.StatusNotFound, "source not found")
	case errors.Is(err, domain.ErrConflict):
		Error(c, http.StatusConflict, http.StatusConflict, "source already exists")
	default:
		Error(c, http.StatusInternalServerError, http.StatusInternalServerError, "source operation failed")
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
