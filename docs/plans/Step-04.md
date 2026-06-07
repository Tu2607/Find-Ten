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

