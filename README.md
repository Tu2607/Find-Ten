# Find Ten Game

A Go backend-first puzzle game inspired by the AZX Service Time style minigame.

The player works on a grid of digits and selects rectangle regions that sum to exactly `10`. Valid selections clear those cells to `0`, score points, and update the set of remaining valid moves. The game ends when the timer runs out or no valid moves remain.

## Features

- **Board sizes:** `9x9`, `10x10`, `11x11`
- **Core gameplay:** rectangle selection, prefix-sum validation, scoring (100 points per newly cleared cell)
- **Skills:** one-use reshuffle, remove a single number, and hint (reveals a valid move)
- **Timer:** 120-second countdown per game
- **Two play modes:** CLI and browser-based WebUI
- **Real-time updates:** SSE snapshot streaming to the browser
- **Session cap:** configurable max concurrent game sessions
- **Dockerized:** multi-stage Dockerfile for easy deployment

## Run

### WebUI (server)

```sh
go run ./cmd/server
```

Then open `http://127.0.0.1:8080/`. The server serves static files from `./static`, so run from the repository root.

Use `-addr` to change the listen address (default `:8080`).

### CLI

```sh
go run ./cmd/play -size 9
```

Enter moves as zero-based rectangle coordinates (`row1 col1 row2 col2`). Type `q` to quit.

### Docker

```sh
docker build -t find-ten .
docker run -p 8080:8080 find-ten
```

## HTTP API

| Method   | Path                          | Description                                      |
|----------|-------------------------------|--------------------------------------------------|
| `GET`    | `/health`                     | Health check                                     |
| `POST`   | `/games`                      | Create a game; returns initial snapshot           |
| `DELETE` | `/games/{id}`                 | Abandon and remove a game session                |
| `GET`    | `/games/{id}/snapshots`       | SSE stream of runtime snapshots                  |
| `POST`   | `/games/{id}/moves`           | Submit a rectangle move (ack only; state via SSE) |
| `POST`   | `/games/{id}/reshuffle`       | Use the one-time reshuffle skill                  |
| `POST`   | `/games/{id}/remove-number`   | Remove a single number from the board             |
| `POST`   | `/games/{id}/hint`            | Get a hint (reveals one valid move)               |

## Test

```sh
go test ./...
```

## Project Docs

- `docs/GOAL.md` — product intent and gameplay scope
- `docs/ARCHITECTURE.md` — architecture decisions
- `docs/plans/` — stepwise implementation roadmap
