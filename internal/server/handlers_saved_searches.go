package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"
)

// SavedSearchesSettingKey holds the library's saved searches as one JSON list.
// It lives in the settings table so a backup zip carries it and a restore
// brings it back, like the other settings.
const SavedSearchesSettingKey = "saved_searches"

const (
	maxSavedSearches   = 50
	maxSavedNameRunes  = 80
	maxSavedQueryRunes = 1000
)

// SavedSearch is a named search-box query. Every library filter can be
// written in the search box, so the query alone is the whole view.
type SavedSearch struct {
	Name  string `json:"name"`
	Query string `json:"query"`
}

func (s *Server) handleGetSavedSearches(w http.ResponseWriter, r *http.Request) {
	v, err := s.Store.GetSetting(r.Context(), SavedSearchesSettingKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	list := []SavedSearch{}
	if v != "" {
		// A corrupt row reads as empty rather than breaking the library.
		_ = json.Unmarshal([]byte(v), &list)
	}
	writeJSON(w, http.StatusOK, list)
}

// handlePutSavedSearches replaces the whole list: the page edits it as one
// ordered list, and there are few enough entries that a diff isn't worth it.
func (s *Server) handlePutSavedSearches(w http.ResponseWriter, r *http.Request) {
	var list []SavedSearch
	if !decodeJSON(w, r, &list) {
		return
	}
	if len(list) > maxSavedSearches {
		http.Error(w, "at most 50 saved searches", http.StatusBadRequest)
		return
	}
	seen := map[string]bool{}
	for i := range list {
		list[i].Name = strings.TrimSpace(list[i].Name)
		list[i].Query = strings.TrimSpace(list[i].Query)
		name, query := list[i].Name, list[i].Query
		if name == "" || utf8.RuneCountInString(name) > maxSavedNameRunes {
			http.Error(w, "each saved search needs a name of 1 to 80 characters", http.StatusBadRequest)
			return
		}
		if query == "" || utf8.RuneCountInString(query) > maxSavedQueryRunes {
			http.Error(w, "each saved search needs a query of 1 to 1000 characters", http.StatusBadRequest)
			return
		}
		key := strings.ToLower(name)
		if seen[key] {
			http.Error(w, "saved search names must be unique", http.StatusConflict)
			return
		}
		seen[key] = true
	}
	if list == nil {
		list = []SavedSearch{}
	}
	b, _ := json.Marshal(list)
	if err := s.Store.SetSetting(r.Context(), SavedSearchesSettingKey, string(b)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, list)
}
