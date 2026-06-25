## Step 22: Once-Per-Session Remove Number Skill - Done

Add a second backend gameplay skill: a once-per-session remove-number action that lets the player set one non-zero board cell to `0`. This skill is a player-triggered state mutation, so it must use the existing player action channel and be processed by `RunGame`.

Completion note: Step 22 was implemented across backend game rules, runtime/session wiring, API, and the static WebUI. The CLI was intentionally not expanded for this skill; existing CLI behavior still compiles and passes tests. Remove-number is exposed through a position-based backend rule and session method, plus `POST /games/{id}/remove-number`. The WebUI now has a minimal remove mode button that submits a selected non-zero cell and waits for the normal SSE snapshot update.

### Design Intent

The remove-number skill gives the player a controlled way to force a board change when a useful sum-to-10 rectangle is hard to find.

Rules:
- The skill removes exactly one board number.
- Removing a number means setting that cell to backend value `0`.
- The skill can be used at most once per game session.
- The skill does not award score.
- The skill is rejected if the game is already over.
- The skill is rejected after the deadline has passed.
- A successful removal rebuilds the valid move cache.
- A successful removal publishes a normal runtime snapshot.
- A rejected removal does not consume the skill and does not publish a snapshot.
- A rejected removal retains the remaining remove-number skill use count.

The backend remains authoritative. Clients may expose a button or mode for the skill, but only backend game state decides whether the skill is available.

### Core Game Rules

Add a position-based helper:

```go
func ApplyRemoveNumber(state *GameState, position Position) error
```

Behavior:
- Return `ErrNilGameState` for a nil state.
- Return `ErrGameOver` if the game is already over.
- Return `ErrUninitializedMove` if the valid move cache is uninitialized.
- Return a remove-number-specific already-used error on second use.
- Return `ErrOutOfBounds` if the position is outside the board.
- Return a remove-number-specific invalid target error if the selected cell is already `0`.
- Set the selected non-zero cell to `0`.
- Do not change `Score`.
- Mark the remove-number skill used only after a successful removal.
- Leave the remove-number skill available after any rejected removal attempt.
- Rebuild `ValidMoves` and the internal valid move set.
- If all cells are cleared, set `GameOverBoardCleared`.
- Otherwise, if no valid moves remain, set `GameOverNoValidMoves`.

Add errors similar to the reshuffle errors:

```go
ErrRemoveNumberAlreadyUsed
ErrRemoveNumberInvalidTarget
```

### Player Action Runtime

Extend the existing player action enum:

```go
const (
	PlayerActionMove PlayerActionType = iota + 1
	PlayerActionReshuffle
	PlayerActionRemoveNumber
)
```

Extend `PlayerActionRequest` with explicit position data:

```go
type PlayerActionRequest struct {
	Type      PlayerActionType
	Selection Selection
	Position  Position
	Result    chan error
}
```

Field usage:
- `Selection` is meaningful only for `PlayerActionMove`.
- `Position` is meaningful only for `PlayerActionRemoveNumber`.
- `PlayerActionReshuffle` ignores both `Selection` and `Position`.

Add:

```go
func (s *GameSession) SubmitRemoveNumber(ctx context.Context, position Position) error
```

`SubmitRemoveNumber` wraps `PlayerActionRemoveNumber` and sends it through the existing action channel. Deadline checks, manual stop behavior, result delivery, and snapshot publishing should match move and reshuffle submissions.

### State And Snapshots

Track skill usage authoritatively:

```go
RemoveNumberUsed bool
```

Add this field to:
- `GameState`
- `GameSnapshot`
- API snapshot DTOs

The initial snapshot should report `removeNumberUsed: false`.

Runtime snapshots after successful removal should report `removeNumberUsed: true`.

### API Behavior

Add:

```text
POST /games/{id}/remove-number
```

Request:

```json
{
  "position": {
    "row": 0,
    "col": 0
  }
}
```

Success response:

```json
{ "accepted": true }
```

Behavior:
- Look up the stored game.
- Return `404 Not Found` for unknown game IDs.
- Decode and validate the JSON position body.
- Call `stored.session.SubmitRemoveNumber(r.Context(), position)`.
- Return `200 OK` with an acknowledgement when accepted.
- Do not include a snapshot in the response.
- Runtime board updates are delivered through the existing SSE snapshot stream.

Error mapping:
- missing or invalid JSON body -> `400 Bad Request`
- out-of-bounds position -> `400 Bad Request`
- already-cleared target cell -> `400 Bad Request`
- game over -> `409 Conflict`
- remove-number already used -> `409 Conflict`
- closed session -> `410 Gone`
- request context cancellation/deadline -> `408 Request Timeout`
- unexpected/internal action errors -> `500 Internal Server Error`

Add method routing so unsupported methods on `/games/{id}/remove-number` return `405 Method Not Allowed` where applicable.

### Static WebUI Behavior

Add a minimal remove-number control to the existing static WebUI.

UI behavior:
- Add a second skill button, `Remove`.
- Clicking `Remove` enters remove mode.
- In remove mode, the next clicked non-zero board cell submits the remove-number request.
- The request uses `POST /games/{id}/remove-number`.
- A successful response is only an acknowledgement; the board updates from the SSE snapshot.
- Clear remove mode when a snapshot arrives, when the request fails, or when a new game starts.
- Normal rectangle selection remains unchanged when remove mode is inactive.
- If remove mode is active and the player clicks a cleared cell, keep the game running and show a small status message.

Disable the remove button when:
- no game is active
- the game is over
- the remove-number skill has already been used
- a remove-number request is pending

Track local pending state so double-clicks cannot submit duplicate requests while waiting for the backend:

```text
disabled = !gameId || gameOver || removeNumberUsed || removeNumberPending
```

Do not migrate to React or add frontend build tooling in this step.

### CLI Position

Do not extend the playable CLI for this step.

The CLI was useful as an early manual testing surface, but the project now has a functional WebUI. The easier and more stable test boundary for this skill is the position-based backend function:

```go
ApplyRemoveNumber(state, position)
```

and the session wrapper:

```go
SubmitRemoveNumber(ctx, position)
```

These can be tested directly without adding more CLI command parsing. The existing CLI should continue to compile and keep its current move and reshuffle behavior.

### Testing

Core game tests should cover:
- successful removal sets exactly the selected non-zero cell to `0`
- score is unchanged
- remove-number usage is marked after success
- valid move cache is rebuilt
- removal can be used only once
- rejected removal does not mutate the board
- rejected removal does not consume the skill or reduce the remaining use count
- out-of-bounds position returns `ErrOutOfBounds`
- already-cleared target returns the invalid target error
- removal after game over returns `ErrGameOver`
- removal can set `GameOverBoardCleared`
- removal can set `GameOverNoValidMoves`

Runtime/session tests should cover:
- `SubmitRemoveNumber` publishes a runtime snapshot after success
- runtime snapshot sequence remains monotonic across moves, reshuffle, and remove-number
- rejected remove-number actions do not publish snapshots
- remove-number after deadline returns `ErrGameOver`
- manual session stop causes remove-number submissions to return `ErrSessionClosed`
- unknown player action types still return an internal action error without mutating state
- concurrent `SubmitMove`, `SubmitReshuffle`, `SubmitRemoveNumber`, and `Stop` do not panic

API tests should cover:
- `POST /games/{id}/remove-number` returns `200 OK` for an accepted removal
- accepted removal response does not include a snapshot
- accepted removal publishes a runtime snapshot through SSE
- unknown game ID returns `404`
- missing or malformed position returns `400`
- out-of-bounds position returns `400`
- already-cleared target returns `400`
- already-used remove-number returns `409`
- stopped session returns `410`
- game-over session returns `409`
- unsupported methods return `405`

Static WebUI/manual verification should cover:
- start a game from the browser
- click `Remove`
- click a non-zero cell
- verify the cell clears after the SSE snapshot
- verify score does not change
- verify the remove button cannot be used successfully a second time
- verify normal rectangle move selection still works when remove mode is inactive

### Acceptance

- Remove-number is implemented as an authoritative backend action.
- Remove-number can be used once per session.
- Remove-number sets one selected non-zero cell to `0`.
- Remove-number does not award score.
- Remove-number rebuilds valid moves and applies normal game-over detection.
- Remove-number emits a normal runtime snapshot on success.
- Rejected remove-number attempts do not emit snapshots.
- Rejected remove-number attempts retain the remaining skill use.
- HTTP API supports `POST /games/{id}/remove-number`.
- Static WebUI exposes a minimal remove-number control.
- The playable CLI is not expanded for this skill.
- Existing CLI behavior still compiles and tests pass.
- No React/npm/build tooling is added.
- Run `gofmt` on changed Go files.
- Run `go test ./...`.
