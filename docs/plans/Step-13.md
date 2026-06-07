## Step 13: Bootstrap Initial Snapshot - Done
- Avoid startup deadlock caused by `RunGame` publishing an initial snapshot to an unbuffered snapshot channel before a receiver is ready.
- Keep runtime snapshot delivery unbuffered to preserve backpressure and avoid queued stale snapshots.
- Treat the initial snapshot as a one-time session creation result, not as persistent `GameSession` API.
- Update `NewGameSession`:
  - creates `GameState`.
  - before starting `RunGame`, creates one hard-copy initial `GameSnapshot`.
  - this snapshot has `Sequence = 1`.
  - returns the session and initial snapshot together:
    - `session, initialSnapshot, err := NewGameSession(ctx, size)`
  - does not store the initial snapshot on `GameSession`.
- After the bootstrap snapshot is created, `RunGame` takes over game-state ownership.
- Update `RunGame`:
  - remove startup snapshot publishing.
  - publish snapshots only after successful move requests.
  - start runtime snapshot sequence at `2`.
  - continue closing the snapshot channel when it exits.
- Update CLI:
  - render the initial snapshot returned by `NewGameSession` before entering the runtime input/snapshot loop.
  - continue rendering later updates from `session.Snapshots()`.
- Keep move result channels strictly error-only:
  - move results do not include snapshots.
  - snapshots are delivered only through the session creation result or snapshot channel.
- Do not buffer the snapshot channel for this fix.
- Do not add multiplayer, session IDs, or API routing in this step.

Acceptance:
- Tests verify `NewGameSession` returns an initial snapshot with sequence `1`.
- Tests verify `RunGame` no longer emits a startup snapshot.
- Tests verify the first runtime snapshot after a valid move has sequence `2`.
- Tests verify `SubmitMove` does not deadlock when no runtime snapshot receiver has consumed from `Snapshots()` yet.
- Tests verify CLI prints the initial snapshot returned by `NewGameSession`.
- Tests verify `go test ./...` and `go test -race ./...` pass.

