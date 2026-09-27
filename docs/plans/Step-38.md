# Step 38 - Personal Stats (Player's Top Scores)

## Goal

Give a logged-in player a view of their own best performances, using the account linkage and the
per-player index that Steps 35-37 already put in place. The score storage already stamps
`player_id` on every authenticated submission and an `idx_leaderboard_scores_player` index already
exists for exactly this query; this step wires that plumbing to a read endpoint and a screen.

Implemented in this step:

- A new authenticated read endpoint returns the signed-in player's top 10 scores for a chosen board
  size and time limit, ranked by the same ordering as the public leaderboard.
- A "My Stats" entry point on the welcome screen that is visible only while a player is logged in.
- A stats screen that mirrors the leaderboard's board-size and time-limit filters and renders the
  player's own top 10 for the selected combination.

## Non-goals

- Do not add aggregate statistics (games played, averages, totals, member-since) or global rank.
  This step is a personal top-10 board scoped to one (grid size, duration) at a time, mirroring the
  existing leaderboard, not a new analytics surface.
- Do not change the public leaderboard endpoint, its response shape, ordering, or the leaderboard
  screen.
- Do not change score submission, the `leaderboard_scores` schema, or any index. The `player_id`
  column and `idx_leaderboard_scores_player` index already exist and are sufficient.
- Do not add pagination or a configurable limit to the personal stats endpoint. The limit is a
  fixed 10.
- Do not expose another player's scores. The endpoint reads the identity from the session cookie
  only; it accepts no player identifier in the request.
- Do not add a new response DTO. The existing `scoreResponse` shape is reused.
- Do not change authentication, session, or cookie behavior established in Steps 35-37.
- Do not rewrite unrelated gameplay, leaderboard, account, or frontend behavior.

## Current State

- `leaderboard.Store` exposes `SubmitScore` and `TopScores(ctx, TopScoresFilter)` only. `TopScores`
  filters on `grid_size` and `duration_seconds`, orders by
  `score DESC, remaining_millis DESC, submitted_at ASC, id ASC`, and caps its limit. There is no
  per-player read method.
- `leaderboard_scores` already has a nullable `player_id INTEGER REFERENCES players(id)` column,
  populated for authenticated submissions in `handleSubmitScore`
  (`internal/api/handlers.go`), and an `idx_leaderboard_scores_player` index keyed on
  `(player_id, grid_size, duration_seconds, score DESC, remaining_millis DESC, submitted_at ASC, id ASC)`.
- `ScoreEntry` carries `ID, GameID, PlayerName, Score, GridSize, DurationSeconds, RemainingMillis,
  SubmittedAt`.
- `Server.authenticatedPlayer(r)` (`internal/api/auth.go`) resolves the session cookie to a
  `player.Account` or returns `player.ErrSessionNotFound`. `writeCurrentPlayerError` already maps
  `ErrSessionNotFound` to `401 authentication required`; `handleCurrentPlayer` uses this pattern.
- `handleTopScores` (`internal/api/handlers.go`) already validates `gridSize` and `duration` query
  parameters via `game.ValidateBoardSize` and `game.ValidateDuration`, and builds `[]scoreResponse`
  with sequential `Rank`. This validation and mapping is the model for the new handler.
- Routes are registered in `Server.routes()` (`internal/api/server.go`). `GET /` is served by the
  static file server; there is currently only `POST /players`.
- The welcome menu (`static/index.html`) has `Play`, `Settings`, and `Leaderboard` buttons plus an
  account panel with a guest block (`#guestAccountActions`) and a signed-in block
  (`#signedInAccountActions`, toggled by `renderAccountState` on `state.player`).
- The leaderboard screen (`#leaderboardScreen`) uses `.chalk-pill-group` filter groups
  (`data-filter="board"` / `data-filter="timer"`), a `#leaderboardFilterSummary`, a
  `#leaderboardBody` table body, and a `#leaderboardEmpty` empty-state paragraph. `app.js` drives
  it with `state.leaderboard*` fields, `loadLeaderboardScores`, `renderLeaderboardScores`,
  `renderLeaderboardStatus`, `updateLeaderboardFilter`, and `fetchWithTimeout`.

## Design Decisions

### A per-player read method on the leaderboard store

Keep `TopScores(ctx, filter)` as the public wrapper and add
`PlayerTopScores(ctx, playerID, filter)` as the personal wrapper. The public wrapper preserves its
existing limit behavior: it honors `filter.Limit`, defaults to 15, and caps the value at
`MaxTopScoresLimit` (100). The personal wrapper overwrites `filter.Limit` with a fixed value of 10,
so callers cannot expand or shrink personal history through the endpoint or another store caller.

Both wrappers delegate query execution and row scanning to one unexported helper. Each wrapper uses
a separate static SQL query: the public query filters by grid size and duration, while the personal
query adds `player_id = ?`. Keeping the query strings separate avoids an optional `OR` predicate
that could prevent SQLite from using `idx_leaderboard_scores_player`, while the helper removes the
duplicated `QueryContext`, scan, timestamp parsing, and row-error handling.

```go
const playerTopScoresLimit = 10

func (s *Store) TopScores(ctx context.Context, filter TopScoresFilter) ([]ScoreEntry, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultTopScoresLimit
	}
	if limit > MaxTopScoresLimit {
		limit = MaxTopScoresLimit
	}

	return s.queryTopScores(
		ctx,
		publicTopScoresQuery,
		filter.GridSize,
		filter.DurationSeconds,
		limit,
	)
}

func (s *Store) PlayerTopScores(ctx context.Context, playerID int64, filter TopScoresFilter) ([]ScoreEntry, error) {
	filter.Limit = playerTopScoresLimit
	return s.queryTopScores(
		ctx,
		playerTopScoresQuery,
		playerID,
		filter.GridSize,
		filter.DurationSeconds,
		filter.Limit,
	)
}
```

`publicTopScoresQuery` and `playerTopScoresQuery` are package-private constants containing the two
static statements. `queryTopScores(ctx, query, args...)` accepts only those internally selected
queries and owns the shared row-reading implementation. It is not an exported general-purpose query
builder.

### Identity comes from the session, never the request

The endpoint takes `gridSize` and `duration` query parameters only. The player is resolved from the
session cookie via `authenticatedPlayer`. There is no `playerId` parameter, so one player cannot
read another's scores, and the handler returns `401` when there is no valid session. This mirrors
`handleCurrentPlayer` and keeps the authorization boundary identical to the rest of the account
surface.

### Reuse the existing response DTO

The handler returns `[]scoreResponse`, the same array shape the leaderboard returns. Each entry's
`PlayerName` is the requester's own display name; it is harmless and keeps the DTO surface
unchanged. No new request or response struct is added.

### A gated welcome-menu entry point

Add a `My Stats` button to the welcome menu next to `Leaderboard`, hidden by default and shown only
when `state.player` is set. Visibility is toggled in `renderAccountState`, alongside the existing
guest/signed-in panel toggles, so it follows login and logout state automatically.

The button lives in the main menu rather than inside the account panel so it reads as a primary
navigation destination like `Leaderboard`, and so the stats screen can reuse the leaderboard's
screen and filter layout wholesale.

### A stats screen mirroring the leaderboard

The stats screen (`#statsScreen`) reuses the leaderboard's structure: board and time filter pill
groups, a filter summary line, a table, and an empty state. Its table shows `#`, `Score`, `Left`,
and `Date`:

- `#` rather than `Rank`, because this is the player's personal ordering, not a global rank.
- `Player` is dropped because every row is the same player.
- `Date` renders `submittedAt` as the browser's local short date and time (for example
  `9/15/26, 2:31 PM`, via `Date`/`toLocaleString`); an absent or unparseable timestamp renders as
  `—`. Cells are always populated with `textContent`, never `innerHTML`.

The frontend fetches `/players/me/stats?gridSize=&duration=` with `credentials: "same-origin"`,
using the same `fetchWithTimeout` pattern as `loadLeaderboardScores`. On every filter change the
previous rows are replaced with a `Loading scores...` status immediately (as the leaderboard already
does), so scores for the old board/duration never linger under a pending request.

Async response ordering follows the existing leaderboard pattern exactly. Each load captures
`const requestId = ++state.statsRequestId`; a response or error may update the stats UI only when
that captured id still equals `state.statsRequestId`. The most recently initiated request therefore
wins, and older responses are discarded even if they finish later. This is the only additional
client-side async guard: the stats flow does not add `authStateVersion` checks, identity-change
reset hooks, a stats-specific `/auth/me` call, cross-tab synchronization, or request cancellation.

A `401` from the most recent stats request means the session has expired or been revoked. The
frontend always logs the player out on the page by calling `clearAuthenticatedPlayer()`, the way the
logout and score-identity flows already do. If the stats screen is still visible, it then calls
`showScreen("welcome")` and `showAccountView("login")` to open the login popup. No session-expired
message is shown. The order matters: `showAccountView` only opens the login view once
`state.player` is cleared, and `showScreen` closes any open account view when switching screens.

Screen navigation is limited to the stats screen because a slow response can arrive after the player
has moved on. A player who has left My Stats, for example to start a game, stays where they are and
simply becomes a guest; an in-progress game is never interrupted. Only a player-specific page such
as My Stats sends the player back to the welcome screen.

This is a reaction to the server's response, not an additional async guard; a stale `401` is
discarded by the request-id check like any other stale response. Every other failure shows
`Could not load scores.`, as on the leaderboard.

`app.js` gains a parallel `state.stats*` block and `loadStatsScores` / `renderStatsScores` /
`renderStatsStatus` / `updateStatsFilter` functions modeled on the leaderboard equivalents. The
leaderboard functions are not generalized to serve both screens; the two are kept independent to
avoid entangling public and personal views, consistent with the backend decision above.

## API Design

### New route

`GET /players/me/stats?gridSize={n}&duration={s}`

- Auth: requires a valid `find_ten_session` cookie.
- Query parameters: `gridSize` and `duration`, both required, validated by `game.ValidateBoardSize`
  and `game.ValidateDuration` exactly as `GET /scores` validates them.
- `200 OK`: JSON array of up to 10 `scoreResponse` objects, ranked `1..10` in the same order as the
  leaderboard, containing only the authenticated player's own scores for that grid size and
  duration. An empty array when the player has no scores for that combination. The response sets
  `Cache-Control: no-store`: the URL is identical for every player, so no browser or shared cache
  may keep a per-player response.
- `400 Bad Request`: missing or non-integer `gridSize`/`duration`, or a value failing board-size or
  duration validation. Same messages as `GET /scores`.
- `401 Unauthorized` (`authentication required`): no session cookie, or an unknown/expired session.
- `408 Request Timeout`: context canceled or deadline exceeded, during either the session lookup or
  the score query.
- `500 Internal Server Error` (`failed to look up current player`): unexpected storage error during
  the session lookup, via `writeCurrentPlayerError`.
- `500 Internal Server Error` (`failed to query scores`): unexpected storage error during the score
  query.

The handler authenticates before validating query parameters. A request without a valid session
therefore returns `401` even when `gridSize` or `duration` is missing or invalid.

Go's `ServeMux` also routes `HEAD` to a `GET` pattern. `HEAD` follows the same authentication and
filter validation as `GET`, with the corresponding status and headers but no response body on the
wire. The HTTP server suppresses the body, so the handler needs no special `HEAD` branch. Other
methods receive Go's automatic `405` (plain-text body, `Allow: GET, HEAD`), the same as
`GET /auth/me`. No explicit `handleMethodNotAllowed` routes are registered: the rejection happens
in the mux before `handlePlayerStats` runs, the security headers and cross-origin protection in
`Server.ServeHTTP` still apply, and the mux rejects every other method rather than only an
enumerated list.

### Handler

```go
func (s *Server) handlePlayerStats(w http.ResponseWriter, r *http.Request) {
	account, err := s.authenticatedPlayer(r)
	if err != nil {
		writeCurrentPlayerError(w, err) // maps ErrSessionNotFound -> 401
		return
	}

	gridSize, duration, ok := parseScoreFilterParams(w, r)
	if !ok {
		return
	}

	entries, err := s.leaderboard.PlayerTopScores(r.Context(), account.ID, leaderboard.TopScoresFilter{
		GridSize:        gridSize,
		DurationSeconds: duration,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			writeError(w, http.StatusRequestTimeout, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to query scores")
		return
	}

	scores := make([]scoreResponse, len(entries))
	for i, entry := range entries {
		scores[i] = scoreResponse{
			Rank:            i + 1,
			PlayerName:      entry.PlayerName,
			Score:           entry.Score,
			GridSize:        entry.GridSize,
			DurationSeconds: entry.DurationSeconds,
			RemainingMillis: entry.RemainingMillis,
			SubmittedAt:     entry.SubmittedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(scores)
}
```

The `gridSize`/`duration` parsing and validation currently inline in `handleTopScores` is extracted
into a small shared helper `parseScoreFilterParams(w, r) (gridSize, duration int, ok bool)` that
writes the appropriate `400` on failure, and both `handleTopScores` and `handlePlayerStats` call
it. This removes duplication of the identical validation block rather than copying it. (If preferred
during review, the block can instead be duplicated to keep `handleTopScores` untouched; the plan's
default is the shared helper.)

## Files To Modify

- `docs/plans/Step-38.md` (new)
- `docs/GOAL.md` — bring limited personal score history into the planned product scope while
  leaving broader statistics and paginated history for future work.
- `docs/ARCHITECTURE.md` — record the personal-stats read path, its session-only authorization, and
  the reuse of the existing per-player index.
- `internal/leaderboard/store.go` — add the fixed `playerTopScoresLimit`, the personal wrapper,
  separate static public/personal queries, and the shared query-and-scan helper.
- `internal/leaderboard/store_test.go` — tests for `PlayerTopScores`.
- `internal/api/handlers.go` — add `handlePlayerStats`; extract `parseScoreFilterParams` and use it
  in `handleTopScores`.
- `internal/api/server.go` — register `GET /players/me/stats`.
- `internal/api/handlers_test.go` (or a new `internal/api/player_stats_test.go`) — endpoint tests.
- `static/index.html` — add the gated `My Stats` menu button and the `#statsScreen` section.
- `static/app.js` — menu wiring, `renderAccountState` visibility toggle, `showScreen("stats")`, the
  `state.stats*` block, and `loadStatsScores` / `renderStatsScores` / `renderStatsStatus` /
  `updateStatsFilter`, including the same latest-request-id response guard used by the leaderboard.
- `static/styles.css` — only if the stats screen needs styles the leaderboard classes do not
  already provide; prefer reusing existing `.leaderboard-*` / `.chalk-*` classes.

## Implementation Sequence

1. Add this Step 38 plan.
2. Update `docs/GOAL.md` to bring limited personal score history into the planned product scope.
3. Refactor `TopScores` into the public wrapper and shared query-and-scan helper, then add the
   `PlayerTopScores` wrapper with its fixed 10-row limit.
4. Add store tests covering per-player filtering, ordering, the 10-row cap, caller-supplied limit
   overrides, and isolation between players.
5. Extract `parseScoreFilterParams` in `internal/api/handlers.go`, refactor `handleTopScores` to
   use it, and add `handlePlayerStats`.
6. Register `GET /players/me/stats` in `internal/api/server.go`.
7. Add API tests for the endpoint (auth required, filtering, ranking, validation, guest rejection).
8. Add the gated `My Stats` button and `#statsScreen` markup to `static/index.html`, reusing
   leaderboard classes.
9. Wire `static/app.js`: the button and `renderAccountState` visibility, the `showScreen("stats")`
   path, and the `state.stats*` load/render/filter functions. Use `statsRequestId` exactly like the
   leaderboard so only the most recently initiated request can update the UI.
10. Add any minimal CSS only if required.
11. Update `docs/ARCHITECTURE.md`.
12. Run `gofmt` on changed Go files.
13. Run `go test ./...`.

## Tests

### Store (`internal/leaderboard/store_test.go`)

- `PlayerTopScores` returns only the given player's rows for the given grid size and duration,
  excluding other players' rows and rows for other grid/duration combinations.
- Results are ordered `score DESC, remaining_millis DESC, submitted_at ASC, id ASC`.
- Exactly the top 10 rows are returned when the player has more than 10 qualifying scores.
- Caller-provided `TopScoresFilter.Limit` values below or above 10 are ignored by
  `PlayerTopScores`; the result remains capped at 10.
- A non-nil empty slice and a nil error are returned when the player has no qualifying scores.
- Rows with `player_id IS NULL` (guest submissions) are never returned.

### API (`internal/api/player_stats_test.go`)

- An authenticated request returns the player's own top 10 for the selected grid size and duration,
  ranked `1..10`, and excludes another authenticated player's scores.
- A request without a session cookie returns `401 authentication required`.
- A request with an unknown/expired session cookie returns `401`.
- An authenticated request with missing or invalid `gridSize`/`duration` returns `400` with the
  same messages as `GET /scores`; an unauthenticated request returns `401` before filter validation.
- A player with no scores for the combination returns `200` with an empty array.
- Authenticated `GET` and `HEAD` requests with valid filters return `200`; `HEAD` has no response
  body over HTTP. Test the empty body through `httptest.NewServer`, since a direct
  `httptest.ResponseRecorder` records handler writes even for `HEAD`. `POST`, `PUT`, `PATCH`, and
  `DELETE` return `405`.
- Unauthenticated `HEAD` returns `401`, and authenticated `HEAD` with invalid filters returns `400`;
  neither response has a body over HTTP.

### Regression

- `handleTopScores` behavior is unchanged after the `parseScoreFilterParams` extraction; existing
  leaderboard endpoint tests pass unmodified.
- All existing account, session, score, and gameplay tests pass with `go test ./...`.

### Manual verification

- Logged out: no `My Stats` button is visible in the welcome menu.
- After login: the `My Stats` button appears; opening it shows the player's top 10 for the selected
  board/time, and changing the board or time pills refreshes the list.
- Changing a filter immediately replaces the old rows with `Loading scores...`; scores for the
  previous board/duration never linger under the pending request.
- Rapidly changing filters renders only the result for the most recently initiated request, even
  when an older request finishes afterward.
- The `Date` column shows a local short date/time; a row with no valid timestamp shows `—`.
- A brand-new account shows the empty state until it submits a score.
- With an expired or revoked session, opening My Stats logs the player out on the page and lands on
  the welcome screen with the login popup open; no session-expired message is shown.
- If the player leaves My Stats and starts a game before a slow `401` arrives (for example with
  network throttling), they are logged out on the page but stay in the running game.

## Acceptance Criteria

- A logged-in player can retrieve their own top 10 scores for a chosen board size and time limit via
  `GET /players/me/stats`, ranked identically to the public leaderboard.
- Successful personal stats responses carry `Cache-Control: no-store`.
- The endpoint derives identity solely from the session cookie, rejects unauthenticated requests
  with `401`, and never exposes another player's scores.
- The route accepts `GET` and implicit `HEAD`; `HEAD` follows the same authentication and filter
  validation and sends no body over HTTP.
- The endpoint reuses the existing `player_id` column, the `idx_leaderboard_scores_player` index,
  the `TopScoresFilter` type, and the `scoreResponse` DTO; no schema, index, or DTO changes are
  made.
- The `My Stats` entry point is present only while logged in and disappears on logout.
- The stats screen reuses the leaderboard's filter layout and shows the player's own top 10 for the
  selected combination, with an empty state when there are none, `#` for personal ordering, and a
  local-format `Date` column (`—` when a timestamp is missing or unparseable).
- Only the most recently initiated stats request may update the stats UI; older responses and errors
  are discarded using the same request-id pattern as the public leaderboard.
- The public leaderboard endpoint and screen are unchanged.
- `gofmt` is clean on changed Go files and `go test ./...` passes.

## Open Questions

1. Resolved: table columns are `# / Score / Left / Date`. `#` is the player's personal ordering,
   while `Player` is dropped because every row is the current user.
2. Resolved: extract the shared `parseScoreFilterParams` helper and use it from both
   `handleTopScores` and `handlePlayerStats`. Do not duplicate the validation block.
3. Resolved: the `My Stats` entry point is a full-width chalk button in the main welcome menu,
   directly below `Leaderboard`, visible only while logged in.
4. Resolved: personal history is fixed at 10 rows. `TopScores` and `PlayerTopScores` are public and
   personal wrappers over shared query-and-scan plumbing; only the public wrapper honors a caller's
   `TopScoresFilter.Limit`.
5. Resolved: stats async handling mirrors the public leaderboard. A monotonically increasing
   `statsRequestId` allows only the most recently initiated request to update the UI; no additional
   auth-version, identity-reset, revalidation, cross-tab, or cancellation mechanism is added. A `401`
   from the latest request always clears the authenticated player, and opens the login popup on the
   welcome screen only if the stats screen is still visible, so an in-progress game is never
   interrupted; this reacts to the server's response and is not an additional async guard.
6. Resolved: `GET` and implicit `HEAD` follow the same authenticated route; `HEAD` has no body on
   the wire, and unauthenticated requests receive `401` before filter validation.

## Review Findings

### Codex review: personal stats response had no cache policy

- **Finding**: `GET /players/me/stats` returned a cookie-dependent, per-player response under a URL
  shared by every player, with no `Cache-Control` header. A browser or shared cache could keep one
  player's list and serve it later.
- **Assessment**: valid as defense-in-depth. Practical exposure was low, since the response has no
  validators or freshness information and nginx caching is not enabled by default.
- **Fix**: `handlePlayerStats` sets `Cache-Control: no-store` on the `200` response.
  `TestPlayerStatsReturnsOwnTopScores` asserts the header.
- **Related pre-existing gap**: `GET /auth/me` also returned account data (display name and account
  handle) without a cache policy. It predates this step and was outside the plan; see the post-work
  update below.

### Codex second review: no issues; three test suggestions

- **Added**: `TestPlayerTopScoresUsesIDAsFinalTieBreaker` covers the final `id ASC` tie-breaker for
  the personal query when score, remaining time, and submission time are identical.
- **Added**: `TestPlayerStatsHEADFollowsGETWithoutBody` asserts `Cache-Control: no-store` on the
  successful `HEAD` response.
- **Deferred**: browser tests for button visibility, rapid filter changes, stale responses, and the
  `401`-after-navigation behavior. The repository has no JavaScript test tooling, and adding it is
  outside this step; the manual verification list covers these behaviors.

## Post-Work Update

### `GET /auth/me` cache policy

Found during the first Codex review and fixed after Step 38's planned work was complete, as a small
separate change included with this step's commit:

- `handleCurrentPlayer` sets `Cache-Control: no-store` on its `200` response, matching
  `GET /players/me/stats`. Its URL is also identical for every player, so no browser or shared cache
  may keep the per-player response.
- `TestCurrentPlayer` asserts the header.
