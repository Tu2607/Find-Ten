## Step 15: API Server Scaffold and Route Dispatch - Done

Add the first HTTP API surface without implementing game-session behavior yet. This step creates the server entrypoint and internal API package structure so later steps can add session creation, SSE snapshots, and move submission incrementally.

### Design Intent

The API layer should be a thin transport boundary over `internal/game`. It should not duplicate game rules, own game state, or bypass `GameSession`.

Use package name `internal/api` instead of `internal/httpapi` for a shorter project-local name.

This step only establishes:
- server executable
- API package
- route dispatch
- basic health endpoint
- unsupported route/method handling

Do not add:
- game creation
- session IDs
- session store
- SSE
- move submission
- WebUI files

### File Structure

Add:

```text
cmd/server/
  main.go

internal/api/
  server.go
  handlers.go
  errors.go

internal/api/
  server_test.go
```

### Server Entrypoint

Add `cmd/server/main.go`.

Behavior:
- parse `-addr`, defaulting to `127.0.0.1:8080`
- create an API server with `api.NewServer()`
- start `http.ListenAndServe`
- print the listening address
- keep game logic out of `cmd/server`

### API Server

Add `internal/api/server.go`.

Responsibilities:
- own an `http.ServeMux` or equivalent standard-library router
- implement `ServeHTTP`
- register initial routes

Initial routes:
- `GET /health`
  - returns `200 OK`
  - response body can be simple plain text or JSON
- placeholder route dispatch for future:
  - `POST /games`
  - `GET /games/{id}/snapshots`
  - `POST /games/{id}/moves`

For this step, future game routes should return `501 Not Implemented` if matched.

Unsupported routes should return `404 Not Found`.

Unsupported methods on known routes should return `405 Method Not Allowed`.

### Error Helpers

Add small helpers in `internal/api/errors.go` for consistent HTTP errors.

Keep this minimal:
- plain text or small JSON errors are both acceptable
- avoid introducing a larger response framework

### Acceptance

- `cmd/server` builds.
- `api.NewServer()` returns an `http.Handler`.
- `GET /health` returns `200 OK`.
- Unknown routes return `404`.
- Unsupported methods on known routes return `405`.
- Placeholder game routes return `501`.
- Tests cover route dispatch behavior.
- Run `gofmt` on changed Go files.
- Run `go test ./...`.

