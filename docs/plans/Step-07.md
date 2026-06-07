## Step 7: Timer/Event Loop - Superseded

Historical note: this step was implemented, but the `EventTick` model is no longer the target runtime design. Step 14 supersedes `EventTick`, per-second `RemainingTime` countdown mutation, and timer ticks flowing through the same event channel as player moves.

- Add event types:
  - `EventMove`
  - `EventTick`
- Add `RunGame(ctx, events, state)` where the loop owns state mutation.
- Track default duration as `120` seconds.
- End the game when remaining time reaches `0`.

Acceptance:
- Tests verify move events are applied serially.
- Tests verify tick events reduce time.
- Tests verify timer expiry sets `GameOver`.

