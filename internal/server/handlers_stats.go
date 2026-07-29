package server

import "net/http"

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.Store.Stats(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, st)
}
