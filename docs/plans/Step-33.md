# Step 33 - HTTP Leaderboard Endpoints

## Goal

Add backend HTTP endpoints for persisted leaderboard scores:

- `POST /scores` submits one completed game's score for the active server-generated game ID.
- `GET /scores` returns the top 15 persisted scores for a required board size and duration filter.

The API must stay a thin transport boundary: decode requests, validate transport-level fields, derive score data from backend-owned session state, call `leaderboard.Store`, map errors to HTTP statuses, and encode responses.

## Non-goals

- Do not wire `static/app.js` to the new endpoints.
- Do not render real leaderboard data in the frontend.
- Do not add submitted-score placement, global rank lookup, or `ScoreByGameID`.
- Do not add accounts, auth, anti-cheat, pagination, admin tools, or migrations.
- Do not let clients submit score, board size, duration, remaining time, submitted timestamp, or independent score IDs.
- Do not replace the in-memory active session store.

## Current State

- `internal/api.Server` already has `leaderboard *leaderboard.Store`.
- Tests create a temporary SQLite leaderboard store through `newTestServer(t)`.
- `leaderboard.Store.SubmitScore(ctx, submission)` inserts one row, validates submission fields, and maps duplicate `game_id` to `leaderboard.ErrDuplicateGameID`.
- `leaderboard.Store.TopScores(ctx, filter)` returns scores ordered by score descending, remaining time descending, submitted time ascending, and SQLite `id` ascending. The default limit is 15.
- Active games are stored in `sessionStore` as `gameID -> storedGame`, where `storedGame` currently contains the `*game.GameSession` and snapshot broker.
- `GameSession` exposes `ExpiresAt()` and action submission methods, but it does not expose configured duration, board size, or a direct final snapshot accessor.
- `GameSnapshot` contains `Board` (so `gridSize = len(board)`), `Score`, `GameOver`, `SnapshotTime`, but not `durationSeconds`.
- `durationSeconds` is only known at game creation time in `handleCreateGame`. `GameSession` converts it to `expiresAt` and does not store the raw value.
- Runtime snapshots are already immutable copies and are the safe API-facing representation of game state.

## Design Decisions

### No blocking wait or grace period for score submission

The frontend only exposes the score submission button on the game-over splash screen. That screen appears after the client receives the game-over SSE event from the broker. By the time `POST /scores` arrives at the server, the broker has already recorded the game-over snapshot in its `latest` field.

Therefore, `handleSubmitScore` checks `stored.broker.latestSnapshot()` and rejects immediately with `409 Conflict` if the snapshot is missing or not game-over. There is no subscribe-and-wait path, no grace period timeout, and no blocking on the snapshot channel.

Earlier iterations explored a 500ms grace period and a blocking wait with channel subscription to handle a potential race between the final move ACK and the game-over snapshot arriving. Both were removed because:

- The grace period was an unnecessary server-side guard against a client-side mistake (submitting before game-over) that the client's own design already prevents.
- The blocking wait would cause active-game submissions to hang until the game eventually ended, then persist a score — the opposite of the intended 409 rejection.
- The frontend design guarantees the game-over snapshot is available before submission, making both mechanisms dead code in practice.

### No `initialSnapshot` on `storedGame`

The `initialSnapshot` field was originally added to `storedGame` as a fallback for `bestSnapshot()` — a helper that returned the initial snapshot when the broker had no runtime snapshot yet. With the blocking wait and `bestSnapshot` removed, `initialSnapshot` is no longer needed on `storedGame`. It is still captured in `handleCreateGame` for the create-game HTTP response, but not stored beyond that.

### All game-over paths publish a final snapshot

`RunGame` was updated to publish a game-over snapshot on timer expiry and on the move-after-expiry path. Previously, these paths set `state.GameOver = true` but returned without publishing a snapshot, relying on `defer close(snapshots)` to signal the broker. This meant the broker never had a game-over snapshot for timer-expired games.

This was a gap in the game layer, not a Step 33 concern. The fix ensures the broker always has a `GameOver: true` snapshot regardless of how the game ended (board cleared, no valid moves, or timer expired), which is a prerequisite for the simple `latestSnapshot()` check in score submission.

The `snapshots` channel remains unbuffered to maintain back pressure. The broker goroutine is always started and draining the channel before `RunGame` can produce any snapshot. If `Stop()` is called externally, `publishPreparedSnapshot`'s `ctx.Done()` select case unblocks the send.

## Proposed Design

### Session Metadata For Score Submission

Extend `storedGame` with only `durationSeconds int`. This is the one value that cannot be derived from the snapshot or `GameSession` after creation.

Update `internal/api/store.go`:

- Add `durationSeconds int` to `storedGame`.
- Change `newStoredGame` and `sessionStore.add` to accept duration.
- Update `handleCreateGame` to pass `durationSeconds` when storing the session.

`gridSize` is derived from `len(snapshot.Board)` at score submission time. Score, remaining time, and game-over state come from the broker's latest snapshot.

### Broker Latest Snapshot Accessor

Update `internal/api/broker.go`:

- Add a mutex-protected accessor: `latestSnapshot() (snapshotResponse, bool)`.
- Returns the already-tracked `latest`/`hasLatest` fields.
- Works even after the broker has closed.

### POST /scores

Add DTOs in `internal/api/dto.go`:

```go
type submitScoreRequest struct {
    GameID     *string `json:"gameId"`
    PlayerName *string `json:"playerName"`
}

type submitScoreResponse struct {
    Accepted bool `json:"accepted"`
}
```

Add `(*Server).handleSubmitScore` in `internal/api/handlers.go`:

1. Decode JSON body.
2. Require `gameId` and `playerName` fields at the transport layer.
3. Look up the game ID in `s.store`.
4. If no stored game exists, return `404 Not Found`.
5. Check `stored.broker.latestSnapshot()`.
6. If latest snapshot has `GameOver == true`, use it.
7. If not (no snapshot or game still active), return `409 Conflict` immediately.
8. Compute server-derived submission fields:
   - `GameID`: request `gameId`.
   - `PlayerName`: request `playerName`; detailed validation remains in `leaderboard.Store`.
   - `Score`: snapshot score.
   - `GridSize`: `len(snapshot.Board)`.
   - `DurationSeconds`: `stored.durationSeconds`.
   - `RemainingMillis`: `0` for timer-expired submissions; otherwise `stored.session.ExpiresAt().Sub(snapshot.SnapshotTime)` in milliseconds, clamped to `[0, durationSeconds*1000]`.
   - `SubmittedAt`: server-stamped `time.Now()`.
9. Call `s.leaderboard.SubmitScore(r.Context(), submission)`.
10. Return `201 Created` and `{ "accepted": true }` on success.

Add a small helper in `handlers.go` for score-specific store errors:

- `leaderboard.ErrDuplicateGameID` -> `409 Conflict`
- `leaderboard.ErrInvalidScoreSubmission` -> `400 Bad Request`
- `context.Canceled` / `context.DeadlineExceeded` -> `408 Request Timeout`
- anything else -> `500 Internal Server Error`

Deleted or abandoned games return `404 Not Found` because `DELETE /games/{id}` removes them from the active session store. Completed sessions that are later pruned also return `404`.

### GET /scores

Add DTO in `internal/api/dto.go`:

```go
type scoreResponse struct {
    Rank            int       `json:"rank"`
    PlayerName      string    `json:"playerName"`
    Score           int       `json:"score"`
    GridSize        int       `json:"gridSize"`
    DurationSeconds int       `json:"durationSeconds"`
    RemainingMillis int       `json:"remainingMillis"`
    SubmittedAt     time.Time `json:"submittedAt"`
}
```

Do not expose leaderboard row `id` or `gameId` in `GET /scores`.

Add `(*Server).handleTopScores` in `internal/api/handlers.go`:

1. Read required query params: `gridSize` and `duration`.
2. Parse both as integers.
3. Validate `gridSize` with `game.ValidateBoardSize`.
4. Validate `duration` with `game.ValidateDuration`.
5. Call `s.leaderboard.TopScores(r.Context(), filter)` with limit 15.
6. Convert entries to a JSON array of `scoreResponse` with `Rank` as `index + 1`.
7. Return `200 OK` with `[]` when no rows match (not `null`).

`GET /scores` queries SQLite directly through `s.leaderboard.TopScores()`. No session store involvement.

### Routes

Update `internal/api/server.go`:

- Register `POST /scores` to `s.handleSubmitScore`.
- Register `GET /scores` to `s.handleTopScores`.
- Add method-not-allowed handling for unsupported `/scores` methods.

## Failure Cases And Status Mapping

### POST /scores

- Invalid JSON -> `400 Bad Request`
- Missing `gameId` -> `400 Bad Request`
- Missing `playerName` -> `400 Bad Request`
- Unknown, deleted, abandoned, or pruned game ID -> `404 Not Found`
- Game not over -> `409 Conflict`
- Duplicate score for the same `game_id` -> `409 Conflict`
- Invalid player name or derived submission rejected by repository validation -> `400 Bad Request`
- Request context cancellation/deadline -> `408 Request Timeout`
- Unexpected leaderboard store error -> `500 Internal Server Error`

### GET /scores

- Missing `gridSize` or `duration` -> `400 Bad Request`
- Non-integer `gridSize` or `duration` -> `400 Bad Request`
- Unsupported board size or duration -> `400 Bad Request`
- Request context cancellation/deadline -> `408 Request Timeout`
- Unexpected leaderboard store error -> `500 Internal Server Error`

## Files Modified

- `internal/api/store.go` — added `durationSeconds` to `storedGame`, updated `newStoredGame`/`add` signatures.
- `internal/api/broker.go` — added `latestSnapshot()` accessor.
- `internal/api/dto.go` — added score request and response DTOs.
- `internal/api/handlers.go` — added submit-score and top-scores handlers, score error mapping.
- `internal/api/server.go` — added `/scores` route registrations.
- `internal/api/server_test.go` — added endpoint, error-mapping, and response-shape tests.
- `internal/api/store_test.go` — updated `add` call sites for new signature.
- `internal/game/play.go` — publish game-over snapshot on timer expiry and move-after-expiry paths.
- `internal/game/play_test.go` — updated tests to expect game-over snapshot on expiry paths.

## Implementation Sequence

1. Update API session storage to retain `durationSeconds`.
2. Add broker `latestSnapshot()` accessor.
3. Add score DTOs.
4. Add `POST /scores` handler with `latestSnapshot()` check and score error mapper.
5. Add `GET /scores` handler and query parsing.
6. Register routes.
7. Update `RunGame` to publish game-over snapshot on all termination paths.
8. Add table-driven API tests.
9. Run `gofmt` on changed Go files.
10. Run `go test ./...`.

## Tests

Add or extend table-driven tests in `internal/api/server_test.go`.

### Route Dispatch

- `GET /scores?gridSize=9&duration=120` returns `200 OK`.
- `POST /scores` with an unknown game ID returns `404 Not Found`.
- Unsupported `/scores` methods return `405 Method Not Allowed`.

### POST /scores

- Happy path:
  - Create a game.
  - Drive it to game over using valid moves and broker snapshots so the API observes the final game-over snapshot.
  - Submit `{ "gameId": "...", "playerName": "Ada" }`.
  - Expect `201 Created` and `{ "accepted": true }`.
  - Query the temporary leaderboard store and verify one row exists with server-derived score, grid size, duration, and remaining millis.
- Timer-expired game:
  - Publish a timer-expired game-over snapshot directly into the broker.
  - Submit score.
  - Expect `201 Created` with `remainingMillis == 0`.
- Active game:
  - Create a game and submit immediately without waiting.
  - Expect `409 Conflict`.
- Duplicate submission:
  - Submit the same completed game twice.
  - First returns `201 Created`; second returns `409 Conflict`.
- Bad request cases:
  - invalid JSON
  - missing `gameId`
  - missing `playerName`
  - invalid player name rejected through `leaderboard.ErrInvalidScoreSubmission`
- Unknown/deleted game:
  - Unknown ID returns `404 Not Found`.
  - Deleted game ID returns `404 Not Found`.

### GET /scores

- Happy path:
  - Seed the temp leaderboard store directly with scores across multiple grid sizes and durations.
  - Request one filter.
  - Expect only matching rows, sorted by repository order, with `rank` values `1..n`.
- Empty result:
  - Valid filter with no rows returns `200 OK` and `[]`.
- Limit:
  - Seed more than 15 matching rows.
  - Expect exactly 15 response entries.
- Bad request cases:
  - missing `gridSize`
  - missing `duration`
  - non-integer `gridSize`
  - non-integer `duration`
  - unsupported `gridSize`
  - unsupported `duration`

## Acceptance Criteria

- `POST /scores` persists a completed active session's score using only `gameId` and `playerName` from the client.
- `POST /scores` checks the broker's latest snapshot for game-over state and rejects immediately if not over.
- `POST /scores` derives score, grid size, duration, remaining time, and submitted timestamp on the server.
- `POST /scores` rejects active games, duplicate submissions, invalid player names, and unknown game IDs with the planned statuses.
- `GET /scores` requires `gridSize` and `duration`, validates both, and returns the top 15 matching scores as a JSON array.
- `GET /scores` queries SQLite directly without involving the session store.
- Responses do not expose internal database row IDs or game IDs.
- All game-over paths in `RunGame` publish a final game-over snapshot.
- Frontend behavior remains unchanged.
- `go test ./...` passes after implementation.

## Open Questions

None.
