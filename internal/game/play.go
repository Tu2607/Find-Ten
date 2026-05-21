package game

import (
	"context"
)

type EventType int

// Enum for different event types
const (
	EventMove EventType = iota
	EventTick
)

type Event struct {
	Type EventType
	Move Selection
}

func NewEvent(event EventType, move Selection) Event {
	return Event{
		Type: event,
		Move: move,
	}
}

// Receive events and update the game state
// Remember this: This function does not own the events channel, DO NOT CLOSE IT HERE
func RunGame(ctx context.Context, events <-chan Event, state *GameState) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			switch event.Type {
			case EventMove:
				_ = ApplyMove(state, event.Move)
			case EventTick:
				updateTimer(state)
			}

			if state == nil || state.GameOver {
				return
			}
		}
	}
}
