# Step 26 — Frontend Robustness & Fluid Scaling

## Context

Step 25 overhauled the UI into a multi-screen chalkboard theme. This step addresses code-review findings from that overhaul and adds fluid scaling so the layout adapts to any viewport size.

## Direction Change: No Per-Move Status Feedback

The old `setStatus()` pattern showed the player a message after every move ("Move accepted", "Invalid selection", etc.). This has been fully removed — the player can tell a move worked when cells clear, and can tell it didn't when the board stays the same. No status text or inline feedback is needed for normal gameplay.

Error feedback is reserved for situations the player **cannot** infer from the board state:
- Failed game start (server down, network error) — error toast on welcome screen
- Failed skill/move due to network disconnect — error toast during gameplay
- Game-over transitions (time expired, no moves, board cleared) — overlay popup

## Changes

### Fluid Scaling (CSS)
- All fixed `font-size` values replaced with `clamp(min, preferred-vw, max)` so text scales smoothly across viewport widths
- Board `max-width` raised from 600px to 780px with a `max(280px, ...)` floor for short landscape viewports
- Welcome title width cap raised from 500px to 700px
- Top bar and divider max-width matched to board at 780px

### Dynamic Formula Scattering (JS)
- Welcome screen chalk formulas are now positioned randomly on each page load via JS
- Avoids the center zone (title + buttons) and enforces minimum spacing between formulas
- Adapts to any screen size since positions are percentage-based

### Error Handling & Game State Fixes (JS)
- `startGame()` wrapped in `try/finally` — Play/Play Again buttons always re-enable
- `startGame()` navigates back to welcome with an error toast on network failure, non-OK response, or invalid JSON
- `startPending` guard prevents double-click races on Play/Play Again
- New `endGame(reason)` helper centralizes all game-ending paths: reverts optimistic moves, stops countdown, closes SSE stream, disables skills, shows overlay
- All 409/410 responses from move/skill endpoints now route through `endGame()`
- `maybeShowGameOver()` (server-sent snapshot) now routes through `endGame()` instead of partially duplicating logic
- SSE `onerror` attempts automatic reconnect after 2s with a stored timer; `closeStream()` cancels pending reconnects
- Snapshot handler ignores events from stale game IDs
- Main Menu button now calls `abandonGame()` to clean up the server-side session
- Error toast is a fixed-position element visible on any screen

### In-Game Navigation Buttons
- Add a **Restart** button on the game screen that abandons the current game and starts a fresh one with the same settings
- Add a **Home** button on the game screen that abandons the current game and returns to the welcome screen
- Both buttons should be accessible from the game top bar area without cluttering the HUD

## Files Modified

- `static/styles.css` — fluid `clamp()` typography, larger board cap, error toast styles
- `static/index.html` — formula spans replaced with empty container, error toast element added
- `static/app.js` — formula scattering, `startGame` guards, `endGame` helper, SSE reconnect, error toasts
