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

