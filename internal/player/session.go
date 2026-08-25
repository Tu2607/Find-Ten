package player

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"
)

const (
	SessionLifetime  = 7 * 24 * time.Hour
	sessionTokenSize = 32
)

func (s *Store) CreateSession(ctx context.Context, playerID int64) (string, error) {
	token, err := generateSessionToken()
	if err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	tokenHash, ok := hashSessionToken(token)
	if !ok {
		return "", errors.New("generated session token is invalid")
	}
	if _, err := s.DeleteExpiredSessions(ctx); err != nil {
		log.Printf("expired player session cleanup failed: %v", err)
	}

	createdAt := s.now().UTC()
	expiresAt := createdAt.Add(SessionLifetime)
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO player_sessions (token_hash, player_id, created_at, expires_at)
		VALUES (?, ?, ?, ?)
	`, tokenHash, playerID, formatTime(createdAt), formatTime(expiresAt)); err != nil {
		return "", fmt.Errorf("create player session: %w", err)
	}

	return token, nil
}

func (s *Store) FindAccountBySessionToken(ctx context.Context, token string) (Account, error) {
	tokenHash, ok := hashSessionToken(token)
	if !ok {
		return Account{}, ErrSessionNotFound
	}

	now := s.now().UTC()
	var account Account
	var createdAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT p.id, p.display_name, p.account_handle, p.created_at
		FROM player_sessions AS ps
		JOIN players AS p ON p.id = ps.player_id
		WHERE ps.token_hash = ? AND ps.expires_at > ?
	`, tokenHash, formatTime(now)).Scan(
		&account.ID,
		&account.DisplayName,
		&account.AccountHandle,
		&createdAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrSessionNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("find player session: %w", err)
	}

	account.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return Account{}, fmt.Errorf("parse player account created_at: %w", err)
	}

	return account, nil
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	tokenHash, ok := hashSessionToken(token)
	if !ok {
		return nil
	}

	if _, err := s.db.ExecContext(ctx, `DELETE FROM player_sessions WHERE token_hash = ?`, tokenHash); err != nil {
		return fmt.Errorf("delete player session: %w", err)
	}

	return nil
}

func (s *Store) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	return s.deleteExpiredSessionsBefore(ctx, s.now().UTC())
}

func (s *Store) deleteExpiredSessionsBefore(ctx context.Context, now time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM player_sessions WHERE expires_at <= ?`, formatTime(now))
	if err != nil {
		return 0, fmt.Errorf("delete expired player sessions: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count deleted player sessions: %w", err)
	}

	return count, nil
}

func generateSessionToken() (string, error) {
	value := make([]byte, sessionTokenSize)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(value), nil
}

func hashSessionToken(token string) (string, bool) {
	value, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(value) != sessionTokenSize || base64.RawURLEncoding.EncodeToString(value) != token {
		return "", false
	}

	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:]), true
}
