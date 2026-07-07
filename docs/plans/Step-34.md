# Step 34 - Wire Leaderboard Endpoints To Frontend

## Goal

Wire the existing browser UI to the backend leaderboard endpoints:

- Submit a completed game's score through `POST /scores`.
- Load and render persisted leaderboard scores through `GET /scores`.

The frontend should use backend-owned game IDs and server-derived score data. It must not calculate or submit score, board size, duration, remaining time, or timestamps.

## Non-goals

- Do not change leaderboard persistence behavior.
- Do not change score ordering or backend validation rules.
- Do not add global all-board-size or all-duration leaderboard queries unless the backend API changes.
- Do not add accounts, pagination, rank lookup, score history, or duplicate-score recovery flows.
- Do not rewrite unrelated frontend layout or gameplay code.

## Current State

- Backend exposes `POST /scores`.
- Backend exposes `GET /scores?gridSize=<size>&duration=<seconds>`.
- `static/app.js` currently marks scores submitted locally without calling `POST /scores`.
- The leaderboard screen exists in `static/index.html`, but filter controls and table rendering are not wired.
- The current leaderboard UI includes `All` filter buttons, but the backend requires both `gridSize` and `duration`.

## Proposed Design

### Score Submission

Update `submitScore()` in `static/app.js` to:

1. Require an active `state.gameId`.
2. Require a non-empty trimmed player name.
3. Disable the submit button while the request is pending.
4. Send `{ "gameId": "<current game id>", "playerName": "<trimmed player name>" }` to `POST /scores`.
5. On `201 Created`, set `state.scoreSubmitted = true`, close the submit overlay, and show the game-over overlay with `Score Submitted`.
6. On failure, keep the form open, re-enable submit, and show a user-facing error.

Recommended status handling:

- `400`: invalid name or request.
- `404`: game no longer exists.
- `409`: already submitted or game is not finished.
- `408` or network error: connection retry message.
- `500+`: server error retry message.

### Leaderboard Loading

Add frontend state for:

- selected leaderboard board size
- selected leaderboard duration
- loading state
- error state
- latest loaded scores

Use concrete defaults:

- board size: `9`
- duration: `120`

When the leaderboard screen opens, call:

```text
GET /scores?gridSize=9&duration=120
```

When a filter button changes, reload with the selected concrete values.

### Filter UI

Remove the current `All` filter buttons because the backend API requires both filters.

Use:

- Board: `9 x 9`, `10 x 10`, `11 x 11`
- Time: `60s`, `120s`, `180s`

Default active filters should be `9 x 9` and `120s`.

### Leaderboard Rendering

Show a compact selected-filter banner above the table, for example:

```text
Showing 9 x 9 - 120s
```

Render each score row with:

- rank
- player name
- score
- remaining time, formatted from `remainingMillis`

Do not render board size or configured duration per row because those are already implied by the active filters.

Show the existing empty message when the response is an empty array.

Add lightweight loading/error text using the existing empty-state area:

- Loading: `Loading scores...`
- Error: `Could not load scores.`

## Files To Modify

- `docs/plans/Step-34.md`
- `static/app.js`
- `static/index.html`
- `static/styles.css`

## Implementation Sequence

1. Add `docs/plans/Step-34.md`.
2. Update leaderboard filter markup to remove `All` buttons.
3. Add frontend leaderboard state and filter event handling.
4. Implement `loadLeaderboardScores()`.
5. Implement leaderboard row rendering and empty/loading/error states.
6. Replace placeholder `submitScore()` with real `POST /scores` wiring.
7. Add pending/error behavior for score submission.
8. Run `go test ./...`.
9. Smoke test the browser flow manually.

## Acceptance Criteria

- Submitting a completed game score persists it through the backend.
- A successful submission updates the game-over overlay to `Score Submitted`.
- Failed submission does not falsely mark the score submitted.
- Opening the leaderboard loads persisted scores for the default filter.
- Changing board size or duration reloads leaderboard data.
- Empty leaderboards show the empty state.
- Backend tests still pass with `go test ./...`.
