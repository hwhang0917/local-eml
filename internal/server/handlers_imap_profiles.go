package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/hwhang0917/local-eml/internal/store"
)

type imapProfileBody struct {
	Name        string  `json:"name"`
	Host        string  `json:"host"`
	Port        *int    `json:"port,omitempty"`
	Username    string  `json:"username"`
	Folder      *string `json:"folder,omitempty"`
	SyncEnabled bool    `json:"sync_enabled,omitempty"`
}

const (
	minIMAPPort = 1
	maxIMAPPort = 65535
)

func (s *Server) handleListIMAPProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := s.Store.ListIMAPProfiles(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, profiles)
}

func (s *Server) handleSaveIMAPProfile(w http.ResponseWriter, r *http.Request) {
	var body imapProfileBody
	if !decodeJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	host := strings.TrimSpace(body.Host)
	username := strings.TrimSpace(body.Username)
	if !validName(name) {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}
	if host == "" {
		http.Error(w, "invalid host", http.StatusBadRequest)
		return
	}
	if username == "" {
		http.Error(w, "invalid username", http.StatusBadRequest)
		return
	}
	if body.Port != nil && (*body.Port < minIMAPPort || *body.Port > maxIMAPPort) {
		http.Error(w, "invalid port", http.StatusBadRequest)
		return
	}
	saved, err := s.Store.UpsertIMAPProfile(r.Context(), store.IMAPProfile{
		Name: name, Host: host, Port: body.Port, Username: username,
		Folder:      trimmedOrNil(body.Folder),
		SyncEnabled: body.SyncEnabled,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleDeleteIMAPProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	if err := s.Store.DeleteIMAPProfile(r.Context(), id); err != nil {
		writeStoreErr(w, err, store.ErrIMAPProfileNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSyncIMAPProfile triggers an incremental sync for a saved profile.
// Returns 202 immediately and runs the import in a goroutine so callers get
// the same import-job lifecycle they're used to.
func (s *Server) handleSyncIMAPProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	profile, err := s.Store.GetIMAPProfile(r.Context(), id)
	if err != nil {
		writeStoreErr(w, err, store.ErrIMAPProfileNotFound)
		return
	}
	if !profile.SyncEnabled {
		http.Error(w, "sync is disabled for this profile", http.StatusBadRequest)
		return
	}
	if !profile.HasPassword {
		http.Error(w, "no stored password — run a manual import once to seed it", http.StatusBadRequest)
		return
	}
	// RunIMAPSync logs its own failures; nothing is left to do with the error.
	go s.RunIMAPSync(context.Background(), id)
	w.WriteHeader(http.StatusAccepted)
}
