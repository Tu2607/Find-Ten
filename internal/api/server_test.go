package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
			name:       "create game",
			method:     http.MethodPost,
			path:       "/games",
			wantStatus: http.StatusCreated,
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

	server := NewServer().(*Server)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := strings.NewReader("")
			if test.method == http.MethodPost && test.path == "/games" {
				body = strings.NewReader(`{"size":9}`)
			}
			request := httptest.NewRequest(test.method, test.path, body)
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.method == http.MethodPost && test.path == "/games" && response.Code == http.StatusCreated {
				stopCreatedGame(t, server, response)
			}
		})
	}
}

func TestCreateGame(t *testing.T) {
	server := NewServer().(*Server)
	request := httptest.NewRequest(http.MethodPost, "/games", strings.NewReader(`{"size":9}`))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}

	var body createGameResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.GameID == "" {
		t.Fatal("GameID is empty, want generated ID")
	}
	if body.InitialSnapshot.Sequence != 1 {
		t.Fatalf("InitialSnapshot.Sequence = %d, want 1", body.InitialSnapshot.Sequence)
	}
	if body.ExpiresAt.IsZero() {
		t.Fatal("ExpiresAt is zero, want session deadline")
	}
	session, ok := server.store.get(body.GameID)
	if !ok {
		t.Fatalf("created session with ID %q was not stored", body.GameID)
	}
	session.Stop()
}

func TestCreateGameUsesRequestedBoardSize(t *testing.T) {
	server := NewServer().(*Server)
	request := httptest.NewRequest(http.MethodPost, "/games", strings.NewReader(`{"size":10}`))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}

	var body createGameResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	session, ok := server.store.get(body.GameID)
	if !ok {
		t.Fatalf("created session with ID %q was not stored", body.GameID)
	}
	session.Stop()

	if len(body.InitialSnapshot.Board) != 10 {
		t.Fatalf("len(board) = %d, want 10", len(body.InitialSnapshot.Board))
	}
	for rowIndex, row := range body.InitialSnapshot.Board {
		if len(row) != 10 {
			t.Fatalf("len(board[%d]) = %d, want 10", rowIndex, len(row))
		}
	}
}

func stopCreatedGame(t *testing.T, server *Server, response *httptest.ResponseRecorder) {
	t.Helper()

	var body createGameResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode create-game response: %v", err)
	}
	session, ok := server.store.get(body.GameID)
	if !ok {
		t.Fatalf("created session with ID %q was not stored", body.GameID)
	}
	session.Stop()
}

func TestCreateGameBadRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "invalid JSON",
			body: `{"size":`,
		},
		{
			name: "empty body",
			body: "",
		},
		{
			name: "missing size",
			body: `{}`,
		},
		{
			name: "unsupported size",
			body: `{"size":12}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := NewServer()
			request := httptest.NewRequest(http.MethodPost, "/games", strings.NewReader(test.body))
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}
