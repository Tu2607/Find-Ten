# Goal

## Product Goal

Build a Go backend-first clone of the AZX Service Time style puzzle minigame.

The game should:
- generate a grid of digits
- let the player select rectangle regions whose values sum to exactly `10`
- clear successful selections
- track score
- end when time expires or no valid moves remain

## MVP Scope

The MVP focuses on backend game logic before WebUI work.

Included:
- board sizes `9x9`, `10x10`, and `11x11`
- rectangle selections
- cleared cells represented as backend value `0`
- valid move detection
- scoring
- timer/game-over behavior
- CLI demo as the first manual testing surface

Excluded for MVP:
- WebUI
- multiplayer
- collapse/refill behavior

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

The default game duration is `120` seconds.

The game ends when:
- the timer reaches `0`
- or no valid moves remain

## Future Direction

Future work may add:
- WebUI rendering and input
- WebSocket transport
- same-board multiplayer race mode
- hints
- replay validation
- AI/bot move selection
- difficulty analysis
