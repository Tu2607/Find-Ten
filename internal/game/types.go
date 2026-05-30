package game

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	MinSupportedBoardSize = 9
	MaxSupportedBoardSize = 11
)

type Board [][]int

type Position struct {
	Row int
	Col int
}

type Selection struct {
	Start Position
	End   Position
}

type EventType int

// Enum for different event types
const (
	EventMove EventType = iota
	EventTick
)

type Event struct {
	Type   EventType
	Move   Selection
	Result chan error
}

type GameOverReason int

const (
	GameOverNone GameOverReason = iota
	GameOverNoValidMoves
	GameOverTimeExpired
)

type GameState struct {
	Board          Board
	ValidMoves     []Selection
	Score          int
	GameOver       bool
	GameOverReason GameOverReason
	RemainingTime  int

	validMoveSet map[Selection]struct{}
}

type GameSnapshot struct {
	Sequence       int64
	Board          Board
	Score          int
	GameOver       bool
	GameOverReason GameOverReason
	RemainingTime  int
	ValidMoveCount int
	SnapshotTime   time.Time
}

type GameSession struct {
	events    chan Event
	snapshots chan GameSnapshot
	cancel    context.CancelFunc
	done      chan struct{}
	once      sync.Once
}

func IsSupportedBoardSize(size int) bool {
	return size >= MinSupportedBoardSize && size <= MaxSupportedBoardSize
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
