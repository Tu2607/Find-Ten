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

