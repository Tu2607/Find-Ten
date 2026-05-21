package game

type prefixSum [][]int

func newPrefixSum(board Board) prefixSum {
	prefix := make(prefixSum, len(board)+1)
	for row := range prefix {
		prefix[row] = make([]int, len(board[0])+1)
	}

	for row := range board {
		for col := range board[row] {
			prefix[row+1][col+1] = board[row][col] +
				prefix[row][col+1] +
				prefix[row+1][col] -
				prefix[row][col]
		}
	}

	return prefix
}

func (p prefixSum) RectangleSum(selection Selection) int {
	normalized := NormalizeSelection(selection)
	startRow := normalized.Start.Row
	startCol := normalized.Start.Col
	endRow := normalized.End.Row
	endCol := normalized.End.Col

	return p[endRow+1][endCol+1] -
		p[startRow][endCol+1] -
		p[endRow+1][startCol] +
		p[startRow][startCol]
}
