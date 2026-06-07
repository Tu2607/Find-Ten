## Step 19: Static WebUI API Integration - Done

Add the first browser-based play surface as a static WebUI served by the existing Go HTTP server.

### Design Intent

This step validates the full HTTP API flow in a real browser without introducing a frontend build tool or React yet. Treat this as the first frontend contract and expect visual/design iteration after seeing it in the browser.

The goal is a playable, compact frontend that connects to:
- `POST /games`
- `GET /games/{id}/snapshots`
- `POST /games/{id}/moves`

The WebUI should be a thin client over the API. It must not duplicate game rules beyond basic input shaping and display state.

### Non-Goals

Do not add:
- React
- npm or a JS build tool
- WebSocket transport
- multiplayer
- authentication
- persistence
- hints or AI assistance
- game-rule logic duplicated from the backend

### File Structure

Add static frontend files:

```text
static/
  index.html
  styles.css
  app.js
```

Serve those files from the existing Go server.

### Server Behavior

Extend `internal/api` static routing:
- use `http.FileServer(http.Dir("./static/"))`
- mount it at `GET /`
- register it after the API routes so more-specific API routes continue to win

Keep all existing API routes unchanged:
- `GET /health`
- `POST /games`
- `GET /games/{id}/snapshots`
- `POST /games/{id}/moves`

This is intentionally the simplest static-serving solution for testing UX quickly. It depends on running the server from the repository root, which is acceptable for the current `go run ./cmd/server` workflow. A later packaging/deployment step can switch to embedded static assets if needed.

Keep `cmd/server` as the only server entrypoint. The WebUI should run through:

```sh
go run ./cmd/server
```

and be reachable at:

```text
http://127.0.0.1:8080/
```

### Frontend Behavior

The first screen should be the actual game setup and play surface, not a marketing landing page.

Required UI:
- board-size selector for `9`, `10`, and `11`
- start-game button
- score display
- valid-move-count display
- countdown display derived from `expiresAt`
- game status/error display
- board grid

Game startup:
- user chooses a board size
- frontend calls `POST /games`
- frontend renders `initialSnapshot`
- frontend stores `gameId` and `expiresAt`
- frontend opens `EventSource` to `/games/{gameId}/snapshots`

Snapshot handling:
- update board, score, valid move count, and game-over status from SSE `snapshot` events
- do not expect the initial snapshot from SSE
- tolerate reconnect/late subscription receiving the latest runtime snapshot

Move input:
- player clicks one cell for selection start
- player clicks a second cell for selection end
- frontend previews/highlights the normalized rectangle
- frontend submits the rectangle to `POST /games/{gameId}/moves`
- on accepted move, wait for SSE to update the board
- on invalid move or out-of-bounds response, show a small error and keep the current board

Timer display:
- derive countdown locally from `expiresAt`
- do not expect timer snapshots from the backend
- stop or mark expired when countdown reaches zero

### Visual Direction

Keep the UI utilitarian and board-first:
- compact controls
- stable square cells
- clear selected rectangle highlight
- visible cleared cells as `0` or a muted empty-looking state while preserving board shape
- responsive layout that works on desktop and mobile widths

Avoid decorative landing-page composition. The player should be able to start and play immediately.

### Testing

Backend/static tests should cover:
- `GET /` returns `200 OK`
- `GET /styles.css` returns `200 OK`
- `GET /app.js` returns `200 OK`
- API routes still behave as before

Manual/browser verification should cover:
- start a `9x9` game from the browser
- board renders from the initial snapshot
- SSE connection opens
- clicking two cells submits a move
- successful move updates the board through SSE
- invalid move shows an error and does not clear cells
- countdown displays from `expiresAt`

### Acceptance

- Static WebUI is served by the Go server.
- User can start a game from the browser.
- Initial board renders.
- SSE runtime snapshots update the browser board.
- User can submit rectangle moves by clicking cells.
- Move response does not need to include a snapshot.
- Invalid moves do not mutate the displayed board.
- Countdown is derived client-side from `expiresAt`.
- No React/npm/build tooling is added in this step.
- Run `gofmt` on changed Go files.
- Run `go test ./...`.
- Manually run the server and verify the WebUI in a browser.

