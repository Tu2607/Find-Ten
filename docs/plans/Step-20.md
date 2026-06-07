## Step 20: Player Action Channel and Once-Per-Session Reshuffle Skill - Done

Add the first backend gameplay skill: a once-per-session reshuffle that preserves the number of cleared cells. This step also generalizes the current move-only runtime channel into a player action channel so rectangle moves and reshuffles share the same actor-owned state mutation path.

Planning note: after defining this feature, earlier forward-looking runtime wording was revised from move-only requests to player action requests where appropriate. Completed historical step descriptions still document the work as it was originally planned, but the current architecture and Step 20 use the broader player action terminology.

Completion note: Step 20 was implemented across backend game rules, runtime/session wiring, API, CLI, and the static WebUI. The final implementation reuses the existing selection-scoped `countNonZeroCells` helper with a full-board selection instead of adding a separate `countNonZeroBoardCells` helper. The WebUI exposes reshuffle with a pending/used disabled state and hides the visible valid-move count to avoid confusing players when valid rectangles include cleared `0` cells.

### Design Intent

The current runtime has a move request channel plus a separate one-shot timer expiry channel. That split remains correct. Timer expiry should stay separate because it is deadline/lifecycle behavior, not a player-triggered command.

The new reshuffle skill is different from timer expiry: it is a player-triggered action that mutates authoritative game state. It should therefore use the same serialized state-owner path as rectangle moves.

This step replaces the move-only request channel with a player action request channel:
- rectangle moves are player actions
- reshuffle skill usage is a player action
- timer expiry remains separate
- snapshots are emitted after successful player actions
- rejected player actions do not emit snapshots

### Player Action Runtime Rework

Replace `MoveRequest` with a more general request type:

```go
type PlayerActionType int

const (
	PlayerActionMove PlayerActionType = iota + 1
	PlayerActionReshuffle
)

type PlayerActionRequest struct {
	Type      PlayerActionType
	Selection Selection
	Result    chan error
}
```

Rules:
- `Selection` is used only for `PlayerActionMove`.
- `PlayerActionReshuffle` ignores selection data.
- `Result` remains error-only.
- Result channels are one-shot response channels and are not closed.
- Unknown action types return an internal action error and do not mutate state.

Rename session/runtime concepts to match the generalized channel:
- `GameSession.moves` becomes an action/request channel.
- `RunGame` receives player action requests instead of move requests.
- `SubmitMove(ctx, selection)` remains a public API and wraps `PlayerActionMove`.
- Add `SubmitReshuffle(ctx)` and wrap `PlayerActionReshuffle`.
- Deadline checks apply to both move and reshuffle actions.
- Manual stop/session close behavior applies to both move and reshuffle submissions.

`RunGame` behavior:
- select on player action requests, timer expiry, and context cancellation.
- on player action:
  - reject if the game is over.
  - reject if the deadline has passed, marking time expiry.
  - switch on action type.
  - apply the requested action.
  - send the error-only result.
  - publish a hard-copy snapshot only if the action succeeds.
  - return if the action causes game over.
- on timer expiry:
  - mark `GameOverTimeExpired`.
  - return without publishing a board snapshot.

### Reshuffle Gameplay Rules

Add a once-per-session reshuffle skill.

Rules:
- A reshuffle can be used at most once per game session.
- Reshuffle is rejected if the game is already over.
- Reshuffle is rejected after the deadline has passed.
- A successful reshuffle preserves the current score.
- A successful reshuffle preserves the exact number of cleared cells currently represented by `0`.
- A successful reshuffle rebuilds the valid move cache.
- A successful reshuffle publishes a normal runtime snapshot.
- A rejected reshuffle does not consume the skill and does not publish a snapshot.

Track usage authoritatively:
- Add `ReshuffleUsed bool` or equivalent to `GameState`.
- Include reshuffle usage/availability in `GameSnapshot`.
- Include reshuffle usage/availability in API snapshot DTOs so clients can disable the skill button.

Use lazy zero counting:
- Do not maintain a running zero count on `GameState`.
- When reshuffle is activated, count non-zero board cells.
- Calculate `zeroCount = totalCellCount - nonZeroCellCount`.
- Board sizes are small enough that scanning the board on skill activation is simpler and sufficiently fast.

Add a whole-board helper rather than forcing the existing selection-scoped helper into this job:

```go
func countNonZeroBoardCells(board Board) int
```

### Reshuffle Algorithm

On successful reshuffle activation:
- Count the current number of zero cells.
- Generate a fresh random board with digits `1-9`.
- Choose exactly `zeroCount` unique random cell positions.
- Set those positions to `0`.
- Rebuild the valid move cache.
- If the board has at least one valid move, accept it.
- If the board has no valid moves, retry generation.
- Consume the reshuffle skill only after a valid reshuffled board is found.

Use an attempt limit similar to board generation so the operation cannot loop forever. If no valid reshuffled board can be generated within the limit, return an internal error and leave the current game state unchanged.

If the board is already fully cleared, reshuffle should not create a playable board. The game should already be over with `GameOverBoardCleared`, so reshuffle submissions should return `ErrGameOver`.

### API Behavior

Add an HTTP endpoint:

```text
POST /games/{id}/reshuffle
```

Behavior:
- Look up the stored game.
- Return `404 Not Found` for unknown game IDs.
- Call `stored.session.SubmitReshuffle(r.Context())`.
- Return `200 OK` with an acknowledgement when accepted.
- Do not include a snapshot in the response.
- Runtime board updates are delivered through the existing SSE snapshot stream.

Suggested response:

```json
{ "accepted": true }
```

Error mapping should match move submission where possible:
- game over -> `409 Conflict`
- session closed -> `410 Gone`
- request context cancellation/deadline -> `408 Request Timeout`
- already used -> `409 Conflict`
- unexpected/internal action errors -> `500 Internal Server Error`

### CLI Behavior

Add CLI support for the skill:
- Accept `reshuffle` as an input command.
- Keep `q` and `quit` behavior unchanged.
- Keep rectangle move input unchanged.
- Update the prompt to mention reshuffle.
- On successful reshuffle, wait for the normal runtime snapshot and render it.
- If reshuffle is already used, print a clear rejection and keep playing.

### Static WebUI Behavior

Add only the minimum UI needed to verify the backend mechanic:
- Add a reshuffle button.
- Disable or mark the button used based on snapshot reshuffle state.
- Call `POST /games/{id}/reshuffle`.
- Wait for SSE to update the board after success.
- Show a small error on rejected reshuffle.

The WebUI should also keep a local pending state for the reshuffle request. Do not rely only on the latest snapshot to disable the button, because there can be a short period after click/submission where the latest received snapshot still says the skill is available.

Suggested client-side button rule:

```text
disabled = gameOver || reshuffleUsed || reshufflePending
```

On click:
- set `reshufflePending = true` immediately.
- submit the reshuffle request.
- if the request succeeds, keep the button disabled and wait for the SSE snapshot with `reshuffleUsed = true`.
- if the request fails with a retryable request/client error, clear `reshufflePending`.
- if the request fails because the skill was already used, the game is over, or the session is closed, keep the button disabled.

The backend remains authoritative. `GameState.ReshuffleUsed` decides whether the skill is actually available, and the API must still reject duplicate reshuffle requests even if the UI temporarily shows stale state.

Do not migrate to React in this step.

### Testing

Core game tests should cover:
- reshuffle preserves the exact zero count.
- reshuffle changes/replaces the board when possible.
- reshuffle preserves score.
- reshuffle rebuilds the valid move cache.
- reshuffled boards have at least one valid move.
- reshuffle can be used only once.
- rejected reshuffle does not mutate the board.
- rejected reshuffle does not consume the skill.
- reshuffle after game over returns `ErrGameOver`.

Runtime/session tests should cover:
- `SubmitMove` still works through the player action channel.
- `SubmitReshuffle` publishes a runtime snapshot.
- invalid/rejected player actions do not publish snapshots.
- runtime snapshot sequence continues monotonically across moves and reshuffle.
- reshuffle after deadline returns `ErrGameOver`.
- manual session stop causes reshuffle submissions to return `ErrSessionClosed`.
- unknown player action types return an internal action error without mutating state.
- concurrent `SubmitMove`, `SubmitReshuffle`, and `Stop` do not panic.

API tests should cover:
- `POST /games/{id}/reshuffle` returns `200 OK` for an accepted reshuffle.
- accepted reshuffle response does not include a snapshot.
- accepted reshuffle publishes a runtime snapshot through SSE.
- unknown game ID returns `404`.
- already-used reshuffle returns `409`.
- stopped session returns `410`.
- game-over session returns `409`.
- unsupported methods return `405`.

CLI tests should cover:
- `reshuffle` input calls the session reshuffle path.
- the prompt mentions reshuffle.
- already-used reshuffle prints an error and keeps the CLI running.

Static WebUI/manual verification should cover:
- start a game from the browser.
- use reshuffle once.
- verify the board updates through SSE.
- verify the number of cleared cells is preserved visually.
- verify the reshuffle button cannot be used successfully a second time.

### Acceptance

- The runtime uses a player action request channel for user-triggered game-state mutations.
- Timer expiry remains a separate one-shot signal.
- Rectangle moves still behave as before.
- Reshuffle is implemented as an authoritative backend action.
- Reshuffle can be used once per session.
- Reshuffle preserves the exact number of zero cells.
- Reshuffle guarantees at least one valid move after completion.
- Reshuffle emits a normal runtime snapshot on success.
- Rejected reshuffle attempts do not emit snapshots.
- CLI supports the reshuffle command.
- HTTP API supports `POST /games/{id}/reshuffle`.
- Static WebUI exposes a minimal reshuffle control.
- No React/npm/build tooling is added in this step.
- Run `gofmt` on changed Go files.
- Run `go test ./...`.
- Run `go test -race ./...`.
- Manually verify the CLI and WebUI flows.

