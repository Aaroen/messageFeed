package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	appRuntime "messagefeed/internal/runtime"
	"net/http"
	"net/http/httptest"
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

func newTestRouter(t *testing.T, options RouterOptions) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	options.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRouter(options)
}
