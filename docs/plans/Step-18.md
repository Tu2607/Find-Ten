## Step 18: Move Submission API Endpoint - Done

Implement `POST /games/{id}/moves` so the API can submit a rectangle move for an existing game session.

### Design Intent

The move endpoint is the HTTP equivalent of the CLI move flow:
- receive a rectangle selection from the request body
- parse it into the backend `game.Selection` type
- submit it through `GameSession.SubmitMove`
- return only an acknowledgement or error

The handler must not call `game.ApplyMove` directly and must not read or mutate `GameState`. Backend game rules remain authoritative inside `GameSession` and `RunGame`.

Successful move snapshots are delivered through the existing SSE endpoint. Move responses do not include snapshots.

### API Contract

Endpoint:

```text
POST /games/{id}/moves
```

Request JSON:

```json
{
  "selection": {
    "start": { "row": 0, "col": 0 },
    "end": { "row": 0, "col": 2 }
  }
}
```

Response JSON on accepted move:

```json
{
  "accepted": true
}
```

### Runtime Boundary

Use the stored `GameSession`:

```go
err := stored.session.SubmitMove(r.Context(), selection)
```

This intentionally uses the HTTP request context because submitting a move is request-scoped. If the client disconnects or the request is canceled while waiting for the game loop, the handler should stop waiting.

Do not use `r.Context()` for game creation. `POST /games` continues to use `context.Background()` for the long-lived session lifecycle.

Do not expose or decode `game.MoveRequest` as the HTTP DTO. `game.MoveRequest` contains the runtime result channel and is internal plumbing. The API should decode JSON coordinates into a DTO, convert that DTO to `game.Selection`, and reuse `GameSession.SubmitMove`, which creates the runtime move request internally.

### DTOs

Extend `internal/api/dto.go` with:
- `submitMoveRequest`
- `selectionRequest`
- `positionRequest`
- `submitMoveResponse`

Use pointer fields for coordinates so missing fields can be distinguished from valid zero coordinates.

### Handler Behavior

For `POST /games/{id}/moves`:
- parse `id` with `r.PathValue("id")`
- look up the stored game
- return `404 Not Found` if missing
- decode JSON body
- return `400 Bad Request` for invalid JSON
- require `selection`, `start`, `end`, and all `row`/`col` fields
- convert the request DTO to `game.Selection`
- call `stored.session.SubmitMove(r.Context(), selection)`
- return `200 OK` with `{ "accepted": true }` when accepted

### Error Mapping

Map known errors:
- `game.ErrInvalidMove` -> `400 Bad Request`
- `game.ErrOutOfBounds` -> `400 Bad Request`
- `game.ErrGameOver` -> `409 Conflict`
- `game.ErrSessionClosed` -> `410 Gone`
- `context.Canceled` -> `408 Request Timeout`
- `context.DeadlineExceeded` -> `408 Request Timeout`
- `game.ErrUninitializedMove` -> `500 Internal Server Error`
- `game.ErrNilGameState` -> `500 Internal Server Error`
- unknown errors -> `500 Internal Server Error`

Invalid moves and out-of-bounds moves do not end the game and do not emit snapshots, matching the CLI flow and core game rules.

### Routing

Replace the Step 17 placeholder for:

```text
POST /games/{id}/moves
```

with the real handler.

Keep the existing SSE handler for:

```text
GET /games/{id}/snapshots
```

### Testing

Tests should cover:
- valid move returns `200 OK`
- response includes `{ "accepted": true }`
- move response does not include a snapshot
- valid move causes the SSE stream to receive a runtime snapshot
- unknown game ID returns `404`
- invalid JSON returns `400`
- missing `selection`, `start`, `end`, or coordinate fields returns `400`
- out-of-bounds selection returns `400`
- invalid move returns `400`
- stopped session returns `410`
- game-over session returns `409`
- `GET /games/{id}/moves` still returns `405`
- `POST /games/{id}/snapshots` still returns `405`

### Acceptance

- `POST /games/{id}/moves` submits moves through `GameSession.SubmitMove`.
- Valid moves return `200 OK`.
- Valid moves publish runtime snapshots through SSE.
- Move responses do not include snapshots.
- Invalid moves and out-of-bounds selections return `400` without ending the game.
- Unknown game ID returns `404`.
- Game-over session returns `409`.
- Stopped session returns `410`.
- No direct `GameState` reads or `ApplyMove` calls are added to the API handler.
- Run `gofmt` on changed Go files.
- Run `go test ./...`.

