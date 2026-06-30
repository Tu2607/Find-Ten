## Step 24: Optimistic Move UI and Incremental Board Rendering

### Context

The WebUI currently waits for the full server round-trip (POST move -> server processes -> broker publishes -> SSE delivers snapshot -> `applySnapshot` -> `renderBoard`) before the player sees any visual feedback. On the Oracle Cloud VM this causes noticeable lag. Additionally, `renderBoard` destroys and recreates every cell element via `boardEl.innerHTML = ""`, which can swallow in-progress clicks when the DOM is torn down mid-interaction.

This step fixes both problems entirely on the client side. No server changes are needed.

### Design Intent

Two independent improvements that reinforce each other:

**Optimistic move UI** — When the player completes a rectangle selection, immediately dim the affected cells and update the displayed score before the POST response arrives. If the server rejects the move (400), revert. If accepted, the SSE snapshot overwrites the optimistic state with authoritative data.

**Incremental DOM rendering** — Replace the destructive `innerHTML = ""` rebuild with a diff-based update. Build the full grid once (or on board size change), store cell references in a 2D array, and on subsequent renders update only changed cells in place.

Rules:
- The server remains authoritative. Optimistic state is always overwritten by the confirming snapshot.
- Optimistic scoring: 100 points per non-zero cell in the selection rectangle.
- Only one optimistic move can be pending at a time. A new selection while one is in flight simply overwrites `state.optimisticMove`.
- Reshuffle, remove-number, and hint during an optimistic move need no special handling — `applySnapshot` clears the optimistic state only when the confirming snapshot arrives.
- Timer expiry during an optimistic move clears `state.optimisticMove` and shows game-over state.

### Static WebUI Behavior

#### State additions (`static/app.js`)

Add to the `state` object:

```
optimisticMove: null,    // { affectedCells: [{row, col}], scoreAdded }
cellRefs: [],            // 2D array [row][col] of DOM button elements
lastBoardSize: 0         // tracks board dimensions for rebuild detection
```

#### Incremental rendering (`renderBoard`)

Split into two paths:

1. **Full build** — when `state.cellRefs.length === 0` or `state.board.length !== state.lastBoardSize`. Clears `boardEl.innerHTML`, creates all cell buttons, stores references in `state.cellRefs[row][col]`, sets `state.lastBoardSize`. Attaches click/hover/focus listeners to each cell.

2. **Diff update** — all other calls. Iterates `state.cellRefs` and for each cell:
   - Updates `textContent` only if the value changed.
   - Recomputes CSS classes (`cleared`, `hint`, `clearing`) and applies only if different.
   - Updates `disabled` only if it differs from `state.gameOver`.

`updateSelectionPreview` continues to work as before since it queries stable `.cell` elements.

On game start (`startGame`), reset `state.cellRefs = []` and `state.lastBoardSize = 0` to force a full rebuild.

#### Optimistic move flow

In `handleCellClick`, after computing the `selection` and before calling `submitMove`:

1. Compute the bounding rectangle from `selection.start` and `selection.end`.
2. Collect affected cells: every `{row, col}` in the rectangle where `state.board[row][col] !== 0`.
3. If no affected cells, skip optimistic UI (move will fail anyway).
4. Store `state.optimisticMove = { affectedCells, scoreAdded: affectedCells.length * 100 }`.
5. Add CSS class `clearing` to each affected cell via `state.cellRefs[row][col]`.
6. Update `scoreEl.textContent = state.score + scoreAdded`.

In `submitMove`:
- On success (200): do nothing extra — SSE snapshot will overwrite.
- On failure (400): call `revertOptimisticMove()`.
- On network error: call `revertOptimisticMove()`.
- On 409/410: clear `state.optimisticMove = null`.

#### `revertOptimisticMove` (new function)

Removes `clearing` class from affected cells, restores `scoreEl.textContent` to `state.score`, sets `state.optimisticMove = null`.

#### `applySnapshot` changes — race-safe optimistic clearing

Do not unconditionally clear `state.optimisticMove`. Instead, check whether the incoming snapshot confirms the optimistic move by verifying that all affected cells are now 0 in the snapshot's board:

```js
if (state.optimisticMove) {
  const confirmed = state.optimisticMove.affectedCells.every(
    ({row, col}) => snapshot.board[row][col] === 0
  );
  if (confirmed) {
    state.optimisticMove = null;
  }
  // otherwise keep optimistic visuals — this snapshot predates our move
}
```

This prevents a flicker when rapid moves produce overlapping snapshots:
1. User makes move A → cells dim (optimistic)
2. User immediately makes move B → B's optimistic state replaces A's, B's cells dim
3. Move A's snapshot arrives — B's affected cells are still non-zero in A's snapshot, so optimistic visuals for B are preserved
4. Move B's snapshot arrives — B's affected cells are now 0, optimistic state is cleared

Edge case: if a reshuffle snapshot arrives while an optimistic move is pending, the reshuffled board will likely have non-zero values in the optimistic cells, so the optimistic visuals persist until the move's own snapshot arrives. This is the correct behavior.

Game-over snapshots: when `snapshot.gameOver` is true or `snapshot.validMoveCount === 0`, unconditionally clear `state.optimisticMove = null` regardless of cell values, since the game is over.

#### Timer expiry

In the countdown expiry block in `renderCountdown`, add `state.optimisticMove = null`.

#### CSS additions (`static/styles.css`)

```css
.cell.clearing {
  opacity: 0.35;
  transition: opacity 0.15s ease-out;
  pointer-events: none;
}
```

`pointer-events: none` prevents clicking cells that are optimistically clearing. Distinct from `.cleared` which uses `color: transparent` on a grey background.

### Critical Files

- `static/app.js` — incremental `renderBoard`, optimistic move logic, `revertOptimisticMove`, state additions, `applySnapshot` and `startGame` changes.
- `static/styles.css` — `.cell.clearing` style.

### Testing

No Go tests needed (client-only changes).

WebUI manual verification:

**Optimistic UI:**
- Start a 9x9 game. Select a valid rectangle. Verify cells dim immediately before server response.
- Verify score updates optimistically and matches server value after snapshot.
- Select an invalid rectangle. Verify cells dim briefly then revert on 400 response. Score reverts.
- With network throttling, make a selection then immediately start a new one. Verify both resolve correctly without flicker on the second selection.
- Use Reshuffle/Remove/Hint during an optimistic move. Verify board updates correctly on snapshot.
- Let timer expire during an optimistic move. Verify game-over state.

**Incremental rendering:**
- In DevTools Elements panel, observe cells update in place (not removed/recreated) after a move.
- Start a new game with different board size. Verify full rebuild occurs.
- Rapidly click cells. Verify no clicks are swallowed.

### Verification Commands

```sh
go test ./...
go run ./cmd/server   # manual WebUI check
```

### Acceptance

- Valid moves show immediate visual clearing (opacity dim) before server round-trip completes.
- Score updates optimistically and is corrected to authoritative value on snapshot.
- Invalid moves (400) revert optimistic visuals and score.
- Rapid sequential moves do not flicker — optimistic visuals for the latest move persist until its confirming snapshot arrives.
- Board uses stable DOM elements updated in place, not destroyed and recreated.
- Clicks are no longer swallowed during board updates.
- Skills (reshuffle, remove, hint) work correctly when an optimistic move is pending.
- Timer expiry during an optimistic move shows game-over correctly.
- No server-side changes.
- `go test ./...` continues to pass.
