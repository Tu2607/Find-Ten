package api

import (
	"net/http"

	"find-ten-game/internal/leaderboard"
)

type Server struct {
	mux         *http.ServeMux
	store       *sessionStore
	leaderboard *leaderboard.Store
}

func NewServer(leaderboardStore *leaderboard.Store) http.Handler {
	if leaderboardStore == nil {
		panic("leaderboard store is required")
	}

	server := &Server{
		mux:         http.NewServeMux(),
		store:       newSessionStore(),
		leaderboard: leaderboardStore,
	}
	server.routes()

	return server
}

// Type Server implements http.Handler, so it can be passed directly to ListenAndServe.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", handleHealth)
	s.mux.HandleFunc("POST /games", s.handleCreateGame)
	s.mux.HandleFunc("GET /games", handleMethodNotAllowed)
	s.mux.HandleFunc("DELETE /games/{id}", s.handleDeleteGame)
	s.mux.HandleFunc("GET /games/{id}/snapshots", s.handleGameSnapshots)
	s.mux.HandleFunc("POST /games/{id}/moves", s.handleSubmitMove)
	s.mux.HandleFunc("GET /games/{id}/moves", handleMethodNotAllowed)
	s.mux.HandleFunc("POST /games/{id}/reshuffle", s.handleSubmitReshuffle)
	s.mux.HandleFunc("GET /games/{id}/reshuffle", handleMethodNotAllowed)
	s.mux.HandleFunc("POST /games/{id}/remove-number", s.handleSubmitRemoveNumber)
	s.mux.HandleFunc("GET /games/{id}/remove-number", handleMethodNotAllowed)
	s.mux.HandleFunc("POST /games/{id}/hint", s.handleHint)
	s.mux.HandleFunc("GET /games/{id}/hint", handleMethodNotAllowed)
	s.mux.HandleFunc("POST /scores", s.handleSubmitScore)
	s.mux.HandleFunc("GET /scores", s.handleTopScores)
	s.mux.HandleFunc("DELETE /scores", handleMethodNotAllowed)
	s.mux.HandleFunc("PUT /scores", handleMethodNotAllowed)
	s.mux.HandleFunc("PATCH /scores", handleMethodNotAllowed)
	s.mux.Handle("GET /", http.FileServer(http.Dir("./static/")))
}
