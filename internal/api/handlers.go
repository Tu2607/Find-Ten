package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"find-ten-game/internal/game"
)

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

func (s *Server) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	var request createGameRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
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

	session, initialSnapshot, err := game.NewGameSession(context.Background(), *request.Size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create game")
		return
	}

	gameID, err := s.store.add(session)
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
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
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

func writeMoveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, game.ErrInvalidMove), errors.Is(err, game.ErrOutOfBounds):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, game.ErrGameOver):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, game.ErrReshuffleAlreadyUsed):
		writeError(w, http.StatusConflict, err.Error())
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
