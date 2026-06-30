package game

import (
	"errors"
	"reflect"
	"testing"
)

func TestApplyHintReturnsFirstValidMoveAndConsumesSkill(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{1, 9},
		},
		Score: 700,
	}
	rebuildValidMoveCache(state)
	beforeBoard := cloneBoardForTest(state.Board)
	beforeValidMoves := append([]Selection(nil), state.ValidMoves...)
	want := state.ValidMoves[0]

	hint, err := ApplyHint(state)
	if err != nil {
		t.Fatalf("ApplyHint returned unexpected error: %v", err)
	}

	if hint != want {
		t.Fatalf("hint = %+v, want %+v", hint, want)
	}
	if !state.HintUsed {
		t.Fatal("state.HintUsed = false, want true")
	}
	if state.Score != 700 {
		t.Fatalf("state.Score = %d, want 700", state.Score)
	}
	if !reflect.DeepEqual(state.Board, beforeBoard) {
		t.Fatalf("state.Board = %+v, want %+v", state.Board, beforeBoard)
	}
	if !reflect.DeepEqual(state.ValidMoves, beforeValidMoves) {
		t.Fatalf("state.ValidMoves = %+v, want %+v", state.ValidMoves, beforeValidMoves)
	}
}

func TestApplyHintReturnedSelectionDoesNotAliasValidMoves(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{1, 9},
		},
	}
	rebuildValidMoveCache(state)

	hint, err := ApplyHint(state)
	if err != nil {
		t.Fatalf("ApplyHint returned unexpected error: %v", err)
	}

	hint.Start.Row = 99
	if state.ValidMoves[0].Start.Row == 99 {
		t.Fatal("returned hint aliases state.ValidMoves[0]")
	}
}

func TestApplyHintRejectsSecondUseWithoutMutation(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{1, 9},
		},
	}
	rebuildValidMoveCache(state)

	if _, err := ApplyHint(state); err != nil {
		t.Fatalf("first ApplyHint returned unexpected error: %v", err)
	}
	beforeBoard := cloneBoardForTest(state.Board)
	beforeValidMoves := append([]Selection(nil), state.ValidMoves...)

	_, err := ApplyHint(state)
	if !errors.Is(err, ErrHintAlreadyUsed) {
		t.Fatalf("second ApplyHint error = %v, want %v", err, ErrHintAlreadyUsed)
	}
	if !state.HintUsed {
		t.Fatal("state.HintUsed = false, want true")
	}
	if !reflect.DeepEqual(state.Board, beforeBoard) {
		t.Fatalf("state.Board = %+v, want %+v", state.Board, beforeBoard)
	}
	if !reflect.DeepEqual(state.ValidMoves, beforeValidMoves) {
		t.Fatalf("state.ValidMoves = %+v, want %+v", state.ValidMoves, beforeValidMoves)
	}
}

func TestApplyHintRejectsInvalidStateWithoutConsumingSkill(t *testing.T) {
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
				Board:    Board{{4, 6}},
				GameOver: true,
			},
			want: ErrGameOver,
		},
		{
			name: "uninitialized valid move cache",
			state: &GameState{
				Board: Board{{4, 6}},
			},
			want: ErrUninitializedMove,
		},
		{
			name: "no valid moves",
			state: &GameState{
				Board:        Board{{9, 9}},
				ValidMoves:   []Selection{},
				validMoveSet: map[Selection]struct{}{},
			},
			want: ErrHintNoValidMoves,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ApplyHint(test.state)
			if !errors.Is(err, test.want) {
				t.Fatalf("ApplyHint error = %v, want %v", err, test.want)
			}
			if test.state != nil && test.state.HintUsed {
				t.Fatal("state.HintUsed = true, want false")
			}
		})
	}
}
