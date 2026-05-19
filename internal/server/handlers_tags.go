package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode"

	"github.com/go-chi/chi/v5"

	"github.com/hwhang0917/local-eml/internal/store"
)

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := s.Store.ListTags(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

type tagBody struct {
	Name string `json:"name"`
}

func (s *Server) handleAddTag(w http.ResponseWriter, r *http.Request) {
	sha := chi.URLParam(r, "sha")
	if !validSHA(sha) {
		http.Error(w, "invalid sha", http.StatusBadRequest)
		return
	}
	var body tagBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(body.Name)
	if !validTagName(name) {
		http.Error(w, "invalid tag name", http.StatusBadRequest)
		return
	}
	if err := s.Store.AddTagToEmail(r.Context(), sha, name); err != nil {
		if errors.Is(err, store.ErrEmailNotFound) {
			http.Error(w, "email not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRemoveTag(w http.ResponseWriter, r *http.Request) {
	sha := chi.URLParam(r, "sha")
	name := chi.URLParam(r, "name")
	if !validSHA(sha) {
		http.Error(w, "invalid sha", http.StatusBadRequest)
		return
	}
	if err := s.Store.RemoveTagFromEmail(r.Context(), sha, name); err != nil {
		if errors.Is(err, store.ErrEmailNotFound) {
			http.Error(w, "email not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validTagName(s string) bool {
	if s == "" || len(s) > 40 {
		return false
	}
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) ||
			r == '-' || r == '_' || r == ' ') {
			return false
		}
	}
	return true
}
