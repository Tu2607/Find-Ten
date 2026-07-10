package leaderboard

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"find-ten-game/internal/sqlitedb"

	sqlitedriver "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const (
	defaultTopScoresLimit = 15
	MaxTopScoresLimit     = 100
)

var (
	ErrDuplicateGameID        = errors.New("leaderboard score already submitted for game")
	ErrInvalidScoreSubmission = errors.New("invalid leaderboard score submission")
)

type Store struct {
	db      *sql.DB
	now     func() time.Time
	closeDB func() error
}

type ScoreSubmission struct {
	GameID          string
	PlayerName      string
	Score           int
	GridSize        int
	DurationSeconds int
	RemainingMillis int
	SubmittedAt     time.Time
}

type ScoreEntry struct {
	ID              int64
	GameID          string
	PlayerName      string
	Score           int
	GridSize        int
	DurationSeconds int
	RemainingMillis int
	SubmittedAt     time.Time
}

type TopScoresFilter struct {
	GridSize        int
	DurationSeconds int
	Limit           int
}

func NewStore(ctx context.Context, db *sql.DB) (*Store, error) {
	return newStore(ctx, db, time.Now)
}

// Open creates a standalone SQLite connection for compatibility with tests and
// single-store callers. Production paths that need multiple persistence stores
// should open one database with sqlitedb.Open and pass it to NewStore.
func Open(ctx context.Context, path string) (*Store, error) {
	return open(ctx, path, time.Now)
}

func open(ctx context.Context, path string, now func() time.Time) (*Store, error) {
	db, err := sqlitedb.Open(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("open leaderboard store: %w", err)
	}

	store, err := newStore(ctx, db, now)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	store.closeDB = db.Close

	return store, nil
}

func (s *Store) Close() error {
	return s.closeDB()
}

// newStore does not own db. Callers that open the database are responsible for
// closing it on both success and initialization failure.
func newStore(ctx context.Context, db *sql.DB, now func() time.Time) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("new leaderboard store: database is required")
	}

	store := &Store{
		db:      db,
		now:     now,
		closeDB: func() error { return nil },
	}
	if err := store.init(ctx); err != nil {
		return nil, err
	}

	return store, nil
}

// Future HTTP handlers should pass r.Context() into Store methods so client
// disconnects and request deadlines cancel database work promptly.
func (s *Store) SubmitScore(ctx context.Context, submission ScoreSubmission) error {
	normalized := normalizeSubmission(submission)
	if err := validateSubmission(normalized, s.now()); err != nil {
		return err
	}

	submittedAt := normalized.SubmittedAt.UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO leaderboard_scores (
			game_id,
			player_name,
			score,
			grid_size,
			duration_seconds,
			remaining_millis,
			submitted_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		normalized.GameID,
		normalized.PlayerName,
		normalized.Score,
		normalized.GridSize,
		normalized.DurationSeconds,
		normalized.RemainingMillis,
		formatTime(submittedAt),
	)
	if err != nil {
		switch code, ok := sqliteConstraintCode(err); {
		case ok && code == sqlite3.SQLITE_CONSTRAINT_UNIQUE:
			return ErrDuplicateGameID
		case ok && code == sqlite3.SQLITE_CONSTRAINT_CHECK:
			return ErrInvalidScoreSubmission
		}
		return fmt.Errorf("submit leaderboard score: %w", err)
	}

	return nil
}

func (s *Store) TopScores(ctx context.Context, filter TopScoresFilter) ([]ScoreEntry, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultTopScoresLimit
	}
	if limit > MaxTopScoresLimit {
		limit = MaxTopScoresLimit
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id,
			game_id,
			player_name,
			score,
			grid_size,
			duration_seconds,
			remaining_millis,
			submitted_at
		FROM leaderboard_scores
		WHERE grid_size = ? AND duration_seconds = ?
		ORDER BY score DESC, remaining_millis DESC, submitted_at ASC, id ASC
		LIMIT ?
	`, filter.GridSize, filter.DurationSeconds, limit)
	if err != nil {
		return nil, fmt.Errorf("query top leaderboard scores: %w", err)
	}
	defer rows.Close()

	scores := make([]ScoreEntry, 0)
	for rows.Next() {
		var score ScoreEntry
		var submittedAt string
		if err := rows.Scan(
			&score.ID,
			&score.GameID,
			&score.PlayerName,
			&score.Score,
			&score.GridSize,
			&score.DurationSeconds,
			&score.RemainingMillis,
			&submittedAt,
		); err != nil {
			return nil, fmt.Errorf("scan leaderboard score: %w", err)
		}
		score.SubmittedAt, err = parseTime(submittedAt)
		if err != nil {
			return nil, fmt.Errorf("parse leaderboard submitted_at: %w", err)
		}
		scores = append(scores, score)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate leaderboard scores: %w", err)
	}

	return scores, nil
}

func (s *Store) init(ctx context.Context) error {
	for _, statement := range schemaStatements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize leaderboard schema: %w", err)
		}
	}

	return nil
}

func sqliteConstraintCode(err error) (int, bool) {
	var sqliteErr *sqlitedriver.Error
	if !errors.As(err, &sqliteErr) {
		return 0, false
	}

	if sqliteErr.Code()&0xff != sqlite3.SQLITE_CONSTRAINT {
		return 0, false
	}

	return sqliteErr.Code(), true
}

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS leaderboard_scores (
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
	)`,
	`CREATE INDEX IF NOT EXISTS idx_leaderboard_scores_rank
	ON leaderboard_scores (
		grid_size,
		duration_seconds,
		score DESC,
		remaining_millis DESC,
		submitted_at ASC,
		id ASC
	)`,
}
