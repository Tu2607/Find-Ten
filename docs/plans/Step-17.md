## Step 17: Per-Session Snapshot Broker and SSE Endpoint - Done

Implement `GET /games/{id}/snapshots` as a Server-Sent Events stream backed by a per-session snapshot broker.

### Design Intent

The frontend receives the initial snapshot from `POST /games`. Runtime snapshots after successful moves should arrive through SSE.

`GameSession.Snapshots()` is a single-consumer channel. The API should not let multiple SSE handlers read that channel directly because concurrent readers would split snapshots between clients. Instead, each stored game owns one snapshot broker.

The broker is the only API-layer consumer of `GameSession.Snapshots()`. SSE handlers subscribe to the broker.

This keeps:
- `RunGame` as the only producer of runtime snapshots
- `GameSession` as the runtime lifecycle owner
- the API store as the game-ID registry
- the broker as the per-game fan-out mechanism

### API Contract

Endpoint:

```text
GET /games/{id}/snapshots
```

Successful response:
- status `200 OK`
- `Content-Type: text/event-stream`
- `Cache-Control: no-cache`
- stream stays open until request cancellation, broker close, or session end
- each runtime snapshot is sent as:

```text
event: snapshot
data: {"sequence":2,...}

```

The initial snapshot is not sent over SSE. It remains only in the `POST /games` response.

New SSE subscribers should receive the latest runtime snapshot immediately if one exists. This gives reconnecting or late-attaching clients the newest board state without replaying the initial creation snapshot. If no runtime snapshot exists yet, the subscriber waits for future runtime snapshots.

### Stored Game Shape

Change the store value from only `*game.GameSession` to a small stored game value:

```go
type storedGame struct {
    session *game.GameSession
    broker  *snapshotBroker
}
```

The struct is only the API store value for a game ID. It is not a new game-state owner and does not replace `GameSession`.

Store responsibilities:
- create a `storedGame` when `POST /games` succeeds
- generate the game ID
- store `gameID -> storedGame`
- look up `storedGame` by ID
- keep no deletion policy in this step

### Snapshot Broker

Add `internal/api/broker.go`.

Broker responsibilities:
- own subscriber channels
- receive runtime snapshots from one `GameSession`
- remember the latest runtime snapshot, if one has been received
- fan out each snapshot to current subscribers
- send the latest runtime snapshot to a new subscriber immediately when available
- close subscribers when the source snapshot channel closes
- allow subscribers to unsubscribe when their request exits

The broker should not know the game ID. One broker belongs to one stored game, so all snapshots it receives already belong to that game.

Subscriber channel ownership:
- the broker is the only closer of subscriber channels
- unsubscribe removes a subscriber channel from the broker but does not close it
- publish, unsubscribe, and broker shutdown coordinate through the broker mutex to avoid send-on-closed-channel races

Suggested shape:

```go
type snapshotBroker struct {
    mu          sync.Mutex
    subscribers map[chan snapshotResponse]struct{}
    done        chan struct{}
}
```

Start the broker when storing the game:

```go
broker := newSnapshotBroker(session.Snapshots())
go broker.run()
```

or have the constructor start it.

### Subscriber Policy

Use a small buffer per subscriber:

```go
make(chan snapshotResponse, 1)
```

When publishing to a subscriber:
- try to send the latest snapshot
- if the subscriber already has one queued, drop the stale queued snapshot and replace it with the latest snapshot
- do not let one slow SSE client block the broker
- do not let one slow SSE client block `RunGame`

This matches the snapshot model because each snapshot is a full board view. The latest snapshot is enough for display.

### Broker Shutdown

When the source snapshot channel closes:
- close the broker `done` channel
- close all subscriber channels
- prevent new subscribers from attaching successfully

Subscription behavior after broker close:
- return `false` or an error so the handler can return `410 Gone` or `404 Not Found`

Recommended response:
- if game ID is unknown: `404 Not Found`
- if game is known but broker is already closed because the session ended: `410 Gone`

### SSE Handler Behavior

For `GET /games/{id}/snapshots`:
- parse `id` using `r.PathValue("id")`
- look up the stored game
- return `404 Not Found` if missing
- verify `http.Flusher` support
- return `500 Internal Server Error` if streaming is not supported
- subscribe to the stored game broker
- return `410 Gone` if the broker is closed
- set SSE headers
- flush headers immediately
- loop on:
  - `snapshot := <-subscriber`
  - `r.Context().Done()`

When a snapshot arrives:
- encode the snapshot DTO as JSON
- write:
  - `event: snapshot`
  - `data: <json>`
  - blank line
- flush

When subscriber channel closes:
- return from the handler

When request context is canceled:
- unsubscribe and return

### Routing

Replace the Step 16 placeholder for:

```text
GET /games/{id}/snapshots
```

with the real SSE handler.

Keep placeholder unchanged for:

```text
POST /games/{id}/moves
```

### Testing

Use `httptest.NewServer` or `httptest.ResponseRecorder` carefully because SSE handlers can block. Prefer request contexts that can be canceled in tests.

Tests should cover:
- known game ID returns SSE headers
- initial snapshot is not written to the stream
- late subscriber receives the latest runtime snapshot if one exists
- runtime snapshot is written as `event: snapshot`
- snapshot `data:` is valid JSON using the existing snapshot DTO shape
- unknown game ID returns `404`
- subscribing after broker close returns `410`
- multiple subscribers to one game can each receive the same runtime snapshot
- slow subscriber does not block broker fan-out
- subscriber is removed when request context is canceled
- subscriber channel closes when source snapshot channel closes
- `POST /games/{id}/snapshots` still returns `405`
- `POST /games/{id}/moves` still returns `501`

### Acceptance

- `GET /games/{id}/snapshots` for a known active game opens an SSE stream.
- SSE response uses `Content-Type: text/event-stream`.
- SSE response uses `Cache-Control: no-cache`.
- Handler flushes the stream open before any runtime snapshot exists.
- Runtime snapshots are delivered as `event: snapshot`.
- Runtime snapshot data is valid JSON.
- Initial snapshot is not replayed through SSE.
- Late subscribers receive the latest runtime snapshot if one exists.
- Multiple concurrent SSE subscribers for one game receive the same runtime snapshots.
- Slow subscribers do not block snapshot fan-out.
- Unknown game ID returns `404`.
- Ended game/broker returns `410`.
- Snapshot stream exits cleanly on request cancellation.
- Snapshot stream exits cleanly when the session snapshot stream closes.
- `POST /games/{id}/moves` remains unimplemented and returns `501`.
- No move submission implementation yet.
- Run `gofmt` on changed Go files.
- Run `go test ./...`.

