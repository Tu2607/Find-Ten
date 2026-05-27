package game

import (
	"errors"
	"reflect"
	"testing"
)

func TestApplyMoveClearsCellsAndScoresNonZeroCells(t *testing.T) {
	state := &GameState{
		Board: Board{
			{5, 0, 5},
			{9, 9, 9},
		},
	}
	rebuildValidMoveCache(state)

	err := ApplyMove(state, Selection{
		Start: Position{Row: 0, Col: 0},
		End:   Position{Row: 0, Col: 2},
	})
	if err != nil {
		t.Fatalf("ApplyMove returned unexpected error: %v", err)
	}

	wantBoard := Board{
		{0, 0, 0},
		{9, 9, 9},
	}
	if !reflect.DeepEqual(state.Board, wantBoard) {
		t.Fatalf("state.Board = %+v, want %+v", state.Board, wantBoard)
	}
	if state.Score != 200 {
		t.Fatalf("state.Score = %d, want 200", state.Score)
	}
}

func TestApplyMoveNormalizesSelection(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{9, 9},
		},
	}
	rebuildValidMoveCache(state)

	err := ApplyMove(state, Selection{
		Start: Position{Row: 0, Col: 1},
		End:   Position{Row: 0, Col: 0},
	})
	if err != nil {
		t.Fatalf("ApplyMove returned unexpected error: %v", err)
	}

	if state.Board[0][0] != 0 || state.Board[0][1] != 0 {
		t.Fatalf("state.Board top row = %+v, want cleared cells", state.Board[0])
	}
}

func TestApplyMoveRejectsInvalidMoveWithoutMutation(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{9, 9},
		},
	}
	rebuildValidMoveCache(state)
	beforeBoard := cloneBoardForTest(state.Board)
	beforeScore := state.Score
	beforeMoves := append([]Selection(nil), state.ValidMoves...)

	err := ApplyMove(state, Selection{
		Start: Position{Row: 1, Col: 0},
		End:   Position{Row: 1, Col: 1},
	})
	if !errors.Is(err, ErrInvalidMove) {
		t.Fatalf("ApplyMove error = %v, want %v", err, ErrInvalidMove)
	}

	if !reflect.DeepEqual(state.Board, beforeBoard) {
		t.Fatalf("state.Board mutated to %+v, want %+v", state.Board, beforeBoard)
	}
	if state.Score != beforeScore {
		t.Fatalf("state.Score = %d, want %d", state.Score, beforeScore)
	}
	if !reflect.DeepEqual(state.ValidMoves, beforeMoves) {
		t.Fatalf("state.ValidMoves mutated to %+v, want %+v", state.ValidMoves, beforeMoves)
	}
}

func TestApplyMoveRejectsOutOfBoundsMove(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{9, 9},
		},
	}
	rebuildValidMoveCache(state)

	err := ApplyMove(state, Selection{
		Start: Position{Row: 0, Col: 0},
		End:   Position{Row: 0, Col: 2},
	})
	if !errors.Is(err, ErrOutOfBounds) {
		t.Fatalf("ApplyMove error = %v, want %v", err, ErrOutOfBounds)
	}
}

func TestApplyMoveRejectsMoveAfterGameOver(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{9, 9},
		},
		GameOver: true,
	}
	rebuildValidMoveCache(state)

	err := ApplyMove(state, Selection{
		Start: Position{Row: 0, Col: 0},
		End:   Position{Row: 0, Col: 1},
	})
	if !errors.Is(err, ErrGameOver) {
		t.Fatalf("ApplyMove error = %v, want %v", err, ErrGameOver)
	}
}

func TestApplyMoveRebuildsCacheAndSetsGameOver(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{9, 9},
		},
	}
	rebuildValidMoveCache(state)

	err := ApplyMove(state, Selection{
		Start: Position{Row: 0, Col: 0},
		End:   Position{Row: 0, Col: 1},
	})
	if err != nil {
		t.Fatalf("ApplyMove returned unexpected error: %v", err)
	}

	if len(state.ValidMoves) != 0 {
		t.Fatalf("len(state.ValidMoves) = %d, want 0", len(state.ValidMoves))
	}
	if len(state.validMoveSet) != 0 {
		t.Fatalf("len(state.validMoveSet) = %d, want 0", len(state.validMoveSet))
	}
	if !state.GameOver {
		t.Fatal("state.GameOver = false, want true")
	}
}

func cloneBoardForTest(board Board) Board {
	clone := make(Board, len(board))
	for row := range board {
		clone[row] = append([]int(nil), board[row]...)
	}

	return clone
}
