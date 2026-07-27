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

const maxCategoryName = 64

func (s *Server) handleListCategories(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListCategories(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// handleRenameCategory is the only write. The palette is fixed and seeded, so
// there is no create, recolour or delete to expose.
func (s *Server) handleRenameCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
		return
	}
	// Empty is allowed and meaningful: it restores the colour's own name.
	name := strings.TrimSpace(body.Name)
	if len(name) > maxCategoryName {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}

	saved, err := s.Store.RenameCategory(r.Context(), id, name)
	if err != nil {
		if errors.Is(err, store.ErrCategoryNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleSetEmailCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	s.setEmailCategory(w, r, &id)
}

func (s *Server) handleClearEmailCategory(w http.ResponseWriter, r *http.Request) {
	s.setEmailCategory(w, r, nil)
}

func (s *Server) setEmailCategory(w http.ResponseWriter, r *http.Request, categoryID *int64) {
	sha := chi.URLParam(r, "sha")
	if !validSHA(sha) {
		http.Error(w, "invalid sha", http.StatusBadRequest)
		return
	}
	if err := s.Store.SetEmailCategory(r.Context(), sha, categoryID); err != nil {
		switch {
		case errors.Is(err, store.ErrEmailNotFound):
			http.Error(w, "email not found", http.StatusNotFound)
		case errors.Is(err, store.ErrCategoryNotFound):
			// The caller named a category that does not exist — their input, not
			// a missing resource on this path.
			http.Error(w, "unknown category", http.StatusBadRequest)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
