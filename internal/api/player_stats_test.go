package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"find-ten-game/internal/game"
	"find-ten-game/internal/leaderboard"
)

func TestPlayerStatsReturnsOwnTopScores(t *testing.T) {
	server := newTestServer(t)
	ada, token := createAPIAuthenticatedSession(t, server, "Ada")
	grace, _ := createAPIAuthenticatedSession(t, server, "Grace")
	duration := game.DefaultDurationSeconds

	for i := 0; i < 12; i++ {
		seedPlayerScore(t, server, fmt.Sprintf("ada-%d", i), "Ada", &ada.ID, 100+i, 9, duration)
	}
	seedPlayerScore(t, server, "ada-other-grid", "Ada", &ada.ID, 9999, 10, duration)
	seedPlayerScore(t, server, "grace", "Grace", &grace.ID, 9999, 9, duration)
	seedPlayerScore(t, server, "guest", "Ada", nil, 9999, 9, duration)

	response := getPlayerStatsForTest(t, server, token, 9, duration)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if cacheControl := response.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("Cache-Control = %q, want %q", cacheControl, "no-store")
	}

	var scores []scoreResponse
	if err := json.NewDecoder(response.Body).Decode(&scores); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(scores) != 10 {
		t.Fatalf("len(scores) = %d, want 10", len(scores))
	}
	for i, score := range scores {
		if score.Rank != i+1 {
			t.Errorf("scores[%d].Rank = %d, want %d", i, score.Rank, i+1)
		}
		if wantScore := 111 - i; score.Score != wantScore {
			t.Errorf("scores[%d].Score = %d, want %d", i, score.Score, wantScore)
		}
		if score.PlayerName != "Ada" || score.GridSize != 9 || score.DurationSeconds != duration {
			t.Errorf("scores[%d] = %#v, want Ada 9 x 9 - %ds", i, score, duration)
		}
	}
}

func TestPlayerStatsEmptyReturnsEmptyArray(t *testing.T) {
	server := newTestServer(t)
	_, token := createAPIAuthenticatedSession(t, server, "Ada")
	seedScores(t, server, 9, game.DefaultDurationSeconds, 3)

	response := getPlayerStatsForTest(t, server, token, 9, game.DefaultDurationSeconds)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if body := strings.TrimSpace(response.Body.String()); body != "[]" {
		t.Fatalf("body = %q, want %q", body, "[]")
	}
}

func TestPlayerStatsRejectsMissingInvalidAndExpiredSessions(t *testing.T) {
	tests := []struct {
		name  string
		token func(t *testing.T, server *Server, db *sql.DB) string
	}{
		{
			name:  "missing",
			token: func(*testing.T, *Server, *sql.DB) string { return "" },
		},
		{
			name:  "invalid",
			token: func(*testing.T, *Server, *sql.DB) string { return "invalid" },
		},
		{
			name: "expired",
			token: func(t *testing.T, server *Server, db *sql.DB) string {
				account, token := createAPIAuthenticatedSession(t, server, "Ada")
				if _, err := db.ExecContext(context.Background(), `
					UPDATE player_sessions SET expires_at = ? WHERE player_id = ?
				`, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano), account.ID); err != nil {
					t.Fatalf("expire session: %v", err)
				}
				return token
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, db := newTestServerWithDatabase(t)
			response := getPlayerStatsForTest(t, server, test.token(t, server, db), 9, game.DefaultDurationSeconds)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
			}
			if body := strings.TrimSpace(response.Body.String()); body != "authentication required" {
				t.Fatalf("body = %q, want %q", body, "authentication required")
			}
		})
	}
}

func TestPlayerStatsBadRequests(t *testing.T) {
	queries := map[string]string{
		"missing both":         "",
		"missing gridSize":     "?duration=120",
		"missing duration":     "?gridSize=9",
		"non-integer gridSize": "?gridSize=abc&duration=120",
		"non-integer duration": "?gridSize=9&duration=abc",
		"unsupported gridSize": "?gridSize=99&duration=120",
		"unsupported duration": "?gridSize=9&duration=45",
	}

	for name, query := range queries {
		t.Run(name, func(t *testing.T) {
			server := newTestServer(t)
			_, token := createAPIAuthenticatedSession(t, server, "Ada")

			request := httptest.NewRequest(http.MethodGet, "/players/me/stats"+query, nil)
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}

			public := httptest.NewRecorder()
			server.ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/scores"+query, nil))
			if response.Body.String() != public.Body.String() {
				t.Fatalf("body = %q, want same as GET /scores %q", response.Body.String(), public.Body.String())
			}

			unauthenticated := httptest.NewRecorder()
			server.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/players/me/stats"+query, nil))
			if unauthenticated.Code != http.StatusUnauthorized {
				t.Fatalf("unauthenticated status = %d, want %d", unauthenticated.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestPlayerStatsRequestTimeout(t *testing.T) {
	server := newTestServer(t)
	_, token := createAPIAuthenticatedSession(t, server, "Ada")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	request := httptest.NewRequest(http.MethodGet, playerStatsPath(9, game.DefaultDurationSeconds), nil).WithContext(ctx)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusRequestTimeout {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestTimeout)
	}
}

func TestPlayerStatsRejectsUnsupportedMethods(t *testing.T) {
	server := newTestServer(t)
	_, token := createAPIAuthenticatedSession(t, server, "Ada")

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			request := httptest.NewRequest(method, playerStatsPath(9, game.DefaultDurationSeconds), nil)
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)

			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

// HEAD is tested over a real HTTP server because httptest.ResponseRecorder
// records handler writes even for HEAD; only the server suppresses the body.
func TestPlayerStatsHEADFollowsGETWithoutBody(t *testing.T) {
	server := newTestServer(t)
	account, token := createAPIAuthenticatedSession(t, server, "Ada")
	seedPlayerScore(t, server, "ada-game", "Ada", &account.ID, 100, 9, game.DefaultDurationSeconds)
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	tests := []struct {
		name       string
		token      string
		query      string
		wantStatus int
	}{
		{name: "authenticated", token: token, query: "?gridSize=9&duration=120", wantStatus: http.StatusOK},
		{name: "unauthenticated", query: "?gridSize=9&duration=120", wantStatus: http.StatusUnauthorized},
		{name: "unauthenticated invalid filters", query: "?gridSize=abc", wantStatus: http.StatusUnauthorized},
		{name: "authenticated invalid filters", token: token, query: "?gridSize=abc", wantStatus: http.StatusBadRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodHead, httpServer.URL+"/players/me/stats"+test.query, nil)
			if err != nil {
				t.Fatalf("NewRequest failed: %v", err)
			}
			if test.token != "" {
				request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: test.token})
			}

			response, err := httpServer.Client().Do(request)
			if err != nil {
				t.Fatalf("HEAD request failed: %v", err)
			}
			defer response.Body.Close()

			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
			if test.wantStatus == http.StatusOK {
				if cacheControl := response.Header.Get("Cache-Control"); cacheControl != "no-store" {
					t.Fatalf("Cache-Control = %q, want %q", cacheControl, "no-store")
				}
			}
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("read body failed: %v", err)
			}
			if len(body) != 0 {
				t.Fatalf("body = %q, want empty", body)
			}
		})
	}
}

func getPlayerStatsForTest(t *testing.T, server *Server, token string, gridSize, duration int) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, playerStatsPath(gridSize, duration), nil)
	if token != "" {
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	return response
}

func playerStatsPath(gridSize, duration int) string {
	return "/players/me/stats?gridSize=" + strconv.Itoa(gridSize) + "&duration=" + strconv.Itoa(duration)
}

func seedPlayerScore(t *testing.T, server *Server, gameID, playerName string, playerID *int64, score, gridSize, duration int) {
	t.Helper()

	if err := server.leaderboard.SubmitScore(context.Background(), leaderboard.ScoreSubmission{
		GameID:          gameID,
		PlayerName:      playerName,
		PlayerID:        playerID,
		Score:           score,
		GridSize:        gridSize,
		DurationSeconds: duration,
		SubmittedAt:     time.Now(),
	}); err != nil {
		t.Fatalf("SubmitScore(%q) failed: %v", gameID, err)
	}
}
