package server

import (
	"net/http"

	"github.com/hwhang0917/local-eml/internal/store"
)

func (s *Server) handleStarEmail(w http.ResponseWriter, r *http.Request) {
	s.setStarred(w, r, true)
}

func (s *Server) handleUnstarEmail(w http.ResponseWriter, r *http.Request) {
	s.setStarred(w, r, false)
}

func (s *Server) setStarred(w http.ResponseWriter, r *http.Request, starred bool) {
	sha, ok := shaParam(w, r)
	if !ok {
		return
	}
	if err := s.Store.SetEmailStarred(r.Context(), sha, starred); err != nil {
		writeStoreErr(w, err, store.ErrEmailNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
