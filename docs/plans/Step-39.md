# Step 39 — Coordinated Game-Session Foundation

## Goal

Allow a future multiplayer manager to create two independent game sessions that share a starting board, open for play at the same instant, close at the same deadline, and expose their final results once both have finished. Preserve the existing single-player entry point and behavior.

The manager coordinates lifecycle and collects results; it does not inspect moves, boards, skills, or snapshot updates.

## Non-goals

- Room management, room codes, game ID assignment, and the multiplayer manager itself.
- HTTP endpoints, error-to-status mapping for the new error, and registration of coordinated sessions in the API session store.
- The frontend countdown screen and hiding the board during the lead-in.
- Winner selection and match result presentation.
- Any change to `RunGame`'s event handling, the single-player timer, the snapshot channel, or the broker.

These belong to later steps. Requirements this step places on them are listed under "Requirements For Later Steps".

## Current State

- `NewGameSession` validates duration, generates state with `NewGame(size)`, sets `expiresAt = time.Now() + duration` at creation (`play.go`), and starts `RunGame` plus an expiry timer. The playing window is fixed at creation, not at a first move.
- Deadline checks run in two places: `submitAction` rejects early once the deadline is reached, and `applyPlayerActionBeforeDeadline` rejects authoritatively when `RunGame` processes the action. A zero `expiresAt` means "no deadline" (`deadlineExpired`).
- `run()` closes `Done()` only after `RunGame` has returned, the timer goroutine has exited, and all in-flight submissions have finished.
- On expiry, `RunGame` publishes a final snapshot, and every snapshot send blocks until the consumer receives it or the session context ends. `RunGame` cannot return, and `Done()` cannot close, until its final snapshot is received or the session is stopped.
- `NewGame(size)` validates the size, then loops: it generates a random board, wraps it in a fresh `GameState`, builds the valid-move cache, and returns the state once the board has at least one valid move. The cache is built once, as part of verifying the board. `cloneBoard` already deep-copies a board.
- There is no session-level final-result interface separate from snapshots.

## Design

### Shared starting state

`NewGame(size)` is unchanged. It generates a board, builds the valid-move cache in the same loop that verifies the board has at least one valid move, and returns a fresh `GameState`. Nothing is rebuilt: single-player builds the cache once per game, as it does today, and `board.go` is unchanged.

The manager calls `NewGame(size)` once and passes the returned `*GameState` to both constructors. `NewGame` guarantees a supported size, at least one valid move, and a fresh game (score `0`, no skills used, not over), so the state needs no further validation. The precondition is strict: the state must be fresh and unmodified as returned by `NewGame`, and must not be mutated while either constructor runs. The manager owns that state exclusively and sessions keep only copies, so the precondition is easy to meet. It is documented on the constructor and not checked at runtime; re-validating would undo the simplification of reusing `NewGame`'s cache.

Each constructor copies the starting state rather than using it directly, through a small unexported helper:

- the board is deep-copied with `cloneBoard`, so the two sessions never share board rows; without the copy, one player's moves would clear cells on the other player's board
- the `ValidMoves` slice and the unexported `validMoveSet` map are copied, not rebuilt, so the cache built by `NewGame` is reused rather than thrown away; copying walks only the valid moves, while rebuilding would check every rectangle on the board
- the cache is copied rather than shared, so correctness does not depend on the cache never being edited in place; today every update replaces it wholesale, but nothing enforces that
- the copy starts with score `0`, no skills used, and not over, taking only the board and its cache from the source state

Neither session shares mutable board or cache data with the caller or the other session, and neither modifies the source state.

### Shared start time

A coordinated session is created with a start time `startsAt`. The manager picks one start time, `CoordinatedStartDelay` (3 seconds) ahead, and passes the same value to both sessions.

- Before `startsAt`, every action is rejected with a new `ErrGameNotStarted`: move, reshuffle, remove-number, and hint. Hint is included because it reveals information. Rejected attempts do not consume a skill, following the existing rule.
- The start check sits beside the existing deadline checks: in `submitAction` as an early reject, and in the authoritative check when `RunGame` processes the action (`applyPlayerActionBeforeDeadline`, renamed to reflect that it now checks the whole window, for example `applyPlayerActionInWindow`). An action processed at or after `startsAt` is accepted; one processed before it is rejected.
- "Started" is a time comparison (`now >= startsAt`), not an event in the game loop. Both sessions use the same `startsAt` value, so both open at the same instant even if their goroutines were created moments apart. No start signal or extra timer is added to `RunGame`.
- A zero `startsAt` means "no start gate", mirroring how a zero `expiresAt` means "no deadline". `NewGameSession` passes zero, so single-player behavior is unchanged.
- `StartsAt()` exposes the start time alongside the existing `ExpiresAt()`, so the API can send both to clients.

The playing window starts at `startsAt`, never at a player's first move.

### Shared deadline and session-owned timers

Each coordinated session sets `expiresAt = startsAt + duration`. Both sessions receive the same `startsAt` and duration, so both get exactly the same deadline.

- Each coordinated session runs the existing expiry timer to `expiresAt`, exactly as single-player sessions do. `timer.go` and the `expired` channel are unchanged.
- The session ends itself at the deadline; it never depends on the manager to signal it.
- A late timer cannot grant extra playing time: every action is checked against `expiresAt` both when submitted and when processed.
- A player whose board is cleared or who runs out of valid moves ends early; that session's `Done()` closes then. The other session continues to the deadline.

### Validation

`NewCoordinatedSession` validates its timing inputs before starting any goroutine. The starting state is not validated: callers must pass a fresh, unmodified state returned by `NewGame`, not mutated while the constructor runs (see Shared starting state). `NewGame` already guarantees a supported size, at least one valid move, and a fresh game.

- the duration passes the existing `ValidateDuration` (60, 120, or 180 seconds)
- `startsAt` is strictly in the future; a start time already reached is an error
- `startsAt` is no later than `now + CoordinatedStartDelay`

`CoordinatedStartDelay` is the single 3-second constant for both the lead-in and its limit. The manager computes `startsAt = time.Now().Add(CoordinatedStartDelay)` and then calls the constructors, whose own `now` is slightly later, so a correctly computed `startsAt` is always just under the limit. Any `startsAt` further ahead can only come from a manager bug and is rejected.

The future-`startsAt` check validates input and catches gross delays: if creating the second session starts after the lead-in has elapsed, its constructor fails and the manager stops the first. It does not guarantee that setup finishes before play opens. A constructor can pass its check and then be delayed (for example by a GC pause or CPU starvation) until after `startsAt`, and further setup such as registering sessions and brokers happens after the constructors return. Guaranteeing that the match is fully set up before play opens is the manager's job; see the start re-check under Requirements For Later Steps. A setup overrun is not a fairness hole, because both players share `startsAt` and the match is exposed to both at once, but it would shorten the playing window.

The start limit and the duration limit bound a coordinated session's lifetime at 3 + 180 seconds even if no manager ever touches it again. This preserves the bounded-lifetime assumption in `docs/ARCHITECTURE.md` that limits a full-registry denial of service.

### Final results

The session records its result after `RunGame` returns and before `Done()` closes. The manager waits for both sessions' `Done()` channels, then reads each result.

```go
// Inside the session lifecycle, after RunGame returns:
s.result = resultFromState(state) // Completed = state.GameOver
s.Stop()
// run() later closes s.done after the existing shutdown bookkeeping.

// Inside the future multiplayer manager:
<-first.Done()
<-second.Done()
firstResult := first.Result()
secondResult := second.Result()
```

- Reading `s.result` after receiving from `Done()` is race-free: closing a channel happens-before a receive that observes the close. No result mutex or notification channel is needed.
- `Result()` is valid only after `Done()` closes. An early finisher keeps its result until the manager reads it, and repeated calls return the same value.
- `Completed` is defined as: the game loop reached a normal terminal state (time expired, no valid moves, or board cleared) before returning. It is exactly `state.GameOver` when `RunGame` returns.
- `Done()` also closes after `Stop()` or parent-context cancellation. Cancellation before the game reaches a terminal state produces `Completed` false, so a partial score is never treated as a final result. Cancellation after it does not invalidate the result: on expiry, `RunGame` sets `GameOver` before blocking on the final snapshot send, so a stop that interrupts that send still yields `Completed` true.
- If cancellation and expiry are ready at the same moment, `RunGame`'s `select` may process either, so `Completed` may be true or false. Both outcomes satisfy the definition, and callers must not assume which one wins.
- Sequential waits are fine: both sessions run independently, and a receive from an already-closed channel returns immediately. A player who finishes early simply waits until the other session finishes.
- `resultFromState` runs after every `RunGame` exit, including a stop, so it is not named for completed games only. It also applies to single-player sessions, which can ignore it.

Do not read final scores from the broker's latest snapshot. `RunGame` can return, and `Done()` can close, before the broker goroutine stores the final snapshot. That is the same class of race as the flaky test recorded in `docs/KNOWN-ISSUES.md`.

## Proposed API

```go
// Existing; unchanged. The manager calls it once per match for the shared
// starting state.
func NewGame(size int) (*GameState, error)

// Existing single-player constructor; signature and behavior unchanged.
func NewGameSession(
    ctx context.Context,
    size int,
    durationSeconds int,
) (*GameSession, GameSnapshot, error)

// Starts a session on a copy of startingState (board and valid-move cache)
// that accepts actions from startsAt until startsAt + durationSeconds, with its
// own expiry timer. startingState must be a fresh, unmodified state returned
// by NewGame and must not be mutated while this constructor runs. It is not
// modified or retained.
func NewCoordinatedSession(
    ctx context.Context,
    startingState *GameState,
    startsAt time.Time,
    durationSeconds int,
) (*GameSession, GameSnapshot, error)

// Returns the session's start time; zero for single-player sessions.
func (s *GameSession) StartsAt() time.Time

// Returns the session's final result. Valid only after Done() has closed.
func (s *GameSession) Result() GameResult

var ErrGameNotStarted = errors.New("game has not started")

// The lead-in before a coordinated session opens, and the furthest ahead
// startsAt may be when the session is created.
const CoordinatedStartDelay = 3 * time.Second

type GameResult struct {
    Score          int
    GameOverReason GameOverReason
    // Completed reports whether the game loop reached a normal terminal state
    // (time expired, no valid moves, or board cleared) before returning. It is
    // false when a stop or parent-context cancellation ended the loop before
    // that; a cancellation after that point does not change it.
    Completed bool
}
```

Both constructors share one internal initializer that takes the prepared state, `startsAt`, and `expiresAt`, allocates channels and lifecycle bookkeeping, builds the initial snapshot (sequence `1`), and starts `RunGame` plus the expiry timer.

`RunGame` is exported and called directly by existing tests. Passing the start time into it changes its signature; update those call sites mechanically, passing a zero start time where tests do not exercise the start gate.

## Implementation Sequence

1. Extract the shared session initializer while preserving single-player behavior.
2. Add `startsAt` to the session and the start-time check beside both deadline checks, with `ErrGameNotStarted`; add `StartsAt()`.
3. Add the starting-state copy helper and `NewCoordinatedSession`, with its timing validation and `CoordinatedStartDelay`.
4. Record `GameResult` after `RunGame` returns and add `Result()`.
5. Add focused tests, including a shared test helper that drains `Snapshots()`.
6. Update `docs/ARCHITECTURE.md`.
7. Run `gofmt`, `go test ./...`, and `go test -race ./internal/game`.

## Expected Files

- `internal/game/play.go`: shared initializer, starting-state copy helper, `NewCoordinatedSession`, start-time checks, result recording, `StartsAt()`, `Result()`.
- `internal/game/types.go`: `startsAt` and `result` fields, `GameResult`, `ErrGameNotStarted`, `CoordinatedStartDelay`.
- Corresponding tests in `internal/game/`.
- `docs/ARCHITECTURE.md`.

`internal/game/board.go`, `internal/game/timer.go`, and `internal/game/valid_moves.go` are unchanged. No API, broker, or frontend changes are planned.

## Acceptance Criteria And Tests

Use table-driven tests where appropriate. No broker consumes coordinated sessions in this step and `RunGame` blocks on every snapshot send, so tests must drain `Snapshots()` through a shared helper. Supported durations are at least 60 seconds, so tests must not wait for a real coordinated deadline. Test the window checks directly with explicit `now` values, and test timer-driven behavior (start gate, expiry, results) by calling the shared internal initializer from package `game` tests with millisecond-scale `startsAt` and `expiresAt`. Validation tests still go through `NewCoordinatedSession`.

**Single-player**

- Existing single-player constructor, timer, actions, snapshots, and tests continue working; `StartsAt()` is zero.

**Board and construction**

- `NewGame`'s existing tests pass unchanged.
- Two coordinated sessions built from the same `NewGame` state, start time, and duration have equal starting boards, equal `StartsAt()`, and equal `ExpiresAt()` equal to `startsAt + duration`.
- Each session starts with the same board and the same valid moves as the source state, with score `0` and no skills used.
- Mutating the source state's board, `ValidMoves`, or valid-move set after construction cannot affect either session, and neither session mutates the source state.
- A move or skill in one session changes neither the other session's board, valid moves, score, nor skill usage.
- Unsupported durations, a `startsAt` at or before now, and a `startsAt` beyond `now + CoordinatedStartDelay` fail without starting goroutines.

**Playing window**

- Before `startsAt`, move, reshuffle, remove-number, and hint are rejected with `ErrGameNotStarted`, and rejected skills remain available.
- At or after `startsAt`, actions are accepted.
- At or after `expiresAt`, actions are rejected with `ErrGameOver`, even before the expiry timer has been processed.
- The window check is applied when `RunGame` processes the action, not only at submission.
- The expiry timer ends a coordinated session at `expiresAt` with `GameOverTimeExpired`.

**Results**

- After `Done()` closes, `Result()` matches the authoritative final score and game-over reason, with `Completed` true, for time expired, no valid moves, and board cleared. Repeated calls return the same value.
- When one session finishes early, its `Result()` is still correct when read after the other session finishes.
- After `Stop()` or parent-context cancellation before the game reaches a terminal state, `Result()` has `Completed` false.
- A session whose game has already ended and is blocked sending its final snapshot (nothing drains `Snapshots()`) still has `Completed` true, with the correct score and reason, after `Stop()` unblocks it.

**Tooling**

- `gofmt` is clean, `go test ./...` passes, and `go test -race ./internal/game` passes.

## Documentation Follow-Up

- `docs/ARCHITECTURE.md` says in two places that timer expiry does not emit a final snapshot (the Player Actions section and the State Sharing And Snapshots section), while `RunGame` does publish one on expiry. Correct both.
- Extend the Game Session Wrapper section with coordinated sessions: shared starting state (copied board and valid-move cache), start gate, session-owned timer to the shared deadline, `StartsAt()`, and `Result()` read after `Done()`.
- Preserve older completed plan files as historical records.

## Requirements For Later Steps

Recorded here so the room and match steps honor them:

- **Countdown:** the manager computes `startsAt = time.Now().Add(game.CoordinatedStartDelay)` immediately before creating both sessions, and passes the state from one `NewGame(size)` call to both sessions.
- **Frontend:** the browser shows a countdown screen and hides the board until `startsAt`. Countdown display should use server-provided times; latency between each player and the server is accepted as outside the game's control.
- **Session registration:** register both sessions of a match in the existing API session store. This attaches a broker that always drains snapshots, which `Done()` depends on, and makes each match count as two sessions against the existing 150-session cap.
- **Partial creation:** if the first session is created and the second constructor fails, stop the first session.
- **Start re-check:** after both sessions and their brokers are registered, and before the match is exposed to either player, check the time again. If `startsAt` has already passed, stop both sessions and fail the match rather than starting it with a shortened window.
- **Waiting:** the manager waits on both `Done()` channels with a limit of `expiresAt` plus a short grace period. If the limit is exceeded, it stops both sessions and reports an error rather than blocking forever.
- **Results:** results are read only after both `Done()` channels have closed. A result with `Completed` false marks a failed match, not a valid score.
- **HTTP:** map `ErrGameNotStarted` to an appropriate status when action endpoints are opened to coordinated sessions.

## Resolved During Review

- Result collection uses `Done()` plus `Result()` instead of a context-aware `FinalResult(ctx)`, which removes the risk of reporting a normally completed game as closed.
- Each session runs its own expiry timer to the shared deadline. Manager-driven expiry (`SignalExpiry`, a buffered signal channel, a coordinated-session flag) is dropped, so a session never depends on the manager to end.
- Fairness comes from one shared start time and one shared deadline, both fixed at creation. The playing window starts at `startsAt`, not at a first move.
- One constant, `CoordinatedStartDelay` (3 seconds), sets both the lead-in and the furthest ahead `startsAt` may be.
- A start time that has already been reached is a construction error rather than an instant expiry.
- An early finisher waits; results are collected only after both sessions finish.
- Board hiding during the countdown is a frontend responsibility; the game package does not withhold the board. Accepted limitation for the MVP: the server enforces the action window, not equal board-reveal time. The initial snapshot already contains the board, so once it reaches the browser, hiding it is cosmetic and a player could read it early through developer tools. The only real remedy would be to deliberately withhold the board from clients until `startsAt`, which is out of scope.
- The shared starting state comes from one existing `NewGame(size)` call, passed whole to both constructors. `NewGame` and `board.go` are unchanged, and no `NewBoard` is added: extracting the generation loop would either throw away the valid-move cache built while verifying the board, or need a verbose multi-value return to carry it out. Each constructor deep-copies the board and copies (not rebuilds, not shares) the valid-move cache, starting a fresh game from them. The state is not re-validated because `NewGame` already guarantees it.

## Open Questions

None.

## Post-Work Update

### Game-ending move reported as a closed session

Found during the final review of this step's implementation, where
`TestCoordinatedSessionResultsForTerminalReasons` failed about half the time with
`game session is closed`. The race predates this step and was outside the plan; it is fixed as a small
separate change included with this step's commit:

- After handing an action to `RunGame`, `submitAction` waited on the result, `closing`, `done`, and its
  request context. On a game-ending move, `RunGame` sends the result, returns, and `Stop()` closes
  `closing`. If the submitter reached that wait late, both cases were ready and `select` picked one at
  random, so an applied, scored move could return `ErrSessionClosed`.
- `sendActionResult` now sends unconditionally instead of giving up when the game context is
  canceled. The result channel is buffered with room for one reply, so the send never blocks, and
  every action `RunGame` accepts gets exactly one result.
- With the reply guaranteed, `submitAction`'s post-handoff wait no longer watches `closing` or `done`.
  It waits only for the result or its own request context. The pre-handoff wait is unchanged.
- `PlayerActionRequest.Result` documents the buffering requirement, and the `ARCHITECTURE.md`
  description of per-action result channels is updated.
- `TestSessionSubmitsReturnWhenSessionEndsWithoutRequestTimeout` submits concurrently with
  `context.Background()` while the session is ended by `Stop()` or parent cancellation, and fails if
  any submit does not return. Existing concurrent-stop tests use request timeouts, which would hide an
  accepted action that never receives a reply.
