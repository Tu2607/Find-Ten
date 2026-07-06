package leaderboard

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

var testNow = time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)

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

	var indexName string
	if err := store.db.QueryRowContext(
		context.Background(),
		`SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'idx_leaderboard_scores_rank'`,
	).Scan(&indexName); err != nil {
		t.Fatalf("idx_leaderboard_scores_rank lookup failed: %v", err)
	}
	if indexName != "idx_leaderboard_scores_rank" {
		t.Fatalf("index name = %q, want idx_leaderboard_scores_rank", indexName)
	}

	var journalMode string
	if err := store.db.QueryRowContext(context.Background(), `PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("journal_mode lookup failed: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
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
