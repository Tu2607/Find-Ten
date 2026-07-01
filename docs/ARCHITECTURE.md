# Architecture

## Overview

This document records architectural decisions for the Go backend-first game implementation. Product goals, MVP scope, gameplay rules, and future feature ideas live in `docs/GOAL.md`.

## Backend Authority

The backend owns authoritative state transitions for:
- board generation
- move validation
- scoring
- game-over detection
- timer-driven state transitions

Clients, including the CLI and future WebUI, must call backend game logic instead of duplicating rules.

## Core Design

The core game logic lives under `internal/game`.

The game state is modeled around:
- `Board`, a rectangular grid of integers
- `Selection`, a rectangular region between two positions
- `GameState`, the authoritative state for one game

Cleared cells are stored as the number `0`. This keeps the board shape stable, avoids nullable cells, and makes prefix sums straightforward.

## Move Validation

Move validation is cache-backed.

Selections are normalized before validation so all drag directions map to one canonical rectangle.

## Valid Move Cache

The repository uses a valid move cache as the authoritative validation mechanism.

When the board changes, the game rebuilds:
- `ValidMoves []Selection`
- an internal `map[Selection]struct{}`

The slice supports CLI display, hints, debugging, and future AI/bot behavior. The map supports O(1) validation of player-submitted moves.

`len(ValidMoves) == 0` means the game is over.

The cache is rebuilt:
- after board generation
- after each successful move

The cache is not rebuilt after rejected moves because the board did not change.

## Validation And Cache Trade-Offs

The board size is fixed and small. At the maximum `11x11` size, there are only `4,356` possible rectangles. That means validation could be implemented without a cache and still run fast enough.

The cache is used because it simplifies the rest of the game:
- move validation becomes a direct lookup
- game-over detection becomes `len(ValidMoves) == 0`
- hints and debug output can read from the same source
- future bots, replay verification, and difficulty analysis can reuse the cached move list

The trade-off is keeping derived state in sync with the board. The project accepts that cost because the sync rule is simple: rebuild the full cache whenever the board changes.

Incremental cache updates are intentionally avoided. They would add more bookkeeping and edge cases, while full rebuilds are deterministic, easy to test, and fast enough for the fixed board sizes.

## Prefix Sums

The backend uses a 2D prefix sum table to compute rectangle sums in O(1).

Rebuilding the valid move cache enumerates every possible rectangle and checks its sum using the prefix table.

For the maximum MVP board size, `11x11`, there are only `4,356` possible rectangles. Full cache recomputation after each successful move is intentionally preferred over incremental invalidation because it is simple, deterministic, and fast enough.

Prefix sums are not required for performance at the current board sizes. A naive rectangle sum would also be fast enough for local play, online play, and server-side replay verification. The important architectural decision is the valid move cache; prefix sums are an isolated implementation helper that can be replaced later if direct summing proves clearer.

The project keeps prefix sums for now to test the suggested optimization and to keep cache rebuilds cheap without changing the cache-oriented design.

## Game Loop

The planned runtime model is a single-owner game loop.

One goroutine owns game state and processes runtime signals serially:
- player action requests
- timer-expiry signals

This avoids race-prone shared mutation and creates a direct path to future WebSocket and multiplayer support.

## Player Actions

Player actions are user-triggered commands that may mutate authoritative game state.

The runtime uses a single player action request channel for these commands. This keeps all player-triggered state mutations serialized through `RunGame` without reintroducing timer countdown traffic into the same channel.

The current player action types are:
- rectangle move
- reshuffle skill
- remove-number skill

The request shape should include an enum-like action type, the data needed by that action, and an error-only result channel. For example:

```go
type PlayerActionType int

const (
	PlayerActionMove PlayerActionType = iota + 1
	PlayerActionReshuffle
	PlayerActionRemoveNumber
)

type PlayerActionRequest struct {
	Type      PlayerActionType
	Selection Selection
	Position  Position
	Result    chan error
}
```

`Selection` is meaningful only for rectangle move actions. `Position` is meaningful only for remove-number actions. Reshuffle actions do not require selection or position data.

`RunGame` is the only consumer of the player action channel. It switches on the action type, applies the requested mutation before the session deadline, sends the result through the action result channel, and publishes a snapshot only when the action succeeds.

Rejected player actions do not publish snapshots. Unknown action types are programming errors and should return an internal action error without mutating the board.

Timer expiry remains a separate one-shot runtime signal. It is not a player action and does not publish a board snapshot by itself.

## State Sharing And Snapshots

External callers should not read `GameState` directly while the game loop is running. The loop owns game-state mutation and should also own game-state reads that are shared outside the loop.

The project will expose hard-copy `GameSnapshot` values for display and future network responses. A snapshot is a point-in-time view of the authoritative state and must not share mutable backing data with `GameState`. In particular, `Board` must be deep-copied.

Snapshots are emitted for:
- initial game start
- successful player actions

Timer countdown changes do not produce board snapshots. The session stores an authoritative expiry deadline, and the UI can derive countdown display from that session-level metadata when a UI/API layer exposes it. Timer expiry is a terminal runtime signal, not a board-state update, so it does not emit a final board snapshot by itself.

Snapshot ordering uses a local sequence counter owned by the runtime. The initial bootstrap snapshot returned by `NewGameSession` has sequence `1`. Runtime snapshots emitted by `RunGame` start at sequence `2` and increment with each successful player action snapshot. The sequence does not belong on `GameState`; it is runtime stream metadata. A future game manager or match session can own this counter when the runtime grows beyond a single loop function.

Snapshots may also include a timestamp. The timestamp helps clients estimate display freshness or smooth a countdown, while the sequence is the authoritative ordering signal.

## Concurrency Trade-Offs

A mutex-based design could protect shared `GameState` reads and writes. That would work for a small CLI, but it spreads lock discipline across every caller. Future HTTP handlers, WebSocket clients, timers, tests, and multiplayer code would all need to remember to lock before touching state.

The chosen design is actor-style ownership:
- producers send player action requests or timer-expiry signals
- `RunGame` serially processes events
- `RunGame` owns all state reads and writes
- callers receive snapshots instead of shared mutable state

This design is slightly more structured than direct state reads with a mutex, but it better matches the planned web backend. It gives future move submission, spectator views, replay verification, and multiplayer match loops a single authoritative path for state changes and state views.

The project keeps separate runtime channels for player action requests and timer expiry. Snapshot delivery uses an output channel, but that channel does not manage state; it only carries immutable views produced by the state owner.

## Game Session Wrapper

The next runtime boundary is a single-game `GameSession`.

`GameSession` is a lifecycle wrapper, not a state owner. It allocates and wires runtime pieces:
- session context and cancel function
- player action request channel
- timer expiry channel
- snapshot channel
- timer goroutine
- game loop goroutine
- completion signal

`RunGame` remains the only code that mutates `GameState`. It also remains the only producer and closer of the snapshot channel. `GameSession` creates the snapshot channel and exposes its receive side, but it does not write snapshots.

The timer remains owned by the session lifecycle but has only one job: signal once when the session deadline expires. It does not mutate game state and does not send through the player action request channel.

The session wrapper should expose a small safe API for callers:
- receive snapshots
- submit player actions with context-aware result waiting
- stop the session
- wait for session completion

This keeps CLI, future HTTP handlers, and future WebSocket handlers from manually wiring channels and goroutines. It also creates a natural path to a later manager that can hold multiple sessions without changing the core game loop.

Multiplayer, session IDs, and API routing are intentionally out of scope for the first `GameSession` step.

## HTTP API Layer

The HTTP API lives under `internal/api` and is a thin transport boundary over `internal/game`.

The API layer is responsible for:
- request decoding
- response encoding
- route dispatch
- HTTP status mapping
- game ID lookup
- SSE fan-out

The API layer is not responsible for:
- move validation
- score calculation
- game-over detection
- timer ownership
- direct `GameState` reads or writes

Handlers must call `GameSession` APIs instead of calling lower-level game functions such as `ApplyMove`. This preserves the actor-style runtime boundary: `RunGame` remains the only code that mutates live game state after a session starts.

## API Endpoints

The API currently exposes:

- `GET /health`
- `POST /games`
- `DELETE /games/{id}`
- `GET /games/{id}/snapshots`
- `POST /games/{id}/moves`
- `POST /games/{id}/reshuffle`
- `POST /games/{id}/remove-number`

### Create Game

`POST /games` creates a new single-player game session.

The request body supplies the board size. The handler validates the size through `internal/game`, calls `game.NewGameSession`, stores the session in the API registry, and returns:
- generated game ID
- initial snapshot
- authoritative expiry deadline

The initial snapshot is the bootstrap snapshot returned by `NewGameSession` and has sequence `1`.

The create-game handler intentionally uses `context.Background()` for the game session lifecycle. It must not pass `r.Context()` into `NewGameSession`, because the request context is canceled when the create-game HTTP request finishes. A game session must continue running after the create response is returned.

If the session store is full after opportunistic cleanup, `POST /games` returns `503 Service Unavailable` and stops the newly created session before returning.

### API Session Store

The API keeps an in-memory registry of created games:

```text
gameID -> storedGame
```

Each stored game contains:
- the `*game.GameSession`
- the per-session snapshot broker

The store owns generated opaque game IDs but does not own game rules or game-state mutation. It is a registry and lookup boundary only.

The store is in-memory only. It has no external persistence and no background cleanup goroutine.

Completed sessions are cleaned opportunistically when a new game is added. Cleanup removes stored sessions whose `GameSession.Done()` channel is already closed. Active sessions are not removed by opportunistic cleanup.

New session creation is capped at `150` stored sessions after cleanup. If the store remains full, creation fails with `503 Service Unavailable`.

### Abandon Game

`DELETE /games/{id}` explicitly abandons a known session.

If the ID exists, the API removes it from the session store, then stops the session outside the store mutex, and returns `204 No Content`.

If the ID is unknown, the API returns `404 Not Found`. Removed game IDs return `404 Not Found` on later move, reshuffle, remove-number, and snapshot requests.

`POST /games` does not implicitly abandon existing sessions. The WebUI abandons its previous `state.gameId` best effort before creating a replacement game, which keeps browser tabs independent.

### Snapshot Stream

`GET /games/{id}/snapshots` opens a Server-Sent Events stream for runtime snapshots.

The initial snapshot is not replayed through SSE. Clients receive that snapshot from `POST /games`.

Runtime snapshots are emitted only after successful player actions. Timer countdown changes, invalid actions, rejected actions, and manual stops do not publish board snapshots.

`GameSession.Snapshots()` is a single-consumer channel, so SSE handlers must not read it directly. Each stored game owns one snapshot broker. The broker is the only API-layer consumer of `GameSession.Snapshots()` and fans runtime snapshots out to all current SSE subscribers.

The broker keeps the latest runtime snapshot. New SSE subscribers receive that latest runtime snapshot immediately if one exists. This supports reconnecting or late-attaching clients without replaying the initial creation snapshot.

Subscriber channels are small buffered channels. If a subscriber is slow, the broker drains a stale queued snapshot and attempts to queue the latest snapshot without blocking. This preserves the invariant that a slow client cannot block broker fan-out or `RunGame`.

The broker closes subscriber channels when the source snapshot channel closes. Unknown game IDs return `404 Not Found`; known games whose broker has closed return `410 Gone`.

### Move Submission

`POST /games/{id}/moves` submits one rectangle selection for an existing game.

The request body contains a JSON selection with start and end coordinates. The handler decodes that request into API DTOs, converts the DTO to `game.Selection`, and calls:

```go
stored.session.SubmitMove(r.Context(), selection)
```

The move endpoint intentionally uses the HTTP request context because move submission is request-scoped. If the client disconnects or the request is canceled while waiting for the game loop, the handler stops waiting. This does not cancel the game session itself.

The API does not expose or decode `game.PlayerActionRequest` as an HTTP DTO. `PlayerActionRequest` contains runtime plumbing, including the per-action result channel. `GameSession` submit methods own creation of player action requests.

Successful move responses are acknowledgements only:

```json
{ "accepted": true }
```

Move responses do not include snapshots. Updated board state is delivered through the SSE snapshot stream.

Invalid moves and out-of-bounds selections return `400 Bad Request` and do not end the game. This matches the CLI flow and core game rules. Game-over move submissions return `409 Conflict`; closed sessions return `410 Gone`.

### Remove-Number Submission

`POST /games/{id}/remove-number` submits one cell position for the once-per-session remove-number skill.

The request body contains a JSON position with row and column coordinates. The handler decodes that request into API DTOs, converts the DTO to `game.Position`, and calls:

```go
stored.session.SubmitRemoveNumber(r.Context(), position)
```

Successful remove-number responses are acknowledgements only:

```json
{ "accepted": true }
```

The response does not include a snapshot. Updated board state is delivered through the SSE snapshot stream.

The skill sets one selected non-zero cell to `0`, does not award score, rebuilds valid moves, and can be used only once per session. Rejected attempts do not consume the remaining skill use. Out-of-bounds positions and already-cleared target cells return `400 Bad Request`; already-used and game-over submissions return `409 Conflict`; closed sessions return `410 Gone`.

## Frontend

The static WebUI lives under `static/` and is served by the Go backend via `http.FileServer`.

### Font: Google Fonts (Indie Flower)

The UI loads Indie Flower from Google Fonts via a `<link>` tag. This is an intentional external dependency — the chalkboard theme relies on a handwriting-style font, and Google Fonts provides the simplest delivery with good caching. Self-hosting was considered but adds build complexity for a game that requires a network connection to play anyway (all game state lives on the server). The CSS declares local fallbacks (`Segoe Print`, `Bradley Hand`, `cursive`) for degraded rendering if the font fails to load.

## CLI

The CLI demo was the first manual testing surface and remains a thin client over `internal/game`.

The CLI is not a separate rules implementation. It is a thin client over `internal/game` and follows the same actor-style ownership model planned for a future web backend.

The CLI wiring has three channels:
- an input line channel fed by a scanner goroutine
- the player action request channel consumed by `RunGame`
- the snapshot channel produced by `RunGame`

The CLI does not call game mutation functions directly after the game loop starts. It parses supported input into a player action, submits that action through `GameSession`, waits for the per-action result channel, and renders state from snapshots.

Now that the static WebUI is functional, new gameplay controls should target the WebUI first. The CLI should continue to compile and preserve existing move and reshuffle behavior, but it does not need to grow every new skill command.

Action result waits are context-aware. If the game context is canceled before a result is sent, the CLI stops waiting instead of blocking forever. Per-action result channels are one-shot response channels: they may receive at most one result and are not closed.

Snapshot rendering is also event-driven. The CLI prints the initial snapshot, then prints later snapshots emitted after successful player actions. This keeps display reads out of shared mutable `GameState` and makes the CLI a useful concurrency demo rather than a shortcut around the backend architecture.
