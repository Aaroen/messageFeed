package handler

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"io"
	"messagefeed/internal/domain"
	"messagefeed/internal/service"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	xhtml "golang.org/x/net/html"
)

type timelineService interface {
	ListItems(ctx context.Context, input service.ListItemsInput) (service.ListItemsResult, error)
	GetItem(ctx context.Context, input service.GetItemInput) (domain.Item, error)
}

type recommendationService interface {
	ListRecommendations(ctx context.Context, input service.ListRecommendationsInput) (service.ListItemsResult, error)
}

type itemStateService interface {
	MarkRead(ctx context.Context, input service.UpdateItemStateInput) (domain.UserItemState, error)
	SetFavorite(ctx context.Context, input service.UpdateItemStateInput) (domain.UserItemState, error)
	SetHidden(ctx context.Context, input service.UpdateItemStateInput) (domain.UserItemState, error)
}

type itemHandler struct {
	timelineService       timelineService
	recommendationService recommendationService
	itemService           itemStateService
}

func registerPublicItemRoutes(router *gin.RouterGroup, timelineService timelineService, recommendationService recommendationService) {
	handler := itemHandler{timelineService: timelineService, recommendationService: recommendationService}
	router.GET("/items", handler.listItems)
	router.GET("/items/:id", handler.getItem)
	router.GET("/feed/timeline", handler.listItems)
	router.GET("/feed/recommendations", handler.listRecommendations)
}

func registerProtectedItemRoutes(router *gin.RouterGroup, itemService itemStateService) {
	handler := itemHandler{itemService: itemService}
	router.POST("/items/:id/read", handler.markRead)
	router.POST("/items/:id/favorite", handler.setFavorite)
	router.POST("/items/:id/hide", handler.setHidden)
}

type itemListResponse struct {
	Items  []itemResponse `json:"items"`
	Total  int64          `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

type itemResponse struct {
	ID             int64      `json:"id"`
	SourceID       int64      `json:"source_id"`
	SourceName     string     `json:"source_name,omitempty"`
	Title          string     `json:"title"`
	URL            string     `json:"url"`
	NormalizedURL  string     `json:"normalized_url"`
	RawGUID        string     `json:"raw_guid,omitempty"`
	ContentHash    string     `json:"content_hash,omitempty"`
	Summary        string     `json:"summary,omitempty"`
	ContentSnippet string     `json:"content_snippet,omitempty"`
	ContentText    string     `json:"content_text,omitempty"`
	ContentHTML    string     `json:"content_html,omitempty"`
	Author         string     `json:"author,omitempty"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
	FetchedAt      time.Time  `json:"fetched_at"`
	IsRead         bool       `json:"is_read"`
	ReadAt         *time.Time `json:"read_at,omitempty"`
	IsFavorite     bool       `json:"is_favorite"`
	FavoritedAt    *time.Time `json:"favorited_at,omitempty"`
	IsHidden       bool       `json:"is_hidden"`
	HiddenAt       *time.Time `json:"hidden_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type updateReadRequest struct {
	IsRead *bool `json:"is_read"`
}

type updateFavoriteRequest struct {
	IsFavorite *bool `json:"is_favorite"`
}

type updateHiddenRequest struct {
	IsHidden *bool `json:"is_hidden"`
}

type userItemStateResponse struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"user_id"`
	ItemID      int64      `json:"item_id"`
	IsRead      bool       `json:"is_read"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
	IsFavorite  bool       `json:"is_favorite"`
	FavoritedAt *time.Time `json:"favorited_at,omitempty"`
	IsHidden    bool       `json:"is_hidden"`
	HiddenAt    *time.Time `json:"hidden_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (h itemHandler) listItems(c *gin.Context) {
	if h.timelineService == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "timeline service unavailable")
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
	sourceID, err := optionalInt64Query(c, "source_id")
	if err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid source_id")
		return
	}
	isRead, err := optionalBoolQuery(c, "is_read")
	if err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid is_read")
		return
	}
	isFavorite, err := optionalBoolQuery(c, "is_favorite")
	if err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid is_favorite")
		return
	}
	isHidden, err := optionalBoolQuery(c, "is_hidden")
	if err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid is_hidden")
		return
	}
	includeHidden, err := optionalBoolQuery(c, "include_hidden")
	if err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid include_hidden")
		return
	}

	result, err := h.timelineService.ListItems(c.Request.Context(), service.ListItemsInput{
		UserID:        currentUserID(c),
		SourceID:      sourceID,
		IsRead:        isRead,
		IsFavorite:    isFavorite,
		IsHidden:      isHidden,
		IncludeHidden: includeHidden != nil && *includeHidden,
		Limit:         limit,
		Offset:        offset,
		Order:         c.Query("order"),
	})
	if err != nil {
		writeItemError(c, err)
		return
	}

	items := make([]itemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, itemResponseFromDomainForAuth(item, currentAuth(c).Authenticated))
	}
	Success(c, itemListResponse{
		Items:  items,
		Total:  result.Total,
		Limit:  result.Limit,
		Offset: result.Offset,
	})
}

func (h itemHandler) listRecommendations(c *gin.Context) {
	if h.recommendationService == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "recommendation service unavailable")
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
	sourceID, err := optionalInt64Query(c, "source_id")
	if err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid source_id")
		return
	}
	refresh, err := optionalBoolQuery(c, "refresh")
	if err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid refresh")
		return
	}

	result, err := h.recommendationService.ListRecommendations(c.Request.Context(), service.ListRecommendationsInput{
		UserID:   currentUserID(c),
		SourceID: sourceID,
		Limit:    limit,
		Offset:   offset,
		Order:    c.Query("order"),
		Refresh:  refresh != nil && *refresh,
	})
	if err != nil {
		writeItemError(c, err)
		return
	}

	items := make([]itemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, itemResponseFromDomainForAuth(item, currentAuth(c).Authenticated))
	}
	Success(c, itemListResponse{
		Items:  items,
		Total:  result.Total,
		Limit:  result.Limit,
		Offset: result.Offset,
	})
}

func (h itemHandler) getItem(c *gin.Context) {
	if h.timelineService == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "timeline service unavailable")
		return
	}
	itemID, ok := parseItemID(c)
	if !ok {
		return
	}

	item, err := h.timelineService.GetItem(c.Request.Context(), service.GetItemInput{
		UserID: currentUserID(c),
		ItemID: itemID,
	})
	if err != nil {
		writeItemError(c, err)
		return
	}
	Success(c, itemResponseFromDomainForAuth(item, currentAuth(c).Authenticated))
}

func (h itemHandler) markRead(c *gin.Context) {
	if h.itemService == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "item service unavailable")
		return
	}
	itemID, ok := parseItemID(c)
	if !ok {
		return
	}
	value, ok := readStateValue(c, "is_read")
	if !ok {
		return
	}
	h.updateItemState(c, itemID, value, h.itemService.MarkRead)
}

func (h itemHandler) setFavorite(c *gin.Context) {
	if h.itemService == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "item service unavailable")
		return
	}
	itemID, ok := parseItemID(c)
	if !ok {
		return
	}
	value, ok := readStateValue(c, "is_favorite")
	if !ok {
		return
	}
	h.updateItemState(c, itemID, value, h.itemService.SetFavorite)
}

func (h itemHandler) setHidden(c *gin.Context) {
	if h.itemService == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "item service unavailable")
		return
	}
	itemID, ok := parseItemID(c)
	if !ok {
		return
	}
	value, ok := readStateValue(c, "is_hidden")
	if !ok {
		return
	}
	h.updateItemState(c, itemID, value, h.itemService.SetHidden)
}

func (h itemHandler) updateItemState(
	c *gin.Context,
	itemID int64,
	value bool,
	update func(context.Context, service.UpdateItemStateInput) (domain.UserItemState, error),
) {
	state, err := update(c.Request.Context(), service.UpdateItemStateInput{
		UserID: currentUserID(c),
		ItemID: itemID,
		Value:  value,
	})
	if err != nil {
		writeItemError(c, err)
		return
	}
	Success(c, userItemStateResponseFromDomain(state))
}

func parseItemID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid item id")
		return 0, false
	}
	return id, true
}

func readStateValue(c *gin.Context, field string) (bool, bool) {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return true, true
	}

	var request map[string]*bool
	if err := json.NewDecoder(c.Request.Body).Decode(&request); err != nil {
		if errors.Is(err, io.EOF) {
			return true, true
		}
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return false, false
	}
	value, ok := request[field]
	if !ok || value == nil {
		return true, true
	}
	return *value, true
}

func optionalIntQuery(c *gin.Context, key string) (int, error) {
	value := c.Query(key)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func optionalInt64Query(c *gin.Context, key string) (int64, error) {
	value := c.Query(key)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func optionalBoolQuery(c *gin.Context, key string) (*bool, error) {
	value := c.Query(key)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func writeItemError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		RenderError(c, err, "invalid item query")
	case errors.Is(err, domain.ErrNotFound):
		RenderError(c, err, "item not found")
	default:
		RenderError(c, err, "item operation failed")
	}
}

func itemResponseFromDomain(item domain.Item) itemResponse {
	return itemResponseFromDomainForAuth(item, true)
}

func itemResponseFromDomainForAuth(item domain.Item, authenticated bool) itemResponse {
	if !authenticated {
		item.IsRead = false
		item.ReadAt = nil
		item.IsFavorite = false
		item.FavoritedAt = nil
		item.IsHidden = false
		item.HiddenAt = nil
	}
	return itemResponse{
		ID:             item.ID,
		SourceID:       item.SourceID,
		SourceName:     item.SourceName,
		Title:          item.Title,
		URL:            item.URL,
		NormalizedURL:  item.NormalizedURL,
		RawGUID:        item.RawGUID,
		ContentHash:    item.ContentHash,
		Summary:        item.Summary,
		ContentSnippet: item.ContentSnippet,
		ContentText:    plainTextFromHTML(item.ContentSnippet),
		ContentHTML:    item.ContentSnippet,
		Author:         item.Author,
		PublishedAt:    item.PublishedAt,
		FetchedAt:      item.FetchedAt,
		IsRead:         item.IsRead,
		ReadAt:         item.ReadAt,
		IsFavorite:     item.IsFavorite,
		FavoritedAt:    item.FavoritedAt,
		IsHidden:       item.IsHidden,
		HiddenAt:       item.HiddenAt,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

func plainTextFromHTML(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	tokenizer := xhtml.NewTokenizer(strings.NewReader(trimmed))
	var builder strings.Builder
	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case xhtml.ErrorToken:
			if builder.Len() == 0 {
				return strings.Join(strings.Fields(html.UnescapeString(trimmed)), " ")
			}
			return strings.Join(strings.Fields(builder.String()), " ")
		case xhtml.TextToken:
			text := strings.TrimSpace(html.UnescapeString(string(tokenizer.Text())))
			if text == "" {
				continue
			}
			if builder.Len() > 0 {
				builder.WriteByte(' ')
			}
			builder.WriteString(text)
		}
	}
}

func userItemStateResponseFromDomain(state domain.UserItemState) userItemStateResponse {
	return userItemStateResponse{
		ID:          state.ID,
		UserID:      state.UserID,
		ItemID:      state.ItemID,
		IsRead:      state.IsRead,
		ReadAt:      state.ReadAt,
		IsFavorite:  state.IsFavorite,
		FavoritedAt: state.FavoritedAt,
		IsHidden:    state.IsHidden,
		HiddenAt:    state.HiddenAt,
		CreatedAt:   state.CreatedAt,
		UpdatedAt:   state.UpdatedAt,
	}
}
