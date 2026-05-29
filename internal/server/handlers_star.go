package server

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hwhang0917/local-eml/internal/store"
)

func (s *Server) handleStarEmail(w http.ResponseWriter, r *http.Request) {
	s.setStarred(w, r, true)
}

func (s *Server) handleUnstarEmail(w http.ResponseWriter, r *http.Request) {
	s.setStarred(w, r, false)
}

func (s *Server) setStarred(w http.ResponseWriter, r *http.Request, starred bool) {
	sha := chi.URLParam(r, "sha")
	if !validSHA(sha) {
		http.Error(w, "invalid sha", http.StatusBadRequest)
		return
	}
	if err := s.Store.SetEmailStarred(r.Context(), sha, starred); err != nil {
		if errors.Is(err, store.ErrEmailNotFound) {
			http.Error(w, "email not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
