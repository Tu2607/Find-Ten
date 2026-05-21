package game

import "testing"

func TestPrefixSumRectangleSum(t *testing.T) {
	board := Board{
		{1, 2, 3, 4},
		{5, 0, 6, 7},
		{8, 9, 1, 2},
	}
	prefix := newPrefixSum(board)

	tests := []struct {
		name      string
		selection Selection
		want      int
	}{
		{
			name: "single cell",
			selection: Selection{
				Start: Position{Row: 1, Col: 2},
				End:   Position{Row: 1, Col: 2},
			},
			want: 6,
		},
		{
			name: "full row",
			selection: Selection{
				Start: Position{Row: 0, Col: 0},
				End:   Position{Row: 0, Col: 3},
			},
			want: 10,
		},
		{
			name: "full column",
			selection: Selection{
				Start: Position{Row: 0, Col: 1},
				End:   Position{Row: 2, Col: 1},
			},
			want: 11,
		},
		{
			name: "multi cell rectangle",
			selection: Selection{
				Start: Position{Row: 1, Col: 1},
				End:   Position{Row: 2, Col: 3},
			},
			want: 25,
		},
		{
			name: "rectangle containing zero",
			selection: Selection{
				Start: Position{Row: 0, Col: 0},
				End:   Position{Row: 1, Col: 1},
			},
			want: 8,
		},
		{
			name: "reversed coordinates",
			selection: Selection{
				Start: Position{Row: 2, Col: 3},
				End:   Position{Row: 1, Col: 1},
			},
			want: 25,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := prefix.RectangleSum(test.selection)
			if got != test.want {
				t.Fatalf("RectangleSum(%+v) = %d, want %d", test.selection, got, test.want)
			}
		})
	}
}
