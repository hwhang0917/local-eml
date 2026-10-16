package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/hwhang0917/local-eml/internal/store"
)

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
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	// Empty is allowed and meaningful: it restores the colour's own name.
	name := strings.TrimSpace(body.Name)
	if len(name) > maxNameLen {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}

	saved, err := s.Store.RenameCategory(r.Context(), id, name)
	if err != nil {
		writeStoreErr(w, err, store.ErrCategoryNotFound)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleSetEmailCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	s.setEmailCategory(w, r, &id)
}

func (s *Server) handleClearEmailCategory(w http.ResponseWriter, r *http.Request) {
	s.setEmailCategory(w, r, nil)
}

func (s *Server) setEmailCategory(w http.ResponseWriter, r *http.Request, categoryID *int64) {
	sha, ok := shaParam(w, r)
	if !ok {
		return
	}
	if err := s.Store.SetEmailCategory(r.Context(), sha, categoryID); err != nil {
		// The caller named a category that does not exist — their input, not
		// a missing resource on this path.
		if errors.Is(err, store.ErrCategoryNotFound) {
			http.Error(w, "unknown category", http.StatusBadRequest)
			return
		}
		writeStoreErr(w, err, store.ErrEmailNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
