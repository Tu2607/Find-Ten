package game

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRunGameProcessesMoveEvents(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{9, 9},
		},
	}
	rebuildValidMoveCache(state)

	moves := make(chan MoveRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 2)
	moves <- MoveRequest{
		Selection: Selection{
			Start: Position{Row: 0, Col: 0},
			End:   Position{Row: 0, Col: 1},
		},
	}

	RunGame(context.Background(), moves, expired, snapshots, state, time.Now().Add(time.Minute))

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

func TestRunGameEndsWhenTimerExpires(t *testing.T) {
	state := &GameState{}
	moves := make(chan MoveRequest)
	expired := make(chan struct{}, 1)
	snapshots := make(chan GameSnapshot, 1)
	expired <- struct{}{}

	RunGame(context.Background(), moves, expired, snapshots, state, time.Now().Add(-time.Second))

	if !state.GameOver {
		t.Fatal("state.GameOver = false, want true")
	}
	if state.GameOverReason != GameOverTimeExpired {
		t.Fatalf("state.GameOverReason = %v, want %v", state.GameOverReason, GameOverTimeExpired)
	}
	_, ok := <-snapshots
	if ok {
		t.Fatal("received timer-expiry snapshot, want no snapshot")
	}
}

func TestRunGameReturnsWhenContextIsCanceled(t *testing.T) {
	state := &GameState{}
	moves := make(chan MoveRequest)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		RunGame(ctx, moves, expired, snapshots, state, time.Now().Add(time.Minute))
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("RunGame did not return after context cancellation")
	}
}

func TestRunGameClosesSnapshotsWhenItReturns(t *testing.T) {
	state := &GameState{}
	moves := make(chan MoveRequest)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	close(moves)

	RunGame(context.Background(), moves, expired, snapshots, state, time.Now().Add(time.Minute))

	_, ok := <-snapshots
	if ok {
		t.Fatal("snapshots channel is open, want closed after RunGame returns")
	}
}

func TestRunGameDoesNotEmitStartupSnapshot(t *testing.T) {
	state := &GameState{}
	moves := make(chan MoveRequest)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	close(moves)

	RunGame(context.Background(), moves, expired, snapshots, state, time.Now().Add(time.Minute))

	_, ok := <-snapshots
	if ok {
		t.Fatal("received startup snapshot, want RunGame to emit only after runtime events")
	}
}

func TestRunGameEmitsMoveSnapshot(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{5, 5},
		},
	}
	rebuildValidMoveCache(state)

	moves := make(chan MoveRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	moves <- MoveRequest{Selection: Selection{
		Start: Position{Row: 0, Col: 0},
		End:   Position{Row: 0, Col: 1},
	}}
	close(moves)

	RunGame(context.Background(), moves, expired, snapshots, state, time.Now().Add(time.Minute))

	first := <-snapshots
	if first.Sequence != 2 {
		t.Fatalf("first.Sequence = %d, want 2", first.Sequence)
	}
	if first.Score != 200 {
		t.Fatalf("first.Score = %d, want 200", first.Score)
	}
}

func TestRunGameDoesNotEmitSnapshotWhileOnlyTimePassesBeforeExpiry(t *testing.T) {
	state := &GameState{}
	moves := make(chan MoveRequest)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		RunGame(ctx, moves, expired, snapshots, state, time.Now().Add(time.Minute))
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()
	<-done

	_, ok := <-snapshots
	if ok {
		t.Fatal("received snapshot without a successful move")
	}
}

func TestRunGameSnapshotBoardIsDeepCopy(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{9, 9},
		},
	}
	rebuildValidMoveCache(state)

	moves := make(chan MoveRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 2)
	moves <- MoveRequest{Selection: Selection{
		Start: Position{Row: 0, Col: 0},
		End:   Position{Row: 0, Col: 1},
	}}

	RunGame(context.Background(), moves, expired, snapshots, state, time.Now().Add(time.Minute))

	moveSnapshot := <-snapshots
	if moveSnapshot.Board[0][0] != 0 {
		t.Fatalf("moveSnapshot.Board[0][0] = %d, want cleared cell", moveSnapshot.Board[0][0])
	}
	state.Board[0][0] = 7
	if moveSnapshot.Board[0][0] != 0 {
		t.Fatalf("moveSnapshot.Board[0][0] = %d after state mutation, want snapshot copy to remain 0", moveSnapshot.Board[0][0])
	}
}

func TestRunGameMoveResultReturnsError(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{9, 9},
		},
	}
	rebuildValidMoveCache(state)

	moves := make(chan MoveRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 2)
	results := make(chan error, 1)
	moves <- MoveRequest{
		Selection: Selection{
			Start: Position{Row: 0, Col: 0},
			End:   Position{Row: 0, Col: 1},
		},
		Result: results,
	}

	RunGame(context.Background(), moves, expired, snapshots, state, time.Now().Add(time.Minute))

	if err := <-results; err != nil {
		t.Fatalf("move result error = %v, want nil", err)
	}

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
	}
	rebuildValidMoveCache(state)

	moves := make(chan MoveRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 2)
	results := make(chan error, 1)
	moves <- MoveRequest{
		Selection: Selection{
			Start: Position{Row: 1, Col: 0},
			End:   Position{Row: 1, Col: 1},
		},
		Result: results,
	}
	close(moves)

	RunGame(context.Background(), moves, expired, snapshots, state, time.Now().Add(time.Minute))

	if err := <-results; err == nil {
		t.Fatal("move result error = nil, want invalid move error")
	}

	_, ok := <-snapshots
	if ok {
		t.Fatal("received snapshot after invalid move, want no runtime snapshot")
	}
}

func TestRunGameRejectsMoveProcessedAfterExpiry(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
		},
	}
	rebuildValidMoveCache(state)

	moves := make(chan MoveRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	results := make(chan error, 1)
	moves <- MoveRequest{
		Selection: Selection{
			Start: Position{Row: 0, Col: 0},
			End:   Position{Row: 0, Col: 1},
		},
		Result: results,
	}

	RunGame(context.Background(), moves, expired, snapshots, state, time.Now().Add(-time.Second))

	if err := <-results; err != ErrGameOver {
		t.Fatalf("move result error = %v, want %v", err, ErrGameOver)
	}
	if state.GameOverReason != GameOverTimeExpired {
		t.Fatalf("state.GameOverReason = %v, want %v", state.GameOverReason, GameOverTimeExpired)
	}
	_, ok := <-snapshots
	if ok {
		t.Fatal("received snapshot for expired move, want none")
	}
}

func TestNewGameSessionReturnsInitialSnapshot(t *testing.T) {
	session, snapshot, err := NewGameSession(context.Background(), MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}
	defer session.Stop()

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

func TestGameSessionExpiresAt(t *testing.T) {
	before := time.Now()
	session, _, err := NewGameSession(context.Background(), MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}
	defer session.Stop()
	after := time.Now()

	expiresAt := session.ExpiresAt()
	if expiresAt.Before(before.Add(DefaultGameDurationSeconds * time.Second)) {
		t.Fatalf("ExpiresAt() = %v, want at or after %v", expiresAt, before.Add(DefaultGameDurationSeconds*time.Second))
	}
	if expiresAt.After(after.Add(DefaultGameDurationSeconds * time.Second)) {
		t.Fatalf("ExpiresAt() = %v, want at or before %v", expiresAt, after.Add(DefaultGameDurationSeconds*time.Second))
	}
}

func TestGameSessionSubmitMove(t *testing.T) {
	session, initialSnapshot, err := NewGameSession(context.Background(), MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}
	defer session.Stop()

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

func TestGameSessionSubmitMoveDoesNotWaitForRuntimeSnapshotReceiver(t *testing.T) {
	session, initialSnapshot, err := NewGameSession(context.Background(), MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}
	defer session.Stop()

	move, ok := firstValidSelection(initialSnapshot.Board)
	if !ok {
		t.Fatal("firstValidSelection returned false, want a valid move")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := session.SubmitMove(ctx, move); err != nil {
		t.Fatalf("SubmitMove returned unexpected error before snapshot receiver consumed runtime snapshots: %v", err)
	}

	select {
	case snapshot := <-session.Snapshots():
		if snapshot.Sequence != 2 {
			t.Fatalf("snapshot.Sequence = %d, want 2", snapshot.Sequence)
		}
	case <-time.After(time.Second):
		t.Fatal("runtime snapshot was not published after SubmitMove")
	}
}

func TestGameSessionStopCancelsLifecycle(t *testing.T) {
	session, _, err := NewGameSession(context.Background(), MinSupportedBoardSize)
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

func TestGameSessionSubmitMoveAfterExpiryReturnsGameOver(t *testing.T) {
	session := &GameSession{
		expiresAt: time.Now().Add(-time.Second),
		done:      make(chan struct{}),
		closing:   make(chan struct{}),
	}

	if err := session.SubmitMove(context.Background(), Selection{}); err != ErrGameOver {
		t.Fatalf("SubmitMove error = %v, want %v", err, ErrGameOver)
	}
}

func TestGameSessionSubmitMoveAfterStopReturnsSessionClosed(t *testing.T) {
	session, _, err := NewGameSession(context.Background(), MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}

	session.Stop()
	<-session.Done()

	if err := session.SubmitMove(context.Background(), Selection{}); err != ErrSessionClosed {
		t.Fatalf("SubmitMove error = %v, want %v", err, ErrSessionClosed)
	}
}

func TestGameSessionClosesMoveChannelAfterStop(t *testing.T) {
	session, _, err := NewGameSession(context.Background(), MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}

	session.Stop()
	<-session.Done()

	select {
	case _, ok := <-session.moves:
		if ok {
			t.Fatal("session.moves is open, want closed")
		}
	default:
		t.Fatal("session.moves is not closed after session done")
	}
}

func TestGameSessionConcurrentSubmitMoveAndStopDoesNotPanic(t *testing.T) {
	session, _, err := NewGameSession(context.Background(), MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = session.SubmitMove(ctx, Selection{
				Start: Position{Row: 99, Col: 99},
				End:   Position{Row: 99, Col: 99},
			})
		}()
	}

	session.Stop()
	wg.Wait()

	select {
	case <-session.Done():
	case <-time.After(time.Second):
		t.Fatal("session did not finish after Stop")
	}
}

func TestGameSessionSnapshotsCloseWhenGameEnds(t *testing.T) {
	session, _, err := NewGameSession(context.Background(), MinSupportedBoardSize)
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

func TestStartExpiryTimerSendsExpiryAndClosesChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	expired := make(chan struct{})
	go startExpiryTimer(ctx, time.Now().Add(time.Millisecond), expired)

	select {
	case <-expired:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("startExpiryTimer did not send expiry")
	}

	_, ok := <-expired
	if ok {
		t.Fatal("expired channel is open, want closed after expiry signal")
	}
}

func firstValidSelection(board Board) (Selection, bool) {
	validMoves, _ := buildValidMoveCache(board)
	if len(validMoves) == 0 {
		return Selection{}, false
	}

	return validMoves[0], true
}
