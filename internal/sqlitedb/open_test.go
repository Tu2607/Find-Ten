package sqlitedb

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesDatabaseWithWALMode(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "data", "find-ten.db")

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file lookup failed: %v", err)
	}

	var journalMode string
	if err := db.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("journal_mode lookup failed: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
}

func TestOpenRejectsEmptyPath(t *testing.T) {
	if _, err := Open(context.Background(), ""); err == nil {
		t.Fatal("Open succeeded, want error")
	}
}

func TestOpenRejectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := Open(ctx, filepath.Join(t.TempDir(), "find-ten.db")); err == nil {
		t.Fatal("Open succeeded, want error")
	}
}
