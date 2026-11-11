# Goal

## Product Goal

Build a Go backend-first clone of the AZX Service Time style puzzle minigame, with the backend remaining authoritative for game rules and state transitions.

The next product milestone is two-player multiplayer competition: players compete for the higher score on separate copies of the same starting board within the same time window.

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

Implemented account foundation (Step 35):
- optional player accounts with password login
- 7-day browser sessions
- account-linked score identity for logged-in submissions

Implemented HTTP hardening (Step 36):
- `8 KiB` limits on JSON request bodies
- Content Security Policy and security headers on application-generated responses

Implemented edge deployment:
- nginx HTTPS termination with HSTS
- per-address rate limiting on login, registration, and game creation

Implemented personal score history (Step 38):
- logged-in players can view their top 10 scores for a selected board size and duration
- broader account statistics, aggregate analysis, global rank, and paginated history remain future work

Planned multiplayer MVP:
- two-player rooms created or joined through a shareable code
- separate game sessions using identical starting boards and a shared match deadline
- a multiplayer coordinator responsible for the two sessions, match timing, and deciding the winner by final score

Out of scope for the current game:
- collapse/refill behavior
- persistent settings
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

## Multiplayer MVP

### Room Flow And Identity

The browser main menu will include a Multiplayer option with two choices:
- Create room: create a multiplayer session and receive a code to share with another player.
- Join room: enter a code to join an existing multiplayer session.

A room is a shared match between two players. The generated room code maps to a multiplayer session ID, which identifies the coordinator. The coordinator tracks both players and their assigned game IDs.

Creating a room creates the multiplayer session while the first player waits. Once the second player has joined and the match is ready to start, the coordinator creates two separate game sessions, each with its own game ID.

### Independent Games

Both players receive independent copies of the same starting board. Each game session owns its own board state and score, so one player's moves do not change the other player's board.

The multiplayer wrapper coordinates the existing game-session machinery, reusing the session store and broker implementation. Each game session has its own broker to keep its snapshot updates separate from the other player's updates. The existing game runtime remains authoritative for gameplay rules and state transitions.

### Shared Timing And Result

The multiplayer coordinator chooses one shared start time, 3 seconds ahead, and creates both game sessions with that start time and the selected duration. Each session's deadline is the start time plus the duration, so both sessions open and close at the same server-side instants. The playing window starts at the shared start time, not when a player submits a first move.

Each game session runs its own timer to the shared deadline, the same way single-player sessions do. Both game loops reject actions before the start time and at or after the deadline, so a late timer cannot give either player extra playing time.

During the 3-second lead-in, the browser shows a countdown screen and hides the board. This gives the server time to finish creating both sessions before play opens. Network latency between each player and the server is outside the game's control; the server's processing time is authoritative for both players.

A player whose game ends early, because the board is cleared or no valid moves remain, waits for the other player to finish. The coordinator collects results only after both game sessions have finished, then compares their final scores. The player with the higher score wins.

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
- replay validation
- AI/bot move selection
- difficulty analysis
- persistent settings or broader player statistics and score history
- unlockable cosmetics or skills
