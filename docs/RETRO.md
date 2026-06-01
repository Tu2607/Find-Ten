# Architecture Retrospective

This document records the design path that led to the current runtime and API direction. It is not an implementation plan. It captures trade-offs, why earlier choices were reasonable, and what weakness pushed the next design step.

## Initial Runtime Model

The original runtime model was a single event-channel-driven game loop.

All runtime events flowed through one channel and were processed by event type:
- player move events
- timer tick events

That design had an important strength: `RunGame` owned state mutation. Serializing all events through one channel avoided shared mutable access to `GameState` and kept game-rule application straightforward.

For a CLI-first backend, this was a reasonable starting point.

## Timer And WebUI Pressure

The weakness appeared when considering a WebUI.

Timer ticks and player moves are different kinds of signals:
- a player move changes the board and should produce a new board snapshot
- time passing changes countdown display but should not force a full board render

With timer ticks flowing through the same event channel as moves, the backend could produce snapshots merely because one second passed. That creates rendering churn for a WebUI and can interfere with transient UI state, such as an in-progress selection.

The correction was to split runtime signals:
- move requests go through a move-only channel
- timer expiry uses a separate one-shot expiry signal

The UI derives countdown display from an authoritative deadline. The backend still owns time-expired game-over behavior, but it does not publish full board snapshots for countdown changes.

## Snapshot Boundary

A core design rule emerged: external callers should not read live `GameState`.

`GameState` is mutable and owned by the game loop after runtime starts. Sharing it directly with CLI, API handlers, or future WebUI code would leak mutable data and create race-prone ownership.

Instead, external reads happen through immutable `GameSnapshot` values.

Snapshots are hard-copy views:
- board data is deep-copied
- score and game-over fields are copied by value
- sequence and timestamp are runtime metadata

This keeps backend authority in `RunGame` while giving outside code safe state views.

## Bootstrap Snapshot Exception

The WebUI and CLI both need an initial board before the player makes a move.

Originally, `RunGame` emitted a startup snapshot. With an unbuffered snapshot channel, that created a startup deadlock risk if no receiver was attached yet.

The current design uses a controlled bootstrap exception:
- `NewGameSession` creates `GameState`
- before starting `RunGame`, it creates the initial hard-copy snapshot
- the initial snapshot has sequence `1`
- `NewGameSession` returns the session and initial snapshot together
- after `RunGame` starts, runtime snapshots are produced only by `RunGame`

This exception is safe because no concurrent mutation exists before `RunGame` starts.

The runtime rule remains clean:
- bootstrap snapshot: created once by `NewGameSession`
- runtime snapshots: created by `RunGame` after successful moves

## GameSession Role

`GameSession` exists as a lifecycle wrapper around one running game.

It owns:
- session context and cancellation
- move request channel
- timer expiry channel
- snapshot channel
- timer goroutine
- game loop goroutine
- done signal

It does not replace `RunGame` as the state owner. `RunGame` remains the code that mutates game state and produces runtime snapshots.

The session wrapper gives CLI and API code a small safe surface:
- submit moves
- receive snapshots
- read expiry deadline
- stop the session
- wait for completion

## API Shape

The planned Web/API flow mirrors the CLI flow but splits it across HTTP requests.

The intended API shape is:
- `POST /games`
  - creates a session
  - returns game ID, initial snapshot, and expiry deadline
- `GET /games/{id}/snapshots`
  - opens an SSE stream for runtime snapshots
- `POST /games/{id}/moves`
  - submits player moves

The frontend starts on a splash screen. The game is not created until the user clicks Start. After `POST /games`, the frontend renders the initial snapshot, opens the SSE stream, and enables moves once the stream is connected.

## Snapshot Channel Weakness

The current snapshot channel is unbuffered.

After a successful move, the game loop applies the move, sends the move result, creates a snapshot, and attempts to publish it.

Because the channel is unbuffered, publishing a snapshot requires an active receiver. If no receiver is draining `session.Snapshots()`, `RunGame` blocks.

For the CLI, this is manageable because one controlled loop both submits moves and receives snapshots. After `SubmitMove` returns, the CLI loop quickly resumes receiving snapshots.

For an API, this is more fragile because calls are separate HTTP requests:
- `POST /games`
- `GET /games/{id}/snapshots`
- `POST /games/{id}/moves`

The server cannot assume that the SSE request exists or remains connected when a move request arrives.

A blocking scenario without a broker:
1. client calls `POST /games`
2. server returns the initial snapshot
3. client never opens the snapshot stream, or the stream disconnects
4. client submits a move
5. `RunGame` applies the move and returns success to the move request
6. `RunGame` tries to publish the runtime snapshot
7. no receiver is draining `session.Snapshots()`
8. `RunGame` blocks

Once blocked, that session cannot process more moves or timer expiry cleanly until a receiver appears or the session is stopped.

## Direct SSE Trade-Off

A direct SSE endpoint could read from `session.Snapshots()`.

That would work only while the SSE request is connected. If the request disconnects, the snapshot drain disappears. A later successful move could block the game loop.

Direct SSE also has single-consumer semantics. If two HTTP clients read from the same snapshot channel, they do not both receive all snapshots. They split snapshots unpredictably.

That means direct SSE requires either:
- a strict one-stream guard
- or a fragile frontend contract that only one stream exists and stays connected

Those constraints are acceptable for a very narrow demo, but brittle for real browser behavior such as refreshes, reconnects, duplicate tabs, or dev tools.

## Broker Direction

The broker is an API transport adapter, not a game-rule component.

The broker exists to adapt:
- one runtime snapshot channel

into:
- many SSE subscribers

The intended broker is per-session, not global.

One game session owns one broker. The broker is the only API-layer consumer of `GameSession.Snapshots()`. SSE clients subscribe to the broker instead of reading the runtime channel directly.

This keeps `RunGame` protected from browser connection churn. Even if no browser is connected, the broker continues draining the runtime snapshot channel.

The broker should stay simple:
- subscribe
- unsubscribe
- fan out immutable snapshots
- close subscribers when the source snapshot channel closes

It should not know game rules, validate moves, mutate state, own timers, or replace `GameSession`.

## Per-Session Broker Trade-Off

A global broker was considered, but the current need is per-game fan-out.

A global broker would require:
- game-ID-tagged messages
- topic or filtering logic
- cross-session routing safeguards
- cleanup of per-game topics

A per-session broker is simpler:
- every snapshot already belongs to one game
- no filtering is required
- cleanup is tied to one session
- accidental cross-session delivery is avoided

The API store still groups a game ID with the runtime pieces needed for that game:
- the `GameSession` for command handling
- the snapshot broker for event fan-out

That grouping can be represented as a small stored value, such as `storedGame`. It is not a new game-state owner; it is only the API store's value type.

## Subscriber Backpressure

The broker must not let one slow SSE client block the game loop.

Because snapshots are complete board views, subscribers do not need every historical snapshot to render the current board. The latest snapshot is enough.

The planned policy is:
- each subscriber gets a small buffered channel
- if a subscriber already has a queued snapshot, replace the stale queued snapshot with the latest snapshot
- do not block broker fan-out on slow subscribers
- do not block `RunGame` on slow subscribers

This preserves the runtime goal: the game loop should not be coupled to browser render speed.

## Current Architectural Boundary

The current intended layering is:

```text
internal/game
  GameState        authoritative mutable state
  RunGame          only runtime mutator and runtime snapshot producer
  GameSession      lifecycle wrapper around one game

internal/api
  store            gameId registry
  storedGame       groups session plus broker for one game ID
  broker           per-session snapshot fan-out
  handlers         HTTP and SSE transport
```

The key boundary is that game rules stay in `internal/game`. API code can transport commands and snapshots, but it should not duplicate game validation or mutate game state directly.
