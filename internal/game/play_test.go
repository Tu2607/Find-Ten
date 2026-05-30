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
	snapshots := make(chan GameSnapshot, 2)
	events <- NewEvent(EventMove, Selection{
		Start: Position{Row: 0, Col: 0},
		End:   Position{Row: 0, Col: 1},
	})

	RunGame(context.Background(), events, snapshots, state)

	if state.Board[0][0] != 0 || state.Board[0][1] != 0 {
		t.Fatalf("state.Board top row = %+v, want cleared cells", state.Board[0])
	}
	if state.Score != 200 {
		t.Fatalf("state.Score = %d, want 200", state.Score)
	}
	if !state.GameOver {
		t.Fatal("state.GameOver = false, want true")
	}
	if state.GameOverReason != GameOverNoValidMoves {
		t.Fatalf("state.GameOverReason = %v, want %v", state.GameOverReason, GameOverNoValidMoves)
	}
}

func TestRunGameProcessesTickEvents(t *testing.T) {
	state := &GameState{
		RemainingTime: 2,
	}

	events := make(chan Event, 1)
	snapshots := make(chan GameSnapshot, 2)
	events <- NewEvent(EventTick, Selection{})
	close(events)

	RunGame(context.Background(), events, snapshots, state)

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
	snapshots := make(chan GameSnapshot, 2)
	events <- NewEvent(EventTick, Selection{})

	RunGame(context.Background(), events, snapshots, state)

	if state.RemainingTime != 0 {
		t.Fatalf("state.RemainingTime = %d, want 0", state.RemainingTime)
	}
	if !state.GameOver {
		t.Fatal("state.GameOver = false, want true")
	}
	if state.GameOverReason != GameOverTimeExpired {
		t.Fatalf("state.GameOverReason = %v, want %v", state.GameOverReason, GameOverTimeExpired)
	}
}

func TestRunGameReturnsWhenContextIsCanceled(t *testing.T) {
	state := &GameState{RemainingTime: DefaultGameDurationSeconds}
	events := make(chan Event)
	snapshots := make(chan GameSnapshot, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		RunGame(ctx, events, snapshots, state)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("RunGame did not return after context cancellation")
	}
}

func TestRunGameClosesSnapshotsWhenItReturns(t *testing.T) {
	state := &GameState{RemainingTime: DefaultGameDurationSeconds}
	events := make(chan Event)
	snapshots := make(chan GameSnapshot, 1)
	close(events)

	RunGame(context.Background(), events, snapshots, state)

	<-snapshots
	_, ok := <-snapshots
	if ok {
		t.Fatal("snapshots channel is open, want closed after RunGame returns")
	}
}

func TestRunGameEmitsInitialMoveAndTickSnapshots(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{5, 5},
		},
		RemainingTime: 2,
	}
	rebuildValidMoveCache(state)

	events := make(chan Event, 2)
	snapshots := make(chan GameSnapshot, 3)
	events <- NewEvent(EventMove, Selection{
		Start: Position{Row: 0, Col: 0},
		End:   Position{Row: 0, Col: 1},
	})
	events <- NewEvent(EventTick, Selection{})
	close(events)

	RunGame(context.Background(), events, snapshots, state)

	first := <-snapshots
	second := <-snapshots
	third := <-snapshots

	if first.Sequence != 1 {
		t.Fatalf("first.Sequence = %d, want 1", first.Sequence)
	}
	if second.Sequence != 2 {
		t.Fatalf("second.Sequence = %d, want 2", second.Sequence)
	}
	if third.Sequence != 3 {
		t.Fatalf("third.Sequence = %d, want 3", third.Sequence)
	}
	if second.Score != 200 {
		t.Fatalf("second.Score = %d, want 200", second.Score)
	}
	if third.RemainingTime != 1 {
		t.Fatalf("third.RemainingTime = %d, want 1", third.RemainingTime)
	}
}

func TestRunGameSnapshotIncludesGameOverReason(t *testing.T) {
	state := &GameState{
		RemainingTime: 1,
	}

	events := make(chan Event, 1)
	snapshots := make(chan GameSnapshot, 2)
	events <- NewEvent(EventTick, Selection{})

	RunGame(context.Background(), events, snapshots, state)

	<-snapshots
	gameOverSnapshot := <-snapshots
	if !gameOverSnapshot.GameOver {
		t.Fatal("gameOverSnapshot.GameOver = false, want true")
	}
	if gameOverSnapshot.GameOverReason != GameOverTimeExpired {
		t.Fatalf("gameOverSnapshot.GameOverReason = %v, want %v", gameOverSnapshot.GameOverReason, GameOverTimeExpired)
	}
}

func TestRunGameSnapshotBoardIsDeepCopy(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{9, 9},
		},
		RemainingTime: DefaultGameDurationSeconds,
	}
	rebuildValidMoveCache(state)

	events := make(chan Event, 1)
	snapshots := make(chan GameSnapshot, 2)
	events <- NewEvent(EventMove, Selection{
		Start: Position{Row: 0, Col: 0},
		End:   Position{Row: 0, Col: 1},
	})

	RunGame(context.Background(), events, snapshots, state)

	initialSnapshot := <-snapshots
	if initialSnapshot.Board[0][0] != 4 {
		t.Fatalf("initialSnapshot.Board[0][0] = %d, want 4", initialSnapshot.Board[0][0])
	}
	if state.Board[0][0] != 0 {
		t.Fatalf("state.Board[0][0] = %d, want 0 after move", state.Board[0][0])
	}
}

func TestRunGameMoveResultReturnsError(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{9, 9},
		},
		RemainingTime: DefaultGameDurationSeconds,
	}
	rebuildValidMoveCache(state)

	events := make(chan Event, 1)
	snapshots := make(chan GameSnapshot, 2)
	results := make(chan error, 1)
	events <- Event{
		Type: EventMove,
		Move: Selection{
			Start: Position{Row: 0, Col: 0},
			End:   Position{Row: 0, Col: 1},
		},
		Result: results,
	}

	RunGame(context.Background(), events, snapshots, state)

	if err := <-results; err != nil {
		t.Fatalf("move result error = %v, want nil", err)
	}

	<-snapshots
	moveSnapshot := <-snapshots
	if moveSnapshot.Sequence != 2 {
		t.Fatalf("moveSnapshot.Sequence = %d, want 2", moveSnapshot.Sequence)
	}
	if moveSnapshot.Score != 200 {
		t.Fatalf("moveSnapshot.Score = %d, want 200", moveSnapshot.Score)
	}
}

func TestRunGameInvalidMoveDoesNotEmitSnapshot(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{9, 9},
		},
		RemainingTime: DefaultGameDurationSeconds,
	}
	rebuildValidMoveCache(state)

	events := make(chan Event, 1)
	snapshots := make(chan GameSnapshot, 2)
	results := make(chan error, 1)
	events <- Event{
		Type: EventMove,
		Move: Selection{
			Start: Position{Row: 1, Col: 0},
			End:   Position{Row: 1, Col: 1},
		},
		Result: results,
	}
	close(events)

	RunGame(context.Background(), events, snapshots, state)

	if err := <-results; err == nil {
		t.Fatal("move result error = nil, want invalid move error")
	}

	<-snapshots
	_, ok := <-snapshots
	if ok {
		t.Fatal("received snapshot after invalid move, want only initial snapshot")
	}
}

func TestNewGameSessionEmitsInitialSnapshot(t *testing.T) {
	session, err := NewGameSession(context.Background(), MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}
	defer session.Stop()

	snapshot := <-session.Snapshots()
	if snapshot.Sequence != 1 {
		t.Fatalf("snapshot.Sequence = %d, want 1", snapshot.Sequence)
	}
	if len(snapshot.Board) != MinSupportedBoardSize {
		t.Fatalf("len(snapshot.Board) = %d, want %d", len(snapshot.Board), MinSupportedBoardSize)
	}
	if snapshot.ValidMoveCount == 0 {
		t.Fatal("snapshot.ValidMoveCount = 0, want at least one valid move")
	}
}

func TestGameSessionSubmitMove(t *testing.T) {
	session, err := NewGameSession(context.Background(), MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}
	defer session.Stop()

	initialSnapshot := <-session.Snapshots()
	move, ok := firstValidSelection(initialSnapshot.Board)
	if !ok {
		t.Fatal("firstValidSelection returned false, want a valid move")
	}

	if err := session.SubmitMove(context.Background(), move); err != nil {
		t.Fatalf("SubmitMove returned unexpected error: %v", err)
	}

	moveSnapshot := <-session.Snapshots()
	if moveSnapshot.Score == 0 {
		t.Fatal("moveSnapshot.Score = 0, want score after valid move")
	}
}

func TestGameSessionStopCancelsLifecycle(t *testing.T) {
	session, err := NewGameSession(context.Background(), MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}

	session.Stop()

	select {
	case <-session.Done():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("session did not stop after Stop")
	}
}

func TestGameSessionSnapshotsCloseWhenGameEnds(t *testing.T) {
	session, err := NewGameSession(context.Background(), MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}

	session.Stop()
	for range session.Snapshots() {
	}

	select {
	case <-session.Done():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("session did not finish after snapshots closed")
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

func firstValidSelection(board Board) (Selection, bool) {
	validMoves, _ := buildValidMoveCache(board)
	if len(validMoves) == 0 {
		return Selection{}, false
	}

	return validMoves[0], true
}
