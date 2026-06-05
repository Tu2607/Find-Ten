package game

import "math/rand"

func ApplyReshuffle(state *GameState) error {
	if state == nil {
		return ErrNilGameState
	}
	if state.GameOver {
		return ErrGameOver
	}
	if state.validMoveSet == nil {
		return ErrUninitializedMove
	}
	if state.ReshuffleUsed {
		return ErrReshuffleAlreadyUsed
	}

	size := len(state.Board)
	fullBoardSelection := Selection{
		Start: Position{Row: 0, Col: 0},
		End:   Position{Row: size - 1, Col: size - 1},
	}
	zeroCount := size*size - countNonZeroCells(fullBoardSelection, state.Board)

	for attempt := 0; attempt < maxBoardGenerationAttempts; attempt++ {
		board := randomBoard(size)
		zeroRandomCells(board, zeroCount)

		validMoves, validMoveSet := buildValidMoveCache(board)
		if len(validMoves) == 0 {
			continue
		}

		state.Board = board
		state.ValidMoves = validMoves
		state.validMoveSet = validMoveSet
		state.ReshuffleUsed = true
		return nil
	}

	return ErrReshuffleFailed
}

func zeroRandomCells(board Board, zeroCount int) {
	size := len(board)
	indexes := rand.Perm(size * size)

	for _, index := range indexes[:zeroCount] {
		row := index / size
		col := index % size
		board[row][col] = 0
	}
}
