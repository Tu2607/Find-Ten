package game

import "errors"

const pointsPerClearedCell = 100

var (
	ErrGameOver          = errors.New("game is over")
	ErrInvalidMove       = errors.New("invalid move")
	ErrOutOfBounds       = errors.New("selection is out of bounds")
	ErrNilGameState      = errors.New("game state is nil")
	ErrUninitializedMove = errors.New("valid move cache is uninitialized")
)

func ApplyMove(state *GameState, selection Selection) error {
	if state == nil {
		return ErrNilGameState
	}
	if state.GameOver {
		return ErrGameOver
	}
	if state.validMoveSet == nil {
		return ErrUninitializedMove
	}

	selection = NormalizeSelection(selection)
	if !boundCheck(selection, state.Board) {
		return ErrOutOfBounds
	}
	if _, ok := state.validMoveSet[selection]; !ok {
		return ErrInvalidMove
	}

	clearedCells := countNonZeroCells(selection, state.Board)
	state.Board = zeroSelection(selection, state.Board)
	state.Score += clearedCells * pointsPerClearedCell

	rebuildValidMoveCache(state)
	if allCellsCleared(state.Board) {
		state.GameOver = true
		state.GameOverReason = GameOverBoardCleared
	} else if len(state.ValidMoves) == 0 {
		state.GameOver = true
		state.GameOverReason = GameOverNoValidMoves
	}

	return nil
}
