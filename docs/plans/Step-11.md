## Step 11: Game-Over Reason - Done
- Add a `GameOverReason` enum for rule-based end states:
  - no game-over reason
  - no valid moves remain
  - time expired
- Store the reason on `GameState` because it is gameplay state, not runtime metadata.
- Copy the reason into `GameSnapshot`.
- Set the reason when the game ends:
  - `ApplyMove` sets no-valid-moves when a successful move empties the valid move cache.
  - timer handling sets time-expired when remaining time reaches `0`.
- Update CLI game-over output to display the reason.

Acceptance:
- Tests verify time expiry produces the time-expired reason.
- Tests verify clearing the last valid move produces the no-valid-moves reason.
- Tests verify snapshots include the game-over reason.
- Tests verify CLI output reports the reason when the game ends.

