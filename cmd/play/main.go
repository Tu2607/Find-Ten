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
	"time"

	"find-ten-game/internal/game"
)

const promptText = "move row1 col1 row2 col2, reshuffle, or q to quit: "

func main() {
	size := flag.Int("size", game.MinSupportedBoardSize, "board size: 9, 10, or 11")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())

	session, initialSnapshot, err := game.NewGameSession(ctx, *size, game.DefaultDurationSeconds)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start game: %v\n", err)
		os.Exit(1)
	}
	defer session.Stop()

	run(ctx, os.Stdin, os.Stdout, session, initialSnapshot)
	cancel()
	session.Stop()
	<-session.Done()
}

func run(ctx context.Context, input io.Reader, output io.Writer, session *game.GameSession, initialSnapshot game.GameSnapshot) {
	expiresAt := session.ExpiresAt()

	printSnapshot(output, initialSnapshot, expiresAt)
	if initialSnapshot.GameOver {
		fmt.Fprintf(output, "game over: %s\n", formatGameOverReason(initialSnapshot.GameOverReason))
		return
	}
	fmt.Fprint(output, promptText)

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
			printSnapshot(output, snapshot, expiresAt)
			if snapshot.GameOver {
				fmt.Fprintf(output, "game over: %s\n", formatGameOverReason(snapshot.GameOverReason))
				return
			}
			fmt.Fprint(output, promptText)

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
	if line == "reshuffle" {
		if err := session.SubmitReshuffle(ctx); err != nil {
			switch {
			case errors.Is(err, game.ErrReshuffleAlreadyUsed):
				fmt.Fprintf(output, "reshuffle unavailable: %v\n", err)
				fmt.Fprint(output, promptText)
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

	selection, err := parseSelection(line)
	if err != nil {
		fmt.Fprintf(output, "invalid input: %v\n", err)
		fmt.Fprint(output, promptText)
		return true
	}

	if err := session.SubmitMove(ctx, selection); err != nil {
		switch {
		case errors.Is(err, game.ErrInvalidMove), errors.Is(err, game.ErrOutOfBounds):
			fmt.Fprintf(output, "invalid move: %v\n", err)
			fmt.Fprint(output, promptText)
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

func printSnapshot(output io.Writer, snapshot game.GameSnapshot, expiresAt time.Time) {
	printBoard(output, snapshot.Board)
	fmt.Fprintf(output, "seq: %d | score: %d | valid moves: %d | reshuffle used: %t | time left: %s | game over: %t\n",
		snapshot.Sequence,
		snapshot.Score,
		snapshot.ValidMoveCount,
		snapshot.ReshuffleUsed,
		formatTimeLeft(time.Until(expiresAt)),
		snapshot.GameOver,
	)
}

func formatTimeLeft(remaining time.Duration) string {
	if remaining < 0 {
		remaining = 0
	}

	totalSeconds := int(remaining.Seconds())
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60

	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func formatGameOverReason(reason game.GameOverReason) string {
	switch reason {
	case game.GameOverBoardCleared:
		return "board cleared"
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
