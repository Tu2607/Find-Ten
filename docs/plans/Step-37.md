# Step 37 - Closing Out the Security Audit

## Goal

Close the remaining application-layer findings from the security audit of the Step 35 account
work, and record an explicit decision for every finding that is not being implemented, so the
audit is fully accounted for rather than partially forgotten.

The deployment now serves HTTPS at `https://find-ten.tuvu.dedyn.io` with nginx terminating TLS.
The session-cookie finding is the last blocker before the Step 35 account build can be deployed.

Implemented in this step:

- Session cookies are issued without the `Secure` attribute behind a TLS-terminating proxy.
- The HTTP server has no read timeouts, idle timeout, or `MaxHeaderBytes` cap.
- Expired player sessions are never deleted.
- Write endpoints accept unsafe cross-origin browser requests.
- The SSE snapshot stream does not defend itself against proxy response buffering.
- The container publishes port `8080` on all host interfaces, so the app's privacy depends on an
  Oracle VCN security rule rather than on the deployment itself.

## Non-goals

- Do not read `X-Forwarded-Proto`, `X-Forwarded-For`, or any other forwarding header. Step 36
  established that the Go application does not infer deployment topology from request headers, and
  a header the application cannot verify is not a sound basis for a security decision.
- Do not add CSRF tokens. Go 1.26's `http.CrossOriginProtection` provides token-free CSRF defense
  through `Sec-Fetch-Site` and an `Origin` fallback; tokens would add plumbing and storage for no
  additional protection in this single-origin application.
- Do not enforce request `Content-Type`. With `CrossOriginProtection` in place, the check becomes
  API hygiene rather than security, and its cost (adding `Content-Type` to 32 existing test
  request fixtures across three files) is not justified when the security control it would provide
  is already covered.
- Do not add a per-client game-session cap, rate limiting, or any other control that requires
  identifying the client. See Accepted Risks.
- Do not change nginx configuration in this step. The container port binding is a deployment
  concern owned by `docker-compose.yaml`; edge configuration is owned separately at the VM.
- Do not change the leaderboard response shape or add verified-account indicators. See Accepted
  Risks.
- Do not change account handle generation. See Accepted Risks.
- Do not set `WriteTimeout` on the HTTP server. See Design Decisions.
- Do not add HTTP Strict Transport Security. HSTS belongs at the HTTPS edge alongside the redirect
  nginx already performs.
- Do not make the session cookie's `Secure` attribute configurable. Authenticated sessions always
  require a secure cookie; local development does not weaken that invariant.
- Do not change `SameSite`, `HttpOnly`, `Path`, or session lifetime.
- Do not update the Docker builder/runtime images or perform broader container hardening. The
  planned Go 1.26.6 image update and remaining deployment hardening follow Step 37.
- Do not rewrite unrelated gameplay, session, leaderboard, or account behavior.

## Current State

- `setSessionCookie` and `clearSessionCookie` in `internal/api/auth.go` are package-level functions
  setting `Secure: r.TLS != nil`. Because nginx terminates TLS and proxies cleartext to
  `127.0.0.1:8080`, `r.TLS` is always `nil` and the attribute is never set, including on the live
  HTTPS site.
- `NewServer(leaderboardStore, playerStore) http.Handler` takes no configuration. It has exactly
  two call sites: `cmd/server/main.go` and the `newTestServerWithDatabase` helper in
  `internal/api/server_test.go`.
- Two existing tests cover cookie security: a plain HTTP handler test currently asserts `Secure` is
  `false`, while `TestLoginUsesSecureCookieForHTTPS` sets `request.TLS` and asserts `true`. The
  plain-HTTP assertion intentionally changes in this step because the cookie becomes unconditionally
  secure; unrelated response assertions remain unchanged.
- `listenAndServe` in `cmd/server/main.go` constructs `&http.Server{Addr, Handler}` with no
  timeouts of any kind.
- `Store.DeleteExpiredSessions` exists in `internal/player/session.go` and has no non-test caller.
  Expired rows accumulate indefinitely. They do not authenticate requests, because session lookup
  filters on `expires_at`.
- An `idx_player_sessions_expires_at` index already exists, so deletion by expiry is cheap.
- `decodeJSONBody` in `internal/api/errors.go` bounds every request body to 8 KiB. No layer
  currently rejects unsafe cross-origin browser requests.
- Go 1.26 provides `http.CrossOriginProtection` in the standard library. For unsafe methods, it
  accepts `Sec-Fetch-Site: same-origin` and `none`, rejects `same-site` and `cross-site`, and falls
  back to validating `Origin` when `Sec-Fetch-Site` is absent. Requests carrying neither header are
  allowed for compatibility with non-browser clients. No trusted origins are needed for this
  single-domain deployment.
- `handleGameSnapshots` sets `Content-Type: text/event-stream` and `Cache-Control: no-cache` but
  not `X-Accel-Buffering`. A proxy with response buffering enabled silently withholds the stream,
  which presents as a frozen board while moves are still accepted.
- `docker-compose.yaml` publishes the container as `"8080:8080"`, which binds all host interfaces.
  Port `8080` is currently unreachable from outside the VM only because an Oracle VCN security
  rule blocks it; Docker's own iptables rules would otherwise bypass firewalld.

## Design Decisions

### Session cookies are always Secure

Set `Secure: true` unconditionally in both `setSessionCookie` and `clearSessionCookie`.

The browser-facing deployment is HTTPS, while nginx forwards cleartext HTTP to Go. Inferring cookie
security from `r.TLS` therefore produces the wrong result, and making the attribute configurable
would introduce an unnecessary production downgrade path. An authenticated session cookie is a
security invariant, not a deployment preference.

No CLI flag, environment variable, API `Config`, or `Server` field is introduced. `NewServer` keeps
its existing two-argument signature. If the cookie helpers use `*http.Request` only to inspect
`r.TLS`, remove that parameter and simplify their call sites.

The local server still runs normally over HTTP. The cookie continues to carry `Secure` during local
testing rather than weakening production behavior for development convenience.

Both login and logout helpers must set `Secure`. Although `Secure` is not part of a cookie's identity
(name, domain, and path determine replacement), issuing and clearing the authentication cookie under
the same security policy avoids a downgrade in either response and makes the invariant explicit.

### Cross-origin protection

Wire `http.CrossOriginProtection` into `Server.ServeHTTP` via `Check(r)` rather than
`Handler(mux)`. Using `Check` preserves the concrete `*Server` return type from `NewServer`
(required by the existing test helper's type assertion) and lets the 403 response carry Step 36's
security headers, since it is written by the application rather than by the middleware.

```go
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    setSecurityHeaders(w)
    if err := s.originProtection.Check(r); err != nil {
        writeError(w, http.StatusForbidden, "cross-origin request denied")
        return
    }
    s.mux.ServeHTTP(w, r)
}
```

`http.NewCrossOriginProtection()` accepts same-origin requests and safe methods (GET, HEAD,
OPTIONS) unconditionally. Cross-site unsafe requests are rejected unless the origin was added via
`AddTrustedOrigin`. For a single-domain deployment where the frontend and API share an origin, no
trusted origins are added — the built-in same-origin rule covers every legitimate request.

This is the primary CSRF defense. `SameSite=Lax` already blocks the session cookie on cross-site
POSTs, and origin protection blocks the request itself before it reaches any handler, covering
both the six body-decoding endpoints and the three bodyless POSTs (`logout`, `reshuffle`, `hint`).

### HTTP server timeouts and header cap, with WriteTimeout deliberately unset

Extract a small `newHTTPServer(addr, handler) *http.Server` constructor and have `listenAndServe`
use it. This keeps signal handling and shutdown behavior unchanged while making the exact server
configuration directly unit-testable.

Set on the constructed `http.Server`:

- `ReadHeaderTimeout: 10s` — the primary defense. Bounds how long a client may dribble request
  headers, which is the Slowloris pattern.
- `ReadTimeout: 15s` — bounds reading the entire request. Every request body in this API is under
  8 KiB, so this is generous.
- `IdleTimeout: 120s` — bounds idle keep-alive connections.
- `MaxHeaderBytes: 16 << 10` (16 KiB) — replaces Go's 1 MiB default with a value matched to the
  API's actual header footprint (Host, Cookie, small standard headers). Bounds memory per pending
  request during header parsing, complementing `ReadHeaderTimeout`.

`WriteTimeout` must remain unset. It bounds the time from the end of request reading to the end of
response writing, and the SSE snapshot stream at `GET /games/{id}/snapshots` writes for the entire
life of a game. Any non-zero value would sever active game streams mid-round. This constraint must
be stated in a comment at the call site, because a future reader will otherwise see the omission
as an oversight and complete the set.

### Opportunistic expired-session cleanup

Call `DeleteExpiredSessions` from `CreateSession`, before inserting the new row.

Login is the natural trigger: it is infrequent, it is already a write path, and the deletion is a
single indexed statement. This avoids a background goroutine, matching the opportunistic-cleanup
style used by the API session store and already sanctioned in `docs/ARCHITECTURE.md`.

Cleanup failure must not fail the login. Log the purge error via `log.Printf` and proceed with
session creation; the only consequence of a failed purge is that stale rows persist a little
longer, which is the status quo. Logging rather than silently discarding gives operators a signal
that expected background hygiene stopped working, without coupling login availability to it.

### Loopback-only container port publication

Change the `docker-compose.yaml` port mapping from `"8080:8080"` to `"127.0.0.1:8080:8080"`.

This makes nginx structurally the only inbound path to the container: exposure to the internet
requires the reverse proxy to explicitly forward traffic, rather than being blocked externally by
a separate Oracle security rule. Docker writes its own iptables rules that bypass firewalld, so
without this binding the container's privacy depends entirely on VCN configuration living outside
the repository.

nginx continues to reach the container over `http://127.0.0.1:8080` unchanged. Local development
on the host does not use `docker-compose` and is unaffected.

### SSE buffering defense

Set `X-Accel-Buffering: no` on the snapshot stream response in `handleGameSnapshots`.

nginx honors this response header by disabling proxy buffering for that response regardless of
`proxy_buffering` configuration. The stream's correctness then no longer depends on a proxy config
file staying right. This is not an audit finding; it is a defense against a failure already
observed in this deployment, where a buffered stream presented as a frozen board while moves were
being accepted normally.

## Accepted Risks

Recorded decisions for audit findings deliberately not implemented.

### Per-client game-session cap

The session store caps concurrent games at 150 globally with no per-client limit, so one client can
exhaust it and cause `503` for everyone until sessions expire.

Not implementable in the application under the standing rule against proxy awareness: behind
nginx, every request carries the same proxy address, so the application cannot distinguish
clients. This belongs at the edge, alongside the existing `limit_req` rules, and should be handled
in a future edge-configuration step. Impact is bounded, since game sessions self-expire within at
most 180 seconds.

### Leaderboard display-name spoofing

An unauthenticated score submission may use any `playerName`, including one matching a registered
account's display name.

Not fixable as stated, because display names are intentionally non-unique per
`docs/ARCHITECTURE.md` — two real accounts may already share one. A guest using an existing name is
therefore indistinguishable from a legitimate second account with that name. The only meaningful
remedy is surfacing account linkage in the leaderboard so the UI can mark verified entries, which
is a product feature affecting the API response shape and the frontend, not a security control.
Deferred to a product step.

### Account handle enumeration

Registering with a taken display name returns a suffixed handle, revealing that the plain name is
taken. Additionally, the first registrant of any name holds a handle equal to their display name,
which the leaderboard publishes.

Accepted. Eliminating it means always generating a suffix, degrading handle quality for every user
to remove a signal the public leaderboard already carries. Login itself leaks nothing: it returns a
generic error and runs a dummy bcrypt hash for unknown handles so timing is equalized.
Authentication throttling at the edge further limits enumeration at scale.

## API Design

No routes, request bodies, or successful response bodies change.

### All unsafe-method endpoints

Added status mapping:

- `403 Forbidden` with `cross-origin request denied`: the request used an unsafe method and was
  identified as cross-origin by `Sec-Fetch-Site` or, when that header was absent, by `Origin`.
  Applies to all POST, PUT, PATCH, and DELETE routes. Same-origin requests, safe-method requests,
  and non-browser requests carrying neither header are unaffected.

All existing status mappings from Steps 34 through 36 remain unchanged.

### Session cookie

`Set-Cookie` for `find_ten_session` always carries the `Secure` attribute on both login and logout.

### Snapshot stream

`GET /games/{id}/snapshots` responses gain `X-Accel-Buffering: no`.

## Files To Modify

- `docs/plans/Step-37.md`
- `docs/ARCHITECTURE.md`
- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `docker-compose.yaml`
- `internal/api/server.go`
- `internal/api/auth.go`
- `internal/api/auth_test.go`
- `internal/api/handlers.go`
- `internal/api/server_test.go`
- `internal/player/session.go`
- `internal/player/session_test.go`

## Implementation Sequence

1. Add this Step 37 plan.
2. Update `docs/ARCHITECTURE.md`: the always-Secure session-cookie invariant, superseding the Step
   35 `r.TLS` note; server timeout and header-size policy including why `WriteTimeout` is unset;
   opportunistic session cleanup with logged failures; cross-origin protection as the CSRF control;
   and the accepted risks above.
3. Add the `originProtection *http.CrossOriginProtection` field to `internal/api/server.go` and
   construct it in `NewServer` without changing the existing function signature.
4. Wire `Check(r)` into `Server.ServeHTTP` between `setSecurityHeaders` and `mux.ServeHTTP`.
5. Set `Secure: true` in both cookie helpers in `internal/api/auth.go`; remove their request
   parameter if it is no longer used, and update the login and logout call sites.
6. Extract `newHTTPServer` in `cmd/server/main.go`; configure `ReadHeaderTimeout`, `ReadTimeout`,
   `IdleTimeout`, and `MaxHeaderBytes` there, with a comment recording why `WriteTimeout` stays
   unset.
7. Call `DeleteExpiredSessions` from `CreateSession` in `internal/player/session.go`, logging
   errors rather than discarding them.
8. Add `X-Accel-Buffering: no` in `handleGameSnapshots`.
9. Add tests for the new behavior, intentionally updating only the existing plain-HTTP cookie
   assertion whose behavior changes.
10. Change the `docker-compose.yaml` port mapping to `"127.0.0.1:8080:8080"`.
11. Run `gofmt` on changed Go files.
12. Run `go test ./...`.
13. Before deploying: verify the live nginx `limit_req` zones cover `POST /auth/login` and
    `POST /players`, and verify nginx preserves the incoming Host for the upstream request (normally
    `proxy_set_header Host $host;`) so `CrossOriginProtection`'s `Origin` fallback can recognize a
    same-origin request. Address missing edge controls before rolling out account features.
14. Deploy and verify: the live `Set-Cookie` header carries `Secure`; the VM listens for container
    port `8080` only on `127.0.0.1` while a local request to that address succeeds; an external probe
    still times out; a same-origin browser request succeeds; and an unsafe cross-origin browser
    request is rejected with `403`.

## Tests

### Session cookies

- A plain HTTP handler request receives a login cookie with `Secure` true. (existing assertion
  intentionally updated)
- A request carrying `TLS` receives a login cookie with `Secure` true.
- Logout clears the cookie with `Secure` true.
- `HttpOnly`, `SameSite=Lax`, `Path=/`, and the 7-day lifetime are unchanged in every case.

### Cross-origin protection

- A POST with `Sec-Fetch-Site: same-origin` is accepted on every unsafe-method route.
- A POST with `Sec-Fetch-Site: none` is accepted.
- A POST with `Sec-Fetch-Site: same-site` is rejected, preventing a sibling subdomain from being
  treated as the application's origin.
- A POST with `Sec-Fetch-Site: cross-site` is rejected with `403 cross-origin request denied`
  on both body-decoding endpoints and the bodyless POSTs (logout, reshuffle, hint).
- A GET with `Sec-Fetch-Site: cross-site` is accepted; safe methods are not filtered.
- An unsafe request without `Sec-Fetch-Site` and without `Origin` is accepted for non-browser
  compatibility.
- An unsafe request without `Sec-Fetch-Site` is accepted when `Origin` matches `Host` and rejected
  when `Origin` is foreign.
- A `403` response carries the Step 36 security headers.

### Expired sessions

- Creating a session deletes rows whose `expires_at` has passed.
- Creating a session does not delete unexpired rows, including other players' sessions.
- Login still succeeds when the cleanup statement fails.

### Server timeouts

- `newHTTPServer` configures `ReadHeaderTimeout`, `ReadTimeout`, `IdleTimeout`, and
  `MaxHeaderBytes`, and leaves `WriteTimeout` zero; `listenAndServe` uses that constructor.

### Snapshot stream

- The SSE response carries `X-Accel-Buffering: no` and still streams snapshot events.

### Regression

- All existing account, session, score, and gameplay tests pass. The plain-HTTP cookie assertion is
  intentionally updated; unrelated assertions remain unchanged.

### Manual verification after deploy

- `Set-Cookie` on a live HTTPS login contains `Secure`, `HttpOnly`, and `SameSite=Lax`.
- Login, reload persistence, and logout work in a browser over HTTPS.
- A full game round updates the board over SSE.
- Guest and logged-in score submission both succeed.
- From a page on a genuinely different origin, a `fetch` using the explicit production
  `https://find-ten.tuvu.dedyn.io/games` URL is shown as `403` in the browser Network panel. The
  JavaScript promise may surface a CORS error instead of exposing the response status.
- On the VM, `ss` or `docker compose ps` shows port `8080` published only as
  `127.0.0.1:8080`, and `curl http://127.0.0.1:8080/` succeeds.
- Probing `163.192.32.31:8080` from a machine outside the VM still times out. This confirms the
  outer firewall remains closed; the VM-local listener check proves the Compose binding changed.

## Acceptance Criteria

- Session cookies always carry `Secure`, including when TLS is terminated upstream, and logout
  clears them under the same security policy. No runtime option can weaken this invariant.
- Cross-origin unsafe-method browser requests are rejected with `403` before reaching handlers,
  covering both body-decoding and bodyless POST endpoints.
- The HTTP server bounds header reading, request reading, idle connections, and header size,
  while SSE streams remain unbounded in write duration.
- Expired session rows are removed as a side effect of normal login activity, without a background
  goroutine; cleanup failures are logged rather than silently discarded.
- The snapshot stream instructs proxies not to buffer it.
- The container publishes port `8080` only on the host loopback interface, so exposure requires
  nginx to explicitly forward traffic rather than depending on an external firewall rule.
- The application reads no forwarding headers and retains no proxy awareness.
- Every Step 35 application-layer audit finding in this plan is either implemented or recorded
  under Accepted Risks with its reasoning; broader container and edge hardening remains explicitly
  deferred.
- Backend tests pass with `go test ./...`; only the existing cookie assertion whose specified
  behavior changes is modified, while unrelated assertions remain unchanged.
