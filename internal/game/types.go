package game

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	MinSupportedBoardSize = 9
	MaxSupportedBoardSize = 11

	MinDurationSeconds     = 60
	DefaultDurationSeconds = 120
	MaxDurationSeconds     = 180
	// CoordinatedStartDelay is the lead-in and maximum allowed advance start time.
	CoordinatedStartDelay = 3 * time.Second
)

// ErrGameNotStarted is returned when an action arrives before a coordinated session opens.
var ErrGameNotStarted = errors.New("game has not started")

var supportedDurations = map[int]struct{}{
	60:  {},
	120: {},
	180: {},
}

type Board [][]int

type Position struct {
	Row int
	Col int
}

type Selection struct {
	Start Position
	End   Position
}

type PlayerActionType int

const (
	PlayerActionMove PlayerActionType = iota + 1
	PlayerActionReshuffle
	PlayerActionRemoveNumber
	PlayerActionHint
)

type PlayerActionRequest struct {
	Type      PlayerActionType
	Selection Selection
	Position  Position
	// Result must be buffered with room for one reply. RunGame sends exactly
	// one result for every action it accepts.
	Result chan error
	Hint   *Selection
}

type GameOverReason int

const (
	GameOverNone GameOverReason = iota
	GameOverNoValidMoves
	GameOverTimeExpired
	GameOverBoardCleared
)

type GameState struct {
	Board            Board
	ValidMoves       []Selection
	Score            int
	GameOver         bool
	GameOverReason   GameOverReason
	ReshuffleUsed    bool
	RemoveNumberUsed bool
	HintUsed         bool

	validMoveSet map[Selection]struct{}
}

type GameSnapshot struct {
	Sequence         int64
	Board            Board
	Score            int
	GameOver         bool
	GameOverReason   GameOverReason
	ValidMoveCount   int
	ReshuffleUsed    bool
	RemoveNumberUsed bool
	HintUsed         bool
	SnapshotTime     time.Time
}

// GameResult is the authoritative result captured when a session's game loop exits.
type GameResult struct {
	Score          int
	GameOverReason GameOverReason
	// Completed reports whether RunGame reached a terminal state before returning.
	// It remains true if cancellation happens after the game has ended.
	Completed bool
}

type GameSession struct {
	actions   chan PlayerActionRequest
	expired   chan struct{}
	snapshots chan GameSnapshot
	startsAt  time.Time
	expiresAt time.Time
	result    GameResult
	cancel    context.CancelFunc
	done      chan struct{}
	closing   chan struct{}

	mu       sync.Mutex
	stopping bool
	submits  sync.WaitGroup
	once     sync.Once
}

func IsSupportedBoardSize(size int) bool {
	return size >= MinSupportedBoardSize && size <= MaxSupportedBoardSize
}

func ValidateDuration(seconds int) error {
	if _, ok := supportedDurations[seconds]; !ok {
		return fmt.Errorf("unsupported duration %d: must be 60, 120, or 180", seconds)
	}
	return nil
}

func ValidateBoardSize(size int) error {
	if IsSupportedBoardSize(size) {
		return nil
	}

	return fmt.Errorf("unsupported board size %d: must be 9, 10, or 11", size)
}

func NormalizeSelection(selection Selection) Selection {
	startRow, endRow := ordered(selection.Start.Row, selection.End.Row)
	startCol, endCol := ordered(selection.Start.Col, selection.End.Col)

	return Selection{
		Start: Position{Row: startRow, Col: startCol},
		End:   Position{Row: endRow, Col: endCol},
	}
}

// Normalize the order of two integers, returning them in ascending order.
// So that (a, b) and (b, a) will be treated as the same selection on board.
func ordered(a, b int) (int, int) {
	if a <= b {
		return a, b
	}

	return b, a
}
