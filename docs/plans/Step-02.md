## Step 2: Core Types - Done
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

