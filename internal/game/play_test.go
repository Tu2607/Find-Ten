package game

import (
	"context"
	"testing"
	"time"
)

func TestRunGameProcessesMoveEvents(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{9, 9},
		},
		RemainingTime: DefaultGameDurationSeconds,
	}
	rebuildValidMoveCache(state)

	events := make(chan Event, 1)
	events <- NewEvent(EventMove, Selection{
		Start: Position{Row: 0, Col: 0},
		End:   Position{Row: 0, Col: 1},
	})

	RunGame(context.Background(), events, state)

	if state.Board[0][0] != 0 || state.Board[0][1] != 0 {
		t.Fatalf("state.Board top row = %+v, want cleared cells", state.Board[0])
	}
	if state.Score != 200 {
		t.Fatalf("state.Score = %d, want 200", state.Score)
	}
	if !state.GameOver {
		t.Fatal("state.GameOver = false, want true")
	}
}

func TestRunGameProcessesTickEvents(t *testing.T) {
	state := &GameState{
		RemainingTime: 2,
	}

	events := make(chan Event, 1)
	events <- NewEvent(EventTick, Selection{})
	close(events)

	RunGame(context.Background(), events, state)

	if state.RemainingTime != 1 {
		t.Fatalf("state.RemainingTime = %d, want 1", state.RemainingTime)
	}
	if state.GameOver {
		t.Fatal("state.GameOver = true, want false")
	}
}

func TestRunGameEndsWhenTimerExpires(t *testing.T) {
	state := &GameState{
		RemainingTime: 1,
	}

	events := make(chan Event, 1)
	events <- NewEvent(EventTick, Selection{})

	RunGame(context.Background(), events, state)

	if state.RemainingTime != 0 {
		t.Fatalf("state.RemainingTime = %d, want 0", state.RemainingTime)
	}
	if !state.GameOver {
		t.Fatal("state.GameOver = false, want true")
	}
}

func TestRunGameReturnsWhenContextIsCanceled(t *testing.T) {
	state := &GameState{RemainingTime: DefaultGameDurationSeconds}
	events := make(chan Event)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		RunGame(ctx, events, state)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("RunGame did not return after context cancellation")
	}
}

func TestStartTimerSendsTickEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan Event, 1)
	go startTimer(ctx, events, time.Millisecond)

	select {
	case event := <-events:
		if event.Type != EventTick {
			t.Fatalf("event.Type = %v, want %v", event.Type, EventTick)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("startTimer did not send EventTick")
	}
}
