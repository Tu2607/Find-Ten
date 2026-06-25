package game

import (
	"context"
	"reflect"
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

	actions := make(chan PlayerActionRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 2)
	actions <- PlayerActionRequest{
		Type: PlayerActionMove,
		Selection: Selection{
			Start: Position{Row: 0, Col: 0},
			End:   Position{Row: 0, Col: 1},
		},
	}

	RunGame(context.Background(), actions, expired, snapshots, state, time.Now().Add(time.Minute))

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
	actions := make(chan PlayerActionRequest)
	expired := make(chan struct{}, 1)
	snapshots := make(chan GameSnapshot, 1)
	expired <- struct{}{}

	RunGame(context.Background(), actions, expired, snapshots, state, time.Now().Add(-time.Second))

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
	actions := make(chan PlayerActionRequest)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		RunGame(ctx, actions, expired, snapshots, state, time.Now().Add(time.Minute))
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("RunGame did not return after context cancellation")
	}
}

func TestRunGameClosesSnapshotsWhenItReturns(t *testing.T) {
	state := &GameState{}
	actions := make(chan PlayerActionRequest)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	close(actions)

	RunGame(context.Background(), actions, expired, snapshots, state, time.Now().Add(time.Minute))

	_, ok := <-snapshots
	if ok {
		t.Fatal("snapshots channel is open, want closed after RunGame returns")
	}
}

func TestRunGameDoesNotEmitStartupSnapshot(t *testing.T) {
	state := &GameState{}
	actions := make(chan PlayerActionRequest)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	close(actions)

	RunGame(context.Background(), actions, expired, snapshots, state, time.Now().Add(time.Minute))

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

	actions := make(chan PlayerActionRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	actions <- PlayerActionRequest{
		Type: PlayerActionMove,
		Selection: Selection{
			Start: Position{Row: 0, Col: 0},
			End:   Position{Row: 0, Col: 1},
		},
	}
	close(actions)

	RunGame(context.Background(), actions, expired, snapshots, state, time.Now().Add(time.Minute))

	first := <-snapshots
	if first.Sequence != 2 {
		t.Fatalf("first.Sequence = %d, want 2", first.Sequence)
	}
	if first.Score != 200 {
		t.Fatalf("first.Score = %d, want 200", first.Score)
	}
}

func TestRunGameEmitsReshuffleSnapshot(t *testing.T) {
	state := &GameState{
		Board: testReshuffleBoard(),
	}
	rebuildValidMoveCache(state)

	beforeZeroCount := countZeroCellsForTest(state.Board)
	actions := make(chan PlayerActionRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	actions <- PlayerActionRequest{Type: PlayerActionReshuffle}
	close(actions)

	RunGame(context.Background(), actions, expired, snapshots, state, time.Now().Add(time.Minute))

	snapshot := <-snapshots
	if snapshot.Sequence != 2 {
		t.Fatalf("snapshot.Sequence = %d, want 2", snapshot.Sequence)
	}
	if !snapshot.ReshuffleUsed {
		t.Fatal("snapshot.ReshuffleUsed = false, want true")
	}
	if !state.ReshuffleUsed {
		t.Fatal("state.ReshuffleUsed = false, want true")
	}
	if got := countZeroCellsForTest(snapshot.Board); got != beforeZeroCount {
		t.Fatalf("snapshot zero count = %d, want %d", got, beforeZeroCount)
	}
	if snapshot.ValidMoveCount == 0 {
		t.Fatal("snapshot.ValidMoveCount = 0, want at least one valid move")
	}
}

func TestRunGameEmitsRemoveNumberSnapshot(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
			{1, 9},
		},
		Score: 500,
	}
	rebuildValidMoveCache(state)

	actions := make(chan PlayerActionRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	actions <- PlayerActionRequest{
		Type:     PlayerActionRemoveNumber,
		Position: Position{Row: 0, Col: 0},
	}
	close(actions)

	RunGame(context.Background(), actions, expired, snapshots, state, time.Now().Add(time.Minute))

	snapshot := <-snapshots
	if snapshot.Sequence != 2 {
		t.Fatalf("snapshot.Sequence = %d, want 2", snapshot.Sequence)
	}
	if snapshot.Score != 500 {
		t.Fatalf("snapshot.Score = %d, want 500", snapshot.Score)
	}
	if !snapshot.RemoveNumberUsed {
		t.Fatal("snapshot.RemoveNumberUsed = false, want true")
	}
	if !state.RemoveNumberUsed {
		t.Fatal("state.RemoveNumberUsed = false, want true")
	}
	if snapshot.Board[0][0] != 0 {
		t.Fatalf("snapshot.Board[0][0] = %d, want 0", snapshot.Board[0][0])
	}
}

func TestRunGameSequencesMoveReshuffleAndRemoveNumberSnapshots(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6, 9},
			{1, 9, 9},
			{5, 5, 9},
		},
	}
	rebuildValidMoveCache(state)

	actions := make(chan PlayerActionRequest, 3)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 3)
	actions <- PlayerActionRequest{
		Type:     PlayerActionRemoveNumber,
		Position: Position{Row: 1, Col: 0},
	}
	actions <- PlayerActionRequest{
		Type: PlayerActionMove,
		Selection: Selection{
			Start: Position{Row: 0, Col: 0},
			End:   Position{Row: 0, Col: 1},
		},
	}
	actions <- PlayerActionRequest{Type: PlayerActionReshuffle}
	close(actions)

	RunGame(context.Background(), actions, expired, snapshots, state, time.Now().Add(time.Minute))

	first := <-snapshots
	second := <-snapshots
	third := <-snapshots
	if first.Sequence != 2 {
		t.Fatalf("first.Sequence = %d, want 2", first.Sequence)
	}
	if second.Sequence != 3 {
		t.Fatalf("second.Sequence = %d, want 3", second.Sequence)
	}
	if third.Sequence != 4 {
		t.Fatalf("third.Sequence = %d, want 4", third.Sequence)
	}
	if !first.RemoveNumberUsed {
		t.Fatal("first.RemoveNumberUsed = false, want true")
	}
	if !third.ReshuffleUsed {
		t.Fatal("third.ReshuffleUsed = false, want true")
	}
}

func TestRunGameDoesNotEmitSnapshotWhileOnlyTimePassesBeforeExpiry(t *testing.T) {
	state := &GameState{}
	actions := make(chan PlayerActionRequest)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		RunGame(ctx, actions, expired, snapshots, state, time.Now().Add(time.Minute))
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

	actions := make(chan PlayerActionRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 2)
	actions <- PlayerActionRequest{
		Type: PlayerActionMove,
		Selection: Selection{
			Start: Position{Row: 0, Col: 0},
			End:   Position{Row: 0, Col: 1},
		},
	}

	RunGame(context.Background(), actions, expired, snapshots, state, time.Now().Add(time.Minute))

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

	actions := make(chan PlayerActionRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 2)
	results := make(chan error, 1)
	actions <- PlayerActionRequest{
		Type: PlayerActionMove,
		Selection: Selection{
			Start: Position{Row: 0, Col: 0},
			End:   Position{Row: 0, Col: 1},
		},
		Result: results,
	}

	RunGame(context.Background(), actions, expired, snapshots, state, time.Now().Add(time.Minute))

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

	actions := make(chan PlayerActionRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 2)
	results := make(chan error, 1)
	actions <- PlayerActionRequest{
		Type: PlayerActionMove,
		Selection: Selection{
			Start: Position{Row: 1, Col: 0},
			End:   Position{Row: 1, Col: 1},
		},
		Result: results,
	}
	close(actions)

	RunGame(context.Background(), actions, expired, snapshots, state, time.Now().Add(time.Minute))

	if err := <-results; err == nil {
		t.Fatal("move result error = nil, want invalid move error")
	}

	_, ok := <-snapshots
	if ok {
		t.Fatal("received snapshot after invalid move, want no runtime snapshot")
	}
}

func TestRunGameRejectedReshuffleDoesNotEmitSnapshot(t *testing.T) {
	state := &GameState{
		Board:         testReshuffleBoard(),
		ReshuffleUsed: true,
	}
	rebuildValidMoveCache(state)

	actions := make(chan PlayerActionRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	results := make(chan error, 1)
	actions <- PlayerActionRequest{
		Type:   PlayerActionReshuffle,
		Result: results,
	}
	close(actions)

	RunGame(context.Background(), actions, expired, snapshots, state, time.Now().Add(time.Minute))

	if err := <-results; err != ErrReshuffleAlreadyUsed {
		t.Fatalf("reshuffle result error = %v, want %v", err, ErrReshuffleAlreadyUsed)
	}
	_, ok := <-snapshots
	if ok {
		t.Fatal("received snapshot after rejected reshuffle, want none")
	}
}

func TestRunGameRejectedRemoveNumberDoesNotEmitSnapshotOrConsumeSkill(t *testing.T) {
	state := &GameState{
		Board: Board{
			{0, 6},
			{1, 9},
		},
	}
	rebuildValidMoveCache(state)

	actions := make(chan PlayerActionRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	results := make(chan error, 1)
	actions <- PlayerActionRequest{
		Type:     PlayerActionRemoveNumber,
		Position: Position{Row: 0, Col: 0},
		Result:   results,
	}
	close(actions)

	RunGame(context.Background(), actions, expired, snapshots, state, time.Now().Add(time.Minute))

	if err := <-results; err != ErrRemoveNumberInvalidTarget {
		t.Fatalf("remove-number result error = %v, want %v", err, ErrRemoveNumberInvalidTarget)
	}
	if state.RemoveNumberUsed {
		t.Fatal("state.RemoveNumberUsed = true, want false")
	}
	_, ok := <-snapshots
	if ok {
		t.Fatal("received snapshot after rejected remove-number, want none")
	}
}

func TestRunGameUnknownActionDoesNotEmitSnapshot(t *testing.T) {
	state := &GameState{
		Board: testReshuffleBoard(),
	}
	rebuildValidMoveCache(state)
	beforeBoard := cloneBoardForTest(state.Board)

	actions := make(chan PlayerActionRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	results := make(chan error, 1)
	actions <- PlayerActionRequest{Result: results}
	close(actions)

	RunGame(context.Background(), actions, expired, snapshots, state, time.Now().Add(time.Minute))

	if err := <-results; err != ErrUnknownPlayerAction {
		t.Fatalf("action result error = %v, want %v", err, ErrUnknownPlayerAction)
	}
	if !reflect.DeepEqual(state.Board, beforeBoard) {
		t.Fatalf("state.Board mutated to %+v, want %+v", state.Board, beforeBoard)
	}
	_, ok := <-snapshots
	if ok {
		t.Fatal("received snapshot after unknown action, want none")
	}
}

func TestRunGameRejectsMoveProcessedAfterExpiry(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
		},
	}
	rebuildValidMoveCache(state)

	actions := make(chan PlayerActionRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	results := make(chan error, 1)
	actions <- PlayerActionRequest{
		Type: PlayerActionMove,
		Selection: Selection{
			Start: Position{Row: 0, Col: 0},
			End:   Position{Row: 0, Col: 1},
		},
		Result: results,
	}

	RunGame(context.Background(), actions, expired, snapshots, state, time.Now().Add(-time.Second))

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

func TestRunGameRejectsReshuffleProcessedAfterExpiry(t *testing.T) {
	state := &GameState{
		Board: testReshuffleBoard(),
	}
	rebuildValidMoveCache(state)

	actions := make(chan PlayerActionRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	results := make(chan error, 1)
	actions <- PlayerActionRequest{
		Type:   PlayerActionReshuffle,
		Result: results,
	}

	RunGame(context.Background(), actions, expired, snapshots, state, time.Now().Add(-time.Second))

	if err := <-results; err != ErrGameOver {
		t.Fatalf("reshuffle result error = %v, want %v", err, ErrGameOver)
	}
	if state.GameOverReason != GameOverTimeExpired {
		t.Fatalf("state.GameOverReason = %v, want %v", state.GameOverReason, GameOverTimeExpired)
	}
	_, ok := <-snapshots
	if ok {
		t.Fatal("received snapshot for expired reshuffle, want none")
	}
}

func TestRunGameRejectsRemoveNumberProcessedAfterExpiry(t *testing.T) {
	state := &GameState{
		Board: Board{
			{4, 6},
		},
	}
	rebuildValidMoveCache(state)

	actions := make(chan PlayerActionRequest, 1)
	expired := make(chan struct{})
	snapshots := make(chan GameSnapshot, 1)
	results := make(chan error, 1)
	actions <- PlayerActionRequest{
		Type:     PlayerActionRemoveNumber,
		Position: Position{Row: 0, Col: 0},
		Result:   results,
	}

	RunGame(context.Background(), actions, expired, snapshots, state, time.Now().Add(-time.Second))

	if err := <-results; err != ErrGameOver {
		t.Fatalf("remove-number result error = %v, want %v", err, ErrGameOver)
	}
	if state.GameOverReason != GameOverTimeExpired {
		t.Fatalf("state.GameOverReason = %v, want %v", state.GameOverReason, GameOverTimeExpired)
	}
	_, ok := <-snapshots
	if ok {
		t.Fatal("received snapshot for expired remove-number, want none")
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
	if snapshot.ReshuffleUsed {
		t.Fatal("snapshot.ReshuffleUsed = true, want false")
	}
	if snapshot.RemoveNumberUsed {
		t.Fatal("snapshot.RemoveNumberUsed = true, want false")
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

func TestGameSessionSubmitReshuffle(t *testing.T) {
	session, initialSnapshot, err := NewGameSession(context.Background(), MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}
	defer session.Stop()

	beforeZeroCount := countZeroCellsForTest(initialSnapshot.Board)

	if err := session.SubmitReshuffle(context.Background()); err != nil {
		t.Fatalf("SubmitReshuffle returned unexpected error: %v", err)
	}

	reshuffleSnapshot := <-session.Snapshots()
	if !reshuffleSnapshot.ReshuffleUsed {
		t.Fatal("reshuffleSnapshot.ReshuffleUsed = false, want true")
	}
	if got := countZeroCellsForTest(reshuffleSnapshot.Board); got != beforeZeroCount {
		t.Fatalf("reshuffle snapshot zero count = %d, want %d", got, beforeZeroCount)
	}
	if reshuffleSnapshot.ValidMoveCount == 0 {
		t.Fatal("reshuffleSnapshot.ValidMoveCount = 0, want at least one valid move")
	}
}

func TestGameSessionSubmitRemoveNumber(t *testing.T) {
	session, initialSnapshot, err := NewGameSession(context.Background(), MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}
	defer session.Stop()

	position, ok := firstNonZeroPosition(initialSnapshot.Board)
	if !ok {
		t.Fatal("firstNonZeroPosition returned false, want a removable number")
	}

	if err := session.SubmitRemoveNumber(context.Background(), position); err != nil {
		t.Fatalf("SubmitRemoveNumber returned unexpected error: %v", err)
	}

	removeSnapshot := <-session.Snapshots()
	if !removeSnapshot.RemoveNumberUsed {
		t.Fatal("removeSnapshot.RemoveNumberUsed = false, want true")
	}
	if removeSnapshot.Score != initialSnapshot.Score {
		t.Fatalf("removeSnapshot.Score = %d, want %d", removeSnapshot.Score, initialSnapshot.Score)
	}
	if removeSnapshot.Board[position.Row][position.Col] != 0 {
		t.Fatalf("removeSnapshot.Board[%d][%d] = %d, want 0", position.Row, position.Col, removeSnapshot.Board[position.Row][position.Col])
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

func TestGameSessionSubmitReshuffleAfterExpiryReturnsGameOver(t *testing.T) {
	session := &GameSession{
		expiresAt: time.Now().Add(-time.Second),
		done:      make(chan struct{}),
		closing:   make(chan struct{}),
	}

	if err := session.SubmitReshuffle(context.Background()); err != ErrGameOver {
		t.Fatalf("SubmitReshuffle error = %v, want %v", err, ErrGameOver)
	}
}

func TestGameSessionSubmitRemoveNumberAfterExpiryReturnsGameOver(t *testing.T) {
	session := &GameSession{
		expiresAt: time.Now().Add(-time.Second),
		done:      make(chan struct{}),
		closing:   make(chan struct{}),
	}

	if err := session.SubmitRemoveNumber(context.Background(), Position{}); err != ErrGameOver {
		t.Fatalf("SubmitRemoveNumber error = %v, want %v", err, ErrGameOver)
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

func TestGameSessionSubmitReshuffleAfterStopReturnsSessionClosed(t *testing.T) {
	session, _, err := NewGameSession(context.Background(), MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}

	session.Stop()
	<-session.Done()

	if err := session.SubmitReshuffle(context.Background()); err != ErrSessionClosed {
		t.Fatalf("SubmitReshuffle error = %v, want %v", err, ErrSessionClosed)
	}
}

func TestGameSessionSubmitRemoveNumberAfterStopReturnsSessionClosed(t *testing.T) {
	session, _, err := NewGameSession(context.Background(), MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}

	session.Stop()
	<-session.Done()

	if err := session.SubmitRemoveNumber(context.Background(), Position{}); err != ErrSessionClosed {
		t.Fatalf("SubmitRemoveNumber error = %v, want %v", err, ErrSessionClosed)
	}
}

func TestGameSessionClosesActionChannelAfterStop(t *testing.T) {
	session, _, err := NewGameSession(context.Background(), MinSupportedBoardSize)
	if err != nil {
		t.Fatalf("NewGameSession returned unexpected error: %v", err)
	}

	session.Stop()
	<-session.Done()

	select {
	case _, ok := <-session.actions:
		if ok {
			t.Fatal("session.actions is open, want closed")
		}
	default:
		t.Fatal("session.actions is not closed after session done")
	}
}

func TestGameSessionConcurrentSubmitActionsAndStopDoesNotPanic(t *testing.T) {
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
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = session.SubmitReshuffle(ctx)
		}()
	}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = session.SubmitRemoveNumber(ctx, Position{Row: 99, Col: 99})
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

func firstNonZeroPosition(board Board) (Position, bool) {
	for row := range board {
		for col := range board[row] {
			if board[row][col] != 0 {
				return Position{Row: row, Col: col}, true
			}
		}
	}

	return Position{}, false
}
