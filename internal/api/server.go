package api

import "net/http"

type Server struct {
	mux   *http.ServeMux
	store *sessionStore
}

func NewServer() http.Handler {
	server := &Server{
		mux:   http.NewServeMux(),
		store: newSessionStore(),
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
	s.mux.Handle("GET /", http.FileServer(http.Dir("./static/")))
}
