package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	if !s.Canceller.Cancel(id) {
		http.Error(w, "job not running", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
