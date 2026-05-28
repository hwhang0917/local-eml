package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/hwhang0917/local-eml/internal/store"
)

type imapProfileBody struct {
	Name     string  `json:"name"`
	Host     string  `json:"host"`
	Port     *int    `json:"port,omitempty"`
	Username string  `json:"username"`
	Folder   *string `json:"folder,omitempty"`
}

const (
	maxIMAPProfileName = 64
	minIMAPPort        = 1
	maxIMAPPort        = 65535
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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(body.Name)
	host := strings.TrimSpace(body.Host)
	username := strings.TrimSpace(body.Username)
	if name == "" || len(name) > maxIMAPProfileName {
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
	var folder *string
	if body.Folder != nil {
		trimmed := strings.TrimSpace(*body.Folder)
		if trimmed != "" {
			folder = &trimmed
		}
	}
	saved, err := s.Store.UpsertIMAPProfile(r.Context(), store.IMAPProfile{
		Name: name, Host: host, Port: body.Port, Username: username, Folder: folder,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleDeleteIMAPProfile(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.Store.DeleteIMAPProfile(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrIMAPProfileNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
