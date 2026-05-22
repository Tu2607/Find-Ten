# Find Ten Game

Find Ten Game is a Go backend-first puzzle game inspired by the AZX Service Time style minigame I came across.

The player works on a grid of digits and selects rectangle regions that sum to exactly `10`. Valid selections clear those cells to `0`, score points, and update the set of remaining valid moves.

At its heart, this project is a learning project for:
- Go game-state modeling
- validation and cache design
- concurrency with game events running alongside a timer
- future frontend work for connecting players

## Current State

Implemented so far:
- core board, position, selection, and game state types
- supported board sizes: `9x9`, `10x10`, `11x11`
- random board generation with at least one valid move
- prefix-sum rectangle lookup
- valid move cache
- move application, scoring, and game-over detection
- event loop scaffolding for move and timer events
- CLI demo for generating a board and applying moves manually

Still in progress:
- richer CLI/game session wiring around the timer
- WebUI
- player connection flow
- multiplayer experiments

## Run

```sh
go run ./cmd/play -size 9
```

Enter moves as zero-based rectangle coordinates:

```text
row1 col1 row2 col2
```

Example:

```text
0 0 0 2
```

Quit with:

```text
q
```

## Test

```sh
go test ./...
```

## Project Docs

- `docs/GOAL.md`: product intent and gameplay scope
- `docs/ARCHITECTURE.md`: architecture decisions
- `docs/PLAN.md`: stepwise implementation roadmap
