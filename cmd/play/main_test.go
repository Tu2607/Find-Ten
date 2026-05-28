package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"find-ten-game/internal/game"
)

func TestParseSelection(t *testing.T) {
	selection, err := parseSelection("1 2 3 4")
	if err != nil {
		t.Fatalf("parseSelection returned unexpected error: %v", err)
	}

	want := game.Selection{
		Start: game.Position{Row: 1, Col: 2},
		End:   game.Position{Row: 3, Col: 4},
	}
	if selection != want {
		t.Fatalf("parseSelection returned %+v, want %+v", selection, want)
	}
}

func TestParseSelectionRejectsWrongFieldCount(t *testing.T) {
	_, err := parseSelection("1 2 3")
	if err == nil {
		t.Fatal("parseSelection returned nil error, want field count error")
	}
}

func TestParseSelectionRejectsNonInteger(t *testing.T) {
	_, err := parseSelection("1 x 3 4")
	if err == nil {
		t.Fatal("parseSelection returned nil error, want integer parse error")
	}
}

func TestFormatGameOverReason(t *testing.T) {
	tests := []struct {
		name   string
		reason game.GameOverReason
		want   string
	}{
		{name: "no valid moves", reason: game.GameOverNoValidMoves, want: "no valid moves"},
		{name: "time expired", reason: game.GameOverTimeExpired, want: "time expired"},
		{name: "unknown", reason: game.GameOverNone, want: "unknown reason"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := formatGameOverReason(test.reason)
			if got != test.want {
				t.Fatalf("formatGameOverReason(%v) = %q, want %q", test.reason, got, test.want)
			}
		})
	}
}

func TestRunAppliesMove(t *testing.T) {
	// NewGame normally initializes this cache. This test uses the public move
	// path through run, so build a real initialized state instead.
	state, err := game.NewGame(game.MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGame returned unexpected error: %v", err)
	}
	move := state.ValidMoves[0]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan game.Event, 4)
	snapshots := make(chan game.GameSnapshot, 4)
	done := make(chan struct{})
	go func() {
		defer close(done)
		game.RunGame(ctx, events, snapshots, state)
	}()

	input := stringsForTest(move) + "\nq\n"
	run(ctx, stringsReader(input), discardWriter{}, events, snapshots)
	close(events)
	<-done

	if state.Score == 0 {
		t.Fatal("state.Score = 0, want score after valid move")
	}
}

func stringsForTest(selection game.Selection) string {
	return fmt.Sprintf("%d %d %d %d", selection.Start.Row, selection.Start.Col, selection.End.Row, selection.End.Col)
}

func stringsReader(value string) *strings.Reader {
	return strings.NewReader(value)
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
