package api

import "net/http"

type Server struct {
	mux *http.ServeMux
}

func NewServer() http.Handler {
	server := &Server{
		mux: http.NewServeMux(),
	}
	server.routes()

	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", handleHealth)
	s.mux.HandleFunc("POST /games", handleNotImplemented)
	s.mux.HandleFunc("GET /games/{id}/snapshots", handleNotImplemented)
	s.mux.HandleFunc("POST /games/{id}/moves", handleNotImplemented)
}
