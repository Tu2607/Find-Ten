package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"find-ten-game/internal/game"
	"find-ten-game/internal/leaderboard"
	"find-ten-game/internal/player"
)

const sessionCookieName = "find_ten_session"

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func handleNotImplemented(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

func handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (s *Server) handleCreatePlayer(w http.ResponseWriter, r *http.Request) {
	var request createPlayerRequest
	if !decodeJSONBody(w, r, &request) {
		return
	}
	if request.DisplayName == nil {
		writeError(w, http.StatusBadRequest, "displayName is required")
		return
	}
	if request.Password == nil {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}

	account, err := s.players.CreateAccount(r.Context(), player.CreateAccountInput{
		DisplayName: *request.DisplayName,
		Password:    *request.Password,
	})
	if err != nil {
		writePlayerCreateError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createPlayerResponse{
		Created: true,
		Player:  newPlayerResponse(account),
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if !decodeJSONBody(w, r, &request) {
		return
	}
	if request.AccountHandle == nil || *request.AccountHandle == "" {
		writeError(w, http.StatusBadRequest, "accountHandle is required")
		return
	}
	if request.Password == nil || *request.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}

	account, err := s.players.Authenticate(r.Context(), *request.AccountHandle, *request.Password)
	if err != nil {
		writeLoginError(w, err)
		return
	}
	token, err := s.players.CreateSession(r.Context(), account.ID)
	if err != nil {
		writePlayerStorageError(w, err, "failed to create login session")
		return
	}

	setSessionCookie(w, r, token)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(authResponse{
		Authenticated: true,
		Player:        newPlayerResponse(account),
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if err := s.players.DeleteSession(r.Context(), cookie.Value); err != nil {
			writePlayerStorageError(w, err, "failed to log out")
			return
		}
	}

	clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCurrentPlayer(w http.ResponseWriter, r *http.Request) {
	account, err := s.authenticatedPlayer(r)
	if err != nil {
		writeCurrentPlayerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(authResponse{
		Authenticated: true,
		Player:        newPlayerResponse(account),
	})
}

func (s *Server) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	var request createGameRequest
	if !decodeJSONBody(w, r, &request) {
		return
	}

	if request.Size == nil {
		writeError(w, http.StatusBadRequest, "size is required")
		return
	}
	if err := game.ValidateBoardSize(*request.Size); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	durationSeconds := game.DefaultDurationSeconds
	if request.Duration != nil {
		durationSeconds = *request.Duration
	}
	if err := game.ValidateDuration(durationSeconds); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	session, initialSnapshot, err := game.NewGameSession(context.Background(), *request.Size, durationSeconds)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create game")
		return
	}

	gameID, err := s.store.add(session, durationSeconds)
	if err != nil {
		session.Stop()
		if errors.Is(err, errSessionStoreFull) {
			writeError(w, http.StatusServiceUnavailable, "session store is full")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to store game")
		return
	}

	response := createGameResponse{
		GameID:          gameID,
		InitialSnapshot: newSnapshotResponse(initialSnapshot),
		ExpiresAt:       session.ExpiresAt(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

func (s *Server) handleDeleteGame(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	stored, ok := s.store.remove(id)
	if !ok {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}

	stored.session.Stop()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSubmitMove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stored, ok := s.store.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}

	// Decode the request body into a submitMoveRequest struct
	// This means the client must send a JSON body like:
	// {
	//   "selection": {
	//     "start": {"row": 0, "col": 0},
	//     "end": {"row": 0, "col": 1}
	//   }
	// }
	var request submitMoveRequest
	if !decodeJSONBody(w, r, &request) {
		return
	}

	selection, ok := request.toSelection()
	if !ok {
		writeError(w, http.StatusBadRequest, "selection is required")
		return
	}

	if err := stored.session.SubmitMove(r.Context(), selection); err != nil {
		writeMoveError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(submitMoveResponse{Accepted: true})
}

func (s *Server) handleSubmitReshuffle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stored, ok := s.store.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}

	if err := stored.session.SubmitReshuffle(r.Context()); err != nil {
		writeMoveError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(submitMoveResponse{Accepted: true})
}

func (s *Server) handleSubmitRemoveNumber(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stored, ok := s.store.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}

	var request submitRemoveNumberRequest
	if !decodeJSONBody(w, r, &request) {
		return
	}

	position, ok := request.toPosition()
	if !ok {
		writeError(w, http.StatusBadRequest, "position is required")
		return
	}

	if err := stored.session.SubmitRemoveNumber(r.Context(), position); err != nil {
		writeMoveError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(submitMoveResponse{Accepted: true})
}

func (s *Server) handleHint(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stored, ok := s.store.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}

	hint, err := stored.session.SubmitHint(r.Context())
	if err != nil {
		writeMoveError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(hintResponse{Selection: newSelectionResponse(hint)})
}

func (s *Server) handleGameSnapshots(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stored, ok := s.store.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}

	snapshots, unsubscribe, ok := stored.broker.subscribe()
	if !ok {
		writeError(w, http.StatusGone, "game snapshot stream is closed")
		return
	}
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case snapshot, ok := <-snapshots:
			if !ok {
				return
			}
			if err := writeSnapshotEvent(w, snapshot); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeSnapshotEvent(w io.Writer, snapshot snapshotResponse) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(w, "event: snapshot\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}

	return nil
}

func (s *Server) handleSubmitScore(w http.ResponseWriter, r *http.Request) {
	var request submitScoreRequest
	if !decodeJSONBody(w, r, &request) {
		return
	}

	if request.GameID == nil || *request.GameID == "" {
		writeError(w, http.StatusBadRequest, "gameId is required")
		return
	}

	account, authenticated, err := s.authenticatedPlayerForScore(r)
	if err != nil {
		writePlayerStorageError(w, err, "failed to authenticate score submission")
		return
	}
	if !authenticated && request.PlayerName == nil {
		writeError(w, http.StatusBadRequest, "playerName is required")
		return
	}

	stored, ok := s.store.get(*request.GameID)
	if !ok {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}

	snapshot, ok := stored.broker.latestSnapshot()
	if !ok || !snapshot.GameOver {
		writeError(w, http.StatusConflict, "game is not over")
		return
	}

	remainingMillis := 0
	if snapshot.GameOver && snapshot.GameOverReason != int(game.GameOverTimeExpired) {
		remaining := stored.session.ExpiresAt().Sub(snapshot.SnapshotTime)
		if remaining > 0 {
			remainingMillis = int(remaining.Milliseconds())
			maxMillis := stored.durationSeconds * 1000
			if remainingMillis > maxMillis {
				remainingMillis = maxMillis
			}
		}
	}

	submission := leaderboard.ScoreSubmission{
		GameID:          *request.GameID,
		Score:           snapshot.Score,
		GridSize:        len(snapshot.Board),
		DurationSeconds: stored.durationSeconds,
		RemainingMillis: remainingMillis,
		SubmittedAt:     time.Now(),
	}
	if authenticated {
		submission.PlayerName = account.DisplayName
		submission.PlayerID = &account.ID
	} else {
		submission.PlayerName = *request.PlayerName
	}

	if err := s.leaderboard.SubmitScore(r.Context(), submission); err != nil {
		writeScoreError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(submitScoreResponse{Accepted: true})
}

func (s *Server) handleTopScores(w http.ResponseWriter, r *http.Request) {
	gridSizeParam := r.URL.Query().Get("gridSize")
	if gridSizeParam == "" {
		writeError(w, http.StatusBadRequest, "gridSize is required")
		return
	}
	gridSize, err := strconv.Atoi(gridSizeParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, "gridSize must be an integer")
		return
	}
	if err := game.ValidateBoardSize(gridSize); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	durationParam := r.URL.Query().Get("duration")
	if durationParam == "" {
		writeError(w, http.StatusBadRequest, "duration is required")
		return
	}
	duration, err := strconv.Atoi(durationParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, "duration must be an integer")
		return
	}
	if err := game.ValidateDuration(duration); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	entries, err := s.leaderboard.TopScores(r.Context(), leaderboard.TopScoresFilter{
		GridSize:        gridSize,
		DurationSeconds: duration,
		Limit:           15,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			writeError(w, http.StatusRequestTimeout, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to query scores")
		return
	}

	scores := make([]scoreResponse, len(entries))
	for i, entry := range entries {
		scores[i] = scoreResponse{
			Rank:            i + 1,
			PlayerName:      entry.PlayerName,
			Score:           entry.Score,
			GridSize:        entry.GridSize,
			DurationSeconds: entry.DurationSeconds,
			RemainingMillis: entry.RemainingMillis,
			SubmittedAt:     entry.SubmittedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(scores)
}

func writeScoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, leaderboard.ErrDuplicateGameID):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, leaderboard.ErrInvalidScoreSubmission):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusRequestTimeout, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "failed to submit score")
	}
}

func writeMoveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, game.ErrInvalidMove), errors.Is(err, game.ErrOutOfBounds), errors.Is(err, game.ErrRemoveNumberInvalidTarget):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, game.ErrGameOver):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, game.ErrReshuffleAlreadyUsed), errors.Is(err, game.ErrRemoveNumberAlreadyUsed), errors.Is(err, game.ErrHintAlreadyUsed), errors.Is(err, game.ErrHintNoValidMoves):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, game.ErrSessionClosed):
		writeError(w, http.StatusGone, err.Error())
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusRequestTimeout, err.Error())
	case errors.Is(err, game.ErrUninitializedMove), errors.Is(err, game.ErrNilGameState), errors.Is(err, game.ErrUnknownPlayerAction), errors.Is(err, game.ErrReshuffleFailed):
		writeError(w, http.StatusInternalServerError, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "failed to submit move")
	}
}
