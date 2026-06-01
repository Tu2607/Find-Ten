package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewServerReturnsHTTPHandler(t *testing.T) {
	var _ http.Handler = NewServer()
}

func TestServerRouteDispatch(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{
			name:       "health",
			method:     http.MethodGet,
			path:       "/health",
			wantStatus: http.StatusOK,
		},
		{
			name:       "create game placeholder",
			method:     http.MethodPost,
			path:       "/games",
			wantStatus: http.StatusNotImplemented,
		},
		{
			name:       "snapshots placeholder",
			method:     http.MethodGet,
			path:       "/games/test-game/snapshots",
			wantStatus: http.StatusNotImplemented,
		},
		{
			name:       "moves placeholder",
			method:     http.MethodPost,
			path:       "/games/test-game/moves",
			wantStatus: http.StatusNotImplemented,
		},
		{
			name:       "unknown route",
			method:     http.MethodGet,
			path:       "/missing",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "unsupported health method",
			method:     http.MethodPost,
			path:       "/health",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "unsupported create game method",
			method:     http.MethodGet,
			path:       "/games",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "unsupported snapshots method",
			method:     http.MethodPost,
			path:       "/games/test-game/snapshots",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "unsupported moves method",
			method:     http.MethodGet,
			path:       "/games/test-game/moves",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	server := NewServer()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}
