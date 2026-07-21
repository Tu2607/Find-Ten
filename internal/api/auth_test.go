package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"find-ten-game/internal/player"
)

func TestCreatePlayer(t *testing.T) {
	server := newTestServer(t)
	request := httptest.NewRequest(http.MethodPost, "/players", strings.NewReader(`{"displayName":"Ada","password":"Correct-password"}`))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	if cookies := response.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("registration set cookies = %#v, want none", cookies)
	}

	var body createPlayerResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if !body.Created {
		t.Fatal("Created = false, want true")
	}
	if body.Player.DisplayName != "Ada" || body.Player.AccountHandle != "Ada" {
		t.Fatalf("player = %#v, want Ada with handle Ada", body.Player)
	}
	if _, err := server.players.FindAccountByHandle(context.Background(), body.Player.AccountHandle); err != nil {
		t.Fatalf("FindAccountByHandle failed: %v", err)
	}
}

func TestCreatePlayerRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{`},
		{name: "missing display name", body: `{"password":"Correct-password"}`},
		{name: "missing password", body: `{"displayName":"Ada"}`},
		{name: "invalid display name", body: `{"displayName":"Al","password":"Correct-password"}`},
		{name: "invalid password", body: `{"displayName":"Ada","password":"short"}`},
		{name: "password without ASCII special character", body: `{"displayName":"Ada","password":"Abcdef123456"}`},
		{name: "password without ASCII uppercase letter", body: `{"displayName":"Ada","password":"abcdef12345!"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			server := newTestServer(t)
			server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/players", strings.NewReader(test.body)))

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestCreatePlayerPasswordValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantBody string
	}{
		{name: "missing ASCII special character", password: "Abcdef123456", wantBody: "invalid password: password must include an ASCII special character"},
		{name: "missing ASCII uppercase letter", password: "abcdef12345!", wantBody: "invalid password: password must include an ASCII uppercase letter"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newTestServer(t)
			request := httptest.NewRequest(http.MethodPost, "/players", strings.NewReader(`{"displayName":"Ada","password":"`+test.password+`"}`))
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
			if body := strings.TrimSpace(response.Body.String()); body != test.wantBody {
				t.Fatalf("response body = %q, want %q", body, test.wantBody)
			}
		})
	}
}

func TestCreatePlayerGeneratesSuffixedHandleForDuplicateDisplayName(t *testing.T) {
	server := newTestServer(t)
	body := `{"displayName":"Ada","password":"Correct-password"}`

	first := createPlayerRequestForTest(t, server, body)
	second := createPlayerRequestForTest(t, server, body)
	if first.Player.AccountHandle != "Ada" {
		t.Fatalf("first handle = %q, want Ada", first.Player.AccountHandle)
	}
	if second.Player.AccountHandle == "Ada" || second.Player.AccountHandle == first.Player.AccountHandle {
		t.Fatalf("second handle = %q, want a distinct suffixed handle", second.Player.AccountHandle)
	}
}

func TestLoginSetsSessionCookie(t *testing.T) {
	server := newTestServer(t)
	account := createAPIAccount(t, server, "Ada")
	request := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"accountHandle":"Ada","password":"Correct-password"}`))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body authResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if !body.Authenticated || body.Player.DisplayName != account.DisplayName || body.Player.AccountHandle != account.AccountHandle {
		t.Fatalf("response = %#v, want authenticated %q", body, account.AccountHandle)
	}

	cookie := responseCookie(t, response, sessionCookieName)
	if cookie.Value == "" {
		t.Fatal("session cookie has an empty value")
	}
	if cookie.Path != "/" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie settings = %#v, want Path=/ HttpOnly SameSite=Lax", cookie)
	}
	if cookie.Secure {
		t.Fatal("local HTTP cookie Secure = true, want false")
	}
	if cookie.MaxAge != int(player.SessionLifetime.Seconds()) || cookie.Expires.Before(time.Now()) {
		t.Fatalf("cookie expiry settings = %#v, want seven-day session", cookie)
	}
	if found, err := server.players.FindAccountBySessionToken(context.Background(), cookie.Value); err != nil || found.ID != account.ID {
		t.Fatalf("session lookup = %#v, %v, want account %d", found, err, account.ID)
	}
}

func TestLoginUsesSecureCookieForHTTPS(t *testing.T) {
	server := newTestServer(t)
	createAPIAccount(t, server, "Ada")
	request := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"accountHandle":"Ada","password":"Correct-password"}`))
	request.TLS = &tls.ConnectionState{}
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !responseCookie(t, response, sessionCookieName).Secure {
		t.Fatal("HTTPS session cookie Secure = false, want true")
	}
}

func TestLoginRejectsInvalidRequestsAndCredentials(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantStatus  int
		withAccount bool
	}{
		{name: "invalid JSON", body: `{`, wantStatus: http.StatusBadRequest},
		{name: "missing handle", body: `{"password":"Correct-password"}`, wantStatus: http.StatusBadRequest},
		{name: "missing password", body: `{"accountHandle":"Ada"}`, wantStatus: http.StatusBadRequest},
		{name: "wrong password", body: `{"accountHandle":"Ada","password":"wrong-password"}`, wantStatus: http.StatusUnauthorized, withAccount: true},
		{name: "unknown handle", body: `{"accountHandle":"Unknown","password":"Correct-password"}`, wantStatus: http.StatusUnauthorized},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newTestServer(t)
			if test.withAccount {
				createAPIAccount(t, server, "Ada")
			}
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(test.body)))

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if cookies := response.Result().Cookies(); len(cookies) != 0 {
				t.Fatalf("failed login set cookies = %#v, want none", cookies)
			}
		})
	}
}

func TestCurrentPlayer(t *testing.T) {
	server := newTestServer(t)
	account, token := createAPIAuthenticatedSession(t, server, "Ada")
	request := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body authResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if !body.Authenticated || body.Player.DisplayName != account.DisplayName || body.Player.AccountHandle != account.AccountHandle {
		t.Fatalf("response = %#v, want authenticated %q", body, account.AccountHandle)
	}
}

func TestCurrentPlayerRejectsMissingAndInvalidSessions(t *testing.T) {
	tests := []struct {
		name   string
		cookie *http.Cookie
	}{
		{name: "missing"},
		{name: "invalid", cookie: &http.Cookie{Name: sessionCookieName, Value: "invalid"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
			if test.cookie != nil {
				request.AddCookie(test.cookie)
			}
			response := httptest.NewRecorder()
			newTestServer(t).ServeHTTP(response, request)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestCurrentPlayerRejectsExpiredSession(t *testing.T) {
	server, db := newTestServerWithDatabase(t)
	account, token := createAPIAuthenticatedSession(t, server, "Ada")
	if _, err := db.ExecContext(context.Background(), `
		UPDATE player_sessions SET expires_at = ? WHERE player_id = ?
	`, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano), account.ID); err != nil {
		t.Fatalf("expire session: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestLogoutDeletesCurrentSessionAndClearsCookie(t *testing.T) {
	server := newTestServer(t)
	account, currentToken := createAPIAuthenticatedSession(t, server, "Ada")
	otherToken, err := server.players.CreateSession(context.Background(), account.ID)
	if err != nil {
		t.Fatalf("second CreateSession failed: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: currentToken})
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if _, err := server.players.FindAccountBySessionToken(context.Background(), currentToken); !errors.Is(err, player.ErrSessionNotFound) {
		t.Fatalf("current session lookup error = %v, want %v", err, player.ErrSessionNotFound)
	}
	if found, err := server.players.FindAccountBySessionToken(context.Background(), otherToken); err != nil || found.ID != account.ID {
		t.Fatalf("other session lookup = %#v, %v, want account %d", found, err, account.ID)
	}

	cookie := responseCookie(t, response, sessionCookieName)
	if cookie.MaxAge != -1 || !cookie.Expires.Before(time.Now()) || !cookie.HttpOnly || cookie.Path != "/" || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cleared cookie settings = %#v", cookie)
	}
}

func TestLogoutSucceedsWithoutSession(t *testing.T) {
	response := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/auth/logout", nil))

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if responseCookie(t, response, sessionCookieName).MaxAge != -1 {
		t.Fatal("logout did not clear the session cookie")
	}
}

func TestAuthHandlersMapCanceledRequestToTimeout(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, server *Server) *http.Request
	}{
		{
			name: "create player",
			setup: func(_ *testing.T, _ *Server) *http.Request {
				return httptest.NewRequest(http.MethodPost, "/players", strings.NewReader(`{"displayName":"Ada","password":"Correct-password"}`))
			},
		},
		{
			name: "login",
			setup: func(t *testing.T, server *Server) *http.Request {
				createAPIAccount(t, server, "Ada")
				return httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"accountHandle":"Ada","password":"Correct-password"}`))
			},
		},
		{
			name: "current player",
			setup: func(t *testing.T, server *Server) *http.Request {
				_, token := createAPIAuthenticatedSession(t, server, "Ada")
				request := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
				request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
				return request
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newTestServer(t)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			response := httptest.NewRecorder()
			server.ServeHTTP(response, test.setup(t, server).WithContext(ctx))

			if response.Code != http.StatusRequestTimeout {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestTimeout)
			}
		})
	}
}

func createPlayerRequestForTest(t *testing.T, server *Server, body string) createPlayerResponse {
	t.Helper()

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/players", strings.NewReader(body)))
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}

	var result createPlayerResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	return result
}

func createAPIAccount(t *testing.T, server *Server, displayName string) player.Account {
	t.Helper()

	account, err := server.players.CreateAccount(context.Background(), player.CreateAccountInput{
		DisplayName: displayName,
		Password:    "Correct-password",
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	return account
}

func createAPIAuthenticatedSession(t *testing.T, server *Server, displayName string) (player.Account, string) {
	t.Helper()

	account := createAPIAccount(t, server, displayName)
	token, err := server.players.CreateSession(context.Background(), account.ID)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	return account, token
}

func responseCookie(t *testing.T, response *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("response does not include %q cookie", name)
	return nil
}
