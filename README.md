# Find Ten Game

A Go backend-first puzzle game inspired by the AZX Service Time style minigame.

The player works on a grid of digits and selects rectangle regions that sum to exactly `10`. Valid selections clear those cells to `0`, score points, and update the set of remaining valid moves. The game ends when the timer runs out or no valid moves remain.

## Features

- **Board sizes:** `9x9`, `10x10`, `11x11`
- **Core gameplay:** rectangle selection, prefix-sum validation, scoring (100 points per newly cleared cell)
- **Skills:** one-use reshuffle, remove a single number, and hint (reveals a valid move)
- **Timer:** `60`, `120`, or `180` seconds per game (`120` by default)
- **Two play modes:** CLI and browser-based WebUI
- **Real-time updates:** SSE snapshot streaming to the browser
- **Leaderboard:** SQLite-backed global scores by board size and duration
- **Accounts:** optional player accounts with 7-day browser sessions and account-linked scores
- **HTTP hardening:** `8 KiB` JSON body limits, Content Security Policy, and security headers
- **Session capacity:** bounded in-memory game-session storage
- **Dockerized:** multi-stage Dockerfile for easy deployment

## Run

### WebUI (server)

```sh
go run ./cmd/server
```

Then open `http://127.0.0.1:8080/`. The server serves static files from `./static`, so run from the repository root.

Use `-addr` to change the listen address (default `:8080`) and `-db-path` to change the SQLite
database path (default `./data/find-ten.db`).

### CLI

```sh
go run ./cmd/play -size 9
```

Enter moves as zero-based rectangle coordinates (`row1 col1 row2 col2`). Type `q` to quit.

### Docker

```sh
docker build -t find-ten .
mkdir -p data
docker run -p 8080:8080 -v "$(pwd)/data:/app/data" find-ten
```

The mounted `data` directory preserves the SQLite leaderboard, accounts, and login sessions when
the container is replaced.

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
| `POST`   | `/scores`                     | Submit the final score for a completed game       |
| `GET`    | `/scores?gridSize=9&duration=120` | List the top scores for a board and duration   |
| `POST`   | `/players`                    | Create an optional player account                 |
| `POST`   | `/auth/login`                 | Log in and create a browser session               |
| `POST`   | `/auth/logout`                | End the current browser session                   |
| `GET`    | `/auth/me`                    | Get the currently authenticated player            |

## Test

```sh
go test ./...
```

## Project Docs

- `docs/GOAL.md` — product intent and gameplay scope
- `docs/ARCHITECTURE.md` — architecture decisions
- `docs/plans/` — stepwise implementation roadmap
