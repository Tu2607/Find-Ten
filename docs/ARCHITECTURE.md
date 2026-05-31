# Architecture

## Overview

This document records architectural decisions for the Go backend-first game implementation. Product goals, MVP scope, gameplay rules, and future feature ideas live in `docs/GOAL.md`.

## Backend Authority

The backend owns authoritative state transitions for:
- board generation
- move validation
- scoring
- game-over detection
- timer-driven state transitions

Clients, including the CLI and future WebUI, must call backend game logic instead of duplicating rules.

## Core Design

The core game logic lives under `internal/game`.

The game state is modeled around:
- `Board`, a rectangular grid of integers
- `Selection`, a rectangular region between two positions
- `GameState`, the authoritative state for one game

Cleared cells are stored as the number `0`. This keeps the board shape stable, avoids nullable cells, and makes prefix sums straightforward.

## Move Validation

Move validation is cache-backed.

Selections are normalized before validation so all drag directions map to one canonical rectangle.

## Valid Move Cache

The repository uses a valid move cache as the authoritative validation mechanism.

When the board changes, the game rebuilds:
- `ValidMoves []Selection`
- an internal `map[Selection]struct{}`

The slice supports CLI display, hints, debugging, and future AI/bot behavior. The map supports O(1) validation of player-submitted moves.

`len(ValidMoves) == 0` means the game is over.

The cache is rebuilt:
- after board generation
- after each successful move

The cache is not rebuilt after rejected moves because the board did not change.

## Validation And Cache Trade-Offs

The board size is fixed and small. At the maximum `11x11` size, there are only `4,356` possible rectangles. That means validation could be implemented without a cache and still run fast enough.

The cache is used because it simplifies the rest of the game:
- move validation becomes a direct lookup
- game-over detection becomes `len(ValidMoves) == 0`
- hints and debug output can read from the same source
- future bots, replay verification, and difficulty analysis can reuse the cached move list

The trade-off is keeping derived state in sync with the board. The project accepts that cost because the sync rule is simple: rebuild the full cache whenever the board changes.

Incremental cache updates are intentionally avoided. They would add more bookkeeping and edge cases, while full rebuilds are deterministic, easy to test, and fast enough for the fixed board sizes.

## Prefix Sums

The backend uses a 2D prefix sum table to compute rectangle sums in O(1).

Rebuilding the valid move cache enumerates every possible rectangle and checks its sum using the prefix table.

For the maximum MVP board size, `11x11`, there are only `4,356` possible rectangles. Full cache recomputation after each successful move is intentionally preferred over incremental invalidation because it is simple, deterministic, and fast enough.

Prefix sums are not required for performance at the current board sizes. A naive rectangle sum would also be fast enough for local play, online play, and server-side replay verification. The important architectural decision is the valid move cache; prefix sums are an isolated implementation helper that can be replaced later if direct summing proves clearer.

The project keeps prefix sums for now to test the suggested optimization and to keep cache rebuilds cheap without changing the cache-oriented design.

## Game Loop

The planned runtime model is a single-owner game loop.

One goroutine owns game state and processes runtime signals serially:
- move requests
- timer-expiry signals

This avoids race-prone shared mutation and creates a direct path to future WebSocket and multiplayer support.

## State Sharing And Snapshots

External callers should not read `GameState` directly while the game loop is running. The loop owns game-state mutation and should also own game-state reads that are shared outside the loop.

The project will expose hard-copy `GameSnapshot` values for display and future network responses. A snapshot is a point-in-time view of the authoritative state and must not share mutable backing data with `GameState`. In particular, `Board` must be deep-copied.

Snapshots are emitted for:
- initial game start
- successful move requests

Timer countdown changes do not produce board snapshots. The session stores an authoritative expiry deadline, and the UI can derive countdown display from that session-level metadata when a UI/API layer exposes it. Timer expiry is a terminal runtime signal, not a board-state update, so it does not emit a final board snapshot by itself.

Snapshot ordering uses a local sequence counter owned by the runtime. The initial bootstrap snapshot returned by `NewGameSession` has sequence `1`. Runtime snapshots emitted by `RunGame` start at sequence `2` and increment with each successful move snapshot. The sequence does not belong on `GameState`; it is runtime stream metadata. A future game manager or match session can own this counter when the runtime grows beyond a single loop function.

Snapshots may also include a timestamp. The timestamp helps clients estimate display freshness or smooth a countdown, while the sequence is the authoritative ordering signal.

## Concurrency Trade-Offs

A mutex-based design could protect shared `GameState` reads and writes. That would work for a small CLI, but it spreads lock discipline across every caller. Future HTTP handlers, WebSocket clients, timers, tests, and multiplayer code would all need to remember to lock before touching state.

The chosen design is actor-style ownership:
- producers send move requests or timer-expiry signals
- `RunGame` serially processes events
- `RunGame` owns all state reads and writes
- callers receive snapshots instead of shared mutable state

This design is slightly more structured than direct state reads with a mutex, but it better matches the planned web backend. It gives future move submission, spectator views, replay verification, and multiplayer match loops a single authoritative path for state changes and state views.

The project keeps separate runtime channels for move requests and timer expiry. Snapshot delivery uses an output channel, but that channel does not manage state; it only carries immutable views produced by the state owner.

## Game Session Wrapper

The next runtime boundary is a single-game `GameSession`.

`GameSession` is a lifecycle wrapper, not a state owner. It allocates and wires runtime pieces:
- session context and cancel function
- move request channel
- timer expiry channel
- snapshot channel
- timer goroutine
- game loop goroutine
- completion signal

`RunGame` remains the only code that mutates `GameState`. It also remains the only producer and closer of the snapshot channel. `GameSession` creates the snapshot channel and exposes its receive side, but it does not write snapshots.

The timer remains owned by the session lifecycle but has only one job: signal once when the session deadline expires. It does not mutate game state and does not send through the move request channel.

The session wrapper should expose a small safe API for callers:
- receive snapshots
- submit moves with context-aware result waiting
- stop the session
- wait for session completion

This keeps CLI, future HTTP handlers, and future WebSocket handlers from manually wiring channels and goroutines. It also creates a natural path to a later manager that can hold multiple sessions without changing the core game loop.

Multiplayer, session IDs, and API routing are intentionally out of scope for the first `GameSession` step.

## CLI First

The CLI demo is the first manual testing surface.

The CLI is not a separate rules implementation. It is a thin client over `internal/game` and follows the same actor-style ownership model planned for a future web backend.

The CLI wiring has three channels:
- an input line channel fed by a scanner goroutine
- the move request channel consumed by `RunGame`
- the snapshot channel produced by `RunGame`

The CLI does not call `ApplyMove` directly after the game loop starts. It parses input into a `Selection`, submits a move through `GameSession`, waits for the per-move result channel, and renders state from snapshots.

Move result waits are context-aware. If the game context is canceled before a result is sent, the CLI stops waiting instead of blocking forever. Per-move result channels are one-shot response channels: they may receive at most one result and are not closed.

Snapshot rendering is also event-driven. The CLI prints the initial snapshot, then prints later snapshots emitted after successful moves. This keeps display reads out of shared mutable `GameState` and makes the CLI a useful concurrency demo rather than a shortcut around the backend architecture.
