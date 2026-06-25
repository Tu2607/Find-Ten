package game

import (
	"errors"
	"reflect"
	"testing"
)

func TestApplyRemoveNumberClearsOneCellWithoutScoring(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{1, 9},
		},
		Score: 300,
	}
	rebuildValidMoveCache(state)

	err := ApplyRemoveNumber(state, Position{Row: 0, Col: 0})
	if err != nil {
		t.Fatalf("ApplyRemoveNumber returned unexpected error: %v", err)
	}

	wantBoard := Board{
		{0, 6},
		{1, 9},
	}
	if !reflect.DeepEqual(state.Board, wantBoard) {
		t.Fatalf("state.Board = %+v, want %+v", state.Board, wantBoard)
	}
	if state.Score != 300 {
		t.Fatalf("state.Score = %d, want 300", state.Score)
	}
	if !state.RemoveNumberUsed {
		t.Fatal("state.RemoveNumberUsed = false, want true")
	}
	if len(state.ValidMoves) == 0 {
		t.Fatal("len(state.ValidMoves) = 0, want remaining valid moves")
	}
	if len(state.validMoveSet) != len(state.ValidMoves) {
		t.Fatalf("len(state.validMoveSet) = %d, want %d", len(state.validMoveSet), len(state.ValidMoves))
	}
}

func TestApplyRemoveNumberRejectsInvalidTargetsWithoutMutation(t *testing.T) {
	tests := []struct {
		name     string
		state    *GameState
		position Position
		want     error
	}{
		{
			name:     "nil state",
			position: Position{Row: 0, Col: 0},
			want:     ErrNilGameState,
		},
		{
			name: "game over",
			state: &GameState{
				Board:    Board{{4, 6}},
				GameOver: true,
			},
			position: Position{Row: 0, Col: 0},
			want:     ErrGameOver,
		},
		{
			name: "uninitialized valid move cache",
			state: &GameState{
				Board: Board{{4, 6}},
			},
			position: Position{Row: 0, Col: 0},
			want:     ErrUninitializedMove,
		},
		{
			name: "already used",
			state: &GameState{
				Board:            Board{{4, 6}},
				RemoveNumberUsed: true,
			},
			position: Position{Row: 0, Col: 0},
			want:     ErrRemoveNumberAlreadyUsed,
		},
		{
			name: "out of bounds",
			state: &GameState{
				Board: Board{{4, 6}},
			},
			position: Position{Row: 0, Col: 2},
			want:     ErrOutOfBounds,
		},
		{
			name: "already cleared",
			state: &GameState{
				Board: Board{{0, 6}},
			},
			position: Position{Row: 0, Col: 0},
			want:     ErrRemoveNumberInvalidTarget,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.state != nil && test.want != ErrUninitializedMove {
				rebuildValidMoveCache(test.state)
			}
			var beforeBoard Board
			var beforeMoves []Selection
			var beforeUsed bool
			if test.state != nil {
				beforeBoard = cloneBoardForTest(test.state.Board)
				beforeMoves = append([]Selection(nil), test.state.ValidMoves...)
				beforeUsed = test.state.RemoveNumberUsed
			}

			err := ApplyRemoveNumber(test.state, test.position)
			if !errors.Is(err, test.want) {
				t.Fatalf("ApplyRemoveNumber error = %v, want %v", err, test.want)
			}

			if test.state == nil {
				return
			}
			if !reflect.DeepEqual(test.state.Board, beforeBoard) {
				t.Fatalf("state.Board mutated to %+v, want %+v", test.state.Board, beforeBoard)
			}
			if !sameSelections(test.state.ValidMoves, beforeMoves) {
				t.Fatalf("state.ValidMoves mutated to %+v, want %+v", test.state.ValidMoves, beforeMoves)
			}
			if test.state.RemoveNumberUsed != beforeUsed {
				t.Fatalf("state.RemoveNumberUsed = %t, want %t", test.state.RemoveNumberUsed, beforeUsed)
			}
		})
	}
}

func sameSelections(a, b []Selection) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}

	return true
}

func TestApplyRemoveNumberRejectsSecondUseWithoutMutation(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{1, 9},
		},
	}
	rebuildValidMoveCache(state)

	if err := ApplyRemoveNumber(state, Position{Row: 0, Col: 0}); err != nil {
		t.Fatalf("first ApplyRemoveNumber returned unexpected error: %v", err)
	}

	beforeBoard := cloneBoardForTest(state.Board)
	beforeMoves := append([]Selection(nil), state.ValidMoves...)

	err := ApplyRemoveNumber(state, Position{Row: 0, Col: 1})
	if !errors.Is(err, ErrRemoveNumberAlreadyUsed) {
		t.Fatalf("second ApplyRemoveNumber error = %v, want %v", err, ErrRemoveNumberAlreadyUsed)
	}
	if !reflect.DeepEqual(state.Board, beforeBoard) {
		t.Fatalf("state.Board mutated to %+v, want %+v", state.Board, beforeBoard)
	}
	if !reflect.DeepEqual(state.ValidMoves, beforeMoves) {
		t.Fatalf("state.ValidMoves mutated to %+v, want %+v", state.ValidMoves, beforeMoves)
	}
	if !state.RemoveNumberUsed {
		t.Fatal("state.RemoveNumberUsed = false, want true")
	}
}

func TestApplyRemoveNumberSetsBoardClearedReasonWhenAllCellsAreCleared(t *testing.T) {
	state := &GameState{
		Board: Board{{7}},
	}
	rebuildValidMoveCache(state)

	err := ApplyRemoveNumber(state, Position{Row: 0, Col: 0})
	if err != nil {
		t.Fatalf("ApplyRemoveNumber returned unexpected error: %v", err)
	}

	if !state.GameOver {
		t.Fatal("state.GameOver = false, want true")
	}
	if state.GameOverReason != GameOverBoardCleared {
		t.Fatalf("state.GameOverReason = %v, want %v", state.GameOverReason, GameOverBoardCleared)
	}
}

func TestApplyRemoveNumberSetsNoValidMovesReason(t *testing.T) {
	state := &GameState{
		Board: Board{{1, 9}},
	}
	rebuildValidMoveCache(state)

	err := ApplyRemoveNumber(state, Position{Row: 0, Col: 0})
	if err != nil {
		t.Fatalf("ApplyRemoveNumber returned unexpected error: %v", err)
	}

	if !state.GameOver {
		t.Fatal("state.GameOver = false, want true")
	}
	if state.GameOverReason != GameOverNoValidMoves {
		t.Fatalf("state.GameOverReason = %v, want %v", state.GameOverReason, GameOverNoValidMoves)
	}
}
