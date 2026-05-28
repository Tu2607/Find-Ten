package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"find-ten-game/internal/game"
)

func main() {
	size := flag.Int("size", game.MinSupportedBoardSize, "board size: 9, 10, or 11")
	flag.Parse()

	state, err := game.NewGame(*size)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start game: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan game.Event, 16)
	snapshots := make(chan game.GameSnapshot, 16)

	go game.RunGame(ctx, events, snapshots, state)
	go game.StartTimer(ctx, events)

	run(ctx, os.Stdin, os.Stdout, events, snapshots)
	cancel()
}

func run(ctx context.Context, input io.Reader, output io.Writer, events chan<- game.Event, snapshots <-chan game.GameSnapshot) {
	lines := scanLines(ctx, input)

	for {
		select {
		case <-ctx.Done():
			return

		case snapshot, ok := <-snapshots:
			if !ok {
				return
			}
			printSnapshot(output, snapshot)
			if snapshot.GameOver {
				fmt.Fprintf(output, "game over: %s\n", formatGameOverReason(snapshot.GameOverReason))
				return
			}
			fmt.Fprint(output, "move row1 col1 row2 col2, or q to quit: ")

		case line, ok := <-lines:
			if !ok {
				fmt.Fprintln(output)
				return
			}
			if !handleInputLine(ctx, output, line, events) {
				return
			}
		}
	}
}

func scanLines(ctx context.Context, input io.Reader) <-chan string {
	lines := make(chan string)

	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case lines <- scanner.Text():
			}
		}
	}()

	return lines
}

func handleInputLine(ctx context.Context, output io.Writer, line string, events chan<- game.Event) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	if line == "q" || line == "quit" {
		fmt.Fprintln(output, "quit")
		return false
	}

	selection, err := parseSelection(line)
	if err != nil {
		fmt.Fprintf(output, "invalid input: %v\n", err)
		fmt.Fprint(output, "move row1 col1 row2 col2, or q to quit: ")
		return true
	}

	results := make(chan game.MoveResult, 1)
	event := game.Event{
		Type:   game.EventMove,
		Move:   selection,
		Result: results,
	}

	select {
	case events <- event:
	case <-ctx.Done():
		return false
	}

	var result game.MoveResult
	select {
	case result = <-results:
	case <-ctx.Done():
		return false
	}

	if result.Err != nil {
		fmt.Fprintf(output, "move rejected: %v\n", result.Err)
		fmt.Fprint(output, "move row1 col1 row2 col2, or q to quit: ")
	}

	return true
}

func parseSelection(line string) (game.Selection, error) {
	fields := strings.Fields(line)
	if len(fields) != 4 {
		return game.Selection{}, fmt.Errorf("expected 4 integers")
	}

	values := make([]int, len(fields))
	for index, field := range fields {
		value, err := strconv.Atoi(field)
		if err != nil {
			return game.Selection{}, fmt.Errorf("%q is not an integer", field)
		}
		values[index] = value
	}

	return game.Selection{
		Start: game.Position{Row: values[0], Col: values[1]},
		End:   game.Position{Row: values[2], Col: values[3]},
	}, nil
}

func printSnapshot(output io.Writer, snapshot game.GameSnapshot) {
	printBoard(output, snapshot.Board)
	fmt.Fprintf(output, "seq: %d | score: %d | valid moves: %d | remaining time: %d | game over: %t\n",
		snapshot.Sequence,
		snapshot.Score,
		snapshot.ValidMoveCount,
		snapshot.RemainingTime,
		snapshot.GameOver,
	)
}

func formatGameOverReason(reason game.GameOverReason) string {
	switch reason {
	case game.GameOverNoValidMoves:
		return "no valid moves"
	case game.GameOverTimeExpired:
		return "time expired"
	default:
		return "unknown reason"
	}
}

func printBoard(output io.Writer, board game.Board) {
	if len(board) == 0 {
		fmt.Fprintln(output, "(empty board)")
		return
	}

	fmt.Fprint(output, "    ")
	for col := range board[0] {
		fmt.Fprintf(output, "%2d ", col)
	}
	fmt.Fprintln(output)

	for row := range board {
		fmt.Fprintf(output, "%2d: ", row)
		for col := range board[row] {
			if board[row][col] == 0 {
				fmt.Fprint(output, " . ")
				continue
			}
			fmt.Fprintf(output, "%2d ", board[row][col])
		}
		fmt.Fprintln(output)
	}
}
