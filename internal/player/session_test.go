package player

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func TestCreateSessionStoresOnlyTokenHash(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	account := createTestAccount(t, store, "Ada")

	token, err := store.CreateSession(ctx, account.ID)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("session token is not base64url: %v", err)
	}
	if len(decoded) != sessionTokenSize {
		t.Fatalf("session token length = %d, want %d", len(decoded), sessionTokenSize)
	}

	expectedHash, ok := hashSessionToken(token)
	if !ok {
		t.Fatal("generated token was rejected by hashSessionToken")
	}
	var storedHash, createdAt, expiresAt string
	if err := store.db.QueryRowContext(ctx, `
		SELECT token_hash, created_at, expires_at
		FROM player_sessions
		WHERE player_id = ?
	`, account.ID).Scan(&storedHash, &createdAt, &expiresAt); err != nil {
		t.Fatalf("stored session lookup failed: %v", err)
	}
	if storedHash != expectedHash {
		t.Fatalf("stored token hash = %q, want %q", storedHash, expectedHash)
	}
	if storedHash == token {
		t.Fatal("raw session token was stored")
	}
	if createdAt != formatTime(accountTestNow) {
		t.Fatalf("created_at = %q, want %q", createdAt, formatTime(accountTestNow))
	}
	if expiresAt != formatTime(accountTestNow.Add(SessionLifetime)) {
		t.Fatalf("expires_at = %q, want %q", expiresAt, formatTime(accountTestNow.Add(SessionLifetime)))
	}
}

func TestFindAccountBySessionToken(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	account := createTestAccount(t, store, "Ada")
	token, err := store.CreateSession(ctx, account.ID)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	found, err := store.FindAccountBySessionToken(ctx, token)
	if err != nil {
		t.Fatalf("FindAccountBySessionToken failed: %v", err)
	}
	if found != account {
		t.Fatalf("found account = %#v, want %#v", found, account)
	}
}

func TestFindAccountBySessionTokenRejectsMissingMalformedAndExpiredTokens(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		setup func(t *testing.T, store *Store) string
	}{
		{
			name: "missing",
			setup: func(t *testing.T, store *Store) string {
				t.Helper()
				token, err := generateSessionToken()
				if err != nil {
					t.Fatalf("generateSessionToken failed: %v", err)
				}
				return token
			},
		},
		{
			name: "malformed",
			setup: func(t *testing.T, _ *Store) string {
				t.Helper()
				return "not-a-session-token"
			},
		},
		{
			name: "expired",
			setup: func(t *testing.T, store *Store) string {
				t.Helper()
				account := createTestAccount(t, store, "Ada")
				token, err := store.CreateSession(ctx, account.ID)
				if err != nil {
					t.Fatalf("CreateSession failed: %v", err)
				}
				store.now = func() time.Time { return accountTestNow.Add(SessionLifetime + time.Second) }
				return token
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := openTestStore(t)
			token := test.setup(t, store)
			_, err := store.FindAccountBySessionToken(ctx, token)
			if !errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("FindAccountBySessionToken error = %v, want %v", err, ErrSessionNotFound)
			}
		})
	}
}

func TestDeleteSessionDeletesOnlyCurrentSession(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	account := createTestAccount(t, store, "Ada")
	firstToken, err := store.CreateSession(ctx, account.ID)
	if err != nil {
		t.Fatalf("first CreateSession failed: %v", err)
	}
	secondToken, err := store.CreateSession(ctx, account.ID)
	if err != nil {
		t.Fatalf("second CreateSession failed: %v", err)
	}

	if err := store.DeleteSession(ctx, firstToken); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}
	if _, err := store.FindAccountBySessionToken(ctx, firstToken); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("first session lookup error = %v, want %v", err, ErrSessionNotFound)
	}
	found, err := store.FindAccountBySessionToken(ctx, secondToken)
	if err != nil {
		t.Fatalf("second session lookup failed: %v", err)
	}
	if found != account {
		t.Fatalf("second session account = %#v, want %#v", found, account)
	}
}

func TestDeleteSessionIsIdempotentForMalformedAndMissingTokens(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	missingToken, err := generateSessionToken()
	if err != nil {
		t.Fatalf("generateSessionToken failed: %v", err)
	}

	for _, token := range []string{"malformed", missingToken} {
		if err := store.DeleteSession(ctx, token); err != nil {
			t.Fatalf("DeleteSession(%q) failed: %v", token, err)
		}
	}
}

func TestCreateSessionRejectsUnknownPlayer(t *testing.T) {
	_, err := openTestStore(t).CreateSession(context.Background(), 999)
	if err == nil {
		t.Fatal("CreateSession succeeded for an unknown player, want error")
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	account := createTestAccount(t, store, "Ada")
	expiredToken, err := store.CreateSession(ctx, account.ID)
	if err != nil {
		t.Fatalf("expired CreateSession failed: %v", err)
	}

	store.now = func() time.Time { return accountTestNow.Add(SessionLifetime + time.Second) }
	validToken, err := store.CreateSession(ctx, account.ID)
	if err != nil {
		t.Fatalf("valid CreateSession failed: %v", err)
	}
	deleted, err := store.DeleteExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("DeleteExpiredSessions failed: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted session count = %d, want 1", deleted)
	}
	if _, err := store.FindAccountBySessionToken(ctx, expiredToken); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expired session lookup error = %v, want %v", err, ErrSessionNotFound)
	}

	if _, err := store.FindAccountBySessionToken(ctx, validToken); err != nil {
		t.Fatalf("valid session lookup failed: %v", err)
	}
}

func TestPlayerSessionsCascadeWhenAccountIsDeleted(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	account := createTestAccount(t, store, "Ada")
	if _, err := store.CreateSession(ctx, account.ID); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if _, err := store.db.ExecContext(ctx, `DELETE FROM players WHERE id = ?`, account.ID); err != nil {
		t.Fatalf("player deletion failed: %v", err)
	}

	var count int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM player_sessions WHERE player_id = ?`, account.ID).Scan(&count); err != nil {
		t.Fatalf("session count lookup failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("session count after player deletion = %d, want 0", count)
	}
}

func createTestAccount(t *testing.T, store *Store, displayName string) Account {
	t.Helper()

	account, err := store.CreateAccount(context.Background(), CreateAccountInput{
		DisplayName: displayName,
		Password:    "correct-password",
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	return account
}
