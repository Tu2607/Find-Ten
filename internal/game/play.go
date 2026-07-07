package game

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrSessionClosed = errors.New("game session is closed")

func NewGameSession(ctx context.Context, size int, durationSeconds int) (*GameSession, GameSnapshot, error) {
	if err := ValidateDuration(durationSeconds); err != nil {
		return nil, GameSnapshot{}, err
	}

	state, err := NewGame(size)
	if err != nil {
		return nil, GameSnapshot{}, err
	}

	expiresAt := time.Now().Add(time.Duration(durationSeconds) * time.Second)
	initialSnapshot := newGameSnapshot(state, 1)
	sessionCtx, cancel := context.WithCancel(ctx)
	session := &GameSession{
		actions:   make(chan PlayerActionRequest),
		expired:   make(chan struct{}),
		snapshots: make(chan GameSnapshot),
		expiresAt: expiresAt,
		cancel:    cancel,
		done:      make(chan struct{}),
		closing:   make(chan struct{}),
	}

	go session.run(sessionCtx, state)

	return session, initialSnapshot, nil
}

func (s *GameSession) run(ctx context.Context, state *GameState) {
	defer close(s.done)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		RunGame(ctx, s.actions, s.expired, s.snapshots, state, s.expiresAt)
		s.Stop()
	}()

	go func() {
		defer wg.Done()
		startExpiryTimer(ctx, s.expiresAt, s.expired)
	}()

	wg.Wait()
	s.submits.Wait()
	close(s.actions)
}

func (s *GameSession) Snapshots() <-chan GameSnapshot {
	return s.snapshots
}

func (s *GameSession) ExpiresAt() time.Time {
	return s.expiresAt
}

func (s *GameSession) SubmitMove(ctx context.Context, selection Selection) error {
	return s.submitAction(ctx, PlayerActionRequest{
		Type:      PlayerActionMove,
		Selection: selection,
	})
}

func (s *GameSession) SubmitReshuffle(ctx context.Context) error {
	return s.submitAction(ctx, PlayerActionRequest{
		Type: PlayerActionReshuffle,
	})
}

func (s *GameSession) SubmitRemoveNumber(ctx context.Context, position Position) error {
	return s.submitAction(ctx, PlayerActionRequest{
		Type:     PlayerActionRemoveNumber,
		Position: position,
	})
}

func (s *GameSession) SubmitHint(ctx context.Context) (Selection, error) {
	var hint Selection

	if err := s.submitAction(ctx, PlayerActionRequest{
		Type: PlayerActionHint,
		Hint: &hint,
	}); err != nil {
		return Selection{}, err
	}

	return hint, nil
}

func (s *GameSession) submitAction(ctx context.Context, request PlayerActionRequest) error {
	if deadlineExpired(s.expiresAt, time.Now()) {
		return ErrGameOver
	}
	if !s.beginSubmit() {
		return s.closedSubmitError()
	}
	defer s.submits.Done()

	results := make(chan error, 1)
	request.Result = results

	select {
	case s.actions <- request:
	case <-s.closing:
		return s.closedSubmitError()
	case <-s.done:
		return s.closedSubmitError()
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-results:
		return err
	case <-s.closing:
		return s.closedSubmitError()
	case <-s.done:
		return s.closedSubmitError()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *GameSession) Stop() {
	s.once.Do(func() {
		s.mu.Lock()
		s.stopping = true
		close(s.closing)
		s.mu.Unlock()
		s.cancel()
	})
}

func (s *GameSession) Done() <-chan struct{} {
	return s.done
}

func (s *GameSession) beginSubmit() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopping {
		return false
	}

	s.submits.Add(1)
	return true
}

func (s *GameSession) closedSubmitError() error {
	if deadlineExpired(s.expiresAt, time.Now()) {
		return ErrGameOver
	}

	return ErrSessionClosed
}

// Receive player action requests and timer expiry signals, then update game state.
func RunGame(ctx context.Context, actions <-chan PlayerActionRequest, expired <-chan struct{}, snapshots chan<- GameSnapshot, state *GameState, expiresAt time.Time) {
	defer close(snapshots)

	var sequence int64 = 2

	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-expired:
			if !ok {
				expired = nil
				continue
			}
			expireGame(state)
			snapshot := newGameSnapshot(state, sequence)
			publishPreparedSnapshot(ctx, snapshots, snapshot)
			return
		case request, ok := <-actions:
			if !ok {
				return
			}
			err := applyPlayerActionBeforeDeadline(state, request, expiresAt, time.Now())
			if request.Result != nil {
				sendActionResult(ctx, request.Result, err)
			}
			if err == nil {
				snapshot := newGameSnapshot(state, sequence)
				sequence++
				publishPreparedSnapshot(ctx, snapshots, snapshot)
			}

			if state == nil || state.GameOver {
				if err != nil && state != nil {
					snapshot := newGameSnapshot(state, sequence)
					publishPreparedSnapshot(ctx, snapshots, snapshot)
				}
				return
			}
		}
	}
}

func applyPlayerActionBeforeDeadline(state *GameState, request PlayerActionRequest, expiresAt time.Time, now time.Time) error {
	if state == nil {
		return ErrNilGameState
	}
	if state.GameOver {
		return ErrGameOver
	}
	if deadlineExpired(expiresAt, now) {
		expireGame(state)
		return ErrGameOver
	}

	switch request.Type {
	case PlayerActionMove:
		return ApplyMove(state, request.Selection)
	case PlayerActionReshuffle:
		return ApplyReshuffle(state)
	case PlayerActionRemoveNumber:
		return ApplyRemoveNumber(state, request.Position)
	case PlayerActionHint:
		hint, err := ApplyHint(state)
		if err != nil {
			return err
		}
		if request.Hint != nil {
			*request.Hint = hint
		}
		return nil
	default:
		return ErrUnknownPlayerAction
	}
}

func expireGame(state *GameState) {
	if state == nil || state.GameOver {
		return
	}

	state.GameOver = true
	state.GameOverReason = GameOverTimeExpired
}

func deadlineExpired(expiresAt time.Time, now time.Time) bool {
	return !expiresAt.IsZero() && !now.Before(expiresAt)
}

func publishPreparedSnapshot(ctx context.Context, snapshots chan<- GameSnapshot, snapshot GameSnapshot) {
	select {
	case <-ctx.Done():
	case snapshots <- snapshot:
	}
}

func sendActionResult(ctx context.Context, results chan<- error, result error) {
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
	snapshot.ValidMoveCount = len(state.ValidMoves)
	snapshot.ReshuffleUsed = state.ReshuffleUsed
	snapshot.RemoveNumberUsed = state.RemoveNumberUsed
	snapshot.HintUsed = state.HintUsed

	return snapshot
}
