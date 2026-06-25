package game

func ApplyRemoveNumber(state *GameState, position Position) error {
	if state == nil {
		return ErrNilGameState
	}
	if state.GameOver {
		return ErrGameOver
	}
	if state.validMoveSet == nil {
		return ErrUninitializedMove
	}
	if state.RemoveNumberUsed {
		return ErrRemoveNumberAlreadyUsed
	}
	if !positionBoundCheck(position, state.Board) {
		return ErrOutOfBounds
	}
	if state.Board[position.Row][position.Col] == 0 {
		return ErrRemoveNumberInvalidTarget
	}

	state.Board[position.Row][position.Col] = 0
	state.RemoveNumberUsed = true

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
