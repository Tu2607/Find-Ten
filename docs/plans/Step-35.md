# Step 35 - Persistent Player Accounts

## Goal

Add the first persistent player-account system so players can create an account, log in with a password, keep a 7-day browser session, and have future server-owned data attached to a stable player identity.

This step is the account foundation for later achievements, unlockables, score history, and progression. Achievements and unlockables are intentionally not implemented here.

## Non-goals

- Do not add achievements, unlockables, progression rewards, or player inventory.
- Do not add personal score-history UI, top-5-per-player pruning, or rank lookup.
- Do not replace the existing global leaderboard query behavior.
- Do not require login to play the game.
- Do not require login to submit a score.
- Do not let display names be used for login.
- Do not automatically log a player in after registration.
- Do not add email, password reset, MFA, account recovery, admin tools, or profile editing.
- Do not add MySQL or split player data into a second database system.
- Do not add rate limiting, bot protection, CSRF tokens, or advanced abuse prevention in this step.
- Do not rewrite unrelated gameplay, session, or leaderboard behavior.

## Current State

- The server uses one SQLite database path, currently `./data/find-ten.db` by default.
- `internal/leaderboard` owns the current SQLite open/init path and the `leaderboard_scores` table.
- `POST /scores` supports guest-style submission with a `gameId` and `playerName`.
- `GET /scores` returns top persisted scores for a required board size and duration.
- The WebUI score-submission flow asks for a player name manually.
- There is no persisted player account table.
- There is no login, logout, session cookie, or authenticated request context.
- `docs/GOAL.md` still lists persisted accounts and scores as out of scope even though persisted scores were added in Steps 32-34. Step 35 should reconcile that documentation drift.

## Design Decisions

### One SQLite database

Keep player accounts, sessions, and leaderboard scores in the same SQLite database file.

Reasons:

- The project already has durable SQLite persistence and Docker volume support.
- Account-linked score queries will be simpler when players and scores live in one database.
- Introducing MySQL only for users would create cross-database lookup and migration complexity before the app needs it.
- A later migration to MySQL or PostgreSQL should move the full persistence layer together, not split accounts from scores.

### Display name versus login handle

Players have two user-facing identity fields:

- `display_name`: shown in the UI and leaderboard. It is not unique and cannot be used to log in.
- `account_handle`: unique login identifier. This is returned after registration and is required for login.

Handle generation:

1. Trim and validate the requested display name.
2. Try `account_handle = display_name`.
3. If that exact handle is available, use it with no suffix.
4. If it is taken, generate a suffixed handle such as `Amanda#482193`.
5. Retry generated suffixes until a unique handle is inserted or a small retry cap is reached.

Handle uniqueness must be enforced by the database with a unique constraint. Application-side availability checks are optional convenience only and must not be trusted as the final authority.

Display names and account handles are case-sensitive for this step. The server must show the final account handle clearly after registration because registration does not log the player in.

### Password storage

Passwords are hashed, not encrypted.

Use `golang.org/x/crypto/bcrypt` for Step 35:

- Store only the bcrypt password hash string.
- Use `bcrypt.GenerateFromPassword`.
- Use `bcrypt.CompareHashAndPassword`.
- Use `bcrypt.DefaultCost` unless implementation testing shows it is too slow on the deployment target.
- Enforce bcrypt's input limit by rejecting passwords longer than 72 bytes.
- Enforce a simple minimum password length, recommended `12` characters.
- Require new account passwords to include at least one ASCII non-alphanumeric, non-whitespace character.
- Require new account passwords to include at least one ASCII uppercase letter.

Bcrypt hashes include the algorithm marker, work factor, salt, and hash output in one stored string. The application should not add a separate password-salt column for bcrypt.

Do not use raw SHA-512, SHA-256, or another fast general-purpose hash for password storage. Fast hashes are appropriate for random session-token lookup but not for human passwords.

### Session storage

Login creates a persistent 7-day browser session.

Use an opaque random session token:

- Generate at least 32 random bytes with `crypto/rand`.
- Send the raw token only to the browser as a cookie.
- Store only a server-side hash of the token in SQLite.
- SHA-256 is acceptable for hashing session tokens because the token is random high-entropy data, not a human password.

Cookie settings:

- Cookie name: `find_ten_session`.
- `HttpOnly: true`.
- `SameSite: Lax`.
- `Path: /`.
- `MaxAge: 604800`.
- `Expires: now + 7 days`.
- `Secure: true` when the request is served over HTTPS. Local HTTP development can leave `Secure` false.

Expired sessions should not authenticate a request. Lookup may delete expired session rows opportunistically.

### Leaderboard linkage

Guest score submission remains supported exactly as it works today.

Logged-in score submission changes only the identity source:

- If the request has a valid player session, the server derives `player_id` and `playerName` from the authenticated player.
- The frontend may prefill the score name field, but the backend must not trust the browser-sent name for authenticated submissions.
- If the request is unauthenticated, `playerName` remains required and is validated as a guest display name.

Persisted score rows should keep the existing `player_name` snapshot for leaderboard rendering. Logged-in scores should also store nullable `player_id` for future account-owned score history, achievements, and unlockables.

Top-score responses do not need to expose `player_id` or `account_handle` in this step.

## Proposed Storage

Use the same SQLite database path as the leaderboard.

The implementation should keep repositories separated by domain while sharing one database connection in production:

- `internal/leaderboard` remains the score repository.
- Add a new account/session repository, likely `internal/player`.
- Extract common SQLite open/DSN/WAL setup into a small shared helper if needed so production does not open and manage unrelated database connections for the same file.
- Initialize account tables before leaderboard schema changes that reference `players`.

### `players`

```sql
CREATE TABLE IF NOT EXISTS players (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    display_name TEXT NOT NULL,
    account_handle TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TEXT NOT NULL CHECK (
        created_at GLOB '????-??-??T*Z'
        AND datetime(created_at) IS NOT NULL
    )
);
```

Repository-level validation should enforce:

- display name after trimming is `3-10` characters.
- display name uses the same safe display character rules as current leaderboard player names unless a narrower account rule is intentionally chosen during implementation.
- password is at least `12` characters.
- password is at most `72` bytes for bcrypt.
- new account passwords include at least one ASCII non-alphanumeric, non-whitespace character.
- new account passwords include at least one ASCII uppercase letter.
- account handle is generated by the server, not supplied by the client.

### `player_sessions`

```sql
CREATE TABLE IF NOT EXISTS player_sessions (
    token_hash TEXT PRIMARY KEY,
    player_id INTEGER NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL CHECK (
        created_at GLOB '????-??-??T*Z'
        AND datetime(created_at) IS NOT NULL
    ),
    expires_at TEXT NOT NULL CHECK (
        expires_at GLOB '????-??-??T*Z'
        AND datetime(expires_at) IS NOT NULL
    )
);
```

Indexes:

```sql
CREATE INDEX IF NOT EXISTS idx_player_sessions_player_id
ON player_sessions (player_id);

CREATE INDEX IF NOT EXISTS idx_player_sessions_expires_at
ON player_sessions (expires_at);
```

### `leaderboard_scores`

Add nullable account linkage:

```sql
ALTER TABLE leaderboard_scores
ADD COLUMN player_id INTEGER REFERENCES players(id);
```

The migration must be idempotent. Check existing columns before running `ALTER TABLE`.

Add an index for future player score-history queries:

```sql
CREATE INDEX IF NOT EXISTS idx_leaderboard_scores_player
ON leaderboard_scores (
    player_id,
    grid_size,
    duration_seconds,
    score DESC,
    remaining_millis DESC,
    submitted_at ASC,
    id ASC
);
```

## API Design

### `POST /players`

Registers a new player account.

Request:

```json
{
  "displayName": "Amanda",
  "password": "example-password"
}
```

Response on success:

```json
{
  "created": true,
  "player": {
    "displayName": "Amanda",
    "accountHandle": "Amanda#482193"
  }
}
```

Registration must not set a session cookie and must not log the player in. The UI should show a success message and send the player to the login flow.

Status mapping:

- `201 Created`: account created.
- `400 Bad Request`: invalid JSON, invalid display name, invalid password.
- `408 Request Timeout`: request context canceled or deadline exceeded.
- `500 Internal Server Error`: unexpected storage or hashing error.
- `503 Service Unavailable`: handle generation exhausted its retry cap.

### `POST /auth/login`

Logs in with account handle and password.

Request:

```json
{
  "accountHandle": "Amanda#482193",
  "password": "example-password"
}
```

Response on success:

```json
{
  "authenticated": true,
  "player": {
    "displayName": "Amanda",
    "accountHandle": "Amanda#482193"
  }
}
```

The handler sets the `find_ten_session` cookie on success.

Status mapping:

- `200 OK`: login accepted.
- `400 Bad Request`: invalid JSON or missing fields.
- `401 Unauthorized`: unknown handle or wrong password.
- `408 Request Timeout`: request context canceled or deadline exceeded.
- `500 Internal Server Error`: unexpected storage, hashing, or random-token error.

### `POST /auth/logout`

Logs out the current browser session.

Behavior:

- If a valid session cookie exists, delete that session row.
- Clear the browser cookie.
- Return success even when the cookie is already missing or invalid.

Recommended response:

- `204 No Content`.

### `GET /auth/me`

Returns the authenticated player for the current cookie.

Response on authenticated request:

```json
{
  "authenticated": true,
  "player": {
    "displayName": "Amanda",
    "accountHandle": "Amanda#482193"
  }
}
```

Recommended unauthenticated response:

- `401 Unauthorized`.

The frontend should treat `401` as normal guest state.

### `POST /scores`

Preserve the existing endpoint shape for guests.

Authenticated behavior:

1. Resolve the optional session cookie.
2. If valid, derive `player_id` and `playerName` from the authenticated player.
3. Ignore any browser-sent `playerName` for authenticated submissions.
4. Insert the leaderboard score with nullable `player_id`.

Guest behavior:

1. If no valid session exists, require `playerName` as Step 34 does today.
2. Insert the leaderboard score with `player_id = NULL`.

Existing score status mapping should remain unchanged unless an account lookup introduces a request-timeout or internal-server-error path.

## Frontend Design

Add a small account flow without changing the core game layout:

- Welcome screen shows login/register entry points when guest.
- Logged-in state shows display name and a logout control.
- Registration form asks for display name and password.
- Registration success clearly shows the generated account handle and directs the player to login.
- Login form asks for account handle and password.
- On page load, call `GET /auth/me` to restore login state from the cookie.
- Logout calls `POST /auth/logout`, clears local account state, and returns the UI to guest mode.

Score submission UI:

- Guest flow remains the current manual player-name input.
- Logged-in flow pre-fills the name with `displayName`.
- Logged-in score name should be read-only or otherwise clearly account-derived.
- If the session expires before submission, the backend should fall back to guest validation; the frontend can show the normal score-name requirement after `GET /auth/me` or a failed submit.

## Files To Modify

- `docs/plans/Step-35.md`
- `docs/GOAL.md`
- `docs/ARCHITECTURE.md`
- `go.mod`
- `go.sum`
- `cmd/server/main.go`
- `internal/api/dto.go`
- `internal/api/handlers.go`
- `internal/api/server.go`
- `internal/api/server_test.go`
- `internal/leaderboard/store.go`
- `internal/leaderboard/store_test.go`
- `internal/leaderboard/validation.go`
- new account/session persistence package, likely `internal/player`
- optional shared SQLite helper package if needed for one production DB connection
- `static/index.html`
- `static/app.js`
- `static/styles.css`

## Implementation Sequence

1. Add this Step 35 plan.
2. Update `docs/GOAL.md` to reflect that persisted scores are already implemented and persistent accounts are now planned/current work rather than permanently out of scope.
3. Update `docs/ARCHITECTURE.md` with player account persistence, password hashing, session cookies, and authenticated score-submission rules.
4. Add `golang.org/x/crypto/bcrypt`.
5. Introduce or extract shared SQLite open/connection plumbing if needed so leaderboard and player repositories use the same database file cleanly.
6. Add player and session schema initialization.
7. Add idempotent migration for nullable `leaderboard_scores.player_id`.
8. Add player account repository methods:
   - create account with generated unique handle
   - find account by handle for login
   - validate display name and password input
9. Add password hashing and verification helpers using bcrypt.
10. Add session repository methods:
    - create session
    - look up session by token hash
    - delete current session
    - delete expired sessions opportunistically
11. Wire the API server with both leaderboard and player/session persistence dependencies.
12. Add auth DTOs, cookie helpers, and authenticated-player request lookup helper.
13. Add `POST /players`, `POST /auth/login`, `POST /auth/logout`, and `GET /auth/me`.
14. Update `POST /scores` to derive score identity from the authenticated player when a valid session cookie is present.
15. Update frontend account state, registration/login/logout UI, and `GET /auth/me` startup restore.
16. Update score submission UI to use the account display name automatically for logged-in players while preserving guest submission.
17. Add focused backend tests for storage, auth handlers, cookie behavior, and authenticated score submission.
18. Add or update frontend smoke-test notes for registration, login, logout, reload persistence, guest score submission, and logged-in score submission.
19. Run `gofmt` on changed Go files.
20. Run `go test ./...`.
21. Manually smoke test the browser flow.

## Tests

### Player repository

- Creating an account stores a bcrypt hash, not plaintext.
- Two accounts with the same password have different stored password hashes.
- Password verification accepts the correct password.
- Password verification rejects the wrong password.
- Display name validation rejects empty, too-short, too-long, and unsafe names.
- New-account password validation rejects empty, too-short, over-72-byte passwords, and passwords without an ASCII special or uppercase character. Login remains compatible with existing bcrypt-safe passwords.
- First account for a display name gets an unsuffixed matching handle.
- Later accounts for the same display name get unique suffixed handles.
- Account-handle uniqueness is enforced by the database.
- Handle-generation retry exhaustion returns a controlled error.

### Session repository

- Login session creation stores only a token hash.
- Looking up a valid token returns the player.
- Looking up a missing, malformed, or expired token does not authenticate.
- Logout deletes only the current session.
- Expired sessions can be removed opportunistically.

### Auth API

- `POST /players` creates an account and does not set a session cookie.
- `POST /players` returns the generated account handle.
- `POST /players` rejects invalid JSON, invalid display name, and invalid password.
- `POST /auth/login` accepts valid handle/password and sets a 7-day cookie.
- `POST /auth/login` rejects wrong password and unknown handle with `401`.
- `GET /auth/me` returns player data for a valid cookie.
- `GET /auth/me` returns unauthenticated status for missing, invalid, or expired cookies.
- `POST /auth/logout` clears the cookie and deletes the session when present.

### Score API

- Guest score submission still requires and stores request `playerName`.
- Logged-in score submission ignores request `playerName` and stores the account display name.
- Logged-in score submission stores `player_id`.
- Guest score submission stores `player_id = NULL`.
- `GET /scores` response shape remains compatible with Step 34.

### Frontend manual smoke tests

- Register account shows success and generated login handle.
- Registration does not log in automatically.
- Login with returned handle persists across page reload for 7 days.
- Logout returns the UI to guest state.
- Guest can still play and submit a score with a manually entered name.
- Logged-in player sees their display name prefilled for score submission.
- Logged-in score submission succeeds and appears on the leaderboard with the display name.

## Acceptance Criteria

- Players can register with display name and password.
- Registration returns a unique account handle and does not create a login session.
- The first account for a display name uses the display name as the handle when available.
- Later handle collisions receive a generated unique suffix.
- Players can log in with account handle and password.
- Login creates a 7-day cookie-backed session.
- Passwords are stored only as bcrypt hashes with library-managed salts.
- Session tokens are random, stored server-side only as hashes, and cleared on logout.
- `GET /auth/me` lets the frontend restore logged-in state after refresh.
- Guests can still submit scores exactly as before.
- Logged-in score submission uses the account display name automatically.
- Logged-in score rows store nullable `player_id` for future account-linked features.
- Global leaderboard rendering remains compatible with existing top-score behavior.
- Backend tests pass with `go test ./...`.
