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

	server := NewServer()
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

func TestCreateGameReturnsServiceUnavailableWhenSessionStoreIsFull(t *testing.T) {
	server := NewServer().(*Server)
	server.store.maxSessions = 0

	request := httptest.NewRequest(http.MethodPost, "/games", strings.NewReader(`{"size":9}`))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestCreateGameReturnsServiceUnavailableAtProductionCapacity(t *testing.T) {
	server := NewServer().(*Server)
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
	server := NewServer().(*Server)
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
	server := NewServer().(*Server)

	request := httptest.NewRequest(http.MethodDelete, "/games/missing", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestDeletedGameRequestsReturnNotFound(t *testing.T) {
	server := NewServer().(*Server)
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
	server := NewServer().(*Server)
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
	server := NewServer().(*Server)
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
	server := NewServer().(*Server)
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
	server := NewServer().(*Server)
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
	server := NewServer().(*Server)
	response := postReshuffleForTest(t, server, "missing")

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestSubmitReshuffleAlreadyUsedReturnsConflict(t *testing.T) {
	server := NewServer().(*Server)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	response := postReshuffleForTest(t, server, created.response.GameID)
	if response.Code != http.StatusOK {
		t.Fatalf("first reshuffle status = %d, want %d", response.Code, http.StatusOK)
	}

	response = postReshuffleForTest(t, server, created.response.GameID)
	if response.Code != http.StatusConflict {
		t.Fatalf("second reshuffle status = %d, want %d", response.Code, http.StatusConflict)
	}
}

func TestSubmitReshuffleStoppedSessionReturnsGone(t *testing.T) {
	server := NewServer().(*Server)
	created := createGameForTest(t, server, 9)
	created.stored.session.Stop()

	response := postReshuffleForTest(t, server, created.response.GameID)

	if response.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusGone)
	}
}

func TestSubmitRemoveNumber(t *testing.T) {
	server := NewServer().(*Server)
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
	server := NewServer().(*Server)
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
	server := NewServer().(*Server)
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
			server := NewServer().(*Server)
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
	server := NewServer().(*Server)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	response := postRemoveNumberForTest(t, server, created.response.GameID, game.Position{Row: 99, Col: 99})

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestSubmitRemoveNumberInvalidTargetDoesNotConsumeSkill(t *testing.T) {
	server := NewServer().(*Server)
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

func TestSubmitRemoveNumberAlreadyUsedReturnsConflict(t *testing.T) {
	server := NewServer().(*Server)
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
	if response.Code != http.StatusConflict {
		t.Fatalf("second remove-number status = %d, want %d", response.Code, http.StatusConflict)
	}
}

func TestSubmitRemoveNumberStoppedSessionReturnsGone(t *testing.T) {
	server := NewServer().(*Server)
	created := createGameForTest(t, server, 9)
	created.stored.session.Stop()

	response := postRemoveNumberForTest(t, server, created.response.GameID, game.Position{})

	if response.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusGone)
	}
}

func TestSubmitHint(t *testing.T) {
	server := NewServer().(*Server)
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
	server := NewServer().(*Server)
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
	server := NewServer().(*Server)
	response := postHintForTest(t, server, "missing")

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestSubmitHintAlreadyUsedReturnsConflict(t *testing.T) {
	server := NewServer().(*Server)
	created := createGameForTest(t, server, 9)
	defer created.stored.session.Stop()

	response := postHintForTest(t, server, created.response.GameID)
	if response.Code != http.StatusOK {
		t.Fatalf("first hint status = %d, want %d", response.Code, http.StatusOK)
	}

	response = postHintForTest(t, server, created.response.GameID)
	if response.Code != http.StatusConflict {
		t.Fatalf("second hint status = %d, want %d", response.Code, http.StatusConflict)
	}
}

func TestSubmitHintStoppedSessionReturnsGone(t *testing.T) {
	server := NewServer().(*Server)
	created := createGameForTest(t, server, 9)
	created.stored.session.Stop()

	response := postHintForTest(t, server, created.response.GameID)

	if response.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusGone)
	}
}

func TestSubmitMoveUnknownGameReturnsNotFound(t *testing.T) {
	server := NewServer().(*Server)
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
			server := NewServer().(*Server)
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
	server := NewServer().(*Server)
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
	server := NewServer().(*Server)
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
	server := NewServer().(*Server)
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
