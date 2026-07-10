package player

import (
	"context"
	"database/sql"
	"fmt"
)

type Store struct {
	db *sql.DB
}

func NewStore(ctx context.Context, db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("new player store: database is required")
	}

	store := &Store{db: db}
	if err := store.init(ctx); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	// Player stores share the process-owned SQLite DB. This hook is a no-op
	// until later account work adds resources owned by the store itself.
	return nil
}

func (s *Store) init(ctx context.Context) error {
	for _, statement := range schemaStatements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize player schema: %w", err)
		}
	}

	return nil
}

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS players (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		display_name TEXT NOT NULL CHECK (length(display_name) BETWEEN 3 AND 10),
		account_handle TEXT NOT NULL UNIQUE CHECK (length(account_handle) BETWEEN 1 AND 32),
		password_hash TEXT NOT NULL CHECK (length(password_hash) BETWEEN 1 AND 255),
		created_at TEXT NOT NULL CHECK (
			created_at GLOB '????-??-??T*Z'
			AND datetime(created_at) IS NOT NULL
		)
	)`,
}
