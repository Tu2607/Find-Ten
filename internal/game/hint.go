package game

func ApplyHint(state *GameState) (Selection, error) {
	if state == nil {
		return Selection{}, ErrNilGameState
	}
	if state.GameOver {
		return Selection{}, ErrGameOver
	}
	if state.validMoveSet == nil {
		return Selection{}, ErrUninitializedMove
	}
	if state.HintUsed {
		return Selection{}, ErrHintAlreadyUsed
	}
	if len(state.ValidMoves) == 0 {
		return Selection{}, ErrHintNoValidMoves
	}

	hint := state.ValidMoves[0]
	state.HintUsed = true

	return hint, nil
}
