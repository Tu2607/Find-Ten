## Step 23: Once-Per-Session Hint Skill

### Context

The player needs an assist when no sum-to-10 rectangle is obvious. Steps 20 and 22 established a once-per-session skill pattern (reshuffle, remove-number) routed through the player action channel. Step 23 adds a third skill of the same shape: a hint that reveals one valid move on demand.

The hint is informational only. It returns one valid `Selection`, marks `HintUsed=true`, and publishes a normal snapshot. The player remains free to play any valid sum-to-10 rectangle. The hint payload is delivered only by the dedicated endpoint response; it does not live on snapshots.

### Design Intent

Rules:
- The skill reveals exactly one valid move (one `Selection` from the cache).
- The skill can be used at most once per game session.
- Activating the skill consumes it. `HintUsed=true` is set the moment the server returns a `Selection`, regardless of whether the player goes on to play that rectangle, plays a different valid rectangle, or makes no move at all.
- Because activation changes game state (`HintUsed` flips to `true`), a successful activation always publishes a normal runtime snapshot reflecting the new `HintUsed` value.
- The returned hint is advisory only. It does not lock, queue, or pre-select any move. The player retains full freedom to play any valid sum-to-10 rectangle on their next move.
- The skill does not award or deduct score.
- The skill does not mutate the board, score, or `ValidMoves`.

Rejected activation — any case where the server receives the request but does not return a `Selection`. No state changes, the skill stays available, no snapshot is published. Cases:
- game already over (player-visible — button is normally disabled).
- deadline expired between request submission and processing.
- skill already used in this session.
- no valid moves remain (largely defensive — typically overlaps with game-over).
- valid move cache uninitialized (defensive internal-state guard).
- session closed/stopped before the action is processed.
- request context cancelled (e.g. client disconnect).

The backend remains authoritative. The WebUI displays the hint visually but does not enforce the cap.

### Core Game Rules

Add a new file `internal/game/hint.go` (mirrors `remove_number.go` / `reshuffle.go`):

```go
func ApplyHint(state *GameState) (Selection, error)
```

Behavior:
- Return `ErrNilGameState` for a nil state.
- Return `ErrGameOver` if the game is already over.
- Return `ErrUninitializedMove` if the valid move cache is uninitialized.
- Return a hint-specific already-used error on second use.
- Return a hint-specific "no valid moves" error if `len(state.ValidMoves) == 0`.
- Copy `state.ValidMoves[0]` into a local variable before returning. Do not alias the slice.
- Mark `state.HintUsed = true` only after success.
- Do not mutate `Board`, `Score`, or `ValidMoves`.
- Leave the hint skill available after any rejected attempt.

Add errors next to the reshuffle/remove-number errors:

```go
ErrHintAlreadyUsed
ErrHintNoValidMoves
```

### Player Action Runtime

Extend the player action enum in `internal/game/types.go`:

```go
const (
    PlayerActionMove PlayerActionType = iota + 1
    PlayerActionReshuffle
    PlayerActionRemoveNumber
    PlayerActionHint
)
```

Hint needs to return a `Selection` to the caller. To keep the existing action lifecycle centralized in `submitAction`, add a pointer output field on `PlayerActionRequest`:

```go
type PlayerActionRequest struct {
    Type      PlayerActionType
    Selection Selection
    Position  Position
    Result    chan error
    Hint      *Selection // only set for PlayerActionHint
}
```

`applyPlayerActionBeforeDeadline` gains a `PlayerActionHint` case that calls `ApplyHint(state)`. On success it writes the returned `Selection` into `request.Hint` when the pointer is non-nil, then returns nil. `RunGame` sends the action result after the hint pointer has been written, so the existing result channel synchronization makes the value visible to the submitter before `SubmitHint` returns.

Add to `internal/game/play.go`:

```go
func (s *GameSession) SubmitHint(ctx context.Context) (Selection, error)
```

Mirrors `SubmitReshuffle` by reusing `submitAction`: create a local `Selection`, pass its address in `PlayerActionRequest.Hint`, and return the written value after `submitAction` succeeds.

This preserves the API/snapshot boundary without adding a second response channel:
- `POST /games/{id}/hint` returns the hint selection.
- snapshots carry only `HintUsed`.
- the hint selection is not broadcast through SSE.

### State And Snapshots

Track skill usage in `GameState`:

```go
HintUsed bool
```

Add `HintUsed bool` to `GameSnapshot` and propagate it in `newGameSnapshot`. The hint `Selection` is not stored on snapshots; it is delivered only by the endpoint response.

The initial snapshot reports `hintUsed: false`. Snapshots after a successful hint report `hintUsed: true`.

### API Behavior

Routes:

```text
POST /games/{id}/hint
GET  /games/{id}/hint  -> handleMethodNotAllowed
```

Request body: none (empty body acceptable).

Success response (`200 OK`):

```json
{
  "selection": {
    "start": {"row": 2, "col": 3},
    "end":   {"row": 4, "col": 5}
  }
}
```

Handler (`handleHint` in `internal/api/handlers.go`):
- Look up the stored game; return `404` for unknown IDs.
- Call `stored.session.SubmitHint(r.Context())`.
- On success, encode `hintResponse{Selection: ...}` with `200 OK`.
- On error, route through the same mapping used by other skills:
  - `ErrHintAlreadyUsed`, `ErrGameOver` → `409 Conflict`
  - `ErrHintNoValidMoves` → `409 Conflict`
  - `ErrSessionClosed` → `410 Gone`
  - context cancel/deadline → `408 Request Timeout`
  - `ErrUninitializedMove`, `ErrNilGameState`, `ErrUnknownPlayerAction` → `500`

Extend `writeMoveError` to recognize the two new hint errors.

In `internal/api/dto.go`, add `HintUsed bool` to `snapshotResponse` (json: `hintUsed`) and propagate it in `newSnapshotResponse`. Define `hintResponse` with a `selection` field that matches the existing selection DTO shape.

### Static WebUI Behavior

Add a third skill button, `Hint`, alongside Reshuffle and Remove in `static/index.html`.

UI behavior in `static/app.js`:
- Add `hintUsed`, `hintPending`, `hintHighlight` to the `state` object. `hintHighlight` stores the `Selection` returned by the server, or `null`.
- Clicking `Hint` posts `POST /games/{state.gameId}/hint`. On success, store the returned `selection` in `state.hintHighlight`, set `state.hintUsed = true`, and re-render.
- `renderBoard` adds a `hint` CSS class to every cell that is inside the highlighted rectangle and has a non-zero value. Cleared cells inside the rectangle are not highlighted.
- The highlight persists until the next snapshot arrives. `applySnapshot` clears `state.hintHighlight`.
- Disable the Hint button when: `!gameId || gameOver || hintUsed || hintPending`.
- Button text mirrors the other skills: `Hint` / `Hinting` / `Used`.
- Hint requests do not interfere with normal rectangle selection or remove mode. Clicking the board never consumes the hint.

Add a `.cell.hint` style in `static/styles.css` (color/outline distinct from `.selected` and `.corner`).

Do not migrate to React or add frontend build tooling.

### CLI Position

Do not extend the playable CLI for this step. The CLI continues to compile and pass its existing tests; the session method `SubmitHint` is the stable test surface.

### Critical Files

- `internal/game/hint.go` (new) — `ApplyHint`, `ErrHintAlreadyUsed`, `ErrHintNoValidMoves`.
- `internal/game/types.go` — add `PlayerActionHint`, `HintUsed` on `GameState`/`GameSnapshot`, `Hint` pointer on `PlayerActionRequest`.
- `internal/game/play.go` — add `SubmitHint`, route `PlayerActionHint` in `applyPlayerActionBeforeDeadline`, propagate `HintUsed` in `newGameSnapshot`.
- `internal/api/server.go` — register `POST /games/{id}/hint`; map `GET` to `handleMethodNotAllowed`.
- `internal/api/handlers.go` — implement `handleHint`; extend `writeMoveError` with hint errors.
- `internal/api/dto.go` — `HintUsed` on `snapshotResponse`; `hintResponse` DTO.
- `static/index.html` — add Hint button.
- `static/app.js` — hint state, click handler, highlight rendering, snapshot reset.
- `static/styles.css` — `.cell.hint` style.

### Testing

Core game tests (`internal/game/hint_test.go`):
- successful hint returns a `Selection` equal to (a copy of) `ValidMoves[0]`.
- mutating the returned `Selection` does not affect `state.ValidMoves`.
- score, board, and `ValidMoves` are unchanged after success.
- `HintUsed` becomes true only after success.
- second hint returns `ErrHintAlreadyUsed`.
- rejected hint leaves `HintUsed` false.
- hint with empty `ValidMoves` returns `ErrHintNoValidMoves` and does not consume the skill.
- hint after game over returns `ErrGameOver`.
- nil state returns `ErrNilGameState`.

Runtime/session tests:
- `SubmitHint` returns the same `Selection` as `ApplyHint` would on the current state.
- `SubmitHint` publishes a snapshot with `HintUsed=true`.
- snapshot sequence remains monotonic across moves, reshuffle, remove-number, and hint.
- rejected hints do not publish snapshots.
- hint after deadline returns `ErrGameOver`.
- manual `Stop` causes `SubmitHint` to return `ErrSessionClosed`.
- concurrent `SubmitMove`/`SubmitReshuffle`/`SubmitRemoveNumber`/`SubmitHint`/`Stop` do not panic.

API tests:
- `POST /games/{id}/hint` returns `200 OK` with a JSON selection on success.
- successful hint publishes an SSE snapshot with `hintUsed:true`.
- unknown game ID returns `404`.
- second hint returns `409 Conflict`.
- hint on a game-over session returns `409`.
- stopped session returns `410`.
- `GET /games/{id}/hint` returns `405`.

WebUI manual verification:
- start a game.
- click `Hint`; verify the non-zero cells of the highlighted rectangle gain the hint style and cleared cells inside the rectangle do not.
- play any valid rectangle (need not be the hinted one); verify the highlight clears on the next snapshot.
- verify the Hint button shows `Used` after one click and is disabled.
- verify Reshuffle and Remove still work independently.

### Verification Commands

```sh
gofmt -w <changed .go files>
go test ./...
go run ./cmd/server   # manual WebUI check
```

### Acceptance

- Hint is implemented as an authoritative once-per-session backend skill.
- Hint mutation is restricted to setting `HintUsed=true`; the board, score, and `ValidMoves` are unchanged.
- Hint payload is delivered only via the `POST /games/{id}/hint` response body; snapshots carry only `HintUsed`.
- Returned `Selection` is a copy and does not alias `state.ValidMoves`.
- WebUI shows a Hint button that highlights non-zero cells of the returned rectangle until the next snapshot.
- Unsupported methods on `/games/{id}/hint` return `405`.
- CLI behavior is unchanged and continues to compile.
- `gofmt` is run on changed Go files; `go test ./...` passes.
