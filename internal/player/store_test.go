package player

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"find-ten-game/internal/sqlitedb"
)

func TestNewStoreCreatesPlayersTable(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	if _, err := NewStore(ctx, db); err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	var tableName string
	if err := db.QueryRowContext(
		ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'players'`,
	).Scan(&tableName); err != nil {
		t.Fatalf("players table lookup failed: %v", err)
	}
	if tableName != "players" {
		t.Fatalf("table name = %q, want players", tableName)
	}
}

func TestNewStoreIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	if _, err := NewStore(ctx, db); err != nil {
		t.Fatalf("first NewStore failed: %v", err)
	}
	_, err := db.ExecContext(
		ctx,
		`INSERT INTO players (display_name, account_handle, password_hash, created_at)
		VALUES (?, ?, ?, ?)`,
		"Ada",
		"Ada",
		"hash-1",
		"2026-07-04T12:00:00Z",
	)
	if err != nil {
		t.Fatalf("player insert failed: %v", err)
	}
	if _, err := NewStore(ctx, db); err != nil {
		t.Fatalf("second NewStore failed: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM players`).Scan(&count); err != nil {
		t.Fatalf("player count lookup failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("player count = %d, want 1", count)
	}
}

func TestNewStoreRejectsCanceledContext(t *testing.T) {
	db := openTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := NewStore(ctx, db); err == nil {
		t.Fatal("NewStore succeeded, want error")
	}
}

func TestNewStoreRejectsNilDatabase(t *testing.T) {
	if _, err := NewStore(context.Background(), nil); err == nil {
		t.Fatal("NewStore succeeded, want error")
	}
}

func TestStoreCloseKeepsSharedDatabaseOpen(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	store, err := NewStore(ctx, db)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	var got int
	if err := db.QueryRowContext(ctx, `SELECT 1`).Scan(&got); err != nil {
		t.Fatalf("query after store Close failed: %v", err)
	}
	if got != 1 {
		t.Fatalf("query result = %d, want 1", got)
	}
}

func TestPlayersSchemaKeepsAccountHandlesCaseSensitive(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	if _, err := NewStore(ctx, db); err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO players (display_name, account_handle, password_hash, created_at)
		VALUES (?, ?, ?, ?)`,
		"Ada",
		"Ada",
		"hash-1",
		"2026-07-04T12:00:00Z",
	); err != nil {
		t.Fatalf("initial player insert failed: %v", err)
	}

	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO players (display_name, account_handle, password_hash, created_at)
		VALUES (?, ?, ?, ?)`,
		"Ada Lower",
		"ada",
		"hash-2",
		"2026-07-04T12:00:01Z",
	); err != nil {
		t.Fatalf("case-varied account_handle insert failed: %v", err)
	}
}

func TestPlayersSchemaAcceptsDisplayNameLengthBoundaries(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	if _, err := NewStore(ctx, db); err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	tests := []struct {
		name          string
		displayName   string
		accountHandle string
	}{
		{
			name:          "minimum",
			displayName:   "Ada",
			accountHandle: "Ada",
		},
		{
			name:          "maximum",
			displayName:   "TenLetters",
			accountHandle: "TenLetters",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := db.ExecContext(
				ctx,
				`INSERT INTO players (display_name, account_handle, password_hash, created_at)
				VALUES (?, ?, ?, ?)`,
				test.displayName,
				test.accountHandle,
				"hash-"+test.name,
				"2026-07-04T12:00:00Z",
			)
			if err != nil {
				t.Fatalf("player insert failed: %v", err)
			}
		})
	}
}

func TestPlayersSchemaEnforcesHandleAndTimestampConstraints(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	if _, err := NewStore(ctx, db); err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	_, err := db.ExecContext(
		ctx,
		`INSERT INTO players (display_name, account_handle, password_hash, created_at)
		VALUES (?, ?, ?, ?)`,
		"Ada",
		"Ada",
		"hash-1",
		"2026-07-04T12:00:00Z",
	)
	if err != nil {
		t.Fatalf("initial player insert failed: %v", err)
	}

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO players (display_name, account_handle, password_hash, created_at)
		VALUES (?, ?, ?, ?)`,
		"Ada 2",
		"Ada",
		"hash-3",
		"2026-07-04T12:00:02Z",
	)
	if err == nil {
		t.Fatal("duplicate account_handle insert succeeded, want error")
	}

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO players (display_name, account_handle, password_hash, created_at)
		VALUES (?, ?, ?, ?)`,
		"Grace",
		"Grace",
		"hash-4",
		"not-a-timestamp",
	)
	if err == nil {
		t.Fatal("invalid created_at insert succeeded, want error")
	}

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO players (display_name, account_handle, password_hash, created_at)
		VALUES (?, ?, ?, ?)`,
		"Grace",
		"Grace-Offset",
		"hash-5",
		"2026-07-04T12:00:00+05:00",
	)
	if err == nil {
		t.Fatal("offset created_at insert succeeded, want error")
	}

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO players (account_handle, password_hash, created_at)
		VALUES (?, ?, ?)`,
		"Linus",
		"hash-6",
		"2026-07-04T12:00:03Z",
	)
	if err == nil {
		t.Fatal("NULL display_name insert succeeded, want error")
	}

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO players (display_name, password_hash, created_at)
		VALUES (?, ?, ?)`,
		"Linus",
		"hash-7",
		"2026-07-04T12:00:03Z",
	)
	if err == nil {
		t.Fatal("NULL account_handle insert succeeded, want error")
	}

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO players (display_name, account_handle, created_at)
		VALUES (?, ?, ?)`,
		"Linus",
		"Linus",
		"2026-07-04T12:00:03Z",
	)
	if err == nil {
		t.Fatal("NULL password_hash insert succeeded, want error")
	}

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO players (display_name, account_handle, password_hash, created_at)
		VALUES (?, ?, ?, ?)`,
		"Al",
		"Al",
		"hash-8",
		"2026-07-04T12:00:03Z",
	)
	if err == nil {
		t.Fatal("short display_name insert succeeded, want error")
	}

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO players (display_name, account_handle, password_hash, created_at)
		VALUES (?, ?, ?, ?)`,
		"Empty",
		"",
		"hash-9",
		"2026-07-04T12:00:03Z",
	)
	if err == nil {
		t.Fatal("empty account_handle insert succeeded, want error")
	}

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO players (display_name, account_handle, password_hash, created_at)
		VALUES (?, ?, ?, ?)`,
		"Empty",
		"EmptyHash",
		"",
		"2026-07-04T12:00:03Z",
	)
	if err == nil {
		t.Fatal("empty password_hash insert succeeded, want error")
	}

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO players (display_name, account_handle, password_hash, created_at)
		VALUES (?, ?, ?, ?)`,
		strings.Repeat("A", 11),
		"LongName",
		"hash-10",
		"2026-07-04T12:00:03Z",
	)
	if err == nil {
		t.Fatal("long display_name insert succeeded, want error")
	}

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO players (display_name, account_handle, password_hash, created_at)
		VALUES (?, ?, ?, ?)`,
		"Handle",
		strings.Repeat("A", 33),
		"hash-11",
		"2026-07-04T12:00:03Z",
	)
	if err == nil {
		t.Fatal("long account_handle insert succeeded, want error")
	}

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO players (display_name, account_handle, password_hash, created_at)
		VALUES (?, ?, ?, ?)`,
		"Hash",
		"Hash",
		strings.Repeat("h", 256),
		"2026-07-04T12:00:03Z",
	)
	if err == nil {
		t.Fatal("long password_hash insert succeeded, want error")
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sqlitedb.Open(context.Background(), filepath.Join(t.TempDir(), "find-ten.db"))
	if err != nil {
		t.Fatalf("sqlitedb.Open failed: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	})

	return db
}
