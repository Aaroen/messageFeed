package handler

import (
	"context"
	"errors"
	"messagefeed/internal/domain"
	"messagefeed/internal/service"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type feedViewService interface {
	GetPreference(ctx context.Context, input service.GetFeedViewPreferenceInput) (domain.FeedViewPreference, error)
	SavePreference(ctx context.Context, input service.SaveFeedViewPreferenceInput) (domain.FeedViewPreference, error)
}

type feedViewHandler struct {
	service feedViewService
}

func registerFeedViewRoutes(router *gin.RouterGroup, service feedViewService) {
	handler := feedViewHandler{service: service}
	router.GET("/feed/view-mode", handler.getPreference)
	router.PUT("/feed/view-mode", handler.savePreference)
}

type saveFeedViewPreferenceRequest struct {
	ViewMode domain.FeedViewMode `json:"view_mode" binding:"required"`
}

type feedViewPreferenceResponse struct {
	ID        int64               `json:"id,omitempty"`
	UserID    int64               `json:"user_id"`
	ViewMode  domain.FeedViewMode `json:"view_mode"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
}

func (h feedViewHandler) getPreference(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "feed view service unavailable")
		return
	}
	preference, err := h.service.GetPreference(c.Request.Context(), service.GetFeedViewPreferenceInput{
		UserID: currentUserID(c),
	})
	if err != nil {
		writeFeedViewError(c, err)
		return
	}
	Success(c, feedViewPreferenceResponseFromDomain(preference))
}

func (h feedViewHandler) savePreference(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "feed view service unavailable")
		return
	}

	var request saveFeedViewPreferenceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}

	preference, err := h.service.SavePreference(c.Request.Context(), service.SaveFeedViewPreferenceInput{
		UserID:   currentUserID(c),
		ViewMode: request.ViewMode,
	})
	if err != nil {
		writeFeedViewError(c, err)
		return
	}
	Success(c, feedViewPreferenceResponseFromDomain(preference))
}

func writeFeedViewError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		RenderError(c, err, "invalid feed view preference")
	default:
		RenderError(c, err, "feed view operation failed")
	}
}

func feedViewPreferenceResponseFromDomain(preference domain.FeedViewPreference) feedViewPreferenceResponse {
	return feedViewPreferenceResponse{
		ID:        preference.ID,
		UserID:    preference.UserID,
		ViewMode:  preference.ViewMode,
		CreatedAt: preference.CreatedAt,
		UpdatedAt: preference.UpdatedAt,
	}
}
