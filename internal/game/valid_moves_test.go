package game

import "testing"

func TestBuildValidMoveCacheFindsExactTenRectangles(t *testing.T) {
	board := Board{
		{5, 0, 5},
		{1, 2, 7},
		{9, 8, 7},
	}

	validMoves, validMoveSet := buildValidMoveCache(board)

	wantMoves := []Selection{
		{
			Start: Position{Row: 0, Col: 0},
			End:   Position{Row: 0, Col: 2},
		},
		{
			Start: Position{Row: 1, Col: 0},
			End:   Position{Row: 1, Col: 2},
		},
	}

	for _, selection := range wantMoves {
		if _, ok := validMoveSet[selection]; !ok {
			t.Fatalf("validMoveSet does not contain expected selection %+v; valid moves: %+v", selection, validMoves)
		}
	}
}

func TestBuildValidMoveCacheExcludesNonTenSums(t *testing.T) {
	board := Board{
		{9, 9},
		{9, 9},
	}

	validMoves, validMoveSet := buildValidMoveCache(board)

	if len(validMoves) != 0 {
		t.Fatalf("len(validMoves) = %d, want 0; valid moves: %+v", len(validMoves), validMoves)
	}
	if len(validMoveSet) != 0 {
		t.Fatalf("len(validMoveSet) = %d, want 0", len(validMoveSet))
	}
}

func TestBuildValidMoveCacheExcludesMultiplesOfTen(t *testing.T) {
	board := Board{
		{5, 5},
		{5, 5},
	}

	validMoves, validMoveSet := buildValidMoveCache(board)
	selectionThatSumsToTwenty := Selection{
		Start: Position{Row: 0, Col: 0},
		End:   Position{Row: 1, Col: 1},
	}

	if _, ok := validMoveSet[selectionThatSumsToTwenty]; ok {
		t.Fatalf("validMoveSet contains selection summing to 20: %+v", selectionThatSumsToTwenty)
	}
	for _, selection := range validMoves {
		if selection == selectionThatSumsToTwenty {
			t.Fatalf("validMoves contains selection summing to 20: %+v", selection)
		}
	}
}

func TestHasValidMove(t *testing.T) {
	tests := []struct {
		name  string
		board Board
		want  bool
	}{
		{
			name: "has valid move",
			board: Board{
				{4, 6},
				{8, 9},
			},
			want: true,
		},
		{
			name: "has no valid moves",
			board: Board{
				{1, 1},
				{1, 1},
			},
			want: false,
		},
		{
			name:  "empty board",
			board: Board{},
			want:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := HasValidMove(test.board)
			if got != test.want {
				t.Fatalf("HasValidMove(%+v) = %t, want %t", test.board, got, test.want)
			}
		})
	}
}

func TestRebuildValidMoveCachePopulatesGameState(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{8, 9},
		},
	}

	rebuildValidMoveCache(state)

	expectedMove := Selection{
		Start: Position{Row: 0, Col: 0},
		End:   Position{Row: 0, Col: 1},
	}
	if len(state.ValidMoves) == 0 {
		t.Fatal("len(state.ValidMoves) = 0, want at least one valid move")
	}
	if _, ok := state.validMoveSet[expectedMove]; !ok {
		t.Fatalf("state.validMoveSet does not contain expected move %+v", expectedMove)
	}
}
