package game

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestNewCoordinatedSessionCopiesStateAndKeepsSessionsIndependent(t *testing.T) {
	source, err := NewGame(MinSupportedBoardSize)
	if err != nil {
		t.Fatal(err)
	}
	wantBoard := cloneBoard(source.Board)
	wantMoves := append([]Selection(nil), source.ValidMoves...)
	move := wantMoves[0]
	wantScore := countNonZeroCells(move, wantBoard) * pointsPerClearedCell
	startsAt := time.Now().Add(time.Second)

	first, firstInitial, err := NewCoordinatedSession(context.Background(), source, startsAt, 60)
	if err != nil {
		t.Fatal(err)
	}
	defer stopCoordinatedSession(t, first)
	second, secondInitial, err := NewCoordinatedSession(context.Background(), source, startsAt, 60)
	if err != nil {
		t.Fatal(err)
	}
	defer stopCoordinatedSession(t, second)
	firstUpdates := drainGameSnapshots(first)
	secondUpdates := drainGameSnapshots(second)

	for name, session := range map[string]*GameSession{"first": first, "second": second} {
		if !session.StartsAt().Equal(startsAt) {
			t.Errorf("%s StartsAt() = %v, want %v", name, session.StartsAt(), startsAt)
		}
		if want := startsAt.Add(60 * time.Second); !session.ExpiresAt().Equal(want) {
			t.Errorf("%s ExpiresAt() = %v, want %v", name, session.ExpiresAt(), want)
		}
	}
	for name, snapshot := range map[string]GameSnapshot{"first": firstInitial, "second": secondInitial} {
		if !reflect.DeepEqual(snapshot.Board, wantBoard) {
			t.Errorf("%s starting board differs from source", name)
		}
		if snapshot.ValidMoveCount != len(wantMoves) || snapshot.Score != 0 || snapshot.GameOver || snapshot.ReshuffleUsed || snapshot.RemoveNumberUsed || snapshot.HintUsed {
			t.Errorf("%s initial snapshot has unexpected state: %+v", name, snapshot)
		}
	}
	if !reflect.DeepEqual(firstInitial.Board, secondInitial.Board) {
		t.Fatal("coordinated sessions have different starting boards")
	}

	// Mutate the source and one returned snapshot after construction.
	source.Board[0][0] = (source.Board[0][0] + 1) % 10
	source.ValidMoves[0] = Selection{Start: Position{Row: -1, Col: -1}, End: Position{Row: -1, Col: -1}}
	for selection := range source.validMoveSet {
		delete(source.validMoveSet, selection)
	}
	source.validMoveSet[Selection{}] = struct{}{}
	mutatedSourceBoard := cloneBoard(source.Board)
	mutatedSourceMoves := append([]Selection(nil), source.ValidMoves...)
	mutatedSourceSet := copySelectionSet(source.validMoveSet)
	firstInitial.Board[0][0] = -1

	waitForCoordinatedStart(t, startsAt)
	hint, err := first.SubmitHint(context.Background())
	if err != nil {
		t.Fatalf("first session hint: %v", err)
	}
	if hint != wantMoves[0] {
		t.Fatalf("first session hint = %+v, want copied first move %+v", hint, wantMoves[0])
	}
	firstHint := nextGameSnapshot(t, firstUpdates)
	if !reflect.DeepEqual(firstHint.Board, wantBoard) || firstHint.Score != 0 || !firstHint.HintUsed || firstHint.ValidMoveCount != len(wantMoves) {
		t.Fatalf("first session hint snapshot = %+v, want unchanged copied state", firstHint)
	}
	if err := second.SubmitMove(context.Background(), move); err != nil {
		t.Fatalf("second session move using copied cache: %v", err)
	}
	secondMove := nextGameSnapshot(t, secondUpdates)
	if secondMove.Score != wantScore || secondMove.HintUsed || !reflect.DeepEqual(secondMove.Board, zeroSelection(move, cloneBoard(wantBoard))) {
		t.Fatalf("second session move snapshot = %+v, want score %d and its own moved board", secondMove, wantScore)
	}
	if err := first.SubmitMove(context.Background(), move); err != nil {
		t.Fatalf("first session move after hint: %v", err)
	}
	firstMove := nextGameSnapshot(t, firstUpdates)
	if firstMove.Score != wantScore || !firstMove.HintUsed || !reflect.DeepEqual(firstMove.Board, secondMove.Board) {
		t.Fatalf("first session move snapshot = %+v, want score %d and same independently moved board", firstMove, wantScore)
	}
	if !reflect.DeepEqual(source.Board, mutatedSourceBoard) || !reflect.DeepEqual(source.ValidMoves, mutatedSourceMoves) || !reflect.DeepEqual(source.validMoveSet, mutatedSourceSet) {
		t.Fatal("a coordinated session mutated the source state")
	}
}

func TestNewCoordinatedSessionValidatesInputsAndSinglePlayerHasNoStartGate(t *testing.T) {
	state, err := NewGame(MinSupportedBoardSize)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	cases := []struct {
		name     string
		start    time.Time
		duration int
	}{
		{name: "unsupported duration", start: now.Add(time.Second), duration: 45},
		{name: "start at now", start: now, duration: 60},
		{name: "start in past", start: now.Add(-time.Second), duration: 60},
		{name: "start too far ahead", start: now.Add(CoordinatedStartDelay + time.Second), duration: 60},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			session, _, err := NewCoordinatedSession(context.Background(), state, tc.start, tc.duration)
			if err == nil || session != nil {
				t.Fatalf("NewCoordinatedSession() = (%v, %v), want nil session and validation error", session, err)
			}
		})
	}

	session, _, err := NewGameSession(context.Background(), MinSupportedBoardSize, 60)
	if err != nil {
		t.Fatal(err)
	}
	defer stopCoordinatedSession(t, session)
	if !session.StartsAt().IsZero() {
		t.Fatalf("single-player StartsAt() = %v, want zero", session.StartsAt())
	}
}

func TestApplyPlayerActionInWindowBoundaries(t *testing.T) {
	base := time.Unix(100, 0)
	startsAt := base.Add(time.Second)
	expiresAt := base.Add(5 * time.Second)
	boundaries := []struct {
		name string
		now  time.Time
		want error
	}{
		{name: "before start", now: startsAt.Add(-time.Nanosecond), want: ErrGameNotStarted},
		{name: "at start", now: startsAt},
		{name: "before expiry", now: expiresAt.Add(-time.Nanosecond)},
		{name: "at expiry", now: expiresAt, want: ErrGameOver},
	}
	actions := []struct {
		name   string
		typeOf PlayerActionType
	}{
		{name: "move", typeOf: PlayerActionMove},
		{name: "reshuffle", typeOf: PlayerActionReshuffle},
		{name: "remove number", typeOf: PlayerActionRemoveNumber},
		{name: "hint", typeOf: PlayerActionHint},
	}
	for _, action := range actions {
		for _, boundary := range boundaries {
			t.Run(action.name+"/"+boundary.name, func(t *testing.T) {
				state := coordinatedTestState()
				before := cloneBoard(state.Board)
				request := coordinatedRequest(action.typeOf, state)
				err := applyPlayerActionInWindow(state, request, startsAt, expiresAt, boundary.now)
				if err != boundary.want {
					t.Fatalf("applyPlayerActionInWindow() error = %v, want %v", err, boundary.want)
				}
				if boundary.want == ErrGameNotStarted && (!reflect.DeepEqual(state.Board, before) || state.Score != 0 || state.ReshuffleUsed || state.RemoveNumberUsed || state.HintUsed) {
					t.Fatalf("rejected pre-start action changed state: %+v", state)
				}
				if boundary.want == ErrGameOver && (!state.GameOver || state.GameOverReason != GameOverTimeExpired) {
					t.Fatalf("deadline action left state %+v, want time-expired game over", state)
				}
			})
		}
	}
}

func TestRunGameChecksStartWindowWhenProcessingAction(t *testing.T) {
	state := coordinatedTestState()
	before := cloneBoard(state.Board)
	ctx, cancel := context.WithCancel(context.Background())
	actions := make(chan PlayerActionRequest, 1)
	result := make(chan error, 1)
	actions <- PlayerActionRequest{Type: PlayerActionHint, Result: result}
	startsAt := time.Now().Add(time.Second)
	snapshots := make(chan GameSnapshot)
	done := make(chan struct{})
	go func() {
		defer close(done)
		RunGame(ctx, actions, make(chan struct{}), snapshots, state, startsAt, startsAt.Add(time.Minute))
	}()
	select {
	case err := <-result:
		if err != ErrGameNotStarted {
			t.Fatalf("RunGame action error = %v, want %v", err, ErrGameNotStarted)
		}
	case <-time.After(time.Second):
		t.Fatal("RunGame did not return a result for the pre-start action")
	}
	cancel()
	waitForDone(t, done)
	if !reflect.DeepEqual(state.Board, before) || state.HintUsed {
		t.Fatalf("RunGame processed pre-start action: %+v", state)
	}
}

func TestCoordinatedSessionStartGateAndExpiry(t *testing.T) {
	startsAt := time.Now().Add(500 * time.Millisecond)
	expiresAt := startsAt.Add(time.Second)
	session, initial := newSession(context.Background(), coordinatedTestState(), startsAt, expiresAt)
	defer stopCoordinatedSession(t, session)
	updates := drainGameSnapshots(session)
	if initial.HintUsed || initial.ReshuffleUsed || initial.RemoveNumberUsed {
		t.Fatalf("initial skills unexpectedly used: %+v", initial)
	}
	if err := session.SubmitMove(context.Background(), Selection{}); err != ErrGameNotStarted {
		t.Errorf("pre-start move error = %v, want %v", err, ErrGameNotStarted)
	}
	if err := session.SubmitReshuffle(context.Background()); err != ErrGameNotStarted {
		t.Errorf("pre-start reshuffle error = %v, want %v", err, ErrGameNotStarted)
	}
	if err := session.SubmitRemoveNumber(context.Background(), Position{}); err != ErrGameNotStarted {
		t.Errorf("pre-start remove error = %v, want %v", err, ErrGameNotStarted)
	}
	if _, err := session.SubmitHint(context.Background()); err != ErrGameNotStarted {
		t.Errorf("pre-start hint error = %v, want %v", err, ErrGameNotStarted)
	}

	waitForCoordinatedStart(t, startsAt)
	if _, err := session.SubmitHint(context.Background()); err != nil {
		t.Fatalf("hint at/after start: %v", err)
	}
	if snapshot := nextGameSnapshot(t, updates); !snapshot.HintUsed {
		t.Fatalf("post-start hint snapshot = %+v, want hint used", snapshot)
	}
	final := nextGameSnapshot(t, updates)
	if !final.GameOver || final.GameOverReason != GameOverTimeExpired {
		t.Fatalf("expiry snapshot = %+v, want time-expired game over", final)
	}
	waitForDone(t, session.Done())
	if got := session.Result(); got != (GameResult{GameOverReason: GameOverTimeExpired, Completed: true}) {
		t.Fatalf("expiry result = %+v, want time-expired completed result", got)
	}
	if err := session.SubmitReshuffle(context.Background()); err != ErrGameOver {
		t.Fatalf("post-expiry action error = %v, want %v", err, ErrGameOver)
	}
}

func TestCoordinatedSessionResultsForTerminalReasons(t *testing.T) {
	cases := []struct {
		name   string
		state  *GameState
		move   Selection
		reason GameOverReason
	}{
		{name: "board cleared", state: stateWithOnlyPair(4, 6), move: pairSelection(), reason: GameOverBoardCleared},
		{name: "no valid moves", state: stateWithMostlyNines(), move: pairSelection(), reason: GameOverNoValidMoves},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			session, _ := newSession(context.Background(), tc.state, time.Time{}, time.Now().Add(time.Minute))
			defer stopCoordinatedSession(t, session)
			updates := drainGameSnapshots(session)
			if err := session.SubmitMove(context.Background(), tc.move); err != nil {
				t.Fatalf("terminal move: %v", err)
			}
			final := nextGameSnapshot(t, updates)
			if !final.GameOver || final.GameOverReason != tc.reason {
				t.Fatalf("final snapshot = %+v, want reason %v", final, tc.reason)
			}
			waitForDone(t, session.Done())
			want := GameResult{Score: 200, GameOverReason: tc.reason, Completed: true}
			if got := session.Result(); got != want {
				t.Fatalf("Result() = %+v, want %+v", got, want)
			}
			if got := session.Result(); got != want {
				t.Fatalf("repeated Result() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestCoordinatedSessionCancellationAndEarlyResultRetention(t *testing.T) {
	for _, stopByParent := range []bool{false, true} {
		name := "Stop"
		if stopByParent {
			name = "parent cancellation"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			session, _ := newSession(ctx, coordinatedTestState(), time.Time{}, time.Now().Add(time.Minute))
			_ = drainGameSnapshots(session)
			if stopByParent {
				cancel()
			} else {
				session.Stop()
			}
			waitForDone(t, session.Done())
			if got := session.Result(); got != (GameResult{}) {
				t.Fatalf("cancelled result = %+v, want incomplete zero result", got)
			}
		})
	}

	first, _ := newSession(context.Background(), stateWithMostlyNines(), time.Time{}, time.Now().Add(time.Minute))
	second, _ := newSession(context.Background(), coordinatedTestState(), time.Time{}, time.Now().Add(time.Minute))
	defer stopCoordinatedSession(t, first)
	defer stopCoordinatedSession(t, second)
	firstUpdates := drainGameSnapshots(first)
	secondUpdates := drainGameSnapshots(second)
	if err := first.SubmitMove(context.Background(), pairSelection()); err != nil {
		t.Fatal(err)
	}
	_ = nextGameSnapshot(t, firstUpdates)
	waitForDone(t, first.Done())
	wantFirst := GameResult{Score: 200, GameOverReason: GameOverNoValidMoves, Completed: true}
	if got := first.Result(); got != wantFirst {
		t.Fatalf("early result = %+v, want %+v", got, wantFirst)
	}
	select {
	case <-second.Done():
		t.Fatal("second session ended before its expiry signal")
	default:
	}
	// Inject expiry so this retention check does not wait for the distant timer.
	second.expired <- struct{}{}
	_ = nextGameSnapshot(t, secondUpdates)
	waitForDone(t, second.Done())
	if got := second.Result(); got.GameOverReason != GameOverTimeExpired || !got.Completed {
		t.Fatalf("second result = %+v, want completed time expiry", got)
	}
	if got := first.Result(); got != wantFirst {
		t.Fatalf("early result changed after other session finished: %+v", got)
	}
}

func TestCoordinatedSessionStopUnblocksFinalSnapshotAndKeepsResult(t *testing.T) {
	session, _ := newSession(context.Background(), stateWithOnlyPair(4, 6), time.Time{}, time.Now().Add(time.Minute))
	defer stopCoordinatedSession(t, session)
	if err := session.SubmitMove(context.Background(), pairSelection()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-session.Done():
		t.Fatal("session completed while its final snapshot had no receiver")
	default:
	}
	session.Stop()
	waitForDone(t, session.Done())
	want := GameResult{Score: 200, GameOverReason: GameOverBoardCleared, Completed: true}
	if got := session.Result(); got != want {
		t.Fatalf("stopped final-snapshot result = %+v, want %+v", got, want)
	}
}

func TestSessionSubmitsReturnWhenSessionEndsWithoutRequestTimeout(t *testing.T) {
	for _, stopByParent := range []bool{false, true} {
		name := "Stop"
		if stopByParent {
			name = "parent cancellation"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			session, _ := newSession(ctx, coordinatedTestState(), time.Time{}, time.Now().Add(time.Minute))
			go func() {
				for range session.Snapshots() {
				}
			}()

			// Background request contexts leave no timeout to fall back on, so each
			// submit returns only if RunGame replies or the session refuses it.
			const submitters = 40
			errs := make(chan error, submitters)
			for i := range submitters {
				go func() {
					if i%2 == 0 {
						errs <- session.SubmitMove(context.Background(), pairSelection())
						return
					}
					_, err := session.SubmitHint(context.Background())
					errs <- err
				}()
			}

			// End the session while the remaining submits are still in flight.
			results := []error{nextSubmitError(t, errs)}
			if stopByParent {
				cancel()
			} else {
				session.Stop()
			}
			for len(results) < submitters {
				results = append(results, nextSubmitError(t, errs))
			}
			waitForDone(t, session.Done())

			for _, err := range results {
				switch {
				case err == nil,
					errors.Is(err, ErrInvalidMove),
					errors.Is(err, ErrHintAlreadyUsed),
					errors.Is(err, ErrSessionClosed),
					errors.Is(err, ErrGameOver):
				default:
					t.Fatalf("submit error = %v, want nil, a game-rule error, or a closed-session error", err)
				}
			}
		})
	}
}

func coordinatedTestState() *GameState {
	board := make(Board, MinSupportedBoardSize)
	for row := range board {
		board[row] = make([]int, MinSupportedBoardSize)
		for col := range board[row] {
			board[row][col] = 1
		}
	}
	board[0][0] = 9
	state := &GameState{Board: board}
	rebuildValidMoveCache(state)
	return state
}

func stateWithOnlyPair(first, second int) *GameState {
	board := make(Board, MinSupportedBoardSize)
	for row := range board {
		board[row] = make([]int, MinSupportedBoardSize)
	}
	board[0][0], board[0][1] = first, second
	state := &GameState{Board: board}
	rebuildValidMoveCache(state)
	return state
}

func stateWithMostlyNines() *GameState {
	board := make(Board, MinSupportedBoardSize)
	for row := range board {
		board[row] = make([]int, MinSupportedBoardSize)
		for col := range board[row] {
			board[row][col] = 9
		}
	}
	board[0][0], board[0][1] = 1, 9
	state := &GameState{Board: board}
	rebuildValidMoveCache(state)
	return state
}

func pairSelection() Selection {
	return Selection{Start: Position{Row: 0, Col: 0}, End: Position{Row: 0, Col: 1}}
}

func coordinatedRequest(action PlayerActionType, state *GameState) PlayerActionRequest {
	request := PlayerActionRequest{Type: action}
	switch action {
	case PlayerActionMove:
		request.Selection = state.ValidMoves[0]
	case PlayerActionRemoveNumber:
		request.Position, _ = firstNonZeroPosition(state.Board)
	case PlayerActionHint:
		request.Hint = new(Selection)
	}
	return request
}

func drainGameSnapshots(session *GameSession) <-chan GameSnapshot {
	updates := make(chan GameSnapshot, 8)
	go func() {
		defer close(updates)
		for snapshot := range session.Snapshots() {
			updates <- snapshot
		}
	}()
	return updates
}

func nextGameSnapshot(t *testing.T, updates <-chan GameSnapshot) GameSnapshot {
	t.Helper()
	select {
	case snapshot, ok := <-updates:
		if !ok {
			t.Fatal("snapshot stream closed before expected update")
		}
		return snapshot
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for game snapshot")
		return GameSnapshot{}
	}
}

func nextSubmitError(t *testing.T, errs <-chan error) error {
	t.Helper()
	select {
	case err := <-errs:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for submit to return")
		return nil
	}
}

func waitForCoordinatedStart(t *testing.T, startsAt time.Time) {
	t.Helper()
	if delay := time.Until(startsAt); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		<-timer.C
	}
}

func waitForDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session completion")
	}
}

func stopCoordinatedSession(t *testing.T, session *GameSession) {
	t.Helper()
	if session == nil {
		return
	}
	session.Stop()
	// Done closes after the timer goroutine exits as well as the game loop.
	waitForDone(t, session.Done())
}

func copySelectionSet(source map[Selection]struct{}) map[Selection]struct{} {
	copy := make(map[Selection]struct{}, len(source))
	for selection := range source {
		copy[selection] = struct{}{}
	}
	return copy
}
