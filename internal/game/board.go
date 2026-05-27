package game

import (
	"fmt"
	"math/rand"
)

const (
	minGeneratedDigit          = 1
	maxGeneratedDigit          = 9
	maxBoardGenerationAttempts = 1000
	DefaultGameDurationSeconds = 120
)

func NewGame(size int) (*GameState, error) {
	if err := ValidateBoardSize(size); err != nil {
		return nil, err
	}

	for attempt := 0; attempt < maxBoardGenerationAttempts; attempt++ {
		state := &GameState{
			Board:         randomBoard(size),
			RemainingTime: DefaultGameDurationSeconds,
		}
		rebuildValidMoveCache(state)

		if len(state.ValidMoves) > 0 {
			return state, nil
		}
	}

	return nil, fmt.Errorf("failed to generate board with valid moves after %d attempts", maxBoardGenerationAttempts)
}

func randomBoard(size int) Board {
	board := make(Board, size)
	for row := range board {
		board[row] = make([]int, size)
		for col := range board[row] {
			board[row][col] = rand.Intn(maxGeneratedDigit-minGeneratedDigit+1) + minGeneratedDigit
		}
	}

	return board
}

// Zero out the selected cells
func zeroSelection(selection Selection, board Board) Board {
	for row := selection.Start.Row; row <= selection.End.Row; row++ {
		for col := selection.Start.Col; col <= selection.End.Col; col++ {
			board[row][col] = 0
		}
	}

	return board
}

func countNonZeroCells(selection Selection, board Board) int {
	count := 0
	for row := selection.Start.Row; row <= selection.End.Row; row++ {
		for col := selection.Start.Col; col <= selection.End.Col; col++ {
			if board[row][col] != 0 {
				count++
			}
		}
	}

	return count
}

func cloneBoard(board Board) Board {
	clone := make(Board, len(board))
	for row := range board {
		clone[row] = append([]int(nil), board[row]...)
	}

	return clone
}
