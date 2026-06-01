package api

import (
	"context"
	"encoding/json"
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
