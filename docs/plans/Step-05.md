## Step 5: Board Generation - Done
- Implement random board generation for supported sizes.
- Fill boards with digits `1-9`.
- Regenerate until at least one valid move exists.
- Return initialized `GameState` with a populated valid-move cache.

Acceptance:
- Tests verify generated board dimensions.
- Tests verify generated boards contain only `1-9`.
- Tests verify generated game states always start with at least one valid move.

