# Goal

## Product Goal

Build a Go backend-first clone of the AZX Service Time style puzzle minigame, with the backend remaining authoritative for game rules and state transitions.

The game:
- generates a grid of digits
- lets the player select rectangle regions whose values sum to exactly `10`
- clears successful selections
- tracks score
- supports one-use helper skills
- ends when time expires, no valid moves remain, or the board is fully cleared

## Current Scope

Implemented gameplay:
- board sizes `9x9`, `10x10`, and `11x11`
- rectangle selections
- cleared cells represented as backend value `0`
- valid move detection and cached valid move lists
- scoring
- timer/game-over behavior
- one-use reshuffle skill
- one-use remove-number skill
- one-use hint skill
- configurable game durations of `60`, `120`, or `180` seconds

Implemented delivery surfaces:
- CLI play mode
- HTTP API for game lifecycle and player actions
- SSE snapshot stream for browser updates
- browser WebUI with welcome, settings, game, and game-over screens
- configurable board font in the WebUI
- Dockerized server deployment

Implemented persistence:
- SQLite-backed global leaderboard score storage

Step 35 account foundation work:
- optional player accounts with password login
- 7-day browser sessions
- account-linked score identity for logged-in submissions

Out of scope for the current game:
- multiplayer
- collapse/refill behavior
- persistent settings
- personal score history
- unlockable progression

Permanently excluded:
- arbitrary quadrilateral selection

## Gameplay Rules

Generated boards use digits `1-9`.

After a valid move:
- selected cells become `0`
- cells do not collapse
- cells are not refilled
- existing `0` cells may be included in future selections

Selections are rectangle-only. Arbitrary quadrilateral shapes are intentionally excluded because they would make finding valid sums too easy.

A move is valid only when the selected rectangle sums to exactly `10`.

Multiples of `10`, such as `20`, are not valid.

An invalid move does not end the game.

Scoring is based on newly cleared cells:
- each newly cleared non-zero cell is worth `100` points
- already-cleared `0` cells do not score again

The player can choose a game duration of `60`, `120`, or `180` seconds. The default is `120` seconds.

The game ends when:
- the timer reaches `0`
- or no valid moves remain
- or all cells are cleared

## Helper Skills

Each game session has three one-use helper skills:

- Reshuffle: replaces remaining non-zero cells while preserving the number of cleared cells and keeping the score unchanged.
- Remove number: clears one selected non-zero cell without awarding score.
- Hint: reveals one currently valid rectangle without changing the board or score.

Rejected skill attempts do not consume the skill.

## User Interfaces

The project currently supports:

- CLI play for manual terminal testing.
- Browser play through the Go HTTP server.
- Real-time browser updates through SSE snapshots.

The browser settings screen supports:

- board size selection: `9x9`, `10x10`, or `11x11`
- timer selection: `60`, `120`, or `180` seconds
- board font selection: Chalk, Clean, or Retro
- board background color selection: Green, Blue, Red, or Purple

## Future Direction

Future work may add:
- WebSocket transport
- same-board multiplayer race mode
- replay validation
- AI/bot move selection
- difficulty analysis
- persistent settings or score history
- unlockable cosmetics or skills
