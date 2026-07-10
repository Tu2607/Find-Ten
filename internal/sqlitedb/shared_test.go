package sqlitedb_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"find-ten-game/internal/leaderboard"
	"find-ten-game/internal/player"
	"find-ten-game/internal/sqlitedb"
)

func TestPlayerAndLeaderboardStoresShareDatabase(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, filepath.Join(t.TempDir(), "find-ten.db"))
	if err != nil {
		t.Fatalf("sqlitedb.Open failed: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}()

	if _, err := player.NewStore(ctx, db); err != nil {
		t.Fatalf("player NewStore failed: %v", err)
	}
	leaderboardStore, err := leaderboard.NewStore(ctx, db)
	if err != nil {
		t.Fatalf("leaderboard NewStore failed: %v", err)
	}

	submission := leaderboard.ScoreSubmission{
		GameID:          "game-1",
		PlayerName:      "Ada",
		Score:           1200,
		GridSize:        10,
		DurationSeconds: 120,
		RemainingMillis: 2000,
		SubmittedAt:     time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
	}
	if err := leaderboardStore.SubmitScore(ctx, submission); err != nil {
		t.Fatalf("SubmitScore failed: %v", err)
	}

	for _, table := range []string{"players", "leaderboard_scores"} {
		var tableName string
		if err := db.QueryRowContext(
			ctx,
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
			table,
		).Scan(&tableName); err != nil {
			t.Fatalf("%s table lookup failed: %v", table, err)
		}
		if tableName != table {
			t.Fatalf("table name = %q, want %s", tableName, table)
		}
	}

	scores, err := leaderboardStore.TopScores(ctx, leaderboard.TopScoresFilter{GridSize: 10, DurationSeconds: 120})
	if err != nil {
		t.Fatalf("TopScores failed: %v", err)
	}
	if len(scores) != 1 {
		t.Fatalf("len(scores) = %d, want 1", len(scores))
	}
	if scores[0].GameID != submission.GameID {
		t.Fatalf("score GameID = %q, want %q", scores[0].GameID, submission.GameID)
	}
}

func TestStoresInitializeConcurrentlyOnSharedDatabase(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, filepath.Join(t.TempDir(), "find-ten.db"))
	if err != nil {
		t.Fatalf("sqlitedb.Open failed: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}()

	errs := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := player.NewStore(ctx, db)
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := leaderboard.NewStore(ctx, db)
		errs <- err
	}()
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent store initialization failed: %v", err)
		}
	}

	assertTableExists(t, ctx, db, "players")
	assertTableExists(t, ctx, db, "leaderboard_scores")
}

func assertTableExists(t *testing.T, ctx context.Context, db queryer, table string) {
	t.Helper()

	var tableName string
	if err := db.QueryRowContext(
		ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
		table,
	).Scan(&tableName); err != nil {
		t.Fatalf("%s table lookup failed: %v", table, err)
	}
	if tableName != table {
		t.Fatalf("table name = %q, want %s", tableName, table)
	}
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
