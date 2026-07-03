# Step 30 — Board Background Color Selection

## Problem Statement

The settings screen has scaffolded background color swatches, but only the default green board is currently usable. Step 28 intentionally left these swatches disabled. This step enables them as a frontend-only cosmetic setting.

## Scope

In scope:
- Enable the green, blue, red, and purple background swatches.
- Remove the "coming soon" label from the Background settings group.
- Track the active swatch in the browser.
- Apply the selected background color to the game screen chalkboard surface when a game starts.
- Keep welcome, settings, and leaderboard screens on the default green chalkboard.

Out of scope:
- Backend awareness of board color.
- localStorage or persisted settings.
- Changing gameplay rules, snapshots, scoring, or API request shapes.
- Changing frame, tray, or non-game screen colors.

## Design

Board color is a WebUI-only setting. The selected swatch is marked with `swatch--active`, and `startGame` reads the active swatch before creating the session. After the session starts, the game screen receives one of the `board-color-*` classes. The base green gradient remains the default, so only blue, red, and purple need explicit variant classes.

The class is applied to `#gameScreen`, and CSS scopes the alternate gradients to `.screen--game .chalkboard__surface`. This keeps all non-game chalkboards unchanged.

## Acceptance Criteria

- All four swatches are enabled and clickable.
- Green is active by default.
- Clicking a swatch activates it and deactivates the others.
- The "coming soon" text is removed.
- Starting a game with blue, red, or purple selected changes the game chalkboard surface.
- Starting a game with green selected uses the original green chalkboard.
- Restart and Play Again reuse the currently selected swatch.
- Timer and font settings continue to work independently.
- `go test ./...` passes.

## Verification

- Run `go test ./...`.
- Manually verify the settings screen swatches and each game-board color.
- Confirm welcome, settings, and leaderboard screens remain green.
