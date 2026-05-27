# Stepwise Implementation Plan

## Step 1: Project Scaffold
- Create `go.mod` with module name `find-ten-game`.
- Add initial package layout:
  - `internal/game`
  - `cmd/play`
- Add a minimal compile-only entrypoint for the CLI demo.

Acceptance:
- `go test ./...` runs successfully with no game logic yet.

## Step 2: Core Types
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

## Step 3: Prefix Sum + Rectangle Sum
- Implement prefix sum construction for `Board`.
- Implement O(1) rectangle sum lookup.
- Treat `0` as a normal numeric cell.

Acceptance:
- Tests verify single-cell, row, column, and multi-cell rectangle sums.
- Tests include rectangles containing `0`.

## Step 4: Valid Move Cache
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

## Step 5: Board Generation
- Implement random board generation for supported sizes.
- Fill boards with digits `1-9`.
- Regenerate until at least one valid move exists.
- Return initialized `GameState` with a populated valid-move cache.

Acceptance:
- Tests verify generated board dimensions.
- Tests verify generated boards contain only `1-9`.
- Tests verify generated game states always start with at least one valid move.

## Step 6: Move Application
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

## Step 7: Timer/Event Loop
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

## Step 8: CLI Demo
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

## Step 9: Final Verification
- Run `gofmt`.
- Run `go test ./...`.
- Manually run the CLI once to verify the loop works.

Acceptance:
- All tests pass.
- CLI demo runs without panic.
- No WebUI or quadrilateral logic is included yet.

## Step 10: Channel-Driven Snapshots
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

## Assumptions
- Implementation proceeds in the order above.
- Each step should compile and pass tests before moving to the next.
- Coding should not begin until the user explicitly approves starting Step 1.
