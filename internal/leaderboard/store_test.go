package leaderboard

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"find-ten-game/internal/player"
	"find-ten-game/internal/sqlitedb"
)

var testNow = time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)

const legacyLeaderboardScoresSchema = `CREATE TABLE leaderboard_scores (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	game_id TEXT NOT NULL UNIQUE,
	player_name TEXT NOT NULL,
	score INTEGER NOT NULL CHECK (score >= 0),
	grid_size INTEGER NOT NULL CHECK (grid_size > 0),
	duration_seconds INTEGER NOT NULL CHECK (duration_seconds > 0),
	remaining_millis INTEGER NOT NULL CHECK (remaining_millis >= 0),
	submitted_at TEXT NOT NULL CHECK (
		submitted_at GLOB '????-??-??T*Z'
		AND datetime(submitted_at) IS NOT NULL
	)
)`

func TestOpenCreatesSchema(t *testing.T) {
	store := openTestStore(t)

	var tableName string
	if err := store.db.QueryRowContext(
		context.Background(),
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'leaderboard_scores'`,
	).Scan(&tableName); err != nil {
		t.Fatalf("leaderboard_scores table lookup failed: %v", err)
	}
	if tableName != "leaderboard_scores" {
		t.Fatalf("table name = %q, want leaderboard_scores", tableName)
	}

	for _, want := range []string{"idx_leaderboard_scores_rank", "idx_leaderboard_scores_player"} {
		var indexName string
		if err := store.db.QueryRowContext(
			context.Background(),
			`SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?`,
			want,
		).Scan(&indexName); err != nil {
			t.Fatalf("%s lookup failed: %v", want, err)
		}
		if indexName != want {
			t.Fatalf("index name = %q, want %q", indexName, want)
		}
	}

	var journalMode string
	if err := store.db.QueryRowContext(context.Background(), `PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("journal_mode lookup failed: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
}

func TestNewStoreMigratesPlayerIDColumnIdempotently(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, filepath.Join(t.TempDir(), "scores.db"))
	if err != nil {
		t.Fatalf("sqlitedb.Open failed: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("database Close failed: %v", err)
		}
	}()

	if _, err := db.ExecContext(ctx, legacyLeaderboardScoresSchema); err != nil {
		t.Fatalf("create legacy leaderboard schema failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO leaderboard_scores (
			game_id, player_name, score, grid_size, duration_seconds, remaining_millis, submitted_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "legacy-game", "Ada", 1200, 10, 120, 2000, "2026-07-04T12:00:00Z"); err != nil {
		t.Fatalf("insert legacy score failed: %v", err)
	}

	if _, err := NewStore(ctx, db); err != nil {
		t.Fatalf("first NewStore failed: %v", err)
	}
	if _, err := NewStore(ctx, db); err != nil {
		t.Fatalf("second NewStore failed: %v", err)
	}

	var columnCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pragma_table_info('leaderboard_scores') WHERE name = 'player_id'
	`).Scan(&columnCount); err != nil {
		t.Fatalf("player_id column lookup failed: %v", err)
	}
	if columnCount != 1 {
		t.Fatalf("player_id column count = %d, want 1", columnCount)
	}

	var playerID sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT player_id FROM leaderboard_scores WHERE game_id = 'legacy-game'`).Scan(&playerID); err != nil {
		t.Fatalf("legacy player_id lookup failed: %v", err)
	}
	if playerID.Valid {
		t.Fatalf("legacy player_id = %d, want NULL", playerID.Int64)
	}
}

func TestSubmitScoreStoresOptionalPlayerIDWithoutChangingTopScores(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, filepath.Join(t.TempDir(), "scores.db"))
	if err != nil {
		t.Fatalf("sqlitedb.Open failed: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("database Close failed: %v", err)
		}
	}()

	playerStore, err := player.NewStore(ctx, db)
	if err != nil {
		t.Fatalf("player.NewStore failed: %v", err)
	}
	store, err := NewStore(ctx, db)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	account, err := playerStore.CreateAccount(ctx, player.CreateAccountInput{
		DisplayName: "Ada",
		Password:    "correct-password",
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	guest := testSubmission("guest-game", "Guest", 1000, 10, 120, 2000, testNow)
	if err := store.SubmitScore(ctx, guest); err != nil {
		t.Fatalf("guest SubmitScore failed: %v", err)
	}
	accountScore := testSubmission("account-game", "Ada", 1200, 10, 120, 3000, testNow.Add(time.Second))
	accountScore.PlayerID = &account.ID
	if err := store.SubmitScore(ctx, accountScore); err != nil {
		t.Fatalf("account SubmitScore failed: %v", err)
	}

	var guestPlayerID, accountPlayerID sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT player_id FROM leaderboard_scores WHERE game_id = ?`, guest.GameID).Scan(&guestPlayerID); err != nil {
		t.Fatalf("guest player_id lookup failed: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT player_id FROM leaderboard_scores WHERE game_id = ?`, accountScore.GameID).Scan(&accountPlayerID); err != nil {
		t.Fatalf("account player_id lookup failed: %v", err)
	}
	if guestPlayerID.Valid {
		t.Fatalf("guest player_id = %d, want NULL", guestPlayerID.Int64)
	}
	if !accountPlayerID.Valid || accountPlayerID.Int64 != account.ID {
		t.Fatalf("account player_id = %#v, want %d", accountPlayerID, account.ID)
	}

	scores, err := store.TopScores(ctx, TopScoresFilter{GridSize: 10, DurationSeconds: 120})
	if err != nil {
		t.Fatalf("TopScores failed: %v", err)
	}
	if len(scores) != 2 {
		t.Fatalf("len(scores) = %d, want 2", len(scores))
	}
	if scores[0].PlayerName != "Ada" || scores[1].PlayerName != "Guest" {
		t.Fatalf("top score names = %q, %q, want Ada, Guest", scores[0].PlayerName, scores[1].PlayerName)
	}
}

func TestOpenReusesExistingDatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "scores.db")
	submission := testSubmission("game-1", "Ada", 1200, 10, 120, 2000, time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC))

	store, err := open(ctx, path, testClock)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if err := store.SubmitScore(ctx, submission); err != nil {
		t.Fatalf("SubmitScore failed: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	defer closeStore(t, reopened)

	scores, err := reopened.TopScores(ctx, TopScoresFilter{GridSize: 10, DurationSeconds: 120})
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

func TestOpenRejectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Open(ctx, filepath.Join(t.TempDir(), "scores.db"))
	if err == nil {
		t.Fatal("Open succeeded, want error")
	}
}

func TestOpenStoreCloseClosesOwnedDatabase(t *testing.T) {
	store, err := open(context.Background(), filepath.Join(t.TempDir(), "scores.db"), testClock)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	db := store.db

	closeStore(t, store)

	if err := db.QueryRowContext(context.Background(), `SELECT 1`).Scan(new(int)); err == nil {
		t.Fatal("query after Close succeeded, want owned database to be closed")
	}
}

func TestNewStoreIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, filepath.Join(t.TempDir(), "scores.db"))
	if err != nil {
		t.Fatalf("sqlitedb.Open failed: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("database Close failed: %v", err)
		}
	}()
	store, err := NewStore(ctx, db)
	if err != nil {
		t.Fatalf("first NewStore failed: %v", err)
	}
	submission := testSubmission("game-1", "Ada", 1200, 10, 120, 2000, time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC))
	if err := store.SubmitScore(ctx, submission); err != nil {
		t.Fatalf("SubmitScore failed: %v", err)
	}

	reinitialized, err := NewStore(ctx, db)
	if err != nil {
		t.Fatalf("second NewStore failed: %v", err)
	}
	scores, err := reinitialized.TopScores(ctx, TopScoresFilter{GridSize: 10, DurationSeconds: 120})
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

func TestNewStoreRejectsCanceledContext(t *testing.T) {
	db, err := sqlitedb.Open(context.Background(), filepath.Join(t.TempDir(), "scores.db"))
	if err != nil {
		t.Fatalf("sqlitedb.Open failed: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("database Close failed: %v", err)
		}
	}()

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

func TestNewStoreCloseKeepsSharedDatabaseOpen(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, filepath.Join(t.TempDir(), "scores.db"))
	if err != nil {
		t.Fatalf("sqlitedb.Open failed: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("database Close failed: %v", err)
		}
	}()

	store, err := NewStore(ctx, db)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("store Close failed: %v", err)
	}

	var got int
	if err := db.QueryRowContext(ctx, `SELECT 1`).Scan(&got); err != nil {
		t.Fatalf("query after store Close failed: %v", err)
	}
	if got != 1 {
		t.Fatalf("query result = %d, want 1", got)
	}
}

func TestSubmitScoreRejectsDuplicateGameID(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	submission := testSubmission("game-1", "Ada", 1200, 10, 120, 2000, time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC))
	if err := store.SubmitScore(ctx, submission); err != nil {
		t.Fatalf("SubmitScore failed: %v", err)
	}

	err := store.SubmitScore(ctx, submission)
	if !errors.Is(err, ErrDuplicateGameID) {
		t.Fatalf("duplicate SubmitScore err = %v, want ErrDuplicateGameID", err)
	}
}

func TestSubmitScoreRejectsUnknownPlayerID(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	playerID := int64(999)
	submission := testSubmission("game-1", "Ada", 1200, 10, 120, 2000, testNow)
	submission.PlayerID = &playerID

	err := store.SubmitScore(ctx, submission)
	if !errors.Is(err, ErrInvalidScoreSubmission) {
		t.Fatalf("SubmitScore error = %v, want %v", err, ErrInvalidScoreSubmission)
	}
}

func TestTopScoresOrdersAndFilters(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	submissions := []ScoreSubmission{
		testSubmission("score-low", "Low", 1000, 10, 120, 3000, time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)),
		testSubmission("score-high-late", "High Late", 2000, 10, 120, 4000, time.Date(2026, 7, 4, 12, 0, 3, 0, time.UTC)),
		testSubmission("score-high-early", "High Early", 2000, 10, 120, 4000, time.Date(2026, 7, 4, 12, 0, 2, 0, time.UTC)),
		testSubmission("score-high-more-time", "High More", 2000, 10, 120, 5000, time.Date(2026, 7, 4, 12, 0, 4, 0, time.UTC)),
		testSubmission("other-grid", "Other Grid", 9999, 11, 120, 9999, time.Date(2026, 7, 4, 12, 0, 5, 0, time.UTC)),
		testSubmission("other-duration", "Other Dur", 9999, 10, 180, 9999, time.Date(2026, 7, 4, 12, 0, 6, 0, time.UTC)),
	}
	for _, submission := range submissions {
		if err := store.SubmitScore(ctx, submission); err != nil {
			t.Fatalf("SubmitScore(%q) failed: %v", submission.GameID, err)
		}
	}

	scores, err := store.TopScores(ctx, TopScoresFilter{GridSize: 10, DurationSeconds: 120, Limit: 3})
	if err != nil {
		t.Fatalf("TopScores failed: %v", err)
	}

	got := gameIDs(scores)
	want := []string{"score-high-more-time", "score-high-early", "score-high-late"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("gameIDs = %v, want %v", got, want)
	}
}

func TestTopScoresUsesIDAsFinalTieBreaker(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	submittedAt := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)

	submissions := []ScoreSubmission{
		testSubmission("same-rank-1", "Ada", 2000, 10, 120, 4000, submittedAt),
		testSubmission("same-rank-2", "Grace", 2000, 10, 120, 4000, submittedAt),
		testSubmission("same-rank-3", "Linus", 2000, 10, 120, 4000, submittedAt),
	}
	for _, submission := range submissions {
		if err := store.SubmitScore(ctx, submission); err != nil {
			t.Fatalf("SubmitScore(%q) failed: %v", submission.GameID, err)
		}
	}

	scores, err := store.TopScores(ctx, TopScoresFilter{GridSize: 10, DurationSeconds: 120})
	if err != nil {
		t.Fatalf("TopScores failed: %v", err)
	}

	got := gameIDs(scores)
	want := []string{"same-rank-1", "same-rank-2", "same-rank-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("gameIDs = %v, want %v", got, want)
	}
	for i, score := range scores {
		if score.ID == 0 {
			t.Fatalf("scores[%d].ID = 0, want database row ID", i)
		}
		if i > 0 && scores[i-1].ID >= score.ID {
			t.Fatalf("scores not ordered by ascending ID: previous=%d current=%d", scores[i-1].ID, score.ID)
		}
	}
}

func TestTopScoresReturnsEmptySliceWhenNoScoresMatch(t *testing.T) {
	store := openTestStore(t)

	scores, err := store.TopScores(context.Background(), TopScoresFilter{GridSize: 10, DurationSeconds: 120})
	if err != nil {
		t.Fatalf("TopScores failed: %v", err)
	}
	if scores == nil {
		t.Fatal("scores is nil, want empty slice")
	}
	if len(scores) != 0 {
		t.Fatalf("len(scores) = %d, want 0", len(scores))
	}
}

func TestTopScoresAppliesDefaultAndMaximumLimit(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	submittedAt := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)

	for i := 0; i < MaxTopScoresLimit+1; i++ {
		submission := testSubmission(
			fmt.Sprintf("game-%04d", i),
			"Ada",
			i,
			10,
			120,
			0,
			submittedAt.Add(time.Duration(i)*time.Second),
		)
		if err := store.SubmitScore(ctx, submission); err != nil {
			t.Fatalf("SubmitScore(%q) failed: %v", submission.GameID, err)
		}
	}

	defaultScores, err := store.TopScores(ctx, TopScoresFilter{GridSize: 10, DurationSeconds: 120})
	if err != nil {
		t.Fatalf("TopScores default failed: %v", err)
	}
	if len(defaultScores) != defaultTopScoresLimit {
		t.Fatalf("default len(scores) = %d, want %d", len(defaultScores), defaultTopScoresLimit)
	}

	cappedScores, err := store.TopScores(ctx, TopScoresFilter{GridSize: 10, DurationSeconds: 120, Limit: MaxTopScoresLimit + 1})
	if err != nil {
		t.Fatalf("TopScores capped failed: %v", err)
	}
	if len(cappedScores) != MaxTopScoresLimit {
		t.Fatalf("capped len(scores) = %d, want %d", len(cappedScores), MaxTopScoresLimit)
	}
}

func TestSubmitScoreHandlesConcurrentSubmissions(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	submittedAt := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)

	const submissionCount = 20
	var wg sync.WaitGroup
	errs := make(chan error, submissionCount)
	for i := 0; i < submissionCount; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			submission := testSubmission(
				fmt.Sprintf("game-%04d", i),
				"Ada",
				i,
				10,
				120,
				0,
				submittedAt.Add(time.Duration(i)*time.Second),
			)
			errs <- store.SubmitScore(ctx, submission)
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("SubmitScore returned error during concurrent submissions: %v", err)
		}
	}

	scores, err := store.TopScores(ctx, TopScoresFilter{GridSize: 10, DurationSeconds: 120, Limit: submissionCount})
	if err != nil {
		t.Fatalf("TopScores failed: %v", err)
	}
	if len(scores) != submissionCount {
		t.Fatalf("len(scores) = %d, want %d", len(scores), submissionCount)
	}
}

func TestSubmitScoreRejectsInvalidSubmission(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	store := openTestStoreWithClock(t, func() time.Time {
		return now
	})

	valid := testSubmission("game-1", "Ada", 1200, 10, 120, 2000, now)
	tests := []struct {
		name       string
		submission ScoreSubmission
	}{
		{
			name:       "missing game ID",
			submission: withGameID(valid, ""),
		},
		{
			name:       "game ID too long",
			submission: withGameID(valid, strings.Repeat("a", MaxGameIDLength+1)),
		},
		{
			name:       "missing player name",
			submission: withPlayerName(valid, ""),
		},
		{
			name:       "whitespace player name",
			submission: withPlayerName(valid, "   "),
		},
		{
			name:       "player name too long",
			submission: withPlayerName(valid, strings.Repeat("a", MaxPlayerNameLength+1)),
		},
		{
			name:       "player name control character",
			submission: withPlayerName(valid, "Ada\nLovelace"),
		},
		{
			name:       "player name HTML",
			submission: withPlayerName(valid, "<script>"),
		},
		{
			name:       "player name has no letter or digit",
			submission: withPlayerName(valid, "'''"),
		},
		{
			name:       "player name non ASCII",
			submission: withPlayerName(valid, "界"),
		},
		{
			name:       "zero player ID",
			submission: withPlayerID(valid, ptrInt64(0)),
		},
		{
			name:       "negative player ID",
			submission: withPlayerID(valid, ptrInt64(-1)),
		},
		{
			name:       "negative score",
			submission: withScore(valid, -1),
		},
		{
			name:       "zero grid size",
			submission: withGridSize(valid, 0),
		},
		{
			name:       "unsupported grid size",
			submission: withGridSize(valid, 99),
		},
		{
			name:       "zero duration",
			submission: withDurationSeconds(valid, 0),
		},
		{
			name:       "unsupported duration",
			submission: withDurationSeconds(valid, 90),
		},
		{
			name:       "negative remaining millis",
			submission: withRemainingMillis(valid, -1),
		},
		{
			name:       "remaining millis above duration",
			submission: withRemainingMillis(valid, 120*1000+1),
		},
		{
			name:       "missing submitted at",
			submission: withSubmittedAt(valid, time.Time{}),
		},
		{
			name:       "submitted at too far in the future",
			submission: withSubmittedAt(valid, now.Add(maxFutureSubmittedAtSkew+time.Second)),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := store.SubmitScore(ctx, test.submission)
			if !errors.Is(err, ErrInvalidScoreSubmission) {
				t.Fatalf("SubmitScore err = %v, want ErrInvalidScoreSubmission", err)
			}
		})
	}
}

func TestOpenRejectsEmptyPath(t *testing.T) {
	_, err := Open(context.Background(), "")
	if err == nil {
		t.Fatal("Open succeeded, want error")
	}
}

func TestOpenAcceptsRelativePath(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	store, err := Open(context.Background(), "./data/scores.db")
	if err != nil {
		t.Fatalf("Open relative path failed: %v", err)
	}
	closeStore(t, store)

	if _, err := os.Stat(filepath.Join(dir, "data", "scores.db")); err != nil {
		t.Fatalf("relative database file lookup failed: %v", err)
	}
}

func TestSubmitScoreAcceptsMaxLengthPlayerName(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	name := strings.Repeat("A", MaxPlayerNameLength)
	submission := testSubmission("game-1", name, 1200, 10, 120, 2000, time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC))
	if err := store.SubmitScore(ctx, submission); err != nil {
		t.Fatalf("SubmitScore failed: %v", err)
	}

	scores, err := store.TopScores(ctx, TopScoresFilter{GridSize: 10, DurationSeconds: 120})
	if err != nil {
		t.Fatalf("TopScores failed: %v", err)
	}
	if len(scores) != 1 {
		t.Fatalf("len(scores) = %d, want 1", len(scores))
	}
	if scores[0].PlayerName != name {
		t.Fatalf("PlayerName = %q, want %q", scores[0].PlayerName, name)
	}
}

func TestSubmitScoreTrimsPlayerName(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	submission := testSubmission("game-1", "  Ada  ", 1200, 10, 120, 2000, time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC))
	if err := store.SubmitScore(ctx, submission); err != nil {
		t.Fatalf("SubmitScore failed: %v", err)
	}

	scores, err := store.TopScores(ctx, TopScoresFilter{GridSize: 10, DurationSeconds: 120})
	if err != nil {
		t.Fatalf("TopScores failed: %v", err)
	}
	if len(scores) != 1 {
		t.Fatalf("len(scores) = %d, want 1", len(scores))
	}
	if scores[0].PlayerName != "Ada" {
		t.Fatalf("PlayerName = %q, want Ada", scores[0].PlayerName)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()

	return openTestStoreWithClock(t, testClock)
}

func openTestStoreWithClock(t *testing.T, now func() time.Time) *Store {
	t.Helper()

	store, err := open(context.Background(), filepath.Join(t.TempDir(), "scores.db"), now)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() {
		closeStore(t, store)
	})

	return store
}

func testClock() time.Time {
	return testNow
}

func closeStore(t *testing.T, store *Store) {
	t.Helper()

	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func testSubmission(gameID, playerName string, score, gridSize, durationSeconds, remainingMillis int, submittedAt time.Time) ScoreSubmission {
	return ScoreSubmission{
		GameID:          gameID,
		PlayerName:      playerName,
		Score:           score,
		GridSize:        gridSize,
		DurationSeconds: durationSeconds,
		RemainingMillis: remainingMillis,
		SubmittedAt:     submittedAt,
	}
}

func withGameID(submission ScoreSubmission, gameID string) ScoreSubmission {
	submission.GameID = gameID
	return submission
}

func withPlayerName(submission ScoreSubmission, playerName string) ScoreSubmission {
	submission.PlayerName = playerName
	return submission
}

func withPlayerID(submission ScoreSubmission, playerID *int64) ScoreSubmission {
	submission.PlayerID = playerID
	return submission
}

func ptrInt64(value int64) *int64 {
	return &value
}

func withScore(submission ScoreSubmission, score int) ScoreSubmission {
	submission.Score = score
	return submission
}

func withGridSize(submission ScoreSubmission, gridSize int) ScoreSubmission {
	submission.GridSize = gridSize
	return submission
}

func withDurationSeconds(submission ScoreSubmission, durationSeconds int) ScoreSubmission {
	submission.DurationSeconds = durationSeconds
	return submission
}

func withRemainingMillis(submission ScoreSubmission, remainingMillis int) ScoreSubmission {
	submission.RemainingMillis = remainingMillis
	return submission
}

func withSubmittedAt(submission ScoreSubmission, submittedAt time.Time) ScoreSubmission {
	submission.SubmittedAt = submittedAt
	return submission
}

func gameIDs(scores []ScoreEntry) []string {
	ids := make([]string, 0, len(scores))
	for _, score := range scores {
		ids = append(ids, score.GameID)
	}

	return ids
}
