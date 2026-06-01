package api

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"find-ten-game/internal/game"
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
			name:       "snapshots unknown game",
			method:     http.MethodGet,
			path:       "/games/test-game/snapshots",
			wantStatus: http.StatusNotFound,
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
	stored, ok := server.store.get(body.GameID)
	if !ok {
		t.Fatalf("created session with ID %q was not stored", body.GameID)
	}
	stored.session.Stop()
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

func TestGameSnapshotsSSEOpensStream(t *testing.T) {
	server := NewServer().(*Server)
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

	assertNoSSELineWithin(t, response.Body, 20*time.Millisecond)
}

func TestGameSnapshotsSSEReceivesRuntimeSnapshot(t *testing.T) {
	server := NewServer().(*Server)
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
	server := NewServer().(*Server)
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
	server := NewServer()
	request := httptest.NewRequest(http.MethodGet, "/games/missing/snapshots", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestGameSnapshotsEndedGameReturnsGone(t *testing.T) {
	server := NewServer().(*Server)
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
