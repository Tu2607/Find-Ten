package game

const targetMoveSum = 10

// HasValidMove is a legacy convenience method for quick checks when callers
// only have a board and do not want to mutate GameState.
func HasValidMove(board Board) bool {
	validMoves, _ := buildValidMoveCache(board)
	return len(validMoves) > 0
}

func rebuildValidMoveCache(state *GameState) {
	validMoves, validMoveSet := buildValidMoveCache(state.Board)
	state.ValidMoves = validMoves
	state.validMoveSet = validMoveSet
}

func buildValidMoveCache(board Board) ([]Selection, map[Selection]struct{}) {
	validMoves := []Selection{}
	validMoveSet := map[Selection]struct{}{}

	if len(board) == 0 || len(board[0]) == 0 {
		return validMoves, validMoveSet
	}

	prefix := newPrefixSum(board)
	rows := len(board)
	cols := len(board[0])

	for startRow := 0; startRow < rows; startRow++ {
		for startCol := 0; startCol < cols; startCol++ {
			for endRow := startRow; endRow < rows; endRow++ {
				for endCol := startCol; endCol < cols; endCol++ {
					selection := Selection{
						Start: Position{Row: startRow, Col: startCol},
						End:   Position{Row: endRow, Col: endCol},
					}
					if prefix.RectangleSum(selection) != targetMoveSum {
						continue
					}

					validMoves = append(validMoves, selection)
					validMoveSet[selection] = struct{}{}
				}
			}
		}
	}

	return validMoves, validMoveSet
}

// selectionBoundCheck checks if the selection is within the bounds of the board.
func selectionBoundCheck(selection Selection, board Board) bool {
	return positionBoundCheck(selection.Start, board) &&
		positionBoundCheck(selection.End, board)
}

func positionBoundCheck(position Position, board Board) bool {
	return len(board) > 0 &&
		len(board[0]) > 0 &&
		position.Row >= 0 &&
		position.Row < len(board) &&
		position.Col >= 0 &&
		position.Col < len(board[0])
}
