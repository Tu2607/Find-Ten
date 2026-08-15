package api

import (
	"net/http"

	"find-ten-game/internal/leaderboard"
	"find-ten-game/internal/player"
)

const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'; object-src 'none'"

type Server struct {
	mux         *http.ServeMux
	store       *sessionStore
	leaderboard *leaderboard.Store
	players     *player.Store
}

func NewServer(leaderboardStore *leaderboard.Store, playerStore *player.Store) http.Handler {
	if leaderboardStore == nil {
		panic("leaderboard store is required")
	}
	if playerStore == nil {
		panic("player store is required")
	}

	server := &Server{
		mux:         http.NewServeMux(),
		store:       newSessionStore(),
		leaderboard: leaderboardStore,
		players:     playerStore,
	}
	server.routes()

	return server
}

// Type Server implements http.Handler, so it can be passed directly to ListenAndServe.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w)
	s.mux.ServeHTTP(w, r)
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
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
	s.mux.HandleFunc("POST /players", s.handleCreatePlayer)
	s.mux.HandleFunc("POST /auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /auth/logout", s.handleLogout)
	s.mux.HandleFunc("GET /auth/me", s.handleCurrentPlayer)
	s.mux.Handle("GET /", http.FileServer(http.Dir("./static/")))
}
