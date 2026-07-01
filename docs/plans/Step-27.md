# Step 27 — Disambiguate HTTP Status Codes for Game-Over vs Skill-Already-Used

## Context

`writeMoveError` in `internal/api/handlers.go` maps both `ErrGameOver` and skill-already-used errors (`ErrReshuffleAlreadyUsed`, `ErrRemoveNumberAlreadyUsed`, `ErrHintAlreadyUsed`, `ErrHintNoValidMoves`) to `409 Conflict`. The frontend skill handlers treat 409 as "skill already used" and skip `endGame()`, so if a skill request gets 409 because the game ended mid-request (e.g. timer expired), the UI stays interactive while the server considers the game over.

The SSE snapshot stream usually delivers a game-over event independently, but if SSE disconnects at the wrong moment, the client has no way to distinguish the two cases. The fix: return `422 Unprocessable Entity` for skill-already-used errors, keep `409 Conflict` for game-over, and update the frontend to handle each code correctly.

## Changes

### 1. Backend: `internal/api/handlers.go`

In `writeMoveError`, change the skill-already-used case from `http.StatusConflict` to `http.StatusUnprocessableEntity`. The `ErrGameOver` case stays at `409 Conflict`.

### 2. Backend Tests: `internal/api/server_test.go`

Three integration tests updated from `http.StatusConflict` to `http.StatusUnprocessableEntity` and renamed:

- `TestSubmitReshuffleAlreadyUsedReturnsUnprocessable`
- `TestSubmitRemoveNumberAlreadyUsedReturnsUnprocessable`
- `TestSubmitHintAlreadyUsedReturnsUnprocessable`

`TestWriteMoveErrorMapsGameOverToConflict` unchanged — game-over stays at 409.

### 3. Frontend: `static/app.js`

Split the combined `409 || 410` checks in three skill functions into separate `409`/`410` (game over → `endGame()`) and `422` (skill already used → mark used, update buttons) branches. `submitMove` unchanged.

### 4. Architecture Docs: `docs/ARCHITECTURE.md`

Updated status code documentation in the Remove-Number Submission section to reflect the new 422 code for already-used submissions.

## Files Modified

- `internal/api/handlers.go`
- `internal/api/server_test.go`
- `static/app.js`
- `docs/ARCHITECTURE.md`
