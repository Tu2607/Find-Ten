## Step 16: Create Game API Endpoint - Done

Implement `POST /games` so the API can start a new single-player game session and return the bootstrap state needed by the frontend splash/start flow.

### Design Intent

`POST /games` is the API equivalent of the CLI startup flow:
- receive a board size from the client
- create a `GameSession`
- return the constructor-provided initial snapshot
- return session-level metadata needed by the UI

This step should not implement SSE snapshots or move submission yet. It only creates sessions and stores them so later API steps can attach a snapshot stream and submit moves.

### API Contract

Endpoint:

```text
POST /games
```

Request JSON:

```json
{
  "size": 9
}
```

Response JSON on success:

```json
{
  "gameId": "generated-id",
  "initialSnapshot": {
    "sequence": 1,
    "board": [[1, 2, 3]],
    "score": 0,
    "gameOver": false,
    "gameOverReason": 0,
    "validMoveCount": 12,
    "snapshotTime": "..."
  },
  "expiresAt": "..."
}
```

Behavior:
- accept only `POST`
- decode JSON request body
- require an explicit `size` value from the request body
- validate board size using existing game validation
- call `game.NewGameSession(context.Background(), size)`
  - do not use `r.Context()` for the game session lifecycle in this step
  - the game session should continue after the create-game HTTP request returns
- store the created session in an in-memory session store
- return `201 Created`
- return the initial snapshot from `NewGameSession`
- return `expiresAt` from `session.ExpiresAt()`

### Session Store

Add `internal/api/store.go`.

The store is only an HTTP-layer registry:
- maps `gameId -> *game.GameSession`
- owns generated game IDs
- does not own game rules
- does not mutate game state
- does not replace `GameSession`

For now:
- use an in-memory map
- protect it with a mutex
- generate opaque IDs with 128 bits of crypto-random entropy encoded as lowercase hex
- do not add an external ID dependency yet
- no persistence
- no cleanup or deletion policy yet
  - created sessions remain in the store until process exit
  - completed sessions are not removed in this step
- no multiplayer/session ownership model yet

### DTOs

Add `internal/api/dto.go`.

Define request/response types:
- `createGameRequest`
- `createGameResponse`
- `snapshotResponse` or equivalent conversion helper

Keep JSON DTOs separate from internal game types so the API shape can evolve without leaking package internals.

### Error Handling

Return:
- `400 Bad Request` for invalid JSON
- `400 Bad Request` for an empty request body
- `400 Bad Request` when `size` is omitted
- `400 Bad Request` for unsupported board size
- `500 Internal Server Error` if session creation fails unexpectedly

Keep error responses consistent with Step 15 helpers.

### Routing

Replace the Step 15 placeholder for:

```text
POST /games
```

with the real handler.

Keep placeholders unchanged for:
- `GET /games/{id}/snapshots`
- `POST /games/{id}/moves`

### Acceptance

- `POST /games` with valid size returns `201 Created`.
- Response includes non-empty `gameId`.
- Response includes initial snapshot with `sequence = 1`.
- Response includes `expiresAt`.
- Created session is stored and retrievable by ID inside the API package.
- Invalid JSON returns `400`.
- Empty request body returns `400`.
- Missing `size` returns `400`.
- Unsupported board size returns `400`.
- `GET /games` still returns `405`.
- Snapshot and move endpoints still return `501`.
- No SSE implementation yet.
- No move submission implementation yet.
- Run `gofmt` on changed Go files.
- Run `go test ./...`.

