package server

import (
	"net/http"

	"github.com/hwhang0917/local-eml/internal/store"
)

// ReadTrackingSettingKey is the settings row that switches read/unread state
// on. It is "" until the user decides, which is how the SPA knows to ask the
// first-time question about existing mail.
const ReadTrackingSettingKey = "read_tracking"

const (
	readTrackingOn  = "on"
	readTrackingOff = "off"

	// Values for the first-time choice: what every existing message becomes.
	existingRead   = "read"
	existingUnread = "unread"
)

type readTrackingState struct {
	Enabled    bool `json:"enabled"`
	Configured bool `json:"configured"`
}

type readTrackingPayload struct {
	Enabled  bool   `json:"enabled"`
	Existing string `json:"existing"`
}

func (s *Server) readTracking(r *http.Request) (readTrackingState, error) {
	v, err := s.Store.GetSetting(r.Context(), ReadTrackingSettingKey)
	return readTrackingState{Enabled: v == readTrackingOn, Configured: v != ""}, err
}

func (s *Server) handleGetReadTracking(w http.ResponseWriter, r *http.Request) {
	state, err := s.readTracking(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handlePutReadTracking(w http.ResponseWriter, r *http.Request) {
	var p readTrackingPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if p.Existing != "" && p.Existing != existingRead && p.Existing != existingUnread {
		http.Error(w, `existing must be "read", "unread" or omitted`, http.StatusBadRequest)
		return
	}
	value := readTrackingOff
	if p.Enabled {
		value = readTrackingOn
	}
	if err := s.Store.SetSetting(r.Context(), ReadTrackingSettingKey, value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Existing mail is only re-labelled when asked; turning the feature off and
	// on again keeps what the user had.
	if p.Enabled && p.Existing != "" {
		if err := s.Store.SetAllEmailsRead(r.Context(), p.Existing == existingRead); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, http.StatusOK, readTrackingState{Enabled: p.Enabled, Configured: true})
}

func (s *Server) handleReadEmail(w http.ResponseWriter, r *http.Request) {
	s.setRead(w, r, true)
}

func (s *Server) handleUnreadEmail(w http.ResponseWriter, r *http.Request) {
	s.setRead(w, r, false)
}

func (s *Server) setRead(w http.ResponseWriter, r *http.Request, read bool) {
	sha, ok := shaParam(w, r)
	if !ok {
		return
	}
	if err := s.Store.SetEmailRead(r.Context(), sha, read); err != nil {
		writeStoreErr(w, err, store.ErrEmailNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
