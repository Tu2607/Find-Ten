# Stepwise Implementation Plan

This file is now used as both a record of completed work and a tracker for planned future work. Completed steps document what has already been built; upcoming steps document the next agreed implementation direction.

Current step: none. Step 19 is complete and the next step is not yet planned.

## Step 1: Project Scaffold - Done
- Create `go.mod` with module name `find-ten-game`.
- Add initial package layout:
  - `internal/game`
  - `cmd/play`
- Add a minimal compile-only entrypoint for the CLI demo.

Acceptance:
- `go test ./...` runs successfully with no game logic yet.

## Step 2: Core Types - Done
- Define:
  - `Board`
  - `Position`
  - `Selection`
  - `GameState`
- Add supported board size validation for `9`, `10`, and `11`.
- Add selection normalization so any drag direction maps to the same rectangle.

Acceptance:
- Tests cover supported/unsupported board sizes.
- Tests cover reversed coordinate normalization.

## Step 3: Prefix Sum + Rectangle Sum - Done
- Implement prefix sum construction for `Board`.
- Implement O(1) rectangle sum lookup.
- Treat `0` as a normal numeric cell.

Acceptance:
- Tests verify single-cell, row, column, and multi-cell rectangle sums.
- Tests include rectangles containing `0`.

## Step 4: Valid Move Cache - Done
- Implement cache rebuild from board state:
  - enumerate every possible rectangle
  - use prefix sums to find rectangles summing exactly `10`
  - populate `ValidMoves []Selection`
  - populate internal `validMoveSet map[Selection]struct{}`
- Implement `HasValidMove(board Board) bool` using the cache-building logic.

Acceptance:
- Tests verify valid rectangles are discovered.
- Tests verify sums other than exactly `10`, including `20`, are excluded.
- Tests verify `len(ValidMoves) == 0` means no valid moves.

## Step 5: Board Generation - Done
- Implement random board generation for supported sizes.
- Fill boards with digits `1-9`.
- Regenerate until at least one valid move exists.
- Return initialized `GameState` with a populated valid-move cache.

Acceptance:
- Tests verify generated board dimensions.
- Tests verify generated boards contain only `1-9`.
- Tests verify generated game states always start with at least one valid move.

## Step 6: Move Application - Done
- Implement `ApplyMove(state, selection)`:
  - reject if game is over
  - normalize selection
  - reject out-of-bounds selection
  - reject if selection is not in valid cache
  - turn selected cells into `0`
  - score `100 * newly cleared non-zero cells`
  - rebuild valid-move cache
  - set `GameOver` if cache becomes empty

Acceptance:
- Tests verify valid moves clear cells.
- Tests verify already-zero cells do not score again.
- Tests verify invalid moves do not mutate state.
- Tests verify cache rebuild and game-over detection after a move.

## Step 7: Timer/Event Loop - Superseded

Historical note: this step was implemented, but the `EventTick` model is no longer the target runtime design. Step 14 supersedes `EventTick`, per-second `RemainingTime` countdown mutation, and timer ticks flowing through the same event channel as player moves.

- Add event types:
  - `EventMove`
  - `EventTick`
- Add `RunGame(ctx, events, state)` where the loop owns state mutation.
- Track default duration as `120` seconds.
- End the game when remaining time reaches `0`.

Acceptance:
- Tests verify move events are applied serially.
- Tests verify tick events reduce time.
- Tests verify timer expiry sets `GameOver`.

## Step 8: CLI Demo - Done
- Implement `cmd/play` as the first manual surface.
- CLI behavior:
  - choose board size, defaulting to `9`
  - print board
  - print score and valid move count
  - accept input as `row1 col1 row2 col2`
  - apply moves until quit or game over
- Keep coordinates zero-based.

Acceptance:
- Manual run can generate a board, submit a move, show updated score/board, and report game over.

## Step 9: Final Verification - Done
- Run `gofmt`.
- Run `go test ./...`.
- Manually run the CLI once to verify the loop works.

Acceptance:
- All tests pass.
- CLI demo runs without panic.
- No WebUI or quadrilateral logic is included yet.

## Step 10: Channel-Driven Snapshots - Partially Superseded

Historical note: hard-copy backend-owned snapshots remain valid. Step 13 supersedes startup snapshot emission from `RunGame`, and Step 14 supersedes tick-driven snapshot publication.

- Update `RunGame` so it owns both game-state writes and game-state reads.
- Add `GameSnapshot` as a hard-copy view of current game state:
  - board
  - score
  - game-over status
  - remaining time
  - valid move count
  - snapshot sequence
  - snapshot timestamp
- Keep snapshot sequence local to `RunGame`.
  - initialize sequence at `1`
  - increment each time `RunGame` emits a snapshot
  - do not store snapshot sequence on `GameState`
- Add a snapshot output channel to `RunGame`.
- Emit snapshots from `RunGame`:
  - once when the loop starts
  - after each processed move event
  - after each processed tick event
- Ensure snapshots deep-copy mutable data.
  - `Board` must not share backing arrays with `GameState.Board`
- Keep a single input event channel for game-state mutation:
  - timer sends `EventTick`
  - CLI/user input sends `EventMove`
  - `RunGame` is the only consumer that mutates `GameState`
- Update the CLI to render snapshots instead of reading `GameState` directly after the game loop starts.

Acceptance:
- Tests verify snapshots are emitted on start, move, and tick.
- Tests verify snapshot board data is not mutated by later game-state changes.
- Tests verify snapshot sequence starts at `1` and increases monotonically.
- Tests verify CLI move input is sent through the event channel instead of calling `ApplyMove` directly.
- Tests verify `go test ./...` passes without data races when run with `-race`.

## Step 11: Game-Over Reason - Done
- Add a `GameOverReason` enum for rule-based end states:
  - no game-over reason
  - no valid moves remain
  - time expired
- Store the reason on `GameState` because it is gameplay state, not runtime metadata.
- Copy the reason into `GameSnapshot`.
- Set the reason when the game ends:
  - `ApplyMove` sets no-valid-moves when a successful move empties the valid move cache.
  - timer handling sets time-expired when remaining time reaches `0`.
- Update CLI game-over output to display the reason.

Acceptance:
- Tests verify time expiry produces the time-expired reason.
- Tests verify clearing the last valid move produces the no-valid-moves reason.
- Tests verify snapshots include the game-over reason.
- Tests verify CLI output reports the reason when the game ends.

## Step 12: Single-Game Session Wrapper - Partially Superseded

Historical note: `GameSession` lifecycle ownership and its caller-facing APIs remain valid. Step 14 supersedes the timer goroutine producing `EventTick` into the shared event channel.

- Add a `GameSession` wrapper for one running game.
- `GameSession` owns runtime lifecycle:
  - context and cancel function
  - event channel allocation
  - snapshot channel allocation
  - timer goroutine startup
  - game loop goroutine startup
  - shutdown signaling
- `RunGame` remains the only game-state mutator.
- `RunGame` remains the only snapshot producer and the only closer of the snapshot channel.
- `StartTimer` remains a tick producer only.
- `GameSession` exposes a safe caller API:
  - create a session for a board size
  - receive snapshots
  - submit a move with context-aware result waiting
  - stop the session
  - wait for session completion
- Refactor the CLI to use `GameSession` instead of manually wiring channels and goroutines.
- Do not add multiplayer, session IDs, or HTTP/API behavior in this step.

Acceptance:
- Tests verify `GameSession` starts a game and emits an initial snapshot.
- Tests verify submitting a move through `GameSession` updates score/board through `RunGame`.
- Tests verify `Stop` cancels timer/game loop lifecycle cleanly.
- Tests verify `Snapshots()` closes when `RunGame` exits.
- Tests verify CLI uses `GameSession` rather than directly creating game event/snapshot channels.
- Tests verify `go test ./...` and `go test -race ./...` pass.

## Step 13: Bootstrap Initial Snapshot - Done
- Avoid startup deadlock caused by `RunGame` publishing an initial snapshot to an unbuffered snapshot channel before a receiver is ready.
- Keep runtime snapshot delivery unbuffered to preserve backpressure and avoid queued stale snapshots.
- Treat the initial snapshot as a one-time session creation result, not as persistent `GameSession` API.
- Update `NewGameSession`:
  - creates `GameState`.
  - before starting `RunGame`, creates one hard-copy initial `GameSnapshot`.
  - this snapshot has `Sequence = 1`.
  - returns the session and initial snapshot together:
    - `session, initialSnapshot, err := NewGameSession(ctx, size)`
  - does not store the initial snapshot on `GameSession`.
- After the bootstrap snapshot is created, `RunGame` takes over game-state ownership.
- Update `RunGame`:
  - remove startup snapshot publishing.
  - publish snapshots only after successful move requests.
  - start runtime snapshot sequence at `2`.
  - continue closing the snapshot channel when it exits.
- Update CLI:
  - render the initial snapshot returned by `NewGameSession` before entering the runtime input/snapshot loop.
  - continue rendering later updates from `session.Snapshots()`.
- Keep move result channels strictly error-only:
  - move results do not include snapshots.
  - snapshots are delivered only through the session creation result or snapshot channel.
- Do not buffer the snapshot channel for this fix.
- Do not add multiplayer, session IDs, or API routing in this step.

Acceptance:
- Tests verify `NewGameSession` returns an initial snapshot with sequence `1`.
- Tests verify `RunGame` no longer emits a startup snapshot.
- Tests verify the first runtime snapshot after a valid move has sequence `2`.
- Tests verify `SubmitMove` does not deadlock when no runtime snapshot receiver has consumed from `Snapshots()` yet.
- Tests verify CLI prints the initial snapshot returned by `NewGameSession`.
- Tests verify `go test ./...` and `go test -race ./...` pass.

## Step 14: Deadline Timer and Move-Only Runtime Refactor - Done

Refactor the runtime so player moves and timer expiry are separate signals. The goal is to make snapshots lazy and move-driven, reduce WebUI rendering churn, simplify event handling, and make channel ownership/cleanup clearer.

### Design Intent

The current runtime sends both player moves and timer ticks through the same event channel. That keeps state mutation serialized, but it also means timer countdown changes can cause full board snapshots. For a future WebUI, repeated timer-driven board snapshots can interfere with transient UI state such as an in-progress selection.

The new design keeps backend authority over game rules while separating:
- player move requests
- timer expiry
- snapshot delivery
- session lifecycle

The UI should render countdown locally from an authoritative deadline. The backend should not publish full board snapshots merely because time passes.

### Timer Model

- Replace decrementing `RemainingTime` as runtime state with an authoritative `ExpiresAt time.Time`.
- Store `ExpiresAt` on `GameSession` as runtime/session metadata.
- Pass `ExpiresAt` into `RunGame` for deadline checks.
- Do not add `ExpiresAt` to `GameSnapshot`.
- Remove per-second timer tick events.
- Remove `EventTick`.
- The UI derives countdown from `ExpiresAt`.
- Backend move handling checks the deadline before applying a move.
- `RunGame` checks the deadline again before applying a move to prevent deadline races.

### Runtime Channels

Replace the mixed event channel with separate runtime signals:

- A move-only request channel.
  - Used only by `SubmitMove`.
  - Carries player move requests.
  - Each request includes:
    - `Selection`
    - `Result chan error`
  - Result channels remain error-only.
  - A nil result means the move was accepted and applied.
  - Snapshots are never returned through move result channels.
- A one-shot timer expiry channel.
  - Used only by the timer goroutine.
  - Signals when `ExpiresAt` is reached.
  - Does not carry snapshots.
  - Does not send per-second updates.
  - Is separate from the move channel.
- The snapshot channel.
  - Produced only by `RunGame`.
  - Closed only by `RunGame`.
  - Publishes snapshots only for successful moves.
  - Does not publish a timer-expiry-only board snapshot.
- The done channel.
  - Closed by `GameSession` after runtime goroutines and cleanup finish.

### RunGame Behavior

Update `RunGame` to select directly on:
- move request channel
- timer expiry channel
- context cancellation

Do not switch on timer event types inside a shared event channel.

Move handling:
- If the game is already over, return `ErrGameOver`.
- If the deadline has passed, mark the game over with `GameOverTimeExpired` and return `ErrGameOver`.
- Otherwise apply the move through existing game rules.
- Invalid moves return their current errors and do not publish snapshots.
- Successful moves publish a hard-copy snapshot.
- Runtime snapshot sequence starts at `2`, because sequence `1` belongs to the bootstrap snapshot returned by `NewGameSession`.

Timer expiry handling:
- When the expiry channel signals, `RunGame` marks:
  - `GameOver = true`
  - `GameOverReason = GameOverTimeExpired`
- `RunGame` returns after handling timer expiry.
- `RunGame` closes the snapshot channel.
- No full board snapshot is emitted solely because the timer expired.

Context cancellation:
- Manual stop remains lifecycle shutdown.
- Manual stop should not be reported as time expiry.
- New move submissions after manual stop return `ErrSessionClosed`.

### Terminal Conditions

A game session can end in exactly these gameplay ways:
- board cleared completely
- no valid moves remain
- timer expired

Add a distinct game-over reason:
- `GameOverBoardCleared`

Terminal precedence after a successful move:
- If all cells are cleared, set `GameOverBoardCleared`.
- Else if no valid moves remain, set `GameOverNoValidMoves`.
- Else continue.

Timer precedence:
- If the deadline has passed before a move is applied, timer expiry wins and the move is rejected with `ErrGameOver`.
- A move submitted before expiry but processed after expiry must still be rejected as expired.

### GameSession Behavior

`NewGameSession(ctx, size)` keeps returning:
- `session`
- `initialSnapshot`
- `error`

The initial snapshot:
- has `Sequence = 1`
- is created before `RunGame` starts
- is returned once as a session creation result
- is not stored on `GameSession`

`GameSession` owns runtime lifecycle:
- create context/cancel
- allocate move channel
- allocate expiry channel
- allocate snapshot channel
- start game loop goroutine
- start one-shot timer goroutine
- expose snapshots
- submit moves
- stop session
- expose done

`SubmitMove` behavior:
- Reject new submissions after session shutdown begins.
- Reject submissions after deadline with `ErrGameOver`.
- Send accepted move requests to the move channel.
- Wait for the error-only result channel.
- Return `ErrSessionClosed` for lifecycle shutdown before the move is accepted or processed.
- Return `ErrGameOver` for game-rule termination.

### Channel Cleanup

Close all session-owned channels cleanly when the session ends.

Ownership rules:
- `RunGame` is the only snapshot sender and closes the snapshot channel.
- Timer goroutine is the only expiry sender and closes the expiry channel when it exits.
- `GameSession` owns the move channel lifecycle.
- `GameSession` closes `done` after the loop, timer, and move-channel cleanup complete.

Move channel close safety:
- `Stop()` must prevent new submissions before the move channel is closed.
- In-flight `SubmitMove` calls must be allowed to unwind or be released by cancellation.
- The move channel must not be closed while a submitter can still send.
- Concurrent `SubmitMove` and `Stop` must not panic.

### Snapshot Policy

Snapshots are generated only for:
- initial session creation
- successful player moves

Snapshots are not generated for:
- invalid moves
- rejected moves
- countdown changes
- timer expiry alone
- manual session stop

Snapshots remain hard-copy views:
- board data must be deep-copied
- sequence is runtime stream metadata
- timer deadline metadata is not included in board snapshots

### CLI Behavior

Update CLI to:
- render the initial snapshot returned by `NewGameSession`
- render later snapshots from `session.Snapshots()`
- show countdown based on `ExpiresAt` if countdown display is needed
- continue to submit moves through `GameSession.SubmitMove`
- display game-over reason when a terminal snapshot is received after a successful move
- exit cleanly when session ends due to timer expiry and snapshots close

No WebUI, HTTP API, WebSocket API, multiplayer, session IDs, or routing should be added in this step.

### Acceptance

- Tests verify `GameSnapshot` does not include `ExpiresAt`.
- Tests verify initial snapshot has sequence `1`.
- Tests verify runtime snapshots start at sequence `2`.
- Tests verify no snapshots are emitted while only time passes before expiry.
- Tests verify successful moves still emit snapshots.
- Tests verify invalid moves do not emit snapshots.
- Tests verify timer expiry alone does not emit a board snapshot.
- Tests verify timer expiry marks game over with `GameOverTimeExpired`.
- Tests verify board cleared ends with `GameOverBoardCleared`.
- Tests verify no valid moves ends with `GameOverNoValidMoves`.
- Tests verify board-cleared reason takes precedence over no-valid-moves after a successful move.
- Tests verify `SubmitMove` after expiry returns `ErrGameOver`.
- Tests verify a move submitted before expiry but processed after expiry returns `ErrGameOver`.
- Tests verify manual session stop causes new submissions to return `ErrSessionClosed`.
- Tests verify result channels remain error-only and do not carry snapshots.
- Tests verify snapshot channel closes when `RunGame` exits.
- Tests verify expiry channel closes when the timer goroutine exits.
- Tests verify move channel closes during session cleanup.
- Tests verify `done` closes after runtime cleanup completes.
- Tests verify concurrent `SubmitMove` and `Stop` do not cause send-on-closed-channel panic.
- Tests verify CLI uses the constructor-returned initial snapshot and runtime snapshot channel.
- Run `gofmt` on changed Go files.
- Run `go test ./...`.
- Run `go test -race ./...`.

## Step 15: API Server Scaffold and Route Dispatch - Done

Add the first HTTP API surface without implementing game-session behavior yet. This step creates the server entrypoint and internal API package structure so later steps can add session creation, SSE snapshots, and move submission incrementally.

### Design Intent

The API layer should be a thin transport boundary over `internal/game`. It should not duplicate game rules, own game state, or bypass `GameSession`.

Use package name `internal/api` instead of `internal/httpapi` for a shorter project-local name.

This step only establishes:
- server executable
- API package
- route dispatch
- basic health endpoint
- unsupported route/method handling

Do not add:
- game creation
- session IDs
- session store
- SSE
- move submission
- WebUI files

### File Structure

Add:

```text
cmd/server/
  main.go

internal/api/
  server.go
  handlers.go
  errors.go

internal/api/
  server_test.go
```

### Server Entrypoint

Add `cmd/server/main.go`.

Behavior:
- parse `-addr`, defaulting to `127.0.0.1:8080`
- create an API server with `api.NewServer()`
- start `http.ListenAndServe`
- print the listening address
- keep game logic out of `cmd/server`

### API Server

Add `internal/api/server.go`.

Responsibilities:
- own an `http.ServeMux` or equivalent standard-library router
- implement `ServeHTTP`
- register initial routes

Initial routes:
- `GET /health`
  - returns `200 OK`
  - response body can be simple plain text or JSON
- placeholder route dispatch for future:
  - `POST /games`
  - `GET /games/{id}/snapshots`
  - `POST /games/{id}/moves`

For this step, future game routes should return `501 Not Implemented` if matched.

Unsupported routes should return `404 Not Found`.

Unsupported methods on known routes should return `405 Method Not Allowed`.

### Error Helpers

Add small helpers in `internal/api/errors.go` for consistent HTTP errors.

Keep this minimal:
- plain text or small JSON errors are both acceptable
- avoid introducing a larger response framework

### Acceptance

- `cmd/server` builds.
- `api.NewServer()` returns an `http.Handler`.
- `GET /health` returns `200 OK`.
- Unknown routes return `404`.
- Unsupported methods on known routes return `405`.
- Placeholder game routes return `501`.
- Tests cover route dispatch behavior.
- Run `gofmt` on changed Go files.
- Run `go test ./...`.

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

## Step 17: Per-Session Snapshot Broker and SSE Endpoint - Done

Implement `GET /games/{id}/snapshots` as a Server-Sent Events stream backed by a per-session snapshot broker.

### Design Intent

The frontend receives the initial snapshot from `POST /games`. Runtime snapshots after successful moves should arrive through SSE.

`GameSession.Snapshots()` is a single-consumer channel. The API should not let multiple SSE handlers read that channel directly because concurrent readers would split snapshots between clients. Instead, each stored game owns one snapshot broker.

The broker is the only API-layer consumer of `GameSession.Snapshots()`. SSE handlers subscribe to the broker.

This keeps:
- `RunGame` as the only producer of runtime snapshots
- `GameSession` as the runtime lifecycle owner
- the API store as the game-ID registry
- the broker as the per-game fan-out mechanism

### API Contract

Endpoint:

```text
GET /games/{id}/snapshots
```

Successful response:
- status `200 OK`
- `Content-Type: text/event-stream`
- `Cache-Control: no-cache`
- stream stays open until request cancellation, broker close, or session end
- each runtime snapshot is sent as:

```text
event: snapshot
data: {"sequence":2,...}

```

The initial snapshot is not sent over SSE. It remains only in the `POST /games` response.

New SSE subscribers should receive the latest runtime snapshot immediately if one exists. This gives reconnecting or late-attaching clients the newest board state without replaying the initial creation snapshot. If no runtime snapshot exists yet, the subscriber waits for future runtime snapshots.

### Stored Game Shape

Change the store value from only `*game.GameSession` to a small stored game value:

```go
type storedGame struct {
    session *game.GameSession
    broker  *snapshotBroker
}
```

The struct is only the API store value for a game ID. It is not a new game-state owner and does not replace `GameSession`.

Store responsibilities:
- create a `storedGame` when `POST /games` succeeds
- generate the game ID
- store `gameID -> storedGame`
- look up `storedGame` by ID
- keep no deletion policy in this step

### Snapshot Broker

Add `internal/api/broker.go`.

Broker responsibilities:
- own subscriber channels
- receive runtime snapshots from one `GameSession`
- remember the latest runtime snapshot, if one has been received
- fan out each snapshot to current subscribers
- send the latest runtime snapshot to a new subscriber immediately when available
- close subscribers when the source snapshot channel closes
- allow subscribers to unsubscribe when their request exits

The broker should not know the game ID. One broker belongs to one stored game, so all snapshots it receives already belong to that game.

Subscriber channel ownership:
- the broker is the only closer of subscriber channels
- unsubscribe removes a subscriber channel from the broker but does not close it
- publish, unsubscribe, and broker shutdown coordinate through the broker mutex to avoid send-on-closed-channel races

Suggested shape:

```go
type snapshotBroker struct {
    mu          sync.Mutex
    subscribers map[chan snapshotResponse]struct{}
    done        chan struct{}
}
```

Start the broker when storing the game:

```go
broker := newSnapshotBroker(session.Snapshots())
go broker.run()
```

or have the constructor start it.

### Subscriber Policy

Use a small buffer per subscriber:

```go
make(chan snapshotResponse, 1)
```

When publishing to a subscriber:
- try to send the latest snapshot
- if the subscriber already has one queued, drop the stale queued snapshot and replace it with the latest snapshot
- do not let one slow SSE client block the broker
- do not let one slow SSE client block `RunGame`

This matches the snapshot model because each snapshot is a full board view. The latest snapshot is enough for display.

### Broker Shutdown

When the source snapshot channel closes:
- close the broker `done` channel
- close all subscriber channels
- prevent new subscribers from attaching successfully

Subscription behavior after broker close:
- return `false` or an error so the handler can return `410 Gone` or `404 Not Found`

Recommended response:
- if game ID is unknown: `404 Not Found`
- if game is known but broker is already closed because the session ended: `410 Gone`

### SSE Handler Behavior

For `GET /games/{id}/snapshots`:
- parse `id` using `r.PathValue("id")`
- look up the stored game
- return `404 Not Found` if missing
- verify `http.Flusher` support
- return `500 Internal Server Error` if streaming is not supported
- subscribe to the stored game broker
- return `410 Gone` if the broker is closed
- set SSE headers
- flush headers immediately
- loop on:
  - `snapshot := <-subscriber`
  - `r.Context().Done()`

When a snapshot arrives:
- encode the snapshot DTO as JSON
- write:
  - `event: snapshot`
  - `data: <json>`
  - blank line
- flush

When subscriber channel closes:
- return from the handler

When request context is canceled:
- unsubscribe and return

### Routing

Replace the Step 16 placeholder for:

```text
GET /games/{id}/snapshots
```

with the real SSE handler.

Keep placeholder unchanged for:

```text
POST /games/{id}/moves
```

### Testing

Use `httptest.NewServer` or `httptest.ResponseRecorder` carefully because SSE handlers can block. Prefer request contexts that can be canceled in tests.

Tests should cover:
- known game ID returns SSE headers
- initial snapshot is not written to the stream
- late subscriber receives the latest runtime snapshot if one exists
- runtime snapshot is written as `event: snapshot`
- snapshot `data:` is valid JSON using the existing snapshot DTO shape
- unknown game ID returns `404`
- subscribing after broker close returns `410`
- multiple subscribers to one game can each receive the same runtime snapshot
- slow subscriber does not block broker fan-out
- subscriber is removed when request context is canceled
- subscriber channel closes when source snapshot channel closes
- `POST /games/{id}/snapshots` still returns `405`
- `POST /games/{id}/moves` still returns `501`

### Acceptance

- `GET /games/{id}/snapshots` for a known active game opens an SSE stream.
- SSE response uses `Content-Type: text/event-stream`.
- SSE response uses `Cache-Control: no-cache`.
- Handler flushes the stream open before any runtime snapshot exists.
- Runtime snapshots are delivered as `event: snapshot`.
- Runtime snapshot data is valid JSON.
- Initial snapshot is not replayed through SSE.
- Late subscribers receive the latest runtime snapshot if one exists.
- Multiple concurrent SSE subscribers for one game receive the same runtime snapshots.
- Slow subscribers do not block snapshot fan-out.
- Unknown game ID returns `404`.
- Ended game/broker returns `410`.
- Snapshot stream exits cleanly on request cancellation.
- Snapshot stream exits cleanly when the session snapshot stream closes.
- `POST /games/{id}/moves` remains unimplemented and returns `501`.
- No move submission implementation yet.
- Run `gofmt` on changed Go files.
- Run `go test ./...`.

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

## Step 19: Static WebUI API Integration - Done

Add the first browser-based play surface as a static WebUI served by the existing Go HTTP server.

### Design Intent

This step validates the full HTTP API flow in a real browser without introducing a frontend build tool or React yet. Treat this as the first frontend contract and expect visual/design iteration after seeing it in the browser.

The goal is a playable, compact frontend that connects to:
- `POST /games`
- `GET /games/{id}/snapshots`
- `POST /games/{id}/moves`

The WebUI should be a thin client over the API. It must not duplicate game rules beyond basic input shaping and display state.

### Non-Goals

Do not add:
- React
- npm or a JS build tool
- WebSocket transport
- multiplayer
- authentication
- persistence
- hints or AI assistance
- game-rule logic duplicated from the backend

### File Structure

Add static frontend files:

```text
static/
  index.html
  styles.css
  app.js
```

Serve those files from the existing Go server.

### Server Behavior

Extend `internal/api` static routing:
- use `http.FileServer(http.Dir("./static/"))`
- mount it at `GET /`
- register it after the API routes so more-specific API routes continue to win

Keep all existing API routes unchanged:
- `GET /health`
- `POST /games`
- `GET /games/{id}/snapshots`
- `POST /games/{id}/moves`

This is intentionally the simplest static-serving solution for testing UX quickly. It depends on running the server from the repository root, which is acceptable for the current `go run ./cmd/server` workflow. A later packaging/deployment step can switch to embedded static assets if needed.

Keep `cmd/server` as the only server entrypoint. The WebUI should run through:

```sh
go run ./cmd/server
```

and be reachable at:

```text
http://127.0.0.1:8080/
```

### Frontend Behavior

The first screen should be the actual game setup and play surface, not a marketing landing page.

Required UI:
- board-size selector for `9`, `10`, and `11`
- start-game button
- score display
- valid-move-count display
- countdown display derived from `expiresAt`
- game status/error display
- board grid

Game startup:
- user chooses a board size
- frontend calls `POST /games`
- frontend renders `initialSnapshot`
- frontend stores `gameId` and `expiresAt`
- frontend opens `EventSource` to `/games/{gameId}/snapshots`

Snapshot handling:
- update board, score, valid move count, and game-over status from SSE `snapshot` events
- do not expect the initial snapshot from SSE
- tolerate reconnect/late subscription receiving the latest runtime snapshot

Move input:
- player clicks one cell for selection start
- player clicks a second cell for selection end
- frontend previews/highlights the normalized rectangle
- frontend submits the rectangle to `POST /games/{gameId}/moves`
- on accepted move, wait for SSE to update the board
- on invalid move or out-of-bounds response, show a small error and keep the current board

Timer display:
- derive countdown locally from `expiresAt`
- do not expect timer snapshots from the backend
- stop or mark expired when countdown reaches zero

### Visual Direction

Keep the UI utilitarian and board-first:
- compact controls
- stable square cells
- clear selected rectangle highlight
- visible cleared cells as `0` or a muted empty-looking state while preserving board shape
- responsive layout that works on desktop and mobile widths

Avoid decorative landing-page composition. The player should be able to start and play immediately.

### Testing

Backend/static tests should cover:
- `GET /` returns `200 OK`
- `GET /styles.css` returns `200 OK`
- `GET /app.js` returns `200 OK`
- API routes still behave as before

Manual/browser verification should cover:
- start a `9x9` game from the browser
- board renders from the initial snapshot
- SSE connection opens
- clicking two cells submits a move
- successful move updates the board through SSE
- invalid move shows an error and does not clear cells
- countdown displays from `expiresAt`

### Acceptance

- Static WebUI is served by the Go server.
- User can start a game from the browser.
- Initial board renders.
- SSE runtime snapshots update the browser board.
- User can submit rectangle moves by clicking cells.
- Move response does not need to include a snapshot.
- Invalid moves do not mutate the displayed board.
- Countdown is derived client-side from `expiresAt`.
- No React/npm/build tooling is added in this step.
- Run `gofmt` on changed Go files.
- Run `go test ./...`.
- Manually run the server and verify the WebUI in a browser.

## Assumptions
- Implementation proceeds in the order above.
- Each step should compile and pass tests before moving to the next.
- Coding should not begin on a planned step until the user explicitly approves that step.
- `NewGameSession` may create one bootstrap snapshot before `RunGame` starts because no concurrent mutation exists yet.
- After `RunGame` starts, all runtime state reads and snapshots come from `RunGame`.
- Snapshot channels remain unbuffered.
- Move result channels remain error-only acknowledgements; snapshots are not returned through move results.
- Step 14 is the new target architecture for timer/session runtime behavior.
- Older completed steps remain as historical records, not rewritten as if the new design had always existed.
