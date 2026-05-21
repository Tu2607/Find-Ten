package game

import (
	"fmt"
	"testing"
)

func TestNewGameRejectsUnsupportedBoardSize(t *testing.T) {
	if _, err := NewGame(8); err == nil {
		t.Fatal("NewGame(8) returned nil error, want unsupported size error")
	}
}

func TestNewGameGeneratesSupportedBoardSizes(t *testing.T) {
	for _, size := range []int{9, 10, 11} {
		t.Run(fmt.Sprintf("%dx%d", size, size), func(t *testing.T) {
			state, err := NewGame(size)
			if err != nil {
				t.Fatalf("NewGame(%d) returned unexpected error: %v", size, err)
			}

			if len(state.Board) != size {
				t.Fatalf("len(state.Board) = %d, want %d", len(state.Board), size)
			}
			for row := range state.Board {
				if len(state.Board[row]) != size {
					t.Fatalf("len(state.Board[%d]) = %d, want %d", row, len(state.Board[row]), size)
				}
			}
		})
	}
}

func TestNewGameGeneratesDigitsOneThroughNine(t *testing.T) {
	state, err := NewGame(MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGame(%d) returned unexpected error: %v", MinSupportedBoardSize, err)
	}

	for row := range state.Board {
		for col := range state.Board[row] {
			value := state.Board[row][col]
			if value < minGeneratedDigit || value > maxGeneratedDigit {
				t.Fatalf("state.Board[%d][%d] = %d, want value from 1 through 9", row, col, value)
			}
		}
	}
}

func TestNewGameStartsWithValidMoveCache(t *testing.T) {
	state, err := NewGame(MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGame(%d) returned unexpected error: %v", MinSupportedBoardSize, err)
	}

	if len(state.ValidMoves) == 0 {
		t.Fatal("len(state.ValidMoves) = 0, want at least one valid move")
	}
	if len(state.validMoveSet) != len(state.ValidMoves) {
		t.Fatalf("len(state.validMoveSet) = %d, want %d", len(state.validMoveSet), len(state.ValidMoves))
	}
	if !HasValidMove(state.Board) {
		t.Fatal("HasValidMove(state.Board) = false, want true")
	}
}
