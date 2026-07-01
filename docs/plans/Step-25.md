# Step 25 — Frontend UI Overhaul: Multi-Screen Game Flow

## Context

The frontend is a single flat page showing everything at once. This step transforms it into a proper game flow with distinct screens, a chalkboard visual theme, and a cartoon title logo.

## Visual Theme

**Chalkboard** — all screens share:
- Dark green chalkboard surface with chalk dust grain texture (SVG noise filter)
- Wooden frame border with grain lines and chalk tray at the bottom
- Chalk texture filters on all text and drawn elements
- Font: **Indie Flower** from Google Fonts (fallback: Segoe Print, Bradley Hand)
- Chalk-drawn UI controls (outlined pills, radio circles, etc.)

Mockups in scratchpad: `welcome-mockup.svg`, `settings-mockup.svg`, `game-mockup.svg`, `gameover-mockup.svg`

## Screens

### 1. Welcome Screen (default on load)
- Chalkboard with scattered chalk formulas (y=ax+b, A=πr², E=mc², etc.) and math symbols
- Spotlight effect from top (radial gradient + vignette)
- Rainbow-arc "FIND TEN" title in yellow chalk
- Easter egg: `9 + 10 = 21` below title
- Three chalk-drawn pill buttons: **Play**, **Settings**, **Leaderboard** (placeholder)
- Formulas avoid overlapping the buttons

### 2. Settings Screen
- Clean chalkboard (no formulas, no spotlight, just grain)
- "Settings" title in yellow chalk with underline
- **Board Size**: chalk radio buttons (filled circle = selected) — 9×9, 10×10, 11×11
- **Background**: chalk color swatches with checkmark, "coming soon"
- **Font**: current selection in chalk box, "coming soon"
- **Timer**: current value in chalk box, "coming soon"
- **← Back** button (chalk pill)

### 3. Game Screen
- Clean chalkboard with grain
- **Top bar** written directly on board: score (yellow chalk, left), timer (white chalk, center), skill buttons (chalk pill outlines, right — Reshuffle, Remove, Hint)
- Chalk divider line below top bar
- **9×9 chalk grid**: heavy chalk outline border, medium-weight inner grid lines, white chalk numbers
- Cleared cells: no number, faint eraser smudge marks
- Selected rectangle: subtle white highlight with brighter corners
- No `setStatus` — remove all status text

### 4. Game Over Overlay
- Game board dimmed heavily (10% opacity content + 65% dark scrim)
- Chalk-drawn double-line popup rectangle centered on board
- **Reason as title** (e.g. "Time's Up!", "No Moves Left!", "Board Cleared!") in large yellow chalk
- Yellow chalk underline
- "Final Score" label + large yellow chalk score value
- Two chalk pill buttons side by side: **Play Again**, **Main Menu**

## Architecture

- Single `index.html` with all screens as `<section>` elements
- CSS class toggle for screen switching via `showScreen(name)` JS function
- Google Fonts loaded via `<link>` tag for Indie Flower
- Shared CSS filters/gradients for chalk effects
- No server-side changes needed

## Already Completed (CSS tweaks, pre-overhaul)

- Board cells: sky-blue with white text, rounded corners, 6px gap
- Background: salmon-pink gradient (will be replaced by chalkboard)

## Files Modified

- `static/index.html` — restructure into screen sections, add Google Fonts link
- `static/styles.css` — chalkboard theme, chalk filters, screen layouts, overlay
- `static/app.js` — screen switching, remove setStatus, game-over overlay instead of alert

## Acceptance Criteria

- Welcome screen shows on load with chalkboard, formulas, spotlight, title, and menu
- Settings allows board size selection, Back returns to welcome
- Play starts game with chalk grid, top-bar HUD, no status text
- Game over shows dimmed overlay with reason, score, and two navigation buttons
- All text renders in Indie Flower
- Mobile responsive: top bar and overlay work on narrow viewports
- `go test ./...` still passes (no backend changes)
