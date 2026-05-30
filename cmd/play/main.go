package main

import (
	"bufio"
	"context"
	"errors"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session, err := game.NewGameSession(ctx, *size)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start game: %v\n", err)
		os.Exit(1)
	}
	defer session.Stop()

	run(ctx, os.Stdin, os.Stdout, session)
	cancel()
	session.Stop()
	<-session.Done()
}

func run(ctx context.Context, input io.Reader, output io.Writer, session *game.GameSession) {
	lines := scanLines(ctx, input)
	snapshots := session.Snapshots()

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
			if !handleInputLine(ctx, output, line, session) {
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

func handleInputLine(ctx context.Context, output io.Writer, line string, session *game.GameSession) bool {
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

	if err := session.SubmitMove(ctx, selection); err != nil {
		switch {
		case errors.Is(err, game.ErrInvalidMove), errors.Is(err, game.ErrOutOfBounds):
			fmt.Fprintf(output, "invalid move: %v\n", err)
			fmt.Fprint(output, "move row1 col1 row2 col2, or q to quit: ")
			return true
		case errors.Is(err, game.ErrGameOver):
			fmt.Fprintln(output, "game is already over")
			return false
		case errors.Is(err, game.ErrSessionClosed), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return false
		default:
			return false

		}
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

	fmt.Fprintln(output) // Newline after board
}
