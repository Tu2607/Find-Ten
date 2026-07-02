# Step 29 — Leaderboard Page Frontend Shell

## Problem Statement

The welcome screen has a disabled Leaderboard button, but there is no leaderboard page yet. This step adds the frontend page shell for leaderboard functionality while keeping score persistence, backend APIs, and real leaderboard data out of scope.

The goal is to create the UI groundwork so future leaderboard work can focus on data collection, storage, and rendering without first adding navigation or layout.

## Scope

### In Scope

- Enable the existing Leaderboard button on the welcome screen
- Add a new leaderboard screen using the existing screen toggling pattern
- Add back navigation from leaderboard to welcome
- Add a leaderboard heading and chalkboard-styled layout
- Add filter controls for:
  - board size: All, 9x9, 10x10, 11x11
  - timer duration: All, 60s, 120s, 180s
- Add a static leaderboard table structure with columns:
  - Rank
  - Player
  - Score
  - Board
  - Time
- Add an empty state shown when no scores are available
- Add responsive CSS for the leaderboard page
- Add minimal frontend JavaScript for navigation and filter active states

### Out of Scope

- Backend leaderboard API
- Score persistence
- localStorage score history
- Player name entry
- Real leaderboard rows
- Sorting
- Pagination
- Filtering actual data
- Updating architecture docs for persistence or leaderboard storage

## Code Changes

### [File: static/index.html] (modify)

**What changes**: Enable the existing Leaderboard button by adding an ID and removing `disabled`.

```diff
-            <button type="button" class="chalk-btn" disabled>Leaderboard</button>
+            <button type="button" class="chalk-btn" id="leaderboardButton">Leaderboard</button>
```

**What changes**: Add a new leaderboard screen between the settings screen and game screen.

```html
  <!-- ====== LEADERBOARD SCREEN ====== -->
  <section id="leaderboardScreen" class="screen screen--leaderboard" hidden>
    <div class="chalkboard">
      <div class="chalkboard__surface">
        <h2 class="chalk-heading">Leaderboard</h2>
        <div class="chalk-underline"></div>

        <div class="leaderboard-filters">
          <div class="leaderboard-filter-group">
            <span class="chalk-filter-label">Board</span>
            <div class="chalk-pill-group" data-filter="board">
              <button type="button" class="chalk-pill chalk-pill--active" data-value="all">All</button>
              <button type="button" class="chalk-pill" data-value="9">9 x 9</button>
              <button type="button" class="chalk-pill" data-value="10">10 x 10</button>
              <button type="button" class="chalk-pill" data-value="11">11 x 11</button>
            </div>
          </div>

          <div class="leaderboard-filter-group">
            <span class="chalk-filter-label">Time</span>
            <div class="chalk-pill-group" data-filter="timer">
              <button type="button" class="chalk-pill chalk-pill--active" data-value="all">All</button>
              <button type="button" class="chalk-pill" data-value="60">60s</button>
              <button type="button" class="chalk-pill" data-value="120">120s</button>
              <button type="button" class="chalk-pill" data-value="180">180s</button>
            </div>
          </div>
        </div>

        <div class="leaderboard-table-wrapper">
          <table class="leaderboard-table">
            <thead>
              <tr>
                <th>Rank</th>
                <th>Player</th>
                <th>Score</th>
                <th>Board</th>
                <th>Time</th>
              </tr>
            </thead>
            <tbody id="leaderboardBody"></tbody>
          </table>

          <p class="leaderboard-empty" id="leaderboardEmpty">No scores yet - play a game!</p>
        </div>

        <button type="button" class="chalk-btn" id="leaderboardBackButton">Back</button>
      </div>
      <div class="chalk-tray" aria-hidden="true"></div>
    </div>
  </section>
```

### [File: static/styles.css] (modify)

**What changes**: Add leaderboard-specific layout, filter, table, and mobile styles.

```css
/* ====== LEADERBOARD SCREEN ====== */

.screen--leaderboard .chalkboard__surface {
  gap: 0;
}

.leaderboard-filters {
  display: flex;
  gap: 32px;
  justify-content: center;
  flex-wrap: wrap;
  width: min(720px, 90vw);
  margin: 0 auto 24px;
}

.leaderboard-filter-group {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.chalk-filter-label {
  font-size: clamp(16px, 2vw, 22px);
  opacity: 0.6;
  white-space: nowrap;
}

.chalk-pill-group {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.chalk-pill {
  padding: 5px 14px;
  border: 1.5px solid rgba(255, 255, 255, 0.5);
  border-radius: 14px;
  background: transparent;
  color: white;
  font-size: clamp(14px, 1.6vw, 18px);
  line-height: 1.2;
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s;
}

.chalk-pill:hover {
  background: rgba(255, 255, 255, 0.08);
}

.chalk-pill--active {
  background: rgba(255, 255, 255, 0.15);
  border-color: rgba(255, 255, 255, 0.85);
}

.leaderboard-table-wrapper {
  width: min(680px, 90vw);
  margin: 0 auto 24px;
  flex: 1;
  display: flex;
  flex-direction: column;
}

.leaderboard-table {
  width: 100%;
  border-collapse: collapse;
  font-size: clamp(16px, 2vw, 22px);
}

.leaderboard-table th {
  padding: 10px 12px;
  text-align: left;
  font-weight: 700;
  color: #ffe680;
  border-bottom: 1.5px solid rgba(255, 255, 255, 0.25);
  white-space: nowrap;
}

.leaderboard-table td {
  padding: 10px 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.leaderboard-table th:first-child,
.leaderboard-table td:first-child {
  text-align: center;
  width: 60px;
}

.leaderboard-table th:nth-child(3),
.leaderboard-table td:nth-child(3) {
  text-align: right;
}

.leaderboard-table th:nth-child(4),
.leaderboard-table td:nth-child(4),
.leaderboard-table th:nth-child(5),
.leaderboard-table td:nth-child(5) {
  text-align: center;
}

.leaderboard-empty {
  margin: 40px 0;
  text-align: center;
  font-size: clamp(18px, 2.2vw, 26px);
  opacity: 0.45;
}

#leaderboardBackButton {
  margin-top: auto;
  align-self: center;
}

@media (max-width: 600px) {
  .leaderboard-filters {
    flex-direction: column;
    align-items: center;
    gap: 16px;
  }

  .leaderboard-filter-group {
    flex-direction: column;
    gap: 8px;
  }

  .chalk-pill-group {
    justify-content: center;
  }

  .leaderboard-table {
    font-size: 14px;
  }

  .leaderboard-table th,
  .leaderboard-table td {
    padding: 8px 6px;
  }
}
```

### [File: static/app.js] (modify)

**What changes**: Add a DOM reference for the leaderboard screen.

```diff
 const welcomeScreen = document.getElementById("welcomeScreen");
 const settingsScreen = document.getElementById("settingsScreen");
+const leaderboardScreen = document.getElementById("leaderboardScreen");
 const gameScreen = document.getElementById("gameScreen");
```

**What changes**: Add navigation handlers.

```diff
 document.getElementById("settingsBackButton").addEventListener("click", () => {
   showScreen("welcome");
 });
+
+document.getElementById("leaderboardButton").addEventListener("click", () => {
+  showScreen("leaderboard");
+});
+
+document.getElementById("leaderboardBackButton").addEventListener("click", () => {
+  showScreen("welcome");
+});
```

**What changes**: Add simple pill active-state behavior.

```js
document.querySelectorAll(".chalk-pill-group").forEach((group) => {
  group.addEventListener("click", (event) => {
    const pill = event.target.closest(".chalk-pill");
    if (!pill) return;

    group.querySelectorAll(".chalk-pill").forEach((item) => {
      item.classList.remove("chalk-pill--active");
    });
    pill.classList.add("chalk-pill--active");
  });
});
```

**What changes**: Extend screen toggling.

```diff
 function showScreen(name) {
   hideError();
   welcomeScreen.hidden = name !== "welcome";
   settingsScreen.hidden = name !== "settings";
+  leaderboardScreen.hidden = name !== "leaderboard";
   gameScreen.hidden = name !== "game";
 }
```

## Task Order

1. Add this plan as `docs/plans/Step-29.md`.
2. Enable the Leaderboard button and add the leaderboard screen markup.
3. Add leaderboard CSS.
4. Wire leaderboard navigation and filter active states in JavaScript.
5. Run `go test ./...`.
6. Manually verify the WebUI navigation and responsive layout.

## Acceptance Criteria

- [ ] `docs/plans/Step-29.md` records this implementation step.
- [ ] Clicking `Leaderboard` from the welcome screen opens the leaderboard screen.
- [ ] The leaderboard screen shows a heading, filter controls, table headers, empty state, and back button.
- [ ] Table headers are `Rank`, `Player`, `Score`, `Board`, and `Time`.
- [ ] The empty state is visible when there are no scores.
- [ ] Board filter pills allow only one active board filter at a time.
- [ ] Timer filter pills allow only one active timer filter at a time.
- [ ] Clicking `Back` returns to the welcome screen.
- [ ] Existing Play and Settings navigation still works.
- [ ] No backend calls or persistence behavior are added.
- [ ] Layout remains usable on mobile widths.

## Test Strategy

- Run `go test ./...`.
- Manually verify:
  - Welcome -> Leaderboard
  - Leaderboard -> Back
  - Welcome -> Settings -> Back
  - Welcome -> Play
  - Leaderboard filter active states
  - Mobile layout below 600px
