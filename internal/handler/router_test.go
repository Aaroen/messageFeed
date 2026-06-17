package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"messagefeed/internal/domain"
	appRuntime "messagefeed/internal/runtime"
	"messagefeed/internal/service"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestReadyzRoute(t *testing.T) {
	router := newTestRouter(t, RouterOptions{
		Now: func() time.Time {
			return time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response appRuntime.ReadinessReport
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Status != appRuntime.ReadinessReady {
		t.Fatalf("Status = %q, want %q", response.Status, appRuntime.ReadinessReady)
	}
	if got, want := len(response.Checks), 1; got != want {
		t.Fatalf("Checks length = %d, want %d", got, want)
	}
	if response.Checks[0].Name != "process" {
		t.Fatalf("process check name = %q", response.Checks[0].Name)
	}
}

func TestRuntimeNodeRoute(t *testing.T) {
	startedAt := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	nodeInfo := appRuntime.NodeInfo{
		NodeID:            "node-a",
		DeploymentMode:    "single_node",
		PublicBaseURL:     "http://127.0.0.1:60001",
		BindAddr:          "127.0.0.1:60001",
		TrustedProxyCIDRs: []string{"100.64.0.0/10"},
		StartedAt:         startedAt,
	}
	router := newTestRouter(t, RouterOptions{NodeInfo: nodeInfo})

	request := httptest.NewRequest(http.MethodGet, "/api/runtime/node", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response appRuntime.NodeInfo
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.NodeID != nodeInfo.NodeID {
		t.Fatalf("NodeID = %q, want %q", response.NodeID, nodeInfo.NodeID)
	}
	if !response.StartedAt.Equal(startedAt) {
		t.Fatalf("StartedAt = %s, want %s", response.StartedAt, startedAt)
	}
}

func TestBasicRoutes(t *testing.T) {
	router := newTestRouter(t, RouterOptions{})

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   map[string]string
	}{
		{
			name:       "root",
			path:       "/",
			wantStatus: http.StatusOK,
			wantBody: map[string]string{
				"service": serviceName,
				"status":  "ok",
			},
		},
		{
			name:       "healthz",
			path:       "/healthz",
			wantStatus: http.StatusOK,
			wantBody: map[string]string{
				"status": "ok",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status code = %d, want %d", recorder.Code, tt.wantStatus)
			}

			var response map[string]string
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			for key, want := range tt.wantBody {
				if got := response[key]; got != want {
					t.Fatalf("response[%q] = %q, want %q", key, got, want)
				}
			}
		})
	}
}

func TestNoRouteUsesUnifiedErrorResponse(t *testing.T) {
	router := newTestRouter(t, RouterOptions{})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil)
	request.Header.Set(requestIDHeader, "test-request-id")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if got := recorder.Header().Get(requestIDHeader); got != "test-request-id" {
		t.Fatalf("response request id header = %q, want %q", got, "test-request-id")
	}

	var response APIResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != http.StatusNotFound {
		t.Fatalf("Code = %d, want %d", response.Code, http.StatusNotFound)
	}
	if response.Message != "not found" {
		t.Fatalf("Message = %q, want %q", response.Message, "not found")
	}
	if response.RequestID != "test-request-id" {
		t.Fatalf("RequestID = %q, want %q", response.RequestID, "test-request-id")
	}
}

func TestCORSPreflightAllowsViteOrigin(t *testing.T) {
	router := newTestRouter(t, RouterOptions{})

	request := httptest.NewRequest(http.MethodOptions, "/api/v1/items", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	request.Header.Set("Access-Control-Request-Headers", "Content-Type")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want http://localhost:5173", got)
	}
}

func TestSourceRoutesRequireConfiguredService(t *testing.T) {
	router := newTestRouter(t, RouterOptions{})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestSourceRoutes(t *testing.T) {
	fakeService := &fakeSourceService{
		sources: []domain.Source{
			testSource(1, "Existing", "https://example.com/feed.xml"),
		},
	}
	router := newTestRouter(t, RouterOptions{SourceService: fakeService})

	createRequest := `{"name":"Created","url":"https://created.example/feed.xml","tags":["go"],"weight":2}`
	createHTTPReq := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(createRequest))
	createHTTPReq.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, createHTTPReq)

	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status code = %d, want %d", createRecorder.Code, http.StatusCreated)
	}

	var createResponse struct {
		Code int            `json:"code"`
		Data sourceResponse `json:"data"`
	}
	if err := json.NewDecoder(createRecorder.Body).Decode(&createResponse); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createResponse.Data.Name != "Created" {
		t.Fatalf("created source name = %q", createResponse.Data.Name)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, listRequest)

	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status code = %d, want %d", listRecorder.Code, http.StatusOK)
	}

	var listResponse struct {
		Code int              `json:"code"`
		Data []sourceResponse `json:"data"`
	}
	if err := json.NewDecoder(listRecorder.Body).Decode(&listResponse); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if got, want := len(listResponse.Data), 2; got != want {
		t.Fatalf("list length = %d, want %d", got, want)
	}

	updateRequest := `{"name":"Updated","status":"inactive"}`
	updateHTTPReq := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/1", strings.NewReader(updateRequest))
	updateHTTPReq.Header.Set("Content-Type", "application/json")
	updateRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRecorder, updateHTTPReq)

	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update status code = %d, want %d", updateRecorder.Code, http.StatusOK)
	}

	var updateResponse struct {
		Code int            `json:"code"`
		Data sourceResponse `json:"data"`
	}
	if err := json.NewDecoder(updateRecorder.Body).Decode(&updateResponse); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updateResponse.Data.Status != string(domain.SourceStatusInactive) {
		t.Fatalf("updated status = %q", updateResponse.Data.Status)
	}

	fetchHTTPReq := httptest.NewRequest(http.MethodPost, "/api/v1/sources/1/fetch", nil)
	fetchRecorder := httptest.NewRecorder()
	router.ServeHTTP(fetchRecorder, fetchHTTPReq)

	if fetchRecorder.Code != http.StatusOK {
		t.Fatalf("fetch status code = %d, want %d", fetchRecorder.Code, http.StatusOK)
	}

	var fetchResponse struct {
		Code int                 `json:"code"`
		Data fetchSourceResponse `json:"data"`
	}
	if err := json.NewDecoder(fetchRecorder.Body).Decode(&fetchResponse); err != nil {
		t.Fatalf("decode fetch response: %v", err)
	}
	if fetchResponse.Data.ItemCount != 1 {
		t.Fatalf("fetch item count = %d, want 1", fetchResponse.Data.ItemCount)
	}
}

func TestItemRoutesRequireConfiguredService(t *testing.T) {
	router := newTestRouter(t, RouterOptions{})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/items", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestItemRoutes(t *testing.T) {
	fakeService := &fakeTimelineService{
		items: []domain.Item{
			testItem(1, "First"),
			testItem(2, "Second"),
		},
	}
	fakeItemService := &fakeItemStateService{}
	router := newTestRouter(t, RouterOptions{
		TimelineService: fakeService,
		ItemService:     fakeItemService,
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/items?limit=2&offset=0&source_id=1&is_read=false&is_favorite=true&include_hidden=true", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("items status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response struct {
		Code int              `json:"code"`
		Data itemListResponse `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode items response: %v", err)
	}
	if got, want := len(response.Data.Items), 2; got != want {
		t.Fatalf("items length = %d, want %d", got, want)
	}
	if response.Data.Total != 2 {
		t.Fatalf("total = %d, want 2", response.Data.Total)
	}
	if fakeService.input.SourceID != 1 {
		t.Fatalf("SourceID = %d, want 1", fakeService.input.SourceID)
	}
	if fakeService.input.IsRead == nil || *fakeService.input.IsRead {
		t.Fatalf("IsRead = %#v, want false", fakeService.input.IsRead)
	}
	if fakeService.input.IsFavorite == nil || !*fakeService.input.IsFavorite {
		t.Fatalf("IsFavorite = %#v, want true", fakeService.input.IsFavorite)
	}
	if !fakeService.input.IncludeHidden {
		t.Fatal("IncludeHidden = false, want true")
	}

	timelineRequest := httptest.NewRequest(http.MethodGet, "/api/v1/feed/timeline", nil)
	timelineRecorder := httptest.NewRecorder()
	router.ServeHTTP(timelineRecorder, timelineRequest)

	if timelineRecorder.Code != http.StatusOK {
		t.Fatalf("timeline status code = %d, want %d", timelineRecorder.Code, http.StatusOK)
	}

	detailRequest := httptest.NewRequest(http.MethodGet, "/api/v1/items/1", nil)
	detailRecorder := httptest.NewRecorder()
	router.ServeHTTP(detailRecorder, detailRequest)

	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("detail status code = %d, want %d", detailRecorder.Code, http.StatusOK)
	}
	var detailResponse struct {
		Code int          `json:"code"`
		Data itemResponse `json:"data"`
	}
	if err := json.NewDecoder(detailRecorder.Body).Decode(&detailResponse); err != nil {
		t.Fatalf("decode detail response: %v", err)
	}
	if detailResponse.Data.SourceName != "Source" {
		t.Fatalf("SourceName = %q, want Source", detailResponse.Data.SourceName)
	}
	if detailResponse.Data.ContentText != "Hello world" {
		t.Fatalf("ContentText = %q, want Hello world", detailResponse.Data.ContentText)
	}
	if !detailResponse.Data.IsRead {
		t.Fatal("IsRead = false, want true")
	}

	readRequest := httptest.NewRequest(http.MethodPost, "/api/v1/items/1/read", nil)
	readRecorder := httptest.NewRecorder()
	router.ServeHTTP(readRecorder, readRequest)

	if readRecorder.Code != http.StatusOK {
		t.Fatalf("read status code = %d, want %d", readRecorder.Code, http.StatusOK)
	}

	var readResponse struct {
		Code int                   `json:"code"`
		Data userItemStateResponse `json:"data"`
	}
	if err := json.NewDecoder(readRecorder.Body).Decode(&readResponse); err != nil {
		t.Fatalf("decode read response: %v", err)
	}
	if !readResponse.Data.IsRead {
		t.Fatal("IsRead = false, want true")
	}

	favoriteRequest := httptest.NewRequest(http.MethodPost, "/api/v1/items/1/favorite", strings.NewReader(`{"is_favorite":false}`))
	favoriteRequest.Header.Set("Content-Type", "application/json")
	favoriteRecorder := httptest.NewRecorder()
	router.ServeHTTP(favoriteRecorder, favoriteRequest)

	if favoriteRecorder.Code != http.StatusOK {
		t.Fatalf("favorite status code = %d, want %d", favoriteRecorder.Code, http.StatusOK)
	}

	hideRequest := httptest.NewRequest(http.MethodPost, "/api/v1/items/1/hide", nil)
	hideRecorder := httptest.NewRecorder()
	router.ServeHTTP(hideRecorder, hideRequest)

	if hideRecorder.Code != http.StatusOK {
		t.Fatalf("hide status code = %d, want %d", hideRecorder.Code, http.StatusOK)
	}
}

func TestFeedViewRoutes(t *testing.T) {
	fakeService := &fakeFeedViewService{}
	router := newTestRouter(t, RouterOptions{FeedViewService: fakeService})

	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/feed/view-mode", nil)
	getRecorder := httptest.NewRecorder()
	router.ServeHTTP(getRecorder, getRequest)

	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get status code = %d, want %d", getRecorder.Code, http.StatusOK)
	}

	var getResponse struct {
		Code int                        `json:"code"`
		Data feedViewPreferenceResponse `json:"data"`
	}
	if err := json.NewDecoder(getRecorder.Body).Decode(&getResponse); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if getResponse.Data.ViewMode != domain.FeedViewModeTimeline {
		t.Fatalf("ViewMode = %q, want %q", getResponse.Data.ViewMode, domain.FeedViewModeTimeline)
	}

	putRequest := httptest.NewRequest(http.MethodPut, "/api/v1/feed/view-mode", strings.NewReader(`{"view_mode":"timeline"}`))
	putRequest.Header.Set("Content-Type", "application/json")
	putRecorder := httptest.NewRecorder()
	router.ServeHTTP(putRecorder, putRequest)

	if putRecorder.Code != http.StatusOK {
		t.Fatalf("put status code = %d, want %d", putRecorder.Code, http.StatusOK)
	}
	if fakeService.saved.ViewMode != domain.FeedViewModeTimeline {
		t.Fatalf("saved ViewMode = %q, want %q", fakeService.saved.ViewMode, domain.FeedViewModeTimeline)
	}
}

func newTestRouter(t *testing.T, options RouterOptions) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	options.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRouter(options)
}

type fakeSourceService struct {
	nextID  int64
	sources []domain.Source
}

func (s *fakeSourceService) CreateSource(_ context.Context, input service.CreateSourceInput) (domain.Source, error) {
	if s.nextID == 0 {
		s.nextID = int64(len(s.sources) + 1)
	}
	source := testSource(s.nextID, input.Name, input.URL)
	source.UserID = input.UserID
	source.Tags = append([]string(nil), input.Tags...)
	source.Weight = input.Weight
	s.nextID++
	s.sources = append(s.sources, source)
	return source, nil
}

func (s *fakeSourceService) ListSources(_ context.Context, _ int64) ([]domain.Source, error) {
	return append([]domain.Source(nil), s.sources...), nil
}

func (s *fakeSourceService) UpdateSource(_ context.Context, input service.UpdateSourceInput) (domain.Source, error) {
	for i, source := range s.sources {
		if source.ID != input.ID {
			continue
		}
		if input.Name != nil {
			source.Name = *input.Name
		}
		if input.Status != nil {
			source.Status = *input.Status
		}
		s.sources[i] = source
		return source, nil
	}
	return domain.Source{}, domain.ErrNotFound
}

func (s *fakeSourceService) TriggerFetch(_ context.Context, input service.FetchSourceInput) (service.FetchSourceResult, error) {
	for _, source := range s.sources {
		if source.ID != input.ID {
			continue
		}
		source.LastFetchStatus = domain.SourceLastFetchStatusSuccess
		itemCount := 1
		source.LastFetchItemCount = &itemCount
		return service.FetchSourceResult{
			Source:       source,
			ItemCount:    1,
			CreatedCount: 1,
		}, nil
	}
	return service.FetchSourceResult{}, domain.ErrNotFound
}

func testSource(id int64, name string, rawURL string) domain.Source {
	now := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	return domain.Source{
		ID:                   id,
		UserID:               defaultUserID,
		Name:                 name,
		Type:                 domain.SourceTypeRSS,
		URL:                  rawURL,
		NormalizedURL:        rawURL,
		Status:               domain.SourceStatusActive,
		FetchIntervalSeconds: service.DefaultSourceFetchIntervalSeconds,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

type fakeTimelineService struct {
	input service.ListItemsInput
	items []domain.Item
}

func (s *fakeTimelineService) ListItems(_ context.Context, input service.ListItemsInput) (service.ListItemsResult, error) {
	s.input = input
	return service.ListItemsResult{
		Items:  append([]domain.Item(nil), s.items...),
		Total:  int64(len(s.items)),
		Limit:  input.Limit,
		Offset: input.Offset,
	}, nil
}

func (s *fakeTimelineService) GetItem(_ context.Context, input service.GetItemInput) (domain.Item, error) {
	for _, item := range s.items {
		if item.ID == input.ItemID {
			return item, nil
		}
	}
	return domain.Item{}, domain.ErrNotFound
}

func testItem(id int64, title string) domain.Item {
	now := time.Date(2026, 6, 17, 9, 0, 0, 0, time.UTC)
	return domain.Item{
		ID:             id,
		SourceID:       1,
		SourceName:     "Source",
		Title:          title,
		URL:            "https://example.com/item",
		NormalizedURL:  "https://example.com/item",
		ContentSnippet: "<p>Hello <strong>world</strong></p>",
		IsRead:         true,
		FetchedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

type fakeItemStateService struct{}

func (s *fakeItemStateService) MarkRead(_ context.Context, input service.UpdateItemStateInput) (domain.UserItemState, error) {
	return testUserItemState(input.ItemID, input.Value, false, false), nil
}

func (s *fakeItemStateService) SetFavorite(_ context.Context, input service.UpdateItemStateInput) (domain.UserItemState, error) {
	return testUserItemState(input.ItemID, false, input.Value, false), nil
}

func (s *fakeItemStateService) SetHidden(_ context.Context, input service.UpdateItemStateInput) (domain.UserItemState, error) {
	return testUserItemState(input.ItemID, false, false, input.Value), nil
}

func testUserItemState(itemID int64, isRead bool, isFavorite bool, isHidden bool) domain.UserItemState {
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	state := domain.UserItemState{
		ID:         1,
		UserID:     defaultUserID,
		ItemID:     itemID,
		IsRead:     isRead,
		IsFavorite: isFavorite,
		IsHidden:   isHidden,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if isRead {
		state.ReadAt = &now
	}
	if isFavorite {
		state.FavoritedAt = &now
	}
	if isHidden {
		state.HiddenAt = &now
	}
	return state
}

type fakeFeedViewService struct {
	saved domain.FeedViewPreference
}

func (s *fakeFeedViewService) GetPreference(_ context.Context, input service.GetFeedViewPreferenceInput) (domain.FeedViewPreference, error) {
	now := time.Date(2026, 6, 17, 11, 0, 0, 0, time.UTC)
	return domain.FeedViewPreference{
		ID:        1,
		UserID:    input.UserID,
		ViewMode:  domain.FeedViewModeTimeline,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (s *fakeFeedViewService) SavePreference(_ context.Context, input service.SaveFeedViewPreferenceInput) (domain.FeedViewPreference, error) {
	now := time.Date(2026, 6, 17, 11, 0, 0, 0, time.UTC)
	s.saved = domain.FeedViewPreference{
		ID:        1,
		UserID:    input.UserID,
		ViewMode:  input.ViewMode,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return s.saved, nil
}
