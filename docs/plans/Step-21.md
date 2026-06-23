## Step 21: Session Store Cleanup, Capacity Guard, and Abandon Endpoint - Done

Keep the current in-memory session store, but add explicit and opportunistic cleanup so old sessions do not accumulate. The WebUI will abandon its current game before starting a new one, and the store will reject new sessions when capacity is full.

### Design Intent

The API session store currently keeps every created game until process exit. A browser tab can also click Start multiple times, which creates fresh sessions while the old active session remains stored.

This step keeps the store in memory and adds two cleanup paths:
- explicit abandon for sessions the WebUI knows it is replacing.
- opportunistic cleanup for completed sessions when a new game is created.

Do not add external storage, persistence, or a background cleanup goroutine in this step.

### API Behavior

Add:

```text
DELETE /games/{id}
```

Behavior:
- `DELETE /games/{id}` abandons a known session.
- If the ID exists, remove it from `sessionStore`, stop the session, and return `204 No Content`.
- If the ID is unknown, return `404 Not Found`.
- Removed game IDs return `404` on later move, reshuffle, and snapshot requests.
- `POST /games` remains focused on creating a new session.
- `POST /games` does not internally call `DELETE /games/{id}`.

Update WebUI Start behavior:
- clicking Start creates a brand-new game session and board.
- if `state.gameId` exists, call `DELETE /games/{state.gameId}` best effort before creating the next game.
- ignore `404` from the delete call because the old game may already be gone.
- then call `POST /games` to create the new session.
- replace `state.gameId` only after the new create response succeeds.

### Store Cleanup and Capacity

Use this default:

```go
const defaultMaxStoredSessions = 150
```

Add a store error:

```go
var errSessionStoreFull = errors.New("session store is full")
```

Cleanup rules:
- Active sessions are not removed by opportunistic cleanup.
- Completed sessions are sessions whose `session.Done()` channel is already closed.
- Completed sessions are removed immediately during cleanup.
- Removing a session means deleting its map entry with `delete(s.sessions, id)`, which also prunes the generated game ID.

During `sessionStore.add`:
- lock the store mutex.
- remove any stored sessions whose `Done()` channel is closed, using a non-blocking select.
- prune each removed session ID with `delete(s.sessions, id)`.
- if `len(s.sessions) >= defaultMaxStoredSessions` after cleanup, return `errSessionStoreFull`.
- otherwise generate and store the new game ID.

During explicit abandon:
- remove the game ID from the store under the store mutex.
- call `stored.session.Stop()` after removing the map entry.
- do not call `Stop()` while holding the store mutex.

### Handler Behavior

Update `handleCreateGame`:
- map `errSessionStoreFull` to `503 Service Unavailable`.
- if session creation succeeds but storing fails, stop the newly created session before returning.

Add `handleDeleteGame`:
- call the store removal method.
- return `404 Not Found` if missing.
- call `stored.session.Stop()` if found.
- return `204 No Content`.

Add method routing:
- `DELETE /games/{id}` uses `handleDeleteGame`.
- unsupported methods on `/games/{id}` should return `405 Method Not Allowed` where applicable.

### Testing

Store tests should cover:
- `remove(id)` deletes the map entry, so `get(id)` returns false.
- cleanup on add removes sessions whose `Done()` channel is closed.
- cleanup on add does not remove active sessions.
- add succeeds when completed cleanup frees capacity.
- add fails with `errSessionStoreFull` when `150` active sessions remain.

API tests should cover:
- `POST /games` returns `503 Service Unavailable` when capacity is full.
- `POST /games` returns `503 Service Unavailable` for the 151st active session when using the production default capacity.
- `DELETE /games/{id}` returns `204` and stops/removes an existing session.
- `DELETE /games/{id}` returns `404` for unknown IDs.
- requests for a deleted game ID return `404`.
- static files still serve successfully.

WebUI tests/manual verification should cover:
- Start calls `DELETE` for the previous `state.gameId` before creating a new game.
- Start still creates a new game if the delete returns `404`.
- Start replaces `state.gameId` only after the new create response succeeds.

### Acceptance

- Browser tabs are treated as independent game sessions.
- Pressing Start means a fresh board/session.
- The WebUI abandons its previous game before starting a new one.
- Completed sessions from non-cooperative clients are cleaned opportunistically on the next `POST /games`.
- New session creation is capped at `150` stored sessions after cleanup.
- The production `150` session cap is covered by an API test that creates 150 active sessions and verifies the next create request returns `503 Service Unavailable`.
- Removed sessions have their generated game IDs pruned from the store map.
- No external storage or persistence is added.
- No background cleanup goroutine is added.
- Run `gofmt` on changed Go files.
- Run `go test ./...`.
