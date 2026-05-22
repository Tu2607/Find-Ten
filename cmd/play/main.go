package main

import (
	"bufio"
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

	run(os.Stdin, os.Stdout, state)
}

func run(input io.Reader, output io.Writer, state *game.GameState) {
	scanner := bufio.NewScanner(input)
	printState(output, state)

	for !state.GameOver {
		fmt.Fprint(output, "move row1 col1 row2 col2, or q to quit: ")
		if !scanner.Scan() {
			fmt.Fprintln(output)
			return
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "q" || line == "quit" {
			fmt.Fprintln(output, "quit")
			return
		}

		selection, err := parseSelection(line)
		if err != nil {
			fmt.Fprintf(output, "invalid input: %v\n", err)
			continue
		}

		if err := game.ApplyMove(state, selection); err != nil {
			fmt.Fprintf(output, "move rejected: %v\n", err)
			continue
		}

		printState(output, state)
	}

	fmt.Fprintln(output, "game over")
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

func printState(output io.Writer, state *game.GameState) {
	printBoard(output, state.Board)
	fmt.Fprintf(output, "score: %d | valid moves: %d | remaining time: %d | game over: %t\n",
		state.Score,
		len(state.ValidMoves),
		state.RemainingTime,
		state.GameOver,
	)
}

func printBoard(output io.Writer, board game.Board) {
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
