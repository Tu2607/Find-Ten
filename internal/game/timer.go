package game

import (
	"context"
	"time"
)

// Updates the timer for the game
func updateTimer(state *GameState) {
	if state == nil || state.GameOver {
		return
	}

	if state.RemainingTime > 0 {
		state.RemainingTime--
	}

	if state.RemainingTime <= 0 {
		state.RemainingTime = 0
		state.GameOver = true
		state.GameOverReason = GameOverTimeExpired
	}
}

// Produces a tick event every second
func StartTimer(ctx context.Context, events chan<- Event) {
	startTimer(ctx, events, time.Second)
}

func startTimer(ctx context.Context, events chan<- Event, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			select {
			case <-ctx.Done():
				return
			case events <- NewEvent(EventTick, Selection{}):
			}
		}
	}
}
