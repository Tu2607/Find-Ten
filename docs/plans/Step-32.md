# Step 32 - SQLite Leaderboard Storage Foundation

## Problem Statement

The leaderboard and score-submission UI exists only as frontend groundwork. The project needs a persistent datastore foundation before backend score submission and real leaderboard rendering can be added.

This step adds SQLite initialization and a small leaderboard storage package. It intentionally stops before adding HTTP leaderboard endpoints or wiring the frontend submit flow to persistence.

## Scope

In scope:
- Add a SQLite-backed leaderboard storage package.
- Use the pure-Go `modernc.org/sqlite` driver so the Docker build can keep `CGO_ENABLED=0`.
- Open the database when the HTTP server starts.
- Create leaderboard tables and indexes idempotently.
- Enable WAL mode and a busy timeout for SQLite connections.
- Add a configurable database path.
- Persist the SQLite database across container rebuilds and restarts with a Docker volume.
- Add repository tests for initialization, persistence across reopen, duplicate game IDs, and leaderboard ordering.

Out of scope:
- Backend leaderboard HTTP endpoints.
- Connecting the game-over score submission popup to the backend.
- Rendering real leaderboard rows in the WebUI.
- Session final-score validation.
- Accounts, authentication, anti-cheat, pagination, admin tools, or migrations beyond the initial schema.
- Replacing the in-memory active game session store.

## Design

Add `internal/leaderboard` as the persistence boundary for submitted scores.

Each score submission is stored as one row:

```text
game_id
player_name
score
grid_size
duration_seconds
remaining_millis
submitted_at
```

The `game_id` column is unique so later API work can prevent duplicate submissions for the same completed game. Leaderboard reads are ordered by:

```text
score DESC
remaining_millis DESC
submitted_at ASC
id ASC
```

The first index supports filtered leaderboard reads by grid size and duration:

```sql
CREATE INDEX idx_leaderboard_scores_rank
ON leaderboard_scores (
    grid_size,
    duration_seconds,
    score DESC,
    remaining_millis DESC,
    submitted_at ASC,
    id ASC
);
```

The server opens the SQLite file at startup, runs idempotent schema creation, and closes the store when the process exits normally. SQLite connection options use DSN pragmas so pooled connections get the intended behavior:

```text
_pragma=busy_timeout(5000)
_pragma=journal_mode(WAL)
```

## Docker Persistence

The default database path is `./data/find-ten.db`, which resolves to `/app/data/find-ten.db` inside the current container workdir. Docker Compose mounts a named volume at `/app/data` so the database survives image rebuilds and container replacement.

## Constructor Cleanup

The leaderboard store is treated as a permanent server dependency in this step. `api.NewServer` requires a leaderboard store argument, and tests create temporary SQLite leaderboard stores instead of using a disabled/no-op leaderboard dependency.

The server constructor shape is:

```go
api.NewServer(store)
```

## Acceptance Criteria

- The server starts after opening and initializing SQLite.
- Starting with a missing DB file creates the parent directory, database file, table, and index.
- Reopening the same DB preserves existing scores.
- Duplicate `game_id` submissions return a repository duplicate error.
- Top-score reads return the requested grid size and duration sorted by score, remaining time, then submitted time.
- Docker Compose defines persistent storage for `/app/data`.
- `CGO_ENABLED=0` remains in the Docker build.
- `go test ./...` passes.

## Verification

- Run `go test ./...`.
- Optionally run `go run ./cmd/server -db-path ./data/find-ten.db` and verify the server starts.
- Optionally rebuild/recreate the Docker service and confirm the named volume keeps `/app/data/find-ten.db`.
