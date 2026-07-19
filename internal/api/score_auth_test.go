package api

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSubmitScoreUsesAuthenticatedPlayerIdentity(t *testing.T) {
	server, db := newTestServerWithDatabase(t)
	account, token := createAPIAuthenticatedSession(t, server, "Ada")
	created := createGameForTest(t, server, 9)
	driveGameToOver(t, server, created)

	request := httptest.NewRequest(
		http.MethodPost,
		"/scores",
		strings.NewReader(scoreJSONForTest(created.response.GameID, "Splash Screen Name")),
	)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}

	playerName, playerID := storedScoreIdentity(t, db, created.response.GameID)
	if playerName != account.DisplayName {
		t.Fatalf("stored player name = %q, want account display name %q", playerName, account.DisplayName)
	}
	if !playerID.Valid || playerID.Int64 != account.ID {
		t.Fatalf("stored player ID = %#v, want %d", playerID, account.ID)
	}
}

func TestSubmitScoreAuthenticatedPlayerNameIsOptional(t *testing.T) {
	server, db := newTestServerWithDatabase(t)
	account, token := createAPIAuthenticatedSession(t, server, "Ada")
	created := createGameForTest(t, server, 9)
	driveGameToOver(t, server, created)

	request := httptest.NewRequest(
		http.MethodPost,
		"/scores",
		strings.NewReader(`{"gameId":"`+created.response.GameID+`"}`),
	)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}

	playerName, playerID := storedScoreIdentity(t, db, created.response.GameID)
	if playerName != account.DisplayName || !playerID.Valid || playerID.Int64 != account.ID {
		t.Fatalf("stored identity = %q, %#v, want account identity", playerName, playerID)
	}
}

func TestSubmitScoreGuestIdentityRemainsManual(t *testing.T) {
	server, db := newTestServerWithDatabase(t)
	created := createGameForTest(t, server, 9)
	driveGameToOver(t, server, created)

	response := postScoreForTest(t, server, created.response.GameID, "Guest Name")
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}

	playerName, playerID := storedScoreIdentity(t, db, created.response.GameID)
	if playerName != "Guest Name" {
		t.Fatalf("stored player name = %q, want Guest Name", playerName)
	}
	if playerID.Valid {
		t.Fatalf("stored player ID = %d, want NULL", playerID.Int64)
	}
}

func TestSubmitScoreInvalidSessionFallsBackToGuest(t *testing.T) {
	server, db := newTestServerWithDatabase(t)
	created := createGameForTest(t, server, 9)
	driveGameToOver(t, server, created)
	request := httptest.NewRequest(
		http.MethodPost,
		"/scores",
		strings.NewReader(scoreJSONForTest(created.response.GameID, "Guest Name")),
	)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "invalid"})
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	playerName, playerID := storedScoreIdentity(t, db, created.response.GameID)
	if playerName != "Guest Name" || playerID.Valid {
		t.Fatalf("stored identity = %q, %#v, want guest identity", playerName, playerID)
	}
}

func TestSubmitScoreExpiredSessionFallsBackToGuest(t *testing.T) {
	server, db := newTestServerWithDatabase(t)
	account, token := createAPIAuthenticatedSession(t, server, "Ada")
	if _, err := db.ExecContext(context.Background(), `
		UPDATE player_sessions SET expires_at = ? WHERE player_id = ?
	`, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano), account.ID); err != nil {
		t.Fatalf("expire session: %v", err)
	}

	created := createGameForTest(t, server, 9)
	driveGameToOver(t, server, created)
	request := httptest.NewRequest(
		http.MethodPost,
		"/scores",
		strings.NewReader(scoreJSONForTest(created.response.GameID, "Guest Name")),
	)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	playerName, playerID := storedScoreIdentity(t, db, created.response.GameID)
	if playerName != "Guest Name" || playerID.Valid {
		t.Fatalf("stored identity = %q, %#v, want guest identity", playerName, playerID)
	}
}

func TestSubmitScoreAuthenticatedRequestTimeout(t *testing.T) {
	server := newTestServer(t)
	_, token := createAPIAuthenticatedSession(t, server, "Ada")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodPost, "/scores", strings.NewReader(`{"gameId":"any-game"}`)).WithContext(ctx)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusRequestTimeout {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestTimeout)
	}
}

func storedScoreIdentity(t *testing.T, db *sql.DB, gameID string) (string, sql.NullInt64) {
	t.Helper()

	var (
		playerName string
		playerID   sql.NullInt64
	)
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT player_name, player_id FROM leaderboard_scores WHERE game_id = ?`,
		gameID,
	).Scan(&playerName, &playerID); err != nil {
		t.Fatalf("stored score identity lookup failed: %v", err)
	}

	return playerName, playerID
}
