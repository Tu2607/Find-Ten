# Step 36 - HTTP Hardening: Request Body Limits and Security Headers

## Status

Completed and accepted on 2026-08-15. This file remains the historical implementation plan and
acceptance record for the step.

## Goal

Close two application-layer findings from the security audit of the Step 35 account work:

- Unbounded JSON request bodies on every write endpoint.
- Missing security response headers and Content Security Policy.

The HTTP changes are contained within `internal/api`. One targeted frontend compatibility change
in `static/app.js` replaces a `style.cssText` assignment with equivalent individual style property
assignments so the CSP does not rely on browser-specific CSSOM enforcement behavior. This step
changes no game rules, score rules, or account semantics, and adds no new dependencies.

## Non-goals

- Do not add rate limiting in this step. The audit's authentication-throttling finding is deferred
  to a separate edge-deployment step. That step must establish and verify the nginx topology and
  trusted client-address source before selecting a rate-limit key; the Go application will not
  reconstruct client IPs or parse forwarding headers.
- Do not add proxy awareness of any kind to the Go codebase.
- Do not change container port bindings or other deployment topology. That belongs with the edge
  discussion.
- Do not add HTTP Strict Transport Security in this step. HSTS belongs with the future HTTPS edge;
  the current application intentionally supports local plain-HTTP development.
- Do not add CSRF tokens or enforce request `Content-Type` in this step. `SameSite=Lax` remains the
  current partial CSRF mitigation; broader CSRF hardening is deferred rather than justified by a
  JSON-only restriction that the API does not currently enforce.
- Do not change the `Secure` cookie rule, HTTP server timeouts, per-client game-session caps,
  expired-session purging, or account-handle enumeration behavior. Those audit findings are
  deferred to a later step.
- Do not self-host the Google Fonts dependency or change the documented font architecture.
- Do not add `DisallowUnknownFields` to JSON decoding. That is a frontend contract change, not a
  security fix.
- Do not change the existing `400 invalid JSON` behavior for malformed request bodies.
- Do not rewrite unrelated gameplay, session, leaderboard, or account behavior.

## Current State

- All six JSON handlers in `internal/api/handlers.go` call `json.NewDecoder(r.Body).Decode(...)`
  with no size limit, so a request body is read into memory before any validation runs. The
  72-byte password cap is enforced only after the body is fully decoded.
- No handler or middleware sets `Content-Security-Policy`, `X-Content-Type-Options`,
  `Referrer-Policy`, or `X-Frame-Options`.
- `static/index.html` loads Indie Flower and Press Start 2P from Google Fonts. `docs/ARCHITECTURE.md`
  records this as an intentional external dependency.
- `static/index.html` contains no inline `style` attributes, no `<style>` blocks, and no inline
  event handlers. All JavaScript is loaded from `/app.js`.
- `internal/api/server.go` returns `*Server` from `NewServer` as an `http.Handler`, and the test
  helper in `internal/api/server_test.go` type-asserts that return value back to `*Server`.
- Deferred but recorded for context: `POST /auth/login` and `POST /players` remain unthrottled, and
  every attempt costs one bcrypt hash at `bcrypt.DefaultCost`. `internal/player/password.go`
  intentionally runs a dummy hash for unknown handles so login timing does not leak account
  existence; that behavior is correct and must be preserved, but it means an unauthenticated
  attacker can compel roughly 100ms of server CPU per request. The `150` stored-session cap in
  `internal/api/store.go` does not apply, because login and registration never allocate a game
  session.

## Design Decisions

### Request body limits

Add one `decodeJSONBody` helper alongside `writeError` in `internal/api/errors.go`, backed by a
named `8 KiB` limit constant. It wraps the request body in `http.MaxBytesReader`, reads the entire
bounded body with `io.ReadAll`, and then decodes the collected bytes with `json.Unmarshal`.

Reading the complete bounded body is required. Calling `json.Decoder.Decode` once could stop after
a small valid JSON value and never inspect oversized trailing content. `io.ReadAll` forces the
reader to either reach EOF within the limit or surface `*http.MaxBytesError` when the request
contains an additional byte.

The helper maps failures:

- `*http.MaxBytesError` returns `413 Request Entity Too Large`.
- Any other read or unmarshal error returns the existing `400 invalid JSON` response unchanged.

Use `errors.As` to identify `*http.MaxBytesError`. The stable `413` response message is
`request body too large`.

The limit is `8 KiB`. The largest legitimate request body in the API is roughly 200 bytes, so the
limit is generous while preventing unbounded request buffering. At most the bounded request bytes
and the small decoded DTO are retained in application memory.

A single shared helper is used rather than per-handler limits because every endpoint has the same
small-JSON shape, and a shared helper keeps the error mapping consistent. All six decode sites in
`internal/api/handlers.go` are replaced with it. Because the helper preserves the existing
`400 invalid JSON` response exactly, no current test assertions change. `json.Unmarshal` also
rejects a second JSON value or other non-whitespace trailing content as malformed JSON.

### Security headers and Content Security Policy

Set the headers in `Server.ServeHTTP` rather than by wrapping the handler returned from
`NewServer`. Wrapping would break the `NewServer(...).(*Server)` type assertion in the existing
test helper, while setting them in `ServeHTTP` covers every route, including static files and the
SSE snapshot stream, with no test churn and no signature change.

The policy is defined in the application rather than at the edge because it is coupled to the
content the application serves: `fonts.googleapis.com` appears in the policy only because
`static/index.html` requests it. Defining it elsewhere would let the policy drift out of sync
whenever the frontend changes, with nothing to catch the divergence.

```text
default-src 'self';
script-src 'self';
style-src 'self' https://fonts.googleapis.com;
font-src 'self' https://fonts.gstatic.com;
img-src 'self';
connect-src 'self';
base-uri 'none';
form-action 'self';
frame-ancestors 'none';
object-src 'none'
```

Also set `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, and
`X-Frame-Options: DENY`.

The Google Fonts origins are allowlisted rather than removed because `docs/ARCHITECTURE.md`
records them as an intentional dependency of the chalkboard theme. Self-hosting the fonts would
permit a stricter policy but reverses a recorded design decision and belongs in its own step.

The policy requires no `'unsafe-inline'`, because the markup carries no inline styles, style
attributes, or event handlers. `connect-src 'self'` covers the SSE `EventSource` connection. The
three `<form>` elements are handled in JavaScript, and `form-action 'self'` permits their fallback
behavior.

`static/app.js` currently sets `span.style.cssText` for the chalk-dust effect. Replace that
assignment proactively with equivalent individual `span.style.top`, `span.style.left`, and
`span.style.transform` assignments. Direct property assignments avoid relying on browser-specific
CSSOM `cssText` enforcement behavior while preserving the existing position and rotation. Adding
`'unsafe-inline'` is not an acceptable remedy. The resulting behavior still requires browser
verification after the policy is applied.

## API Design

No successful request or response DTOs change. One new error status and stable error message are
introduced.

### All JSON endpoints

Added status mapping:

- `413 Request Entity Too Large` with `request body too large`: the request body exceeded `8 KiB`.

`400 Bad Request` continues to mean invalid JSON or missing required fields. All existing status
mappings from Steps 34 and 35 remain unchanged. JSON with a second value or non-whitespace
trailing content is treated as malformed and returns the existing `400 invalid JSON` response.

Move and remove-number handlers continue to look up the game before decoding their request bodies.
An oversized request for an unknown game therefore continues to return `404 Not Found` without
reading or buffering the body. Body-limit tests for those endpoints must use known game IDs.

### All responses

Every application-generated response, including static file responses and the SSE snapshot stream,
carries the security headers and Content Security Policy described above. Responses rejected by
`net/http` before `Server.ServeHTTP` runs are outside this application-handler boundary.

## Files To Modify

- `docs/plans/Step-36.md`
- `docs/ARCHITECTURE.md`
- `internal/api/errors.go`
- `internal/api/handlers.go`
- `internal/api/server.go`
- `internal/api/server_test.go`
- `static/app.js`

## Implementation Sequence

1. Add this Step 36 plan.
2. Update `docs/ARCHITECTURE.md` with an HTTP hardening section covering the request body limit,
   the Content Security Policy and why it is owned by the application, and a note that
   authentication rate limiting is deferred to the edge in a later step.
3. Add `decodeJSONBody` to `internal/api/errors.go`.
4. Replace all six decode sites in `internal/api/handlers.go` with the helper.
5. Add `setSecurityHeaders` to `internal/api/server.go` and apply it in `ServeHTTP`.
6. Replace the chalk-dust `style.cssText` assignment in `static/app.js` with individual `top`,
   `left`, and `transform` property assignments.
7. Add API tests for bounded JSON bodies and security headers.
8. Run `gofmt` on changed Go files.
9. Run `go test ./...`.
10. Manually smoke test the browser flow with the developer console open.

## Tests

### Request bodies

- A body of exactly `8 KiB`, formed from a valid JSON object plus trailing whitespace, is accepted
  by the decoder and proceeds to normal endpoint validation.
- A body of `8 KiB + 1 byte` returns `413 request body too large`.
- A small valid JSON object followed by enough trailing content to exceed `8 KiB` returns `413`;
  a valid prefix cannot bypass the total-body limit.
- Oversized-body coverage exercises all six JSON endpoints. Move and remove-number cases use known
  game IDs so the requests reach body decoding.
- Malformed JSON still returns `400 invalid JSON` with the existing response message unchanged.
- A second JSON value or non-whitespace trailing content under the limit returns
  `400 invalid JSON`.
- Valid requests behave exactly as before on every endpoint.

### Security headers

- Successful API responses carry the Content Security Policy and the three other headers.
- Application-generated error responses carry the same headers.
- Unknown static-file `404 Not Found` and ServeMux-generated `405 Method Not Allowed` responses
  carry the same headers.
- The new `413 Request Entity Too Large` response carries the same headers.
- Static file responses carry the same headers.
- The SSE snapshot stream carries the same headers and still streams events.

### Frontend manual smoke tests

- The game loads with no CSP violations reported in the browser console.
- Both custom fonts render, and all three board font settings work.
- The chalk-dust animation still positions and rotates correctly.
- A full game round updates the board over SSE.
- Guest and logged-in score submission both succeed.
- Registration, login, reload persistence, and logout all still work.

## Acceptance Criteria

- Every JSON decoder is bounded to `8 KiB`, preventing unbounded request buffering.
- Requests that reach JSON decoding reject bodies over `8 KiB` with
  `413 request body too large`; a valid JSON prefix cannot bypass the limit.
- Bodies of exactly `8 KiB` are accepted by the decoder, while bodies of `8 KiB + 1 byte` are
  rejected.
- Malformed JSON still returns `400 invalid JSON`.
- Every application-generated response carries a Content Security Policy that requires no
  `'unsafe-inline'` and still permits the documented Google Fonts dependency.
- Every application-generated response carries `X-Content-Type-Options`, `Referrer-Policy`, and
  `X-Frame-Options`.
- The Go codebase contains no proxy awareness, client IP handling, or rate-limiting logic.
- Game, score, account, and SSE endpoints are functionally unchanged.
- The browser game plays through a full round with no CSP violations, and the chalk-dust effect
  preserves its position and rotation.
- Backend tests pass with `go test ./...` and no existing test assertions are modified.
