package sqlitedb

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const (
	defaultBusyTimeoutMillis = 5000
	// The app can hold more active game sessions than DB connections because
	// SQLite is used only for short persisted reads/writes, not live gameplay.
	maxOpenConnections = 16
)

func Open(ctx context.Context, path string) (*sql.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("open sqlite database: database path is required")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: resolve database path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return nil, fmt.Errorf("open sqlite database: create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dsn(absPath))
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(maxOpenConnections)
	db.SetMaxIdleConns(maxOpenConnections)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open sqlite database: ping database: %w", err)
	}
	if err := verifyWALMode(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func verifyWALMode(ctx context.Context, db *sql.DB) error {
	var mode string
	if err := db.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&mode); err != nil {
		return fmt.Errorf("open sqlite database: verify journal mode: %w", err)
	}
	if mode != "wal" {
		return fmt.Errorf("open sqlite database: journal mode = %q, want wal", mode)
	}

	return nil
}

func dsn(path string) string {
	databaseURL := url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(path),
	}
	values := url.Values{}
	values.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", defaultBusyTimeoutMillis))
	values.Add("_pragma", "journal_mode(WAL)")
	values.Add("_pragma", "foreign_keys(ON)")
	databaseURL.RawQuery = values.Encode()

	return databaseURL.String()
}
