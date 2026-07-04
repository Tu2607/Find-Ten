# Step 31 — Game-Over Score Submission Frontend Groundwork

## Problem Statement

The game-over popup shows the final score, but there is not yet a player-facing path to submit that score for a future leaderboard. This step adds the smallest frontend-only submission flow so a later backend/database step can wire the submit action to persistence.

## Scope

In scope:
- Add a `Submit Score` button to the game-over popup.
- Add a separate score-submission popup with:
  - player name input
  - `Submit` button
  - `Cancel` button
- Switch from the game-over popup to the score-submission popup when `Submit Score` is clicked.
- Switch back to the game-over popup when the player cancels or submits.
- Require a non-empty player name before submission.
- Show a confirmed `Score Submitted` state on the game-over popup after submission.
- Reset the submitted state when a new game starts.

Out of scope:
- Backend leaderboard API.
- Database writes or score persistence.
- localStorage or browser score history.
- Rendering real leaderboard rows.
- Sorting, pagination, filtering actual leaderboard data, accounts, or anti-cheat.
- Changes to `docs/GOAL.md` or `docs/ARCHITECTURE.md`.

## Design

The WebUI keeps this as a local popup state only. The existing game-over overlay remains the final-status popup. A second overlay handles name entry and returns to the game-over overlay after either `Cancel` or `Submit`.

Submission only records that the current game score was submitted in local frontend state. The future backend step can replace the submit handler body with an API request that sends the player name and final score to persistent storage.

## Acceptance Criteria

- The game-over popup shows a `Submit Score` button between the final score and existing navigation buttons.
- Clicking `Submit Score` hides the game-over popup and shows the score-submission popup.
- The score-submission popup includes a player name input, `Submit`, and `Cancel`.
- The name input is focused when the score-submission popup opens.
- `Submit` is disabled while the player name is empty or whitespace-only.
- Clicking `Cancel` returns to the game-over popup without marking the score submitted.
- Clicking `Submit` with a valid name returns to the game-over popup and shows a confirmed `Score Submitted` state.
- The score cannot be submitted twice for the same game-over popup.
- Starting a new game resets the submission state.
- No backend code, persistence, or leaderboard rendering is added.
- `go test ./...` passes.

## Verification

- Run `go test ./...`.
- Manually verify the game-over submit flow in the browser.
- Manually verify the submit state resets after `Play Again`.
- Manually verify `Cancel` returns to game over without confirming submission.
