# Step 28 — Game Settings: Configurable Timer and Board Font

## Problem Statement

The settings screen has placeholder "coming soon" labels for Timer and Font. This step makes both configurable:

- **Timer**: 60s, 120s (default), 180s — backed by the server since the backend owns the authoritative expiry deadline.
- **Font**: Chalk (Indie Flower, default), Clean (system sans-serif), Retro (Press Start 2P) — frontend-only, applied to board cells only.

This lays the groundwork for future unlockable features (not implemented here).

## Scope

### In Scope

- Duration validation on the backend (`60`, `120`, `180`)
- `NewGameSession` accepts a `durationSeconds` parameter
- `POST /games` accepts an optional `duration` field
- Settings screen UI for timer and font with radio buttons
- Font CSS applied to board cells only
- Google Fonts link updated for Press Start 2P
- Architecture doc updated for new API field and font additions
- New tests for duration validation and API duration handling
- All existing `NewGameSession` callers updated

### Out of Scope

- localStorage persistence (settings reset each visit)
- Background color swatches (remain disabled)
- Unlockable system
- Font change for non-board-cell text

## Code Changes

### [File: internal/game/types.go] (modify)

**What changes**: Add supported duration constants, a duration set, and a `ValidateDuration` function.

```diff
 const (
 	MinSupportedBoardSize = 9
 	MaxSupportedBoardSize = 11
+
+	MinDurationSeconds     = 60
+	DefaultDurationSeconds = 120
+	MaxDurationSeconds     = 180
 )
+
+var supportedDurations = map[int]struct{}{
+	60:  {},
+	120: {},
+	180: {},
+}
```

```diff
+func ValidateDuration(seconds int) error {
+	if _, ok := supportedDurations[seconds]; !ok {
+		return fmt.Errorf("unsupported duration %d: must be 60, 120, or 180", seconds)
+	}
+	return nil
+}
```

### [File: internal/game/types_test.go] (modify)

**What changes**: Add table-driven tests for `ValidateDuration`.

```diff
+func TestValidateDuration(t *testing.T) {
+	tests := []struct {
+		name    string
+		seconds int
+		wantErr bool
+	}{
+		{"valid 60", 60, false},
+		{"valid 120", 120, false},
+		{"valid 180", 180, false},
+		{"invalid 0", 0, true},
+		{"invalid 30", 30, true},
+		{"invalid 90", 90, true},
+		{"invalid 240", 240, true},
+		{"invalid negative", -1, true},
+	}
+
+	for _, tt := range tests {
+		t.Run(tt.name, func(t *testing.T) {
+			err := ValidateDuration(tt.seconds)
+			if (err != nil) != tt.wantErr {
+				t.Errorf("ValidateDuration(%d) error = %v, wantErr %v", tt.seconds, err, tt.wantErr)
+			}
+		})
+	}
+}
```

### [File: internal/game/board.go] (modify)

**What changes**: Remove `DefaultGameDurationSeconds` (replaced by `DefaultDurationSeconds` in `types.go`).

```diff
 const (
 	minGeneratedDigit          = 1
 	maxGeneratedDigit          = 9
 	maxBoardGenerationAttempts = 1000
-	DefaultGameDurationSeconds = 120
 )
```

### [File: internal/game/play.go] (modify)

**What changes**: `NewGameSession` accepts `durationSeconds int` as a third parameter instead of using the hardcoded constant.

```diff
-func NewGameSession(ctx context.Context, size int) (*GameSession, GameSnapshot, error) {
+func NewGameSession(ctx context.Context, size int, durationSeconds int) (*GameSession, GameSnapshot, error) {
 	state, err := NewGame(size)
 	if err != nil {
 		return nil, GameSnapshot{}, err
 	}

-	expiresAt := time.Now().Add(DefaultGameDurationSeconds * time.Second)
+	expiresAt := time.Now().Add(time.Duration(durationSeconds) * time.Second)
 	initialSnapshot := newGameSnapshot(state, 1)
```

### [File: internal/game/play_test.go] (modify)

**What changes**: All 17 `NewGameSession` calls gain `DefaultDurationSeconds` as the third argument. Update the `ExpiresAt` check to use `DefaultDurationSeconds`. Example of the pattern (applied identically to all call sites):

```diff
-	session, snapshot, err := NewGameSession(context.Background(), MinSupportedBoardSize)
+	session, snapshot, err := NewGameSession(context.Background(), MinSupportedBoardSize, DefaultDurationSeconds)
```

Lines affected: 722, 750, 767, 789, 818, 843, 871, 903, 948, 973, 1036, 1050, 1064, 1078, 1092, 1111, 1168.

Also update the `ExpiresAt` tolerance check:

```diff
-	if expiresAt.Before(before.Add(DefaultGameDurationSeconds * time.Second)) {
-		t.Fatalf("ExpiresAt() = %v, want at or after %v", expiresAt, before.Add(DefaultGameDurationSeconds*time.Second))
+	if expiresAt.Before(before.Add(DefaultDurationSeconds * time.Second)) {
+		t.Fatalf("ExpiresAt() = %v, want at or after %v", expiresAt, before.Add(DefaultDurationSeconds*time.Second))
 	}
-	if expiresAt.After(after.Add(DefaultGameDurationSeconds * time.Second)) {
-		t.Fatalf("ExpiresAt() = %v, want at or before %v", expiresAt, after.Add(DefaultGameDurationSeconds*time.Second))
+	if expiresAt.After(after.Add(DefaultDurationSeconds * time.Second)) {
+		t.Fatalf("ExpiresAt() = %v, want at or before %v", expiresAt, after.Add(DefaultDurationSeconds*time.Second))
 	}
```

Add a new test verifying custom duration changes expiry:

```diff
+func TestNewGameSessionCustomDuration(t *testing.T) {
+	before := time.Now()
+	session, _, err := NewGameSession(context.Background(), MinSupportedBoardSize, 60)
+	if err != nil {
+		t.Fatalf("NewGameSession returned unexpected error: %v", err)
+	}
+	after := time.Now()
+	defer session.Stop()
+
+	expiresAt := session.ExpiresAt()
+	if expiresAt.Before(before.Add(60 * time.Second)) {
+		t.Fatalf("ExpiresAt() = %v, want at or after %v", expiresAt, before.Add(60*time.Second))
+	}
+	if expiresAt.After(after.Add(60 * time.Second)) {
+		t.Fatalf("ExpiresAt() = %v, want at or before %v", expiresAt, after.Add(60*time.Second))
+	}
+}
```

### [File: cmd/play/main.go] (modify)

**What changes**: Pass `DefaultDurationSeconds` to `NewGameSession`.

```diff
-	session, initialSnapshot, err := game.NewGameSession(ctx, *size)
+	session, initialSnapshot, err := game.NewGameSession(ctx, *size, game.DefaultDurationSeconds)
```

### [File: cmd/play/main_test.go] (modify)

**What changes**: All 4 `NewGameSession` calls gain `game.DefaultDurationSeconds` as the third argument. Lines affected: 86, 118, 143, 164.

```diff
-	session, initialSnapshot, err := game.NewGameSession(context.Background(), game.MinSupportedBoardSize)
+	session, initialSnapshot, err := game.NewGameSession(context.Background(), game.MinSupportedBoardSize, game.DefaultDurationSeconds)
```

### [File: internal/api/dto.go] (modify)

**What changes**: Add optional `Duration` field to `createGameRequest`.

```diff
 type createGameRequest struct {
 	Size     *int `json:"size"`
+	Duration *int `json:"duration"`
 }
```

### [File: internal/api/handlers.go] (modify)

**What changes**: `handleCreateGame` validates and passes the optional duration.

```diff
 	if err := game.ValidateBoardSize(*request.Size); err != nil {
 		writeError(w, http.StatusBadRequest, err.Error())
 		return
 	}

-	session, initialSnapshot, err := game.NewGameSession(context.Background(), *request.Size)
+	durationSeconds := game.DefaultDurationSeconds
+	if request.Duration != nil {
+		durationSeconds = *request.Duration
+	}
+	if err := game.ValidateDuration(durationSeconds); err != nil {
+		writeError(w, http.StatusBadRequest, err.Error())
+		return
+	}
+
+	session, initialSnapshot, err := game.NewGameSession(context.Background(), *request.Size, durationSeconds)
```

### [File: internal/api/store_test.go] (modify)

**What changes**: All 9 `NewGameSession` calls gain `game.DefaultDurationSeconds` as the third argument. Lines affected: 13, 39, 50, 68, 79, 98, 108, 123, 133.

```diff
-	session, _, err := game.NewGameSession(context.Background(), 9)
+	session, _, err := game.NewGameSession(context.Background(), 9, game.DefaultDurationSeconds)
```

### [File: internal/api/server_test.go] (modify)

**What changes**: Add tests for the new duration field in `POST /games`. The existing test at line 132 gains a route dispatch case for invalid duration. Add two new integration tests.

Add to `TestServerRouteDispatch` cases:

```diff
+		{
+			name:       "create game with valid duration",
+			method:     http.MethodPost,
+			path:       "/games",
+			wantStatus: http.StatusCreated,
+		},
```

The existing `"create game"` case sends `{"size":9}` (no duration) and expects `201`; it continues to pass since duration is optional.

Add new tests:

```diff
+func TestCreateGameWithDuration(t *testing.T) {
+	server := newTestServer()
+
+	before := time.Now()
+	request := httptest.NewRequest(http.MethodPost, "/games", strings.NewReader(`{"size":9,"duration":60}`))
+	response := httptest.NewRecorder()
+	server.ServeHTTP(response, request)
+
+	if response.Code != http.StatusCreated {
+		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
+	}
+
+	var body createGameResponse
+	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
+		t.Fatalf("failed to decode response: %v", err)
+	}
+
+	stored, ok := server.store.get(body.GameID)
+	if !ok {
+		t.Fatalf("game %q not found in store", body.GameID)
+	}
+	defer stored.session.Stop()
+
+	expiresAt := stored.session.ExpiresAt()
+	expectedMin := before.Add(60 * time.Second)
+	if expiresAt.Before(expectedMin) {
+		t.Errorf("ExpiresAt = %v, want at or after %v", expiresAt, expectedMin)
+	}
+}
+
+func TestCreateGameInvalidDuration(t *testing.T) {
+	server := newTestServer()
+
+	request := httptest.NewRequest(http.MethodPost, "/games", strings.NewReader(`{"size":9,"duration":45}`))
+	response := httptest.NewRecorder()
+	server.ServeHTTP(response, request)
+
+	if response.Code != http.StatusBadRequest {
+		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
+	}
+}
```

### [File: static/index.html] (modify)

**What changes**: Update Google Fonts link to include Press Start 2P. Replace timer and font "coming soon" placeholders with radio buttons.

Google Fonts link:

```diff
-  <link href="https://fonts.googleapis.com/css2?family=Indie+Flower&display=swap" rel="stylesheet">
+  <link href="https://fonts.googleapis.com/css2?family=Indie+Flower&family=Press+Start+2P&display=swap" rel="stylesheet">
```

Timer settings group:

```diff
          <div class="settings-group">
            <h3 class="chalk-label">Timer</h3>
-            <div class="chalk-box">120s (default)</div>
-            <p class="chalk-coming-soon">coming soon</p>
+            <label class="chalk-radio">
+              <input type="radio" name="timer" value="60">
+              <span>60s</span>
+            </label>
+            <label class="chalk-radio">
+              <input type="radio" name="timer" value="120" checked>
+              <span>120s</span>
+            </label>
+            <label class="chalk-radio">
+              <input type="radio" name="timer" value="180">
+              <span>180s</span>
+            </label>
          </div>
```

Font settings group:

```diff
          <div class="settings-group">
            <h3 class="chalk-label">Font</h3>
-            <div class="chalk-box">Chalk (default)</div>
-            <p class="chalk-coming-soon">coming soon</p>
+            <label class="chalk-radio">
+              <input type="radio" name="font" value="chalk" checked>
+              <span class="font-preview font-preview--chalk">Chalk</span>
+            </label>
+            <label class="chalk-radio">
+              <input type="radio" name="font" value="clean">
+              <span class="font-preview font-preview--clean">Clean</span>
+            </label>
+            <label class="chalk-radio">
+              <input type="radio" name="font" value="retro">
+              <span class="font-preview font-preview--retro">Retro</span>
+            </label>
          </div>
```

### [File: static/styles.css] (modify)

**What changes**: Add font preview styles for the settings screen and board font variant styles for game cells.

```diff
+/* Font previews in settings */
+.font-preview--clean {
+  font-family: system-ui, -apple-system, 'Segoe UI', sans-serif;
+}
+
+.font-preview--retro {
+  font-family: 'Press Start 2P', monospace;
+  font-size: 0.75em;
+}
+
+/* Board cell font variants */
+.board.font-clean .cell {
+  font-family: system-ui, -apple-system, 'Segoe UI', sans-serif;
+}
+
+.board.font-retro .cell {
+  font-family: 'Press Start 2P', monospace;
+  font-size: clamp(14px, 2vw, 24px);
+}
```

### [File: static/app.js] (modify)

**What changes**: Add `getSelectedTimer()` and `getSelectedFont()` helpers. Modify `startGame` to read all settings internally, send `duration` in the POST body, and apply the font class to the board.

Add helper functions:

```diff
+function getSelectedTimer() {
+  const checked = document.querySelector('input[name="timer"]:checked');
+  return checked ? Number(checked.value) : 120;
+}
+
+function getSelectedFont() {
+  const checked = document.querySelector('input[name="font"]:checked');
+  return checked ? checked.value : "chalk";
+}
```

Update `startGame` to read settings internally (no parameters):

```diff
-async function startGame(size) {
+async function startGame() {
   if (state.startPending) return;
   state.startPending = true;
   playButtonEl.disabled = true;
   playAgainButtonEl.disabled = true;

   try {
     const gen = ++state.startGeneration;
     const previousGameId = state.gameId;
+    const size = getSelectedBoardSize();
+    const duration = getSelectedTimer();
+    const font = getSelectedFont();

     closeStream();
     ...

     let response;
     try {
       response = await fetch("/games", {
         method: "POST",
         headers: { "Content-Type": "application/json" },
-        body: JSON.stringify({ size })
+        body: JSON.stringify({ size, duration })
       });
```

Apply font class after state reset, before `applySnapshot`:

```diff
     state.lastBoardWidth = 0;

+    boardEl.classList.remove("font-chalk", "font-clean", "font-retro");
+    boardEl.classList.add(`font-${font}`);
+
     applySnapshot(game.initialSnapshot);
```

Update all callers of `startGame` to pass no arguments:

```diff
 playButtonEl.addEventListener("click", () => {
-  const size = getSelectedBoardSize();
   showScreen("game");
-  startGame(size);
+  startGame();
 });
```

```diff
 playAgainButtonEl.addEventListener("click", () => {
   hideGameOverOverlay();
-  const size = getSelectedBoardSize();
-  startGame(size);
+  startGame();
 });
```

```diff
 document.getElementById("restartButton").addEventListener("click", () => {
-  const size = getSelectedBoardSize();
-  startGame(size);
+  startGame();
 });
```

### [File: docs/ARCHITECTURE.md] (modify)

**What changes**: Update the `POST /games` documentation and font section.

In the Create Game section:

```diff
-The request body supplies the board size. The handler validates the size through `internal/game`, calls `game.NewGameSession`, stores the session in the API registry, and returns:
+The request body supplies the board size and an optional game duration in seconds. The handler validates both fields through `internal/game`, calls `game.NewGameSession`, stores the session in the API registry, and returns:
```

In the Font section:

```diff
-### Font: Google Fonts (Indie Flower)
+### Fonts: Google Fonts (Indie Flower, Press Start 2P)

-The UI loads Indie Flower from Google Fonts via a `<link>` tag. This is an intentional external dependency — the chalkboard theme relies on a handwriting-style font, and Google Fonts provides the simplest delivery with good caching. Self-hosting was considered but adds build complexity for a game that requires a network connection to play anyway (all game state lives on the server). The CSS declares local fallbacks (`Segoe Print`, `Bradley Hand`, `cursive`) for degraded rendering if the font fails to load.
+The UI loads Indie Flower and Press Start 2P from Google Fonts via a `<link>` tag. These are intentional external dependencies — the chalkboard theme relies on a handwriting-style font, and the retro option uses a pixel font. Google Fonts provides the simplest delivery with good caching. Self-hosting was considered but adds build complexity for a game that requires a network connection to play anyway (all game state lives on the server). The CSS declares local fallbacks for degraded rendering if fonts fail to load.
+
+The player can choose between three board cell fonts in the settings screen: Chalk (Indie Flower), Clean (system sans-serif), and Retro (Press Start 2P). Font selection is a frontend-only setting and does not affect the backend. The font class is applied to the board element and scoped to cell styles only — menus, labels, and overlays always use Indie Flower.
```

### [File: docs/GOAL.md] (modify)

**What changes**: Update game duration description.

```diff
-The default game duration is `120` seconds.
+The player can choose a game duration of `60`, `120`, or `180` seconds. The default is `120` seconds.
```

## Task Order

1. Add duration constants and `ValidateDuration` to `internal/game/types.go`
2. Add `ValidateDuration` tests to `internal/game/types_test.go`
3. Remove `DefaultGameDurationSeconds` from `internal/game/board.go`
4. Update `NewGameSession` signature in `internal/game/play.go`
5. Update all `NewGameSession` callers in `internal/game/play_test.go`
6. Update all `NewGameSession` callers in `internal/api/store_test.go`
7. Update all `NewGameSession` callers in `cmd/play/main.go` and `cmd/play/main_test.go`
8. Update `createGameRequest` DTO in `internal/api/dto.go`
9. Update `handleCreateGame` in `internal/api/handlers.go`
10. Add API duration tests to `internal/api/server_test.go`
11. Update `static/index.html` (Google Fonts link, timer/font radio buttons)
12. Update `static/styles.css` (font preview + board font variant styles)
13. Update `static/app.js` (settings helpers, `startGame` refactor, font class)
14. Update `docs/ARCHITECTURE.md` and `docs/GOAL.md`

## Acceptance Criteria

- [ ] `POST /games` with `{"size":9}` (no duration) creates a 120s game
- [ ] `POST /games` with `{"size":9,"duration":60}` creates a 60s game
- [ ] `POST /games` with `{"size":9,"duration":45}` returns `400 Bad Request`
- [ ] Settings screen shows three timer radio buttons (60s, 120s checked, 180s)
- [ ] Settings screen shows three font radio buttons (Chalk checked, Clean, Retro)
- [ ] Font preview text in settings uses the actual font
- [ ] Selecting a timer option sends the correct duration to the backend
- [ ] Selecting a font option changes the board cell font during gameplay
- [ ] Font change is scoped to board cells only
- [ ] CLI continues to compile and use the 120s default
- [ ] All existing tests pass after `NewGameSession` signature update
- [ ] New duration validation tests pass
- [ ] New API duration tests pass
- [ ] `go test ./...` passes
- [ ] `gofmt` clean

## Test Strategy

- Table-driven `TestValidateDuration` for all boundary cases
- `TestNewGameSessionCustomDuration` to verify non-default expiry
- `TestCreateGameWithDuration` API integration test for valid custom duration
- `TestCreateGameInvalidDuration` API integration test for rejected invalid duration
- Run full suite: `go test ./...`
- Manual verification: settings screen, timer works at 60s/180s, fonts render correctly on board

## Open Questions

None.
