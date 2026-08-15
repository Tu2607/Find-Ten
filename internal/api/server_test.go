package api

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"find-ten-game/internal/game"
	"find-ten-game/internal/leaderboard"
	"find-ten-game/internal/player"
	"find-ten-game/internal/sqlitedb"
)

func TestNewServerReturnsHTTPHandler(t *testing.T) {
	var _ http.Handler = newTestServer(t)
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	server, _ := newTestServerWithDatabase(t)
	return server
}

func newTestServerWithDatabase(t *testing.T) (*Server, *sql.DB) {
	t.Helper()

	db, err := sqlitedb.Open(context.Background(), filepath.Join(t.TempDir(), "find-ten.db"))
	if err != nil {
		t.Fatalf("sqlitedb.Open failed: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("database Close failed: %v", err)
		}
	})

	playerStore, err := player.NewStore(context.Background(), db)
	if err != nil {
		t.Fatalf("player.NewStore failed: %v", err)
	}
	leaderboardStore, err := leaderboard.NewStore(context.Background(), db)
	if err != nil {
		t.Fatalf("leaderboard.NewStore failed: %v", err)
	}

	server, ok := NewServer(leaderboardStore, playerStore).(*Server)
	if !ok {
		t.Fatal("NewServer did not return *Server")
	}

	return server, db
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
			name:       "snapshots unknown game",
			method:     http.MethodGet,
			path:       "/games/test-game/snapshots",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "moves unknown game",
			method:     http.MethodPost,
			path:       "/games/test-game/moves",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "reshuffle unknown game",
			method:     http.MethodPost,
			path:       "/games/test-game/reshuffle",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "remove number unknown game",
			method:     http.MethodPost,
			path:       "/games/test-game/remove-number",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "hint unknown game",
			method:     http.MethodPost,
			path:       "/games/test-game/hint",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "delete unknown game",
			method:     http.MethodDelete,
			path:       "/games/test-game",
			wantStatus: http.StatusNotFound,
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
		{
			name:       "unsupported reshuffle method",
			method:     http.MethodGet,
			path:       "/games/test-game/reshuffle",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "unsupported remove number method",
			method:     http.MethodGet,
			path:       "/games/test-game/remove-number",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "unsupported hint method",
			method:     http.MethodGet,
			path:       "/games/test-game/hint",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "top scores default",
			method:     http.MethodGet,
			path:       "/scores?gridSize=9&duration=120",
			wantStatus: http.StatusOK,
		},
	}

	server := newTestServer(t)
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

func TestStaticFiles(t *testing.T) {
	t.Chdir("../..")

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "index",
			path:       "/",
			wantStatus: http.StatusOK,
		},
		{
			name:       "styles",
			path:       "/styles.css",
			wantStatus: http.StatusOK,
		},
		{
			name:       "app",
			path:       "/app.js",
			wantStatus: http.StatusOK,
		},
	}

	server := newTestServer(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}

func TestDecodeJSONBodyBounds(t *testing.T) {
	validJSON := `{"size":9}`
	tests := []struct {
		name       string
		body       string
		wantOK     bool
		wantStatus int
		wantBody   string
	}{
		{
			name:   "exactly at limit",
			body:   validJSON + strings.Repeat(" ", int(maxJSONBodyBytes)-len(validJSON)),
			wantOK: true,
		},
		{
			name:       "one byte over limit",
			body:       validJSON + strings.Repeat(" ", int(maxJSONBodyBytes)+1-len(validJSON)),
			wantStatus: http.StatusRequestEntityTooLarge,
			wantBody:   "request body too large\n",
		},
		{
			name:       "valid prefix cannot bypass limit",
			body:       validJSON + strings.Repeat("x", int(maxJSONBodyBytes)+1-len(validJSON)),
			wantStatus: http.StatusRequestEntityTooLarge,
			wantBody:   "request body too large\n",
		},
		{
			name:       "malformed JSON",
			body:       `{`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid JSON\n",
		},
		{
			name:       "second JSON value",
			body:       `{"size":9} {"size":10}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid JSON\n",
		},
		{
			name:       "trailing non-whitespace content",
			body:       `{"size":9}xyz`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid JSON\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/games", strings.NewReader(test.body))
			response := httptest.NewRecorder()
			var destination createGameRequest

			ok := decodeJSONBody(response, request, &destination)
			if ok != test.wantOK {
				t.Fatalf("decodeJSONBody returned %t, want %t", ok, test.wantOK)
			}
			if test.wantOK {
				if destination.Size == nil || *destination.Size != 9 {
					t.Fatalf("decoded size = %v, want 9", destination.Size)
				}
				return
			}
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if body := response.Body.String(); body != test.wantBody {
				t.Fatalf("response body = %q, want %q", body, test.wantBody)
			}
		})
	}
}

func TestJSONEndpointsRejectOversizedBodies(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	tests := []struct {
		name string
		path string
	}{
		{name: "create player", path: "/players"},
		{name: "login", path: "/auth/login"},
		{name: "create game", path: "/games"},
		{name: "submit move", path: "/games/" + created.response.GameID + "/moves"},
		{name: "remove number", path: "/games/" + created.response.GameID + "/remove-number"},
		{name: "submit score", path: "/scores"},
	}
	body := `{}` + strings.Repeat(" ", int(maxJSONBodyBytes)+1-len(`{}`))

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(body))
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
			}
			if body := response.Body.String(); body != "request body too large\n" {
				t.Fatalf("response body = %q, want %q", body, "request body too large\n")
			}
			assertSecurityHeaders(t, response.Header())
		})
	}
}

func TestCreateGameAcceptsBodyAtLimit(t *testing.T) {
	server := newTestServer(t)
	validJSON := `{"size":9}`
	body := validJSON + strings.Repeat(" ", int(maxJSONBodyBytes)-len(validJSON))
	request := httptest.NewRequest(http.MethodPost, "/games", strings.NewReader(body))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	stopCreatedGame(t, server, response)
}

func TestOversizedMoveBodiesForUnknownGamesReturnNotFound(t *testing.T) {
	server := newTestServer(t)
	body := `{}` + strings.Repeat(" ", int(maxJSONBodyBytes)+1-len(`{}`))
	paths := []string{
		"/games/missing/moves",
		"/games/missing/remove-number",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
			}
			if body := response.Body.String(); body != "game not found\n" {
				t.Fatalf("response body = %q, want %q", body, "game not found\n")
			}
			assertSecurityHeaders(t, response.Header())
		})
	}
}

func TestSecurityHeadersOnResponses(t *testing.T) {
	t.Chdir("../..")

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{
			name:       "API success",
			method:     http.MethodGet,
			path:       "/health",
			wantStatus: http.StatusOK,
		},
		{
			name:       "application error",
			method:     http.MethodPost,
			path:       "/games",
			body:       `{`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "static not found",
			method:     http.MethodGet,
			path:       "/missing-security-test",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "method not allowed",
			method:     http.MethodPost,
			path:       "/health",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "static file",
			method:     http.MethodGet,
			path:       "/app.js",
			wantStatus: http.StatusOK,
		},
	}

	server := newTestServer(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			assertSecurityHeaders(t, response.Header())
		})
	}
}

func TestCreateGame(t *testing.T) {
	server := newTestServer(t)
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
	if body.InitialSnapshot.ReshuffleUsed {
		t.Fatal("InitialSnapshot.ReshuffleUsed = true, want false")
	}
	if body.InitialSnapshot.RemoveNumberUsed {
		t.Fatal("InitialSnapshot.RemoveNumberUsed = true, want false")
	}
	if body.InitialSnapshot.HintUsed {
		t.Fatal("InitialSnapshot.HintUsed = true, want false")
	}
	if body.ExpiresAt.IsZero() {
		t.Fatal("ExpiresAt is zero, want session deadline")
	}
	stored, ok := server.store.get(body.GameID)
	if !ok {
		t.Fatalf("created session with ID %q was not stored", body.GameID)
	}
	stored.session.Stop()
}

func TestCreateGameUsesRequestedBoardSize(t *testing.T) {
	server := newTestServer(t)
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
	stored, ok := server.store.get(body.GameID)
	if !ok {
		t.Fatalf("created session with ID %q was not stored", body.GameID)
	}
	stored.session.Stop()

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
	stored, ok := server.store.get(body.GameID)
	if !ok {
		t.Fatalf("created session with ID %q was not stored", body.GameID)
	}
	stored.session.Stop()
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
		{
			name: "unsupported duration",
			body: `{"size":9,"duration":45}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newTestServer(t)
			request := httptest.NewRequest(http.MethodPost, "/games", strings.NewReader(test.body))
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestCreateGameWithDuration(t *testing.T) {
	server := newTestServer(t)

	before := time.Now()
	request := httptest.NewRequest(http.MethodPost, "/games", strings.NewReader(`{"size":9,"duration":60}`))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	after := time.Now()

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}

	var body createGameResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	stored, ok := server.store.get(body.GameID)
	if !ok {
		t.Fatalf("game %q not found in store", body.GameID)
	}
	defer stored.session.Stop()

	expiresAt := stored.session.ExpiresAt()
	expectedMin := before.Add(60 * time.Second)
	expectedMax := after.Add(60 * time.Second)
	if expiresAt.Before(expectedMin) {
		t.Errorf("ExpiresAt = %v, want at or after %v", expiresAt, expectedMin)
	}
	if expiresAt.After(expectedMax) {
		t.Errorf("ExpiresAt = %v, want at or before %v", expiresAt, expectedMax)
	}
}

func TestCreateGameInvalidDuration(t *testing.T) {
	server := newTestServer(t)

	request := httptest.NewRequest(http.MethodPost, "/games", strings.NewReader(`{"size":9,"duration":45}`))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestCreateGameReturnsServiceUnavailableWhenSessionStoreIsFull(t *testing.T) {
	server := newTestServer(t)
	server.store.maxSessions = 0

	request := httptest.NewRequest(http.MethodPost, "/games", strings.NewReader(`{"size":9}`))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestCreateGameReturnsServiceUnavailableAtProductionCapacity(t *testing.T) {
	server := newTestServer(t)
	createdGames := make([]createdGameForTest, 0, defaultMaxStoredSessions)

	defer func() {
		for _, created := range createdGames {
			created.stored.session.Stop()
		}
	}()

	for i := 0; i < defaultMaxStoredSessions; i++ {
		createdGames = append(createdGames, createGameForTest(t, server, 9))
	}

	request := httptest.NewRequest(http.MethodPost, "/games", strings.NewReader(`{"size":9}`))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestDeleteGameRemovesAndStopsSession(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)

	request := httptest.NewRequest(http.MethodDelete, "/games/"+created.response.GameID, nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if _, ok := server.store.get(created.response.GameID); ok {
		t.Fatal("deleted game remained in store")
	}

	select {
	case <-created.stored.session.Done():
	case <-time.After(time.Second):
		t.Fatal("deleted game session did not stop")
	}
}

func TestDeleteGameUnknownReturnsNotFound(t *testing.T) {
	server := newTestServer(t)

	request := httptest.NewRequest(http.MethodDelete, "/games/missing", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestDeletedGameRequestsReturnNotFound(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/games/"+created.response.GameID, nil)
	deleteResponse := httptest.NewRecorder()
	server.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d", deleteResponse.Code, http.StatusNoContent)
	}

	tests := []struct {
		name   string
		method string
		path   string
		body   io.Reader
	}{
		{
			name:   "move",
			method: http.MethodPost,
			path:   "/games/" + created.response.GameID + "/moves",
			body:   strings.NewReader(moveJSONForTest(game.Selection{})),
		},
		{
			name:   "reshuffle",
			method: http.MethodPost,
			path:   "/games/" + created.response.GameID + "/reshuffle",
		},
		{
			name:   "remove number",
			method: http.MethodPost,
			path:   "/games/" + created.response.GameID + "/remove-number",
			body:   strings.NewReader(removeNumberJSONForTest(game.Position{})),
		},
		{
			name:   "hint",
			method: http.MethodPost,
			path:   "/games/" + created.response.GameID + "/hint",
		},
		{
			name:   "snapshots",
			method: http.MethodGet,
			path:   "/games/" + created.response.GameID + "/snapshots",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, test.body)
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
			}
		})
	}
}

func TestSubmitMove(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	move, ok := firstValidSelection(created.response.InitialSnapshot.Board)
	if !ok {
		t.Fatal("firstValidSelection returned false, want valid move")
	}

	response := postMoveForTest(t, server, created.response.GameID, move)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode move response: %v", err)
	}
	if accepted, ok := body["accepted"].(bool); !ok || !accepted {
		t.Fatalf("accepted = %v, want true", body["accepted"])
	}
	if _, ok := body["snapshot"]; ok {
		t.Fatal("move response included snapshot, want acknowledgement only")
	}
	if _, ok := body["initialSnapshot"]; ok {
		t.Fatal("move response included initialSnapshot, want acknowledgement only")
	}
}

func TestSubmitMovePublishesSnapshotToSSE(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	testServer := httptest.NewServer(server)
	defer testServer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	snapshotRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, testServer.URL+"/games/"+created.response.GameID+"/snapshots", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext returned error: %v", err)
	}

	snapshotResponse, err := testServer.Client().Do(snapshotRequest)
	if err != nil {
		t.Fatalf("GET snapshots returned error: %v", err)
	}
	defer snapshotResponse.Body.Close()

	move, ok := firstValidSelection(created.response.InitialSnapshot.Board)
	if !ok {
		t.Fatal("firstValidSelection returned false, want valid move")
	}
	moveResponse := postMoveHTTPForTest(t, testServer.Client(), testServer.URL, created.response.GameID, move)
	defer moveResponse.Body.Close()
	if moveResponse.StatusCode != http.StatusOK {
		t.Fatalf("move status = %d, want %d", moveResponse.StatusCode, http.StatusOK)
	}

	event := readSnapshotEvent(t, snapshotResponse.Body)
	if event.Sequence != 2 {
		t.Fatalf("event.Sequence = %d, want 2", event.Sequence)
	}
}

func TestSubmitReshuffle(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	response := postReshuffleForTest(t, server, created.response.GameID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode reshuffle response: %v", err)
	}
	if accepted, ok := body["accepted"].(bool); !ok || !accepted {
		t.Fatalf("accepted = %v, want true", body["accepted"])
	}
	if _, ok := body["snapshot"]; ok {
		t.Fatal("reshuffle response included snapshot, want acknowledgement only")
	}
	if _, ok := body["initialSnapshot"]; ok {
		t.Fatal("reshuffle response included initialSnapshot, want acknowledgement only")
	}
}

func TestSubmitReshufflePublishesSnapshotToSSE(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	testServer := httptest.NewServer(server)
	defer testServer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	snapshotRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, testServer.URL+"/games/"+created.response.GameID+"/snapshots", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext returned error: %v", err)
	}

	snapshotResponse, err := testServer.Client().Do(snapshotRequest)
	if err != nil {
		t.Fatalf("GET snapshots returned error: %v", err)
	}
	defer snapshotResponse.Body.Close()

	reshuffleResponse := postReshuffleHTTPForTest(t, testServer.Client(), testServer.URL, created.response.GameID)
	defer reshuffleResponse.Body.Close()
	if reshuffleResponse.StatusCode != http.StatusOK {
		t.Fatalf("reshuffle status = %d, want %d", reshuffleResponse.StatusCode, http.StatusOK)
	}

	event := readSnapshotEvent(t, snapshotResponse.Body)
	if event.Sequence != 2 {
		t.Fatalf("event.Sequence = %d, want 2", event.Sequence)
	}
	if !event.ReshuffleUsed {
		t.Fatal("event.ReshuffleUsed = false, want true")
	}
	if got, want := countZeroCellsForTest(event.Board), countZeroCellsForTest(created.response.InitialSnapshot.Board); got != want {
		t.Fatalf("reshuffled zero count = %d, want %d", got, want)
	}
}

func TestSubmitReshuffleUnknownGameReturnsNotFound(t *testing.T) {
	server := newTestServer(t)
	response := postReshuffleForTest(t, server, "missing")

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestSubmitReshuffleAlreadyUsedReturnsUnprocessable(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	response := postReshuffleForTest(t, server, created.response.GameID)
	if response.Code != http.StatusOK {
		t.Fatalf("first reshuffle status = %d, want %d", response.Code, http.StatusOK)
	}

	response = postReshuffleForTest(t, server, created.response.GameID)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("second reshuffle status = %d, want %d", response.Code, http.StatusUnprocessableEntity)
	}
}

func TestSubmitReshuffleStoppedSessionReturnsGone(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	created.stored.session.Stop()

	response := postReshuffleForTest(t, server, created.response.GameID)

	if response.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusGone)
	}
}

func TestSubmitRemoveNumber(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	position, ok := firstNonZeroPosition(created.response.InitialSnapshot.Board)
	if !ok {
		t.Fatal("firstNonZeroPosition returned false, want removable number")
	}

	response := postRemoveNumberForTest(t, server, created.response.GameID, position)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode remove-number response: %v", err)
	}
	if accepted, ok := body["accepted"].(bool); !ok || !accepted {
		t.Fatalf("accepted = %v, want true", body["accepted"])
	}
	if _, ok := body["snapshot"]; ok {
		t.Fatal("remove-number response included snapshot, want acknowledgement only")
	}
	if _, ok := body["initialSnapshot"]; ok {
		t.Fatal("remove-number response included initialSnapshot, want acknowledgement only")
	}
}

func TestSubmitRemoveNumberPublishesSnapshotToSSE(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	testServer := httptest.NewServer(server)
	defer testServer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	snapshotRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, testServer.URL+"/games/"+created.response.GameID+"/snapshots", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext returned error: %v", err)
	}

	snapshotResponse, err := testServer.Client().Do(snapshotRequest)
	if err != nil {
		t.Fatalf("GET snapshots returned error: %v", err)
	}
	defer snapshotResponse.Body.Close()

	position, ok := firstNonZeroPosition(created.response.InitialSnapshot.Board)
	if !ok {
		t.Fatal("firstNonZeroPosition returned false, want removable number")
	}

	removeResponse := postRemoveNumberHTTPForTest(t, testServer.Client(), testServer.URL, created.response.GameID, position)
	defer removeResponse.Body.Close()
	if removeResponse.StatusCode != http.StatusOK {
		t.Fatalf("remove-number status = %d, want %d", removeResponse.StatusCode, http.StatusOK)
	}

	event := readSnapshotEvent(t, snapshotResponse.Body)
	if event.Sequence != 2 {
		t.Fatalf("event.Sequence = %d, want 2", event.Sequence)
	}
	if !event.RemoveNumberUsed {
		t.Fatal("event.RemoveNumberUsed = false, want true")
	}
	if event.Score != created.response.InitialSnapshot.Score {
		t.Fatalf("event.Score = %d, want %d", event.Score, created.response.InitialSnapshot.Score)
	}
	if event.Board[position.Row][position.Col] != 0 {
		t.Fatalf("event.Board[%d][%d] = %d, want 0", position.Row, position.Col, event.Board[position.Row][position.Col])
	}
}

func TestSubmitRemoveNumberUnknownGameReturnsNotFound(t *testing.T) {
	server := newTestServer(t)
	response := postRemoveNumberForTest(t, server, "missing", game.Position{})

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestSubmitRemoveNumberBadRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "invalid JSON",
			body: `{"position":`,
		},
		{
			name: "missing position",
			body: `{}`,
		},
		{
			name: "missing row",
			body: `{"position":{"col":0}}`,
		},
		{
			name: "missing col",
			body: `{"position":{"row":0}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newTestServer(t)
			created := createGameForTest(t, server, 9)
			defer created.stored.session.Stop()

			request := httptest.NewRequest(http.MethodPost, "/games/"+created.response.GameID+"/remove-number", strings.NewReader(test.body))
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestSubmitRemoveNumberOutOfBoundsReturnsBadRequest(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	response := postRemoveNumberForTest(t, server, created.response.GameID, game.Position{Row: 99, Col: 99})

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestSubmitRemoveNumberInvalidTargetDoesNotConsumeSkill(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	move, ok := firstValidSelection(created.response.InitialSnapshot.Board)
	if !ok {
		t.Fatal("firstValidSelection returned false, want valid move")
	}
	moveResponse := postMoveForTest(t, server, created.response.GameID, move)
	if moveResponse.Code != http.StatusOK {
		t.Fatalf("move status = %d, want %d", moveResponse.Code, http.StatusOK)
	}

	response := postRemoveNumberForTest(t, server, created.response.GameID, move.Start)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("cleared-target status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	position, ok := firstNonZeroPositionOutsideSelection(created.response.InitialSnapshot.Board, move)
	if !ok {
		t.Fatal("firstNonZeroPositionOutsideSelection returned false, want removable number")
	}
	response = postRemoveNumberForTest(t, server, created.response.GameID, position)
	if response.Code != http.StatusOK {
		t.Fatalf("valid remove after invalid target status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestSubmitRemoveNumberAlreadyUsedReturnsUnprocessable(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	position, ok := firstNonZeroPosition(created.response.InitialSnapshot.Board)
	if !ok {
		t.Fatal("firstNonZeroPosition returned false, want removable number")
	}
	response := postRemoveNumberForTest(t, server, created.response.GameID, position)
	if response.Code != http.StatusOK {
		t.Fatalf("first remove-number status = %d, want %d", response.Code, http.StatusOK)
	}

	nextPosition, ok := firstNonZeroPositionExcluding(created.response.InitialSnapshot.Board, position)
	if !ok {
		t.Fatal("firstNonZeroPositionExcluding returned false, want another removable number")
	}
	response = postRemoveNumberForTest(t, server, created.response.GameID, nextPosition)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("second remove-number status = %d, want %d", response.Code, http.StatusUnprocessableEntity)
	}
}

func TestSubmitRemoveNumberStoppedSessionReturnsGone(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	created.stored.session.Stop()

	response := postRemoveNumberForTest(t, server, created.response.GameID, game.Position{})

	if response.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusGone)
	}
}

func TestSubmitHint(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	want, ok := firstValidSelection(created.response.InitialSnapshot.Board)
	if !ok {
		t.Fatal("firstValidSelection returned false, want valid move")
	}

	response := postHintForTest(t, server, created.response.GameID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body hintResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode hint response: %v", err)
	}
	got := selectionResponseToGameSelection(body.Selection)
	if got != want {
		t.Fatalf("selection = %+v, want %+v", got, want)
	}
}

func TestSubmitHintPublishesSnapshotToSSE(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	testServer := httptest.NewServer(server)
	defer testServer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	snapshotRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, testServer.URL+"/games/"+created.response.GameID+"/snapshots", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext returned error: %v", err)
	}

	snapshotResponse, err := testServer.Client().Do(snapshotRequest)
	if err != nil {
		t.Fatalf("GET snapshots returned error: %v", err)
	}
	defer snapshotResponse.Body.Close()

	hintResponse := postHintHTTPForTest(t, testServer.Client(), testServer.URL, created.response.GameID)
	defer hintResponse.Body.Close()
	if hintResponse.StatusCode != http.StatusOK {
		t.Fatalf("hint status = %d, want %d", hintResponse.StatusCode, http.StatusOK)
	}

	event := readSnapshotEvent(t, snapshotResponse.Body)
	if event.Sequence != 2 {
		t.Fatalf("event.Sequence = %d, want 2", event.Sequence)
	}
	if !event.HintUsed {
		t.Fatal("event.HintUsed = false, want true")
	}
	if event.Score != created.response.InitialSnapshot.Score {
		t.Fatalf("event.Score = %d, want %d", event.Score, created.response.InitialSnapshot.Score)
	}
	if got, want := countZeroCellsForTest(event.Board), countZeroCellsForTest(created.response.InitialSnapshot.Board); got != want {
		t.Fatalf("hint snapshot zero count = %d, want %d", got, want)
	}
}

func TestSubmitHintUnknownGameReturnsNotFound(t *testing.T) {
	server := newTestServer(t)
	response := postHintForTest(t, server, "missing")

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestSubmitHintAlreadyUsedReturnsUnprocessable(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	response := postHintForTest(t, server, created.response.GameID)
	if response.Code != http.StatusOK {
		t.Fatalf("first hint status = %d, want %d", response.Code, http.StatusOK)
	}

	response = postHintForTest(t, server, created.response.GameID)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("second hint status = %d, want %d", response.Code, http.StatusUnprocessableEntity)
	}
}

func TestSubmitHintStoppedSessionReturnsGone(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	created.stored.session.Stop()

	response := postHintForTest(t, server, created.response.GameID)

	if response.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusGone)
	}
}

func TestSubmitMoveUnknownGameReturnsNotFound(t *testing.T) {
	server := newTestServer(t)
	response := postMoveForTest(t, server, "missing", game.Selection{})

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestSubmitMoveBadRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "invalid JSON",
			body: `{"selection":`,
		},
		{
			name: "missing selection",
			body: `{}`,
		},
		{
			name: "missing start",
			body: `{"selection":{"end":{"row":0,"col":1}}}`,
		},
		{
			name: "missing end",
			body: `{"selection":{"start":{"row":0,"col":0}}}`,
		},
		{
			name: "missing start row",
			body: `{"selection":{"start":{"col":0},"end":{"row":0,"col":1}}}`,
		},
		{
			name: "missing start col",
			body: `{"selection":{"start":{"row":0},"end":{"row":0,"col":1}}}`,
		},
		{
			name: "missing end row",
			body: `{"selection":{"start":{"row":0,"col":0},"end":{"col":1}}}`,
		},
		{
			name: "missing end col",
			body: `{"selection":{"start":{"row":0,"col":0},"end":{"row":0}}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newTestServer(t)
			created := createGameForTest(t, server, 9)
			defer created.stored.session.Stop()

			request := httptest.NewRequest(http.MethodPost, "/games/"+created.response.GameID+"/moves", strings.NewReader(test.body))
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestSubmitMoveOutOfBoundsReturnsBadRequest(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	response := postMoveForTest(t, server, created.response.GameID, game.Selection{
		Start: game.Position{Row: 99, Col: 99},
		End:   game.Position{Row: 99, Col: 99},
	})

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestSubmitMoveInvalidMoveReturnsBadRequestAndDoesNotEndGame(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	invalid := game.Selection{
		Start: game.Position{Row: 0, Col: 0},
		End:   game.Position{Row: 0, Col: 0},
	}
	response := postMoveForTest(t, server, created.response.GameID, invalid)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid move status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	valid, ok := firstValidSelection(created.response.InitialSnapshot.Board)
	if !ok {
		t.Fatal("firstValidSelection returned false, want valid move")
	}
	response = postMoveForTest(t, server, created.response.GameID, valid)
	if response.Code != http.StatusOK {
		t.Fatalf("valid move after invalid move status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestSubmitMoveStoppedSessionReturnsGone(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	created.stored.session.Stop()

	move, ok := firstValidSelection(created.response.InitialSnapshot.Board)
	if !ok {
		t.Fatal("firstValidSelection returned false, want valid move")
	}
	response := postMoveForTest(t, server, created.response.GameID, move)

	if response.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusGone)
	}
}

func TestWriteMoveErrorMapsGameOverToConflict(t *testing.T) {
	response := httptest.NewRecorder()

	writeMoveError(response, game.ErrGameOver)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
}

func TestGameSnapshotsSSEOpensStream(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	testServer := httptest.NewServer(server)
	defer testServer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, testServer.URL+"/games/"+created.response.GameID+"/snapshots", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext returned error: %v", err)
	}

	response, err := testServer.Client().Do(request)
	if err != nil {
		t.Fatalf("GET snapshots returned error: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if contentType := response.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", contentType)
	}
	if cacheControl := response.Header.Get("Cache-Control"); cacheControl != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", cacheControl)
	}
	assertSecurityHeaders(t, response.Header)

	assertNoSSELineWithin(t, response.Body, 20*time.Millisecond)
}

func TestGameSnapshotsSSEReceivesRuntimeSnapshot(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	testServer := httptest.NewServer(server)
	defer testServer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, testServer.URL+"/games/"+created.response.GameID+"/snapshots", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext returned error: %v", err)
	}

	response, err := testServer.Client().Do(request)
	if err != nil {
		t.Fatalf("GET snapshots returned error: %v", err)
	}
	defer response.Body.Close()

	move, ok := firstValidSelection(created.response.InitialSnapshot.Board)
	if !ok {
		t.Fatal("firstValidSelection returned false, want valid move")
	}
	if err := created.stored.session.SubmitMove(context.Background(), move); err != nil {
		t.Fatalf("SubmitMove returned error: %v", err)
	}

	event := readSnapshotEvent(t, response.Body)
	if event.Sequence != 2 {
		t.Fatalf("event.Sequence = %d, want 2", event.Sequence)
	}
}

func TestGameSnapshotsSSELateSubscriberReceivesLatestRuntimeSnapshot(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	move, ok := firstValidSelection(created.response.InitialSnapshot.Board)
	if !ok {
		t.Fatal("firstValidSelection returned false, want valid move")
	}
	if err := created.stored.session.SubmitMove(context.Background(), move); err != nil {
		t.Fatalf("SubmitMove returned error: %v", err)
	}

	testServer := httptest.NewServer(server)
	defer testServer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, testServer.URL+"/games/"+created.response.GameID+"/snapshots", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext returned error: %v", err)
	}

	response, err := testServer.Client().Do(request)
	if err != nil {
		t.Fatalf("GET snapshots returned error: %v", err)
	}
	defer response.Body.Close()

	event := readSnapshotEvent(t, response.Body)
	if event.Sequence != 2 {
		t.Fatalf("event.Sequence = %d, want 2", event.Sequence)
	}
}

func TestGameSnapshotsUnknownGameReturnsNotFound(t *testing.T) {
	server := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/games/missing/snapshots", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestGameSnapshotsEndedGameReturnsGone(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)

	created.stored.session.Stop()
	select {
	case <-created.stored.broker.done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broker to close")
	}

	request := httptest.NewRequest(http.MethodGet, "/games/"+created.response.GameID+"/snapshots", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusGone)
	}
}

type createdGameForTest struct {
	response createGameResponse
	stored   storedGame
}

func createGameForTest(t *testing.T, server *Server, size int) createdGameForTest {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/games", strings.NewReader(`{"size":`+strconv.Itoa(size)+`}`))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("create game status = %d, want %d", response.Code, http.StatusCreated)
	}

	var body createGameResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode create-game response: %v", err)
	}

	stored, ok := server.store.get(body.GameID)
	if !ok {
		t.Fatalf("created session with ID %q was not stored", body.GameID)
	}

	return createdGameForTest{
		response: body,
		stored:   stored,
	}
}

func firstValidSelection(board [][]int) (game.Selection, bool) {
	validMoves, _ := buildValidMovesForTest(board)
	if len(validMoves) == 0 {
		return game.Selection{}, false
	}

	return validMoves[0], true
}

func firstNonZeroPosition(board [][]int) (game.Position, bool) {
	for row := range board {
		for col := range board[row] {
			if board[row][col] != 0 {
				return game.Position{Row: row, Col: col}, true
			}
		}
	}

	return game.Position{}, false
}

func firstNonZeroPositionExcluding(board [][]int, excluded game.Position) (game.Position, bool) {
	for row := range board {
		for col := range board[row] {
			position := game.Position{Row: row, Col: col}
			if position != excluded && board[row][col] != 0 {
				return position, true
			}
		}
	}

	return game.Position{}, false
}

func firstNonZeroPositionOutsideSelection(board [][]int, selection game.Selection) (game.Position, bool) {
	selection = game.NormalizeSelection(selection)
	for row := range board {
		for col := range board[row] {
			position := game.Position{Row: row, Col: col}
			if positionInsideSelection(position, selection) || board[row][col] == 0 {
				continue
			}
			return position, true
		}
	}

	return game.Position{}, false
}

func positionInsideSelection(position game.Position, selection game.Selection) bool {
	return position.Row >= selection.Start.Row &&
		position.Row <= selection.End.Row &&
		position.Col >= selection.Start.Col &&
		position.Col <= selection.End.Col
}

func buildValidMovesForTest(board [][]int) ([]game.Selection, error) {
	var validMoves []game.Selection
	for startRow := range board {
		for startCol := range board[startRow] {
			for endRow := startRow; endRow < len(board); endRow++ {
				for endCol := startCol; endCol < len(board[endRow]); endCol++ {
					selection := game.Selection{
						Start: game.Position{Row: startRow, Col: startCol},
						End:   game.Position{Row: endRow, Col: endCol},
					}
					if rectangleSumForTest(board, selection) == 10 {
						validMoves = append(validMoves, selection)
					}
				}
			}
		}
	}

	return validMoves, nil
}

func rectangleSumForTest(board [][]int, selection game.Selection) int {
	sum := 0
	for row := selection.Start.Row; row <= selection.End.Row; row++ {
		for col := selection.Start.Col; col <= selection.End.Col; col++ {
			sum += board[row][col]
		}
	}

	return sum
}

func postMoveForTest(t *testing.T, server http.Handler, gameID string, selection game.Selection) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/games/"+gameID+"/moves", strings.NewReader(moveJSONForTest(selection)))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	return response
}

func postMoveHTTPForTest(t *testing.T, client *http.Client, serverURL string, gameID string, selection game.Selection) *http.Response {
	t.Helper()

	response, err := client.Post(serverURL+"/games/"+gameID+"/moves", "application/json", strings.NewReader(moveJSONForTest(selection)))
	if err != nil {
		t.Fatalf("POST move returned error: %v", err)
	}

	return response
}

func postReshuffleForTest(t *testing.T, server http.Handler, gameID string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/games/"+gameID+"/reshuffle", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	return response
}

func postReshuffleHTTPForTest(t *testing.T, client *http.Client, serverURL string, gameID string) *http.Response {
	t.Helper()

	response, err := client.Post(serverURL+"/games/"+gameID+"/reshuffle", "application/json", nil)
	if err != nil {
		t.Fatalf("POST reshuffle returned error: %v", err)
	}

	return response
}

func postRemoveNumberForTest(t *testing.T, server http.Handler, gameID string, position game.Position) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/games/"+gameID+"/remove-number", strings.NewReader(removeNumberJSONForTest(position)))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	return response
}

func postRemoveNumberHTTPForTest(t *testing.T, client *http.Client, serverURL string, gameID string, position game.Position) *http.Response {
	t.Helper()

	response, err := client.Post(serverURL+"/games/"+gameID+"/remove-number", "application/json", strings.NewReader(removeNumberJSONForTest(position)))
	if err != nil {
		t.Fatalf("POST remove-number returned error: %v", err)
	}

	return response
}

func postHintForTest(t *testing.T, server http.Handler, gameID string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/games/"+gameID+"/hint", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	return response
}

func postHintHTTPForTest(t *testing.T, client *http.Client, serverURL string, gameID string) *http.Response {
	t.Helper()

	response, err := client.Post(serverURL+"/games/"+gameID+"/hint", "application/json", nil)
	if err != nil {
		t.Fatalf("POST hint returned error: %v", err)
	}

	return response
}

func moveJSONForTest(selection game.Selection) string {
	return `{"selection":{"start":{"row":` +
		strconv.Itoa(selection.Start.Row) +
		`,"col":` +
		strconv.Itoa(selection.Start.Col) +
		`},"end":{"row":` +
		strconv.Itoa(selection.End.Row) +
		`,"col":` +
		strconv.Itoa(selection.End.Col) +
		`}}}`
}

func removeNumberJSONForTest(position game.Position) string {
	return `{"position":{"row":` +
		strconv.Itoa(position.Row) +
		`,"col":` +
		strconv.Itoa(position.Col) +
		`}}`
}

func selectionResponseToGameSelection(selection selectionResponse) game.Selection {
	return game.Selection{
		Start: game.Position{
			Row: selection.Start.Row,
			Col: selection.Start.Col,
		},
		End: game.Position{
			Row: selection.End.Row,
			Col: selection.End.Col,
		},
	}
}

func countZeroCellsForTest(board [][]int) int {
	count := 0
	for row := range board {
		for col := range board[row] {
			if board[row][col] == 0 {
				count++
			}
		}
	}

	return count
}

func assertNoSSELineWithin(t *testing.T, body io.Reader, duration time.Duration) {
	t.Helper()

	lines := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(body).ReadString('\n')
		lines <- line
	}()

	select {
	case line := <-lines:
		t.Fatalf("received SSE line %q before runtime snapshot", line)
	case <-time.After(duration):
	}
}

// --- Leaderboard endpoint tests ---

func TestSubmitScoreHappyPath(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)

	driveGameToOver(t, server, created)

	response := postScoreForTest(t, server, created.response.GameID, "Ada")
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}

	var body submitScoreResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !body.Accepted {
		t.Fatal("accepted = false, want true")
	}

	scores, err := server.leaderboard.TopScores(context.Background(), leaderboard.TopScoresFilter{
		GridSize:        9,
		DurationSeconds: game.DefaultDurationSeconds,
		Limit:           1,
	})
	if err != nil {
		t.Fatalf("TopScores returned error: %v", err)
	}
	if len(scores) != 1 {
		t.Fatalf("len(scores) = %d, want 1", len(scores))
	}
	if scores[0].GameID != created.response.GameID {
		t.Fatalf("GameID = %q, want %q", scores[0].GameID, created.response.GameID)
	}
	if scores[0].PlayerName != "Ada" {
		t.Fatalf("PlayerName = %q, want %q", scores[0].PlayerName, "Ada")
	}
	if scores[0].GridSize != 9 {
		t.Fatalf("GridSize = %d, want 9", scores[0].GridSize)
	}
	if scores[0].DurationSeconds != game.DefaultDurationSeconds {
		t.Fatalf("DurationSeconds = %d, want %d", scores[0].DurationSeconds, game.DefaultDurationSeconds)
	}
	if scores[0].Score < 0 {
		t.Fatalf("Score = %d, want non-negative", scores[0].Score)
	}
	if scores[0].RemainingMillis < 0 {
		t.Fatalf("RemainingMillis = %d, want non-negative", scores[0].RemainingMillis)
	}
	if scores[0].RemainingMillis > game.DefaultDurationSeconds*1000 {
		t.Fatalf("RemainingMillis = %d, exceeds duration", scores[0].RemainingMillis)
	}
}

func TestSubmitScoreServerDerivedValuesMatchFinalSnapshot(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)

	driveGameToOver(t, server, created)

	finalSnapshot, ok := created.stored.broker.latestSnapshot()
	if !ok {
		t.Fatal("broker has no latest snapshot after game over")
	}

	response := postScoreForTest(t, server, created.response.GameID, "Ada")
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}

	scores, err := server.leaderboard.TopScores(context.Background(), leaderboard.TopScoresFilter{
		GridSize:        9,
		DurationSeconds: game.DefaultDurationSeconds,
		Limit:           1,
	})
	if err != nil {
		t.Fatalf("TopScores returned error: %v", err)
	}
	if len(scores) != 1 {
		t.Fatalf("len(scores) = %d, want 1", len(scores))
	}
	if scores[0].Score != finalSnapshot.Score {
		t.Fatalf("persisted Score = %d, want %d (from final snapshot)", scores[0].Score, finalSnapshot.Score)
	}
}

func TestSubmitScoreActiveGameReturnsConflict(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	response := postScoreForTest(t, server, created.response.GameID, "Ada")
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
}

func TestSubmitScoreTimerExpiredGameReturnsCreated(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	created.stored.broker.publish(snapshotResponse{
		Sequence:       2,
		Board:          created.response.InitialSnapshot.Board,
		Score:          0,
		GameOver:       true,
		GameOverReason: int(game.GameOverTimeExpired),
		SnapshotTime:   created.stored.session.ExpiresAt(),
	})

	response := postScoreForTest(t, server, created.response.GameID, "Ada")
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}

	scores, err := server.leaderboard.TopScores(context.Background(), leaderboard.TopScoresFilter{
		GridSize:        9,
		DurationSeconds: game.DefaultDurationSeconds,
		Limit:           1,
	})
	if err != nil {
		t.Fatalf("TopScores returned error: %v", err)
	}
	if len(scores) != 1 {
		t.Fatalf("len(scores) = %d, want 1", len(scores))
	}
	if scores[0].RemainingMillis != 0 {
		t.Fatalf("RemainingMillis = %d, want 0", scores[0].RemainingMillis)
	}
}

func TestSubmitScoreDuplicateReturnsConflict(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)

	driveGameToOver(t, server, created)

	response := postScoreForTest(t, server, created.response.GameID, "Ada")
	if response.Code != http.StatusCreated {
		t.Fatalf("first submit status = %d, want %d", response.Code, http.StatusCreated)
	}

	response = postScoreForTest(t, server, created.response.GameID, "Ada")
	if response.Code != http.StatusConflict {
		t.Fatalf("second submit status = %d, want %d", response.Code, http.StatusConflict)
	}
}

func TestSubmitScoreUnknownGameReturnsNotFound(t *testing.T) {
	server := newTestServer(t)

	response := postScoreForTest(t, server, "missing", "Ada")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestSubmitScoreDeletedGameReturnsNotFound(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)
	created.stored.session.Stop()
	server.store.remove(created.response.GameID)

	response := postScoreForTest(t, server, created.response.GameID, "Ada")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestSubmitScoreBadRequests(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "invalid JSON",
			body:       `{"gameId":`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty body",
			body:       "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing gameId",
			body:       `{"playerName":"Ada"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty gameId",
			body:       `{"gameId":"","playerName":"Ada"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing playerName",
			body:       `{"gameId":"nonexistent"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newTestServer(t)
			request := httptest.NewRequest(http.MethodPost, "/scores", strings.NewReader(test.body))
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}

func TestSubmitScoreInvalidPlayerNameReturnsBadRequest(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)

	driveGameToOver(t, server, created)

	response := postScoreForTest(t, server, created.response.GameID, "!!!")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestTopScoresHappyPath(t *testing.T) {
	server := newTestServer(t)

	seedScores(t, server, 9, game.DefaultDurationSeconds, 3)

	response := getScoresForTest(t, server, 9, game.DefaultDurationSeconds)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var scores []scoreResponse
	if err := json.NewDecoder(response.Body).Decode(&scores); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(scores) != 3 {
		t.Fatalf("len(scores) = %d, want 3", len(scores))
	}
	for i, score := range scores {
		if score.Rank != i+1 {
			t.Errorf("scores[%d].Rank = %d, want %d", i, score.Rank, i+1)
		}
		if score.GridSize != 9 {
			t.Errorf("scores[%d].GridSize = %d, want 9", i, score.GridSize)
		}
		if score.DurationSeconds != game.DefaultDurationSeconds {
			t.Errorf("scores[%d].DurationSeconds = %d, want %d", i, score.DurationSeconds, game.DefaultDurationSeconds)
		}
	}
	if scores[0].Score < scores[1].Score || scores[1].Score < scores[2].Score {
		t.Fatal("scores are not in descending order")
	}
}

func TestTopScoresFiltersCorrectly(t *testing.T) {
	server := newTestServer(t)

	seedScores(t, server, 9, game.DefaultDurationSeconds, 2)
	seedScores(t, server, 10, game.DefaultDurationSeconds, 1)

	response := getScoresForTest(t, server, 9, game.DefaultDurationSeconds)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var scores []scoreResponse
	if err := json.NewDecoder(response.Body).Decode(&scores); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(scores) != 2 {
		t.Fatalf("len(scores) = %d, want 2", len(scores))
	}
}

func TestTopScoresEmptyReturnsEmptyArray(t *testing.T) {
	server := newTestServer(t)

	response := getScoresForTest(t, server, 9, game.DefaultDurationSeconds)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	body := strings.TrimSpace(response.Body.String())
	if body != "[]" {
		t.Fatalf("body = %q, want %q", body, "[]")
	}
}

func TestTopScoresLimitsTo15(t *testing.T) {
	server := newTestServer(t)

	seedScores(t, server, 9, game.DefaultDurationSeconds, 20)

	response := getScoresForTest(t, server, 9, game.DefaultDurationSeconds)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var scores []scoreResponse
	if err := json.NewDecoder(response.Body).Decode(&scores); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(scores) != 15 {
		t.Fatalf("len(scores) = %d, want 15", len(scores))
	}
}

func TestTopScoresFiltersByDuration(t *testing.T) {
	server := newTestServer(t)

	seedScores(t, server, 9, 60, 2)
	seedScores(t, server, 9, 120, 3)

	response := getScoresForTest(t, server, 9, 60)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var scores []scoreResponse
	if err := json.NewDecoder(response.Body).Decode(&scores); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(scores) != 2 {
		t.Fatalf("len(scores) = %d, want 2", len(scores))
	}
	for i, score := range scores {
		if score.DurationSeconds != 60 {
			t.Errorf("scores[%d].DurationSeconds = %d, want 60", i, score.DurationSeconds)
		}
	}
}

func TestTopScoresTieBreakOrdering(t *testing.T) {
	server := newTestServer(t)

	now := time.Now()
	for i, sub := range []leaderboard.ScoreSubmission{
		{GameID: "tie-a", PlayerName: "Alice", Score: 50, GridSize: 9, DurationSeconds: 120, RemainingMillis: 5000, SubmittedAt: now.Add(2 * time.Second)},
		{GameID: "tie-b", PlayerName: "Bob", Score: 50, GridSize: 9, DurationSeconds: 120, RemainingMillis: 5000, SubmittedAt: now.Add(1 * time.Second)},
		{GameID: "tie-c", PlayerName: "Carol", Score: 50, GridSize: 9, DurationSeconds: 120, RemainingMillis: 10000, SubmittedAt: now},
	} {
		if err := server.leaderboard.SubmitScore(context.Background(), sub); err != nil {
			t.Fatalf("seed score %d: %v", i, err)
		}
	}

	response := getScoresForTest(t, server, 9, 120)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var scores []scoreResponse
	if err := json.NewDecoder(response.Body).Decode(&scores); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(scores) != 3 {
		t.Fatalf("len(scores) = %d, want 3", len(scores))
	}
	if scores[0].PlayerName != "Carol" {
		t.Fatalf("rank 1 = %q, want Carol (highest remaining millis)", scores[0].PlayerName)
	}
	if scores[1].PlayerName != "Bob" {
		t.Fatalf("rank 2 = %q, want Bob (earlier submitted_at)", scores[1].PlayerName)
	}
	if scores[2].PlayerName != "Alice" {
		t.Fatalf("rank 3 = %q, want Alice (later submitted_at)", scores[2].PlayerName)
	}
}

func TestTopScoresBadRequests(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{
			name:  "missing gridSize",
			query: "?duration=120",
		},
		{
			name:  "missing duration",
			query: "?gridSize=9",
		},
		{
			name:  "non-integer gridSize",
			query: "?gridSize=abc&duration=120",
		},
		{
			name:  "non-integer duration",
			query: "?gridSize=9&duration=abc",
		},
		{
			name:  "unsupported gridSize",
			query: "?gridSize=99&duration=120",
		},
		{
			name:  "unsupported duration",
			query: "?gridSize=9&duration=45",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newTestServer(t)
			request := httptest.NewRequest(http.MethodGet, "/scores"+test.query, nil)
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestSubmitScoreImmediatelyAfterFinalMove(t *testing.T) {
	server := newTestServer(t)
	created := createGameForTest(t, server, 9)

	board := created.response.InitialSnapshot.Board
	for {
		move, ok := firstValidSelection(board)
		if !ok {
			break
		}

		response := postMoveForTest(t, server, created.response.GameID, move)
		if response.Code != http.StatusOK {
			t.Fatalf("move status = %d, want %d", response.Code, http.StatusOK)
		}

		for row := move.Start.Row; row <= move.End.Row; row++ {
			for col := move.Start.Col; col <= move.End.Col; col++ {
				board[row][col] = 0
			}
		}
	}

	response := postScoreForTest(t, server, created.response.GameID, "Ada")
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
}

func TestTopScoresDoesNotExposeIDOrGameID(t *testing.T) {
	server := newTestServer(t)
	seedScores(t, server, 9, game.DefaultDurationSeconds, 1)

	response := getScoresForTest(t, server, 9, game.DefaultDurationSeconds)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var rawScores []map[string]any
	if err := json.NewDecoder(response.Body).Decode(&rawScores); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(rawScores) != 1 {
		t.Fatalf("len(scores) = %d, want 1", len(rawScores))
	}
	if _, ok := rawScores[0]["id"]; ok {
		t.Fatal("response exposes 'id' field")
	}
	if _, ok := rawScores[0]["gameId"]; ok {
		t.Fatal("response exposes 'gameId' field")
	}
}

func TestScoresRouteDispatch(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{
			name:       "unsupported DELETE method",
			method:     http.MethodDelete,
			path:       "/scores",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "unsupported PUT method",
			method:     http.MethodPut,
			path:       "/scores",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "unsupported PATCH method",
			method:     http.MethodPatch,
			path:       "/scores",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	server := newTestServer(t)
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

// --- Leaderboard test helpers ---

func postScoreForTest(t *testing.T, server *Server, gameID, playerName string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/scores", strings.NewReader(scoreJSONForTest(gameID, playerName)))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	return response
}

func getScoresForTest(t *testing.T, server *Server, gridSize, duration int) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, "/scores?gridSize="+strconv.Itoa(gridSize)+"&duration="+strconv.Itoa(duration), nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	return response
}

func scoreJSONForTest(gameID, playerName string) string {
	return `{"gameId":"` + gameID + `","playerName":"` + playerName + `"}`
}

func assertSecurityHeaders(t *testing.T, header http.Header) {
	t.Helper()

	want := map[string]string{
		"Content-Security-Policy": "default-src 'self'; script-src 'self'; style-src 'self' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'; object-src 'none'",
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
		"X-Frame-Options":         "DENY",
	}
	for name, wantValue := range want {
		if value := header.Get(name); value != wantValue {
			t.Errorf("%s = %q, want %q", name, value, wantValue)
		}
	}
	if strings.Contains(header.Get("Content-Security-Policy"), "'unsafe-inline'") {
		t.Error("Content-Security-Policy contains 'unsafe-inline'")
	}
}

func driveGameToOver(t *testing.T, server *Server, created createdGameForTest) {
	t.Helper()

	board := created.response.InitialSnapshot.Board
	for {
		move, ok := firstValidSelection(board)
		if !ok {
			break
		}

		response := postMoveForTest(t, server, created.response.GameID, move)
		if response.Code != http.StatusOK {
			t.Fatalf("move status = %d, want %d", response.Code, http.StatusOK)
		}

		for row := move.Start.Row; row <= move.End.Row; row++ {
			for col := move.Start.Col; col <= move.End.Col; col++ {
				board[row][col] = 0
			}
		}
	}

	select {
	case <-created.stored.session.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for game session to end")
	}
}

func seedScores(t *testing.T, server *Server, gridSize, durationSeconds, count int) {
	t.Helper()

	for i := 0; i < count; i++ {
		submission := leaderboard.ScoreSubmission{
			GameID:          fmt.Sprintf("seed-%d-%d-%d-%d", gridSize, durationSeconds, count, i),
			PlayerName:      fmt.Sprintf("Player%d", i),
			Score:           (count - i) * 10,
			GridSize:        gridSize,
			DurationSeconds: durationSeconds,
			RemainingMillis: i * 1000,
			SubmittedAt:     time.Now(),
		}
		if err := server.leaderboard.SubmitScore(context.Background(), submission); err != nil {
			t.Fatalf("seedScores: SubmitScore returned error: %v", err)
		}
	}
}

func readSnapshotEvent(t *testing.T, body io.Reader) snapshotResponse {
	t.Helper()

	reader := bufio.NewReader(body)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read event line: %v", err)
	}
	if line != "event: snapshot\n" {
		t.Fatalf("event line = %q, want %q", line, "event: snapshot\n")
	}

	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read data line: %v", err)
	}
	if !strings.HasPrefix(line, "data: ") {
		t.Fatalf("data line = %q, want data prefix", line)
	}

	var snapshot snapshotResponse
	data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
	if err := json.Unmarshal([]byte(data), &snapshot); err != nil {
		t.Fatalf("failed to decode snapshot data: %v", err)
	}

	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read blank event terminator: %v", err)
	}
	if line != "\n" {
		t.Fatalf("event terminator = %q, want blank line", line)
	}

	return snapshot
}
