package game

import (
	"errors"
	"reflect"
	"testing"
)

func TestApplyReshufflePreservesZeroCountScoreAndValidMoves(t *testing.T) {
	state := &GameState{
		Board: testReshuffleBoard(),
		Score: 300,
	}
	rebuildValidMoveCache(state)

	beforeZeroCount := countZeroCellsForTest(state.Board)

	err := ApplyReshuffle(state)
	if err != nil {
		t.Fatalf("ApplyReshuffle returned unexpected error: %v", err)
	}

	if got := countZeroCellsForTest(state.Board); got != beforeZeroCount {
		t.Fatalf("zero count = %d, want %d", got, beforeZeroCount)
	}
	if state.Score != 300 {
		t.Fatalf("state.Score = %d, want 300", state.Score)
	}
	if !state.ReshuffleUsed {
		t.Fatal("state.ReshuffleUsed = false, want true")
	}
	if len(state.ValidMoves) == 0 {
		t.Fatal("len(state.ValidMoves) = 0, want at least one valid move")
	}
	if state.validMoveSet == nil {
		t.Fatal("state.validMoveSet = nil, want rebuilt cache")
	}
	if !HasValidMove(state.Board) {
		t.Fatal("HasValidMove(state.Board) = false, want true")
	}
}

func TestApplyReshuffleRejectsSecondUseWithoutMutation(t *testing.T) {
	state := &GameState{
		Board:         testReshuffleBoard(),
		Score:         300,
		ReshuffleUsed: true,
	}
	rebuildValidMoveCache(state)

	beforeBoard := cloneBoardForTest(state.Board)
	beforeValidMoves := append([]Selection(nil), state.ValidMoves...)
	beforeMoveSet := cloneValidMoveSetForTest(state.validMoveSet)

	err := ApplyReshuffle(state)
	if !errors.Is(err, ErrReshuffleAlreadyUsed) {
		t.Fatalf("ApplyReshuffle error = %v, want %v", err, ErrReshuffleAlreadyUsed)
	}

	if !reflect.DeepEqual(state.Board, beforeBoard) {
		t.Fatalf("state.Board mutated to %+v, want %+v", state.Board, beforeBoard)
	}
	if !reflect.DeepEqual(state.ValidMoves, beforeValidMoves) {
		t.Fatalf("state.ValidMoves mutated to %+v, want %+v", state.ValidMoves, beforeValidMoves)
	}
	if !reflect.DeepEqual(state.validMoveSet, beforeMoveSet) {
		t.Fatalf("state.validMoveSet mutated to %+v, want %+v", state.validMoveSet, beforeMoveSet)
	}
	if state.Score != 300 {
		t.Fatalf("state.Score = %d, want 300", state.Score)
	}
	if !state.ReshuffleUsed {
		t.Fatal("state.ReshuffleUsed = false, want true")
	}
}

func TestApplyReshuffleRejectsInvalidState(t *testing.T) {
	tests := []struct {
		name  string
		state *GameState
		want  error
	}{
		{
			name:  "nil state",
			state: nil,
			want:  ErrNilGameState,
		},
		{
			name: "game over",
			state: &GameState{
				Board:    testReshuffleBoard(),
				GameOver: true,
			},
			want: ErrGameOver,
		},
		{
			name: "uninitialized cache",
			state: &GameState{
				Board: testReshuffleBoard(),
			},
			want: ErrUninitializedMove,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ApplyReshuffle(test.state)
			if !errors.Is(err, test.want) {
				t.Fatalf("ApplyReshuffle error = %v, want %v", err, test.want)
			}
		})
	}
}

func testReshuffleBoard() Board {
	return Board{
		{1, 9, 3, 4, 5, 6, 7, 8, 9},
		{0, 0, 4, 5, 6, 7, 8, 9, 1},
		{2, 8, 5, 6, 7, 8, 9, 1, 2},
		{3, 7, 6, 7, 8, 9, 1, 2, 3},
		{4, 6, 7, 8, 9, 1, 2, 3, 4},
		{5, 5, 8, 9, 1, 2, 3, 4, 5},
		{6, 4, 9, 1, 2, 3, 4, 5, 6},
		{7, 3, 1, 2, 3, 4, 5, 6, 7},
		{8, 2, 2, 3, 4, 5, 6, 7, 8},
	}
}

func countZeroCellsForTest(board Board) int {
	count := 0
	for row := range board {
		for col := range board[row] {
			if board[row][col] == 0 {
				count++
			}
		}
	}

	return count
}

func cloneValidMoveSetForTest(moveSet map[Selection]struct{}) map[Selection]struct{} {
	clone := make(map[Selection]struct{}, len(moveSet))
	for selection := range moveSet {
		clone[selection] = struct{}{}
	}

	return clone
}
