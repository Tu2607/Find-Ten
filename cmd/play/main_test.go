package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

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
		{name: "board cleared", reason: game.GameOverBoardCleared, want: "board cleared"},
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

func TestFormatTimeLeft(t *testing.T) {
	tests := []struct {
		name      string
		remaining time.Duration
		want      string
	}{
		{name: "minutes and seconds", remaining: 65 * time.Second, want: "01:05"},
		{name: "zero", remaining: 0, want: "00:00"},
		{name: "negative", remaining: -time.Second, want: "00:00"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := formatTimeLeft(test.remaining)
			if got != test.want {
				t.Fatalf("formatTimeLeft(%v) = %q, want %q", test.remaining, got, test.want)
			}
		})
	}
}

func TestHandleInputLineSubmitsMoveThroughSession(t *testing.T) {
	session, initialSnapshot, err := game.NewGameSession(context.Background(), game.MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}
	defer session.Stop()

	move, ok := findValidSelection(initialSnapshot.Board)
	if !ok {
		t.Fatal("findValidSelection returned false, want valid move on initialized board")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if ok := handleInputLine(ctx, discardWriter{}, stringsForTest(move), session); !ok {
		t.Fatal("handleInputLine returned false, want true")
	}

	moveSnapshot := <-session.Snapshots()
	if moveSnapshot.Score == 0 {
		t.Fatal("moveSnapshot.Score = 0, want score after valid move")
	}

	session.Stop()
	<-session.Done()

	if err := session.SubmitMove(ctx, move); err == nil {
		t.Fatal("SubmitMove after stopped session succeeded, want error")
	}
}

func TestHandleInputLineSubmitsReshuffleThroughSession(t *testing.T) {
	session, initialSnapshot, err := game.NewGameSession(context.Background(), game.MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}
	defer session.Stop()

	beforeZeroCount := countZeroCells(initialSnapshot.Board)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if ok := handleInputLine(ctx, discardWriter{}, "reshuffle", session); !ok {
		t.Fatal("handleInputLine returned false, want true")
	}

	snapshot := <-session.Snapshots()
	if !snapshot.ReshuffleUsed {
		t.Fatal("snapshot.ReshuffleUsed = false, want true")
	}
	if got := countZeroCells(snapshot.Board); got != beforeZeroCount {
		t.Fatalf("zero count = %d, want %d", got, beforeZeroCount)
	}
}

func TestHandleInputLineAlreadyUsedReshuffleKeepsRunning(t *testing.T) {
	session, _, err := game.NewGameSession(context.Background(), game.MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}
	defer session.Stop()

	if err := session.SubmitReshuffle(context.Background()); err != nil {
		t.Fatalf("SubmitReshuffle returned unexpected error: %v", err)
	}
	<-session.Snapshots()

	var output strings.Builder
	if ok := handleInputLine(context.Background(), &output, "reshuffle", session); !ok {
		t.Fatal("handleInputLine returned false, want true")
	}
	if !strings.Contains(output.String(), "reshuffle unavailable") {
		t.Fatalf("output = %q, want reshuffle unavailable message", output.String())
	}
}

func TestRunPrintsInitialSnapshot(t *testing.T) {
	session, initialSnapshot, err := game.NewGameSession(context.Background(), game.MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}
	defer session.Stop()

	var output strings.Builder
	run(context.Background(), strings.NewReader("q\n"), &output, session, initialSnapshot)

	got := output.String()
	if !strings.Contains(got, "seq: 1") {
		t.Fatalf("run output does not include initial snapshot sequence: %q", got)
	}
	if !strings.Contains(got, "time left:") {
		t.Fatalf("run output does not include time left: %q", got)
	}
	if !strings.Contains(got, "reshuffle used: false") {
		t.Fatalf("run output does not include reshuffle state: %q", got)
	}
	if !strings.Contains(got, "reshuffle") {
		t.Fatalf("run output does not include reshuffle prompt: %q", got)
	}
}

func findValidSelection(board game.Board) (game.Selection, bool) {
	for startRow := range board {
		for startCol := range board[startRow] {
			for endRow := startRow; endRow < len(board); endRow++ {
				for endCol := startCol; endCol < len(board[endRow]); endCol++ {
					selection := game.Selection{
						Start: game.Position{Row: startRow, Col: startCol},
						End:   game.Position{Row: endRow, Col: endCol},
					}
					if rectangleSum(board, selection) == 10 {
						return selection, true
					}
				}
			}
		}
	}

	return game.Selection{}, false
}

func rectangleSum(board game.Board, selection game.Selection) int {
	sum := 0
	for row := selection.Start.Row; row <= selection.End.Row; row++ {
		for col := selection.Start.Col; col <= selection.End.Col; col++ {
			sum += board[row][col]
		}
	}

	return sum
}

func stringsForTest(selection game.Selection) string {
	return fmt.Sprintf("%d %d %d %d", selection.Start.Row, selection.Start.Col, selection.End.Row, selection.End.Col)
}

func countZeroCells(board game.Board) int {
	count := 0
	for row := range board {
		for col := range board[row] {
			if board[row][col] == 0 {
				count++
			}
		}
	}

	return count
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
