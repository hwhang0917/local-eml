package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/hwhang0917/local-eml/internal/store"
)

func (s *Server) handleFlagEmail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Flag string `json:"flag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Flag == "" {
		http.Error(w, `body must be {"flag":"spam"|"phishing"|"safe"}`, http.StatusBadRequest)
		return
	}
	s.setFlag(w, r, body.Flag)
}

func (s *Server) handleUnflagEmail(w http.ResponseWriter, r *http.Request) {
	s.setFlag(w, r, "")
}

func (s *Server) setFlag(w http.ResponseWriter, r *http.Request, flag string) {
	sha, ok := shaParam(w, r)
	if !ok {
		return
	}
	err := s.Store.SetEmailFlag(r.Context(), sha, flag)
	switch {
	case errors.Is(err, store.ErrInvalidFlag):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case err != nil:
		writeStoreErr(w, err, store.ErrEmailNotFound)
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
	if err != nil || !store.Quarantined(e.Flag) {
		return false
	}
	http.Error(w, "message is flagged as "+e.Flag+"; plain text only", http.StatusForbidden)
	return true
}
