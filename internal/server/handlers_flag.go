package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hwhang0917/local-eml/internal/store"
)

// PUT /api/emails/{sha}/flag  {"flag":"spam"|"phishing"}
func (s *Server) handleFlagEmail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Flag string `json:"flag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Flag == "" {
		http.Error(w, `body must be {"flag":"spam"|"phishing"}`, http.StatusBadRequest)
		return
	}
	s.setFlag(w, r, body.Flag)
}

// DELETE /api/emails/{sha}/flag
func (s *Server) handleUnflagEmail(w http.ResponseWriter, r *http.Request) {
	s.setFlag(w, r, "")
}

func (s *Server) setFlag(w http.ResponseWriter, r *http.Request, flag string) {
	sha := chi.URLParam(r, "sha")
	if !validSHA(sha) {
		http.Error(w, "invalid sha", http.StatusBadRequest)
		return
	}
	err := s.Store.SetEmailFlag(r.Context(), sha, flag)
	switch {
	case errors.Is(err, store.ErrInvalidFlag):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, store.ErrEmailNotFound):
		http.Error(w, "email not found", http.StatusNotFound)
	case err != nil:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// refuseIfFlagged is the server-side half of "plain text only": HTML, inline
// images and attachments of flagged mail are never served, whatever the UI
// asks for. Unknown rows (blob on disk, no DB row) are not flagged.
func (s *Server) refuseIfFlagged(w http.ResponseWriter, r *http.Request, sha string) bool {
	if s.Store == nil { // blob-only servers (tests) have nothing to flag
		return false
	}
	e, err := s.Store.GetEmailBySHA(r.Context(), sha)
	if err != nil || e.Flag == "" {
		return false
	}
	http.Error(w, "message is flagged as "+e.Flag+"; plain text only", http.StatusForbidden)
	return true
}
