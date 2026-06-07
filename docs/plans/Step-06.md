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

