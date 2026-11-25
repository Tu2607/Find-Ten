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

## Redeploying the server ends active games

- **Found**: while discussing graceful shutdown for the Kubernetes service-split experiment.
- **What happens**: when the server receives `SIGTERM`, for example when `docker compose up`
  replaces the container, every active game is lost within a few seconds. Players mid-game lose
  their board, and players who have finished but not yet submitted a score cannot submit it.
- **Cause**: game sessions live only in process memory. `cmd/server/main.go` calls
  `http.Server.Shutdown` with a 5-second `shutdownTimeout`. `Shutdown` closes the listener
  immediately, so moves, skills, and score submissions arriving as new requests are refused, and
  open SSE snapshot streams hold `Shutdown` until the timeout expires. Nothing stops or preserves
  game sessions on exit. `docker-compose.yaml` sets no `stop_grace_period`, so Docker's default
  10-second grace period applies.
- **Impact**: accepted behavior rather than a bug. Games last at most 180 seconds and deploys are
  infrequent, so at most one round per active player is lost per deploy. Leaderboard scores and
  accounts are persisted and unaffected.
- **Possible fix**: drain before shutting down. On `SIGTERM`, keep the listener open, return `503`
  from `POST /games`, and keep serving every other route until no game is active or about
  180 seconds have passed, plus a short window for score submission after the last game ends.
  Then call `Shutdown` with the existing short timeout. Set `stop_grace_period` in
  `docker-compose.yaml` above the drain ceiling. Lengthening `shutdownTimeout` alone does not help,
  because `Shutdown` already refuses the new requests a game needs. The trade-off is that new
  games cannot start for up to a few minutes during each deploy.
