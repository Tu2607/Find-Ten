# Known Issues

Accepted, documented issues that are not currently scheduled for a fix. Each entry records what
happens, why it happens, its impact, and what a fix would look like if it is ever picked up.

## Flaky test: `TestSubmitScoreImmediatelyAfterFinalMove`

- **Found**: during Step 38 verification. The test failed once with `status = 409, want 201`
  (`internal/api/server_test.go`) and passed on every rerun.
- **What happens**: the test plays a game to completion with direct handler calls, then submits a
  score immediately after the final move's response. Occasionally the submission is rejected with
  `409 game is not over`.
- **Cause**: in `RunGame` (`internal/game/play.go`), a move's result is sent back to the waiting
  HTTP handler before the resulting snapshot is published. The game-over snapshot then travels
  through the snapshot channel to the per-session broker. `handleSubmitScore` checks
  `broker.latestSnapshot()`, so a score submitted in the gap between the final move's response and
  the broker receiving the game-over snapshot sees a game that is not over yet.
- **Impact**: test-only. The browser opens the score-submission form only after it receives the
  game-over snapshot over SSE, and that snapshot comes from the same broker, so a real player's
  submission always happens after the broker has recorded the game as over.
- **Possible fix**: have the test wait for the game to end before submitting, the way the
  `driveGameToOver` helper waits on `session.Done()`, or reuse `driveGameToOver` directly.
