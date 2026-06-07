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

