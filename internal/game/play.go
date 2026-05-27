package game

import (
	"context"
	"time"
)

type EventType int

// Enum for different event types
const (
	EventMove EventType = iota
	EventTick
)

type Event struct {
	Type   EventType
	Move   Selection
	Result chan MoveResult
}

func NewEvent(event EventType, move Selection) Event {
	return Event{
		Type: event,
		Move: move,
	}
}

// Receive events and update the game state
// Remember this: This function does not own the events channel, DO NOT CLOSE IT HERE
func RunGame(ctx context.Context, events <-chan Event, snapshots chan<- GameSnapshot, state *GameState) {
	defer close(snapshots)

	var sequence int64 = 1
	publishSnapshot(ctx, snapshots, state, &sequence)

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
				err := ApplyMove(state, event.Move)
				snapshot := newGameSnapshot(state, sequence)
				sequence++
				if event.Result != nil {
					sendMoveResult(ctx, event.Result, MoveResult{Err: err, Snapshot: snapshot})
				}
				publishPreparedSnapshot(ctx, snapshots, snapshot)
			case EventTick:
				updateTimer(state)
				publishSnapshot(ctx, snapshots, state, &sequence)
			}

			if state == nil || state.GameOver {
				return
			}
		}
	}
}

func publishSnapshot(ctx context.Context, snapshots chan<- GameSnapshot, state *GameState, sequence *int64) {
	snapshot := newGameSnapshot(state, *sequence)
	(*sequence)++
	publishPreparedSnapshot(ctx, snapshots, snapshot)
}

func publishPreparedSnapshot(ctx context.Context, snapshots chan<- GameSnapshot, snapshot GameSnapshot) {
	select {
	case <-ctx.Done():
	case snapshots <- snapshot:
	}
}

func sendMoveResult(ctx context.Context, results chan<- MoveResult, result MoveResult) {
	select {
	case <-ctx.Done():
	case results <- result:
	}
}

func newGameSnapshot(state *GameState, sequence int64) GameSnapshot {
	snapshot := GameSnapshot{
		Sequence:     sequence,
		SnapshotTime: time.Now(),
	}
	if state == nil {
		return snapshot
	}

	snapshot.Board = cloneBoard(state.Board)
	snapshot.Score = state.Score
	snapshot.GameOver = state.GameOver
	snapshot.RemainingTime = state.RemainingTime
	snapshot.ValidMoveCount = len(state.ValidMoves)

	return snapshot
}
