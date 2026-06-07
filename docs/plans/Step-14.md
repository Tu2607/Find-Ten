## Step 14: Deadline Timer and Move-Only Runtime Refactor - Done

Refactor the runtime so player moves and timer expiry are separate signals. The goal is to make snapshots lazy and move-driven, reduce WebUI rendering churn, simplify event handling, and make channel ownership/cleanup clearer.

### Design Intent

The current runtime sends both player moves and timer ticks through the same event channel. That keeps state mutation serialized, but it also means timer countdown changes can cause full board snapshots. For a future WebUI, repeated timer-driven board snapshots can interfere with transient UI state such as an in-progress selection.

The new design keeps backend authority over game rules while separating:
- player move requests
- timer expiry
- snapshot delivery
- session lifecycle

The UI should render countdown locally from an authoritative deadline. The backend should not publish full board snapshots merely because time passes.

### Timer Model

- Replace decrementing `RemainingTime` as runtime state with an authoritative `ExpiresAt time.Time`.
- Store `ExpiresAt` on `GameSession` as runtime/session metadata.
- Pass `ExpiresAt` into `RunGame` for deadline checks.
- Do not add `ExpiresAt` to `GameSnapshot`.
- Remove per-second timer tick events.
- Remove `EventTick`.
- The UI derives countdown from `ExpiresAt`.
- Backend move handling checks the deadline before applying a move.
- `RunGame` checks the deadline again before applying a move to prevent deadline races.

### Runtime Channels

Replace the mixed event channel with separate runtime signals:

- A move-only request channel.
  - Used only by `SubmitMove`.
  - Carries player move requests.
  - Each request includes:
    - `Selection`
    - `Result chan error`
  - Result channels remain error-only.
  - A nil result means the move was accepted and applied.
  - Snapshots are never returned through move result channels.
- A one-shot timer expiry channel.
  - Used only by the timer goroutine.
  - Signals when `ExpiresAt` is reached.
  - Does not carry snapshots.
  - Does not send per-second updates.
  - Is separate from the move channel.
- The snapshot channel.
  - Produced only by `RunGame`.
  - Closed only by `RunGame`.
  - Publishes snapshots only for successful moves.
  - Does not publish a timer-expiry-only board snapshot.
- The done channel.
  - Closed by `GameSession` after runtime goroutines and cleanup finish.

### RunGame Behavior

Update `RunGame` to select directly on:
- move request channel
- timer expiry channel
- context cancellation

Do not switch on timer event types inside a shared event channel.

Move handling:
- If the game is already over, return `ErrGameOver`.
- If the deadline has passed, mark the game over with `GameOverTimeExpired` and return `ErrGameOver`.
- Otherwise apply the move through existing game rules.
- Invalid moves return their current errors and do not publish snapshots.
- Successful moves publish a hard-copy snapshot.
- Runtime snapshot sequence starts at `2`, because sequence `1` belongs to the bootstrap snapshot returned by `NewGameSession`.

Timer expiry handling:
- When the expiry channel signals, `RunGame` marks:
  - `GameOver = true`
  - `GameOverReason = GameOverTimeExpired`
- `RunGame` returns after handling timer expiry.
- `RunGame` closes the snapshot channel.
- No full board snapshot is emitted solely because the timer expired.

Context cancellation:
- Manual stop remains lifecycle shutdown.
- Manual stop should not be reported as time expiry.
- New move submissions after manual stop return `ErrSessionClosed`.

### Terminal Conditions

A game session can end in exactly these gameplay ways:
- board cleared completely
- no valid moves remain
- timer expired

Add a distinct game-over reason:
- `GameOverBoardCleared`

Terminal precedence after a successful move:
- If all cells are cleared, set `GameOverBoardCleared`.
- Else if no valid moves remain, set `GameOverNoValidMoves`.
- Else continue.

Timer precedence:
- If the deadline has passed before a move is applied, timer expiry wins and the move is rejected with `ErrGameOver`.
- A move submitted before expiry but processed after expiry must still be rejected as expired.

### GameSession Behavior

`NewGameSession(ctx, size)` keeps returning:
- `session`
- `initialSnapshot`
- `error`

The initial snapshot:
- has `Sequence = 1`
- is created before `RunGame` starts
- is returned once as a session creation result
- is not stored on `GameSession`

`GameSession` owns runtime lifecycle:
- create context/cancel
- allocate move channel
- allocate expiry channel
- allocate snapshot channel
- start game loop goroutine
- start one-shot timer goroutine
- expose snapshots
- submit moves
- stop session
- expose done

`SubmitMove` behavior:
- Reject new submissions after session shutdown begins.
- Reject submissions after deadline with `ErrGameOver`.
- Send accepted move requests to the move channel.
- Wait for the error-only result channel.
- Return `ErrSessionClosed` for lifecycle shutdown before the move is accepted or processed.
- Return `ErrGameOver` for game-rule termination.

### Channel Cleanup

Close all session-owned channels cleanly when the session ends.

Ownership rules:
- `RunGame` is the only snapshot sender and closes the snapshot channel.
- Timer goroutine is the only expiry sender and closes the expiry channel when it exits.
- `GameSession` owns the move channel lifecycle.
- `GameSession` closes `done` after the loop, timer, and move-channel cleanup complete.

Move channel close safety:
- `Stop()` must prevent new submissions before the move channel is closed.
- In-flight `SubmitMove` calls must be allowed to unwind or be released by cancellation.
- The move channel must not be closed while a submitter can still send.
- Concurrent `SubmitMove` and `Stop` must not panic.

### Snapshot Policy

Snapshots are generated only for:
- initial session creation
- successful player moves

Snapshots are not generated for:
- invalid moves
- rejected moves
- countdown changes
- timer expiry alone
- manual session stop

Snapshots remain hard-copy views:
- board data must be deep-copied
- sequence is runtime stream metadata
- timer deadline metadata is not included in board snapshots

### CLI Behavior

Update CLI to:
- render the initial snapshot returned by `NewGameSession`
- render later snapshots from `session.Snapshots()`
- show countdown based on `ExpiresAt` if countdown display is needed
- continue to submit moves through `GameSession.SubmitMove`
- display game-over reason when a terminal snapshot is received after a successful move
- exit cleanly when session ends due to timer expiry and snapshots close

No WebUI, HTTP API, WebSocket API, multiplayer, session IDs, or routing should be added in this step.

### Acceptance

- Tests verify `GameSnapshot` does not include `ExpiresAt`.
- Tests verify initial snapshot has sequence `1`.
- Tests verify runtime snapshots start at sequence `2`.
- Tests verify no snapshots are emitted while only time passes before expiry.
- Tests verify successful moves still emit snapshots.
- Tests verify invalid moves do not emit snapshots.
- Tests verify timer expiry alone does not emit a board snapshot.
- Tests verify timer expiry marks game over with `GameOverTimeExpired`.
- Tests verify board cleared ends with `GameOverBoardCleared`.
- Tests verify no valid moves ends with `GameOverNoValidMoves`.
- Tests verify board-cleared reason takes precedence over no-valid-moves after a successful move.
- Tests verify `SubmitMove` after expiry returns `ErrGameOver`.
- Tests verify a move submitted before expiry but processed after expiry returns `ErrGameOver`.
- Tests verify manual session stop causes new submissions to return `ErrSessionClosed`.
- Tests verify result channels remain error-only and do not carry snapshots.
- Tests verify snapshot channel closes when `RunGame` exits.
- Tests verify expiry channel closes when the timer goroutine exits.
- Tests verify move channel closes during session cleanup.
- Tests verify `done` closes after runtime cleanup completes.
- Tests verify concurrent `SubmitMove` and `Stop` do not cause send-on-closed-channel panic.
- Tests verify CLI uses the constructor-returned initial snapshot and runtime snapshot channel.
- Run `gofmt` on changed Go files.
- Run `go test ./...`.
- Run `go test -race ./...`.

