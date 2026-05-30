package game

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrSessionClosed = errors.New("game session is closed")

func NewGameSession(ctx context.Context, size int) (*GameSession, error) {
	state, err := NewGame(size)
	if err != nil {
		return nil, err
	}

	sessionCtx, cancel := context.WithCancel(ctx)
	session := &GameSession{
		events:    make(chan Event),
		snapshots: make(chan GameSnapshot),
		cancel:    cancel,
		done:      make(chan struct{}),
	}

	go session.run(sessionCtx, state)

	return session, nil
}

func (s *GameSession) run(ctx context.Context, state *GameState) {
	defer close(s.done)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		RunGame(ctx, s.events, s.snapshots, state)
		s.Stop()
	}()

	go func() {
		defer wg.Done()
		StartTimer(ctx, s.events)
	}()

	wg.Wait()
}

func (s *GameSession) Snapshots() <-chan GameSnapshot {
	return s.snapshots
}

func (s *GameSession) SubmitMove(ctx context.Context, selection Selection) error {
	results := make(chan error, 1)
	event := Event{
		Type:   EventMove,
		Move:   selection,
		Result: results,
	}

	select {
	case s.events <- event:
	case <-s.done:
		return ErrSessionClosed
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-results:
		return err
	case <-s.done:
		return ErrSessionClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *GameSession) Stop() {
	s.once.Do(s.cancel)
}

func (s *GameSession) Done() <-chan struct{} {
	return s.done
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
				if event.Result != nil {
					sendMoveResult(ctx, event.Result, err)
				}
				if err == nil {
					snapshot := newGameSnapshot(state, sequence)
					sequence++
					publishPreparedSnapshot(ctx, snapshots, snapshot)
				}
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

func sendMoveResult(ctx context.Context, results chan<- error, result error) {
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
	snapshot.GameOverReason = state.GameOverReason
	snapshot.RemainingTime = state.RemainingTime
	snapshot.ValidMoveCount = len(state.ValidMoves)

	return snapshot
}
