package game

import "testing"

func TestIsSupportedBoardSize(t *testing.T) {
	tests := []struct {
		name string
		size int
		want bool
	}{
		{name: "below supported range", size: 8, want: false},
		{name: "minimum supported size", size: 9, want: true},
		{name: "middle supported size", size: 10, want: true},
		{name: "maximum supported size", size: 11, want: true},
		{name: "above supported range", size: 12, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := IsSupportedBoardSize(test.size)
			if got != test.want {
				t.Fatalf("IsSupportedBoardSize(%d) = %t, want %t", test.size, got, test.want)
			}
		})
	}
}

func TestValidateBoardSize(t *testing.T) {
	for _, size := range []int{9, 10, 11} {
		if err := ValidateBoardSize(size); err != nil {
			t.Fatalf("ValidateBoardSize(%d) returned unexpected error: %v", size, err)
		}
	}

	for _, size := range []int{0, 8, 12} {
		if err := ValidateBoardSize(size); err == nil {
			t.Fatalf("ValidateBoardSize(%d) returned nil error, want unsupported size error", size)
		}
	}
}

func TestNormalizeSelection(t *testing.T) {
	tests := []struct {
		name      string
		selection Selection
		want      Selection
	}{
		{
			name: "already normalized",
			selection: Selection{
				Start: Position{Row: 1, Col: 2},
				End:   Position{Row: 3, Col: 4},
			},
			want: Selection{
				Start: Position{Row: 1, Col: 2},
				End:   Position{Row: 3, Col: 4},
			},
		},
		{
			name: "reverse rows and columns",
			selection: Selection{
				Start: Position{Row: 5, Col: 6},
				End:   Position{Row: 2, Col: 1},
			},
			want: Selection{
				Start: Position{Row: 2, Col: 1},
				End:   Position{Row: 5, Col: 6},
			},
		},
		{
			name: "reverse columns only",
			selection: Selection{
				Start: Position{Row: 2, Col: 7},
				End:   Position{Row: 4, Col: 3},
			},
			want: Selection{
				Start: Position{Row: 2, Col: 3},
				End:   Position{Row: 4, Col: 7},
			},
		},
		{
			name: "single cell",
			selection: Selection{
				Start: Position{Row: 3, Col: 3},
				End:   Position{Row: 3, Col: 3},
			},
			want: Selection{
				Start: Position{Row: 3, Col: 3},
				End:   Position{Row: 3, Col: 3},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := NormalizeSelection(test.selection)
			if got != test.want {
				t.Fatalf("NormalizeSelection(%+v) = %+v, want %+v", test.selection, got, test.want)
			}
		})
	}
}
