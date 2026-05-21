# Architecture

## Overview

This document records architectural decisions for the Go backend-first game implementation. Product goals, MVP scope, gameplay rules, and future feature ideas live in `docs/GOAL.md`.

## Backend Authority

The backend owns authoritative state transitions for:
- board generation
- move validation
- scoring
- game-over detection
- timer-driven state transitions

Clients, including the CLI and future WebUI, must call backend game logic instead of duplicating rules.

## Core Design

The core game logic lives under `internal/game`.

The game state is modeled around:
- `Board`, a rectangular grid of integers
- `Selection`, a rectangular region between two positions
- `GameState`, the authoritative state for one game

Cleared cells are stored as the number `0`. This keeps the board shape stable, avoids nullable cells, and makes prefix sums straightforward.

## Move Validation

Move validation is cache-backed.

Selections are normalized before validation so all drag directions map to one canonical rectangle.

## Valid Move Cache

The repository uses a valid move cache as the authoritative validation mechanism.

When the board changes, the game rebuilds:
- `ValidMoves []Selection`
- an internal `map[Selection]struct{}`

The slice supports CLI display, hints, debugging, and future AI/bot behavior. The map supports O(1) validation of player-submitted moves.

`len(ValidMoves) == 0` means the game is over.

The cache is rebuilt:
- after board generation
- after each successful move

The cache is not rebuilt after rejected moves because the board did not change.

## Prefix Sums

The backend uses a 2D prefix sum table to compute rectangle sums in O(1).

Rebuilding the valid move cache enumerates every possible rectangle and checks its sum using the prefix table.

For the maximum MVP board size, `11x11`, there are only `4,356` possible rectangles. Full cache recomputation after each successful move is intentionally preferred over incremental invalidation because it is simple, deterministic, and fast enough.

Prefix sums are not required for performance at the current board sizes. A naive rectangle sum would also be fast enough for local play, online play, and server-side replay verification. The important architectural decision is the valid move cache; prefix sums are an isolated implementation helper that can be replaced later if direct summing proves clearer.

The project keeps prefix sums for now to test the suggested optimization and to keep cache rebuilds cheap without changing the cache-oriented design.

## Game Loop

The planned runtime model is a single-owner game loop.

One goroutine owns game state and processes events serially:
- move events
- timer tick events

This avoids race-prone shared mutation and creates a direct path to future WebSocket and multiplayer support.

## CLI First

The CLI demo is the first manual testing surface.

It should:
- generate a supported board size
- print the board
- accept zero-based rectangle coordinates
- apply moves through the same backend validation path
- print score, valid move count, and game-over state

The CLI is not a separate rules implementation. It is only a thin client over `internal/game`.
