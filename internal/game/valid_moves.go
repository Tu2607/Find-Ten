package game

const targetMoveSum = 10

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

// boundCheck checks if the selection is within the bounds of the board
func boundCheck(selection Selection, board Board) bool {
	return selection.Start.Row >= 0 && selection.Start.Row < len(board) &&
		selection.Start.Col >= 0 && selection.Start.Col < len(board[0]) &&
		selection.End.Row >= 0 && selection.End.Row < len(board) &&
		selection.End.Col >= 0 && selection.End.Col < len(board[0])
}
