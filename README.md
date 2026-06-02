# Find Ten Game

Find Ten Game is a Go backend-first puzzle game inspired by the AZX Service Time style minigame I came across.

The player works on a grid of digits and selects rectangle regions that sum to exactly `10`. Valid selections clear those cells to `0`, score points, and update the set of remaining valid moves.

At its heart, this project is a learning project for:
- Go game-state modeling
- validation and cache design
- actor-style game session runtime
- HTTP API and SSE snapshot streaming
- browser UI integration without duplicating backend rules

## Current State

MVP implemented so far:
- core board, position, selection, and game state types
- supported board sizes: `9x9`, `10x10`, `11x11`
- random board generation with at least one valid move
- prefix-sum rectangle lookup
- valid move cache
- move application, scoring, and game-over detection
- deadline-based timer behavior
- single-game `GameSession` runtime wrapper
- CLI demo over the same session/runtime path
- HTTP API server
- in-memory game session registry
- SSE runtime snapshot streaming
- browser-based static WebUI

Still future work:
- frontend polish and UX iteration
- optional React migration
- session cleanup policy
- multiplayer experiments
- hints, replay validation, and bot/difficulty analysis

## Run CLI

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

## Run WebUI

Run from the repository root because the server currently serves static files from `./static`:

```sh
go run ./cmd/server
```

Open:

```text
http://127.0.0.1:8080/
```

The WebUI can:
- start a `9x9`, `10x10`, or `11x11` game
- render the initial board
- submit rectangle moves by clicking two cells
- receive board updates through SSE
- show score, valid move count, countdown, and game-over state

## HTTP API

The server exposes:

- `GET /health`
- `POST /games`
- `GET /games/{id}/snapshots`
- `POST /games/{id}/moves`

`POST /games` creates a game and returns the initial snapshot plus `expiresAt`.

`GET /games/{id}/snapshots` opens an SSE stream for runtime snapshots after successful moves.

`POST /games/{id}/moves` submits a rectangle selection. Move responses are acknowledgements only; updated board state arrives through SSE.

## Test

```sh
go test ./...
```

## Project Docs

- `docs/GOAL.md`: product intent and gameplay scope
- `docs/ARCHITECTURE.md`: architecture decisions
- `docs/PLAN.md`: stepwise implementation roadmap
