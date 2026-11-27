package server

import (
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/hwhang0917/local-eml/internal/store"
)

// maxMergeGroups bounds one hand-picked merge request; "all" has no bound.
const maxMergeGroups = 2000

type duplicatesResponse struct {
	Summary store.DuplicateSummary `json:"summary"`
	Groups  []store.DuplicateGroup `json:"groups"`
}

func (s *Server) handleListDuplicates(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ctx := r.Context()
	sum, err := s.Store.DuplicateSummary(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	groups, err := s.Store.ListDuplicates(ctx, intParam(q.Get("limit"), store.DefaultPageSize), intParam(q.Get("offset"), 0))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, duplicatesResponse{Summary: sum, Groups: groups})
}

type mergePayload struct {
	// All merges every group, each keeping its oldest import.
	All    bool                 `json:"all"`
	Groups []store.MergeRequest `json:"groups"`
}

type mergeResult struct {
	Removed int `json:"removed"`
}

// handleMergeDuplicates keeps one copy of each group and deletes the rest,
// rows first and then files, as a bulk delete does.
func (s *Server) handleMergeDuplicates(w http.ResponseWriter, r *http.Request) {
	var p mergePayload
	if !decodeJSON(w, r, &p) {
		return
	}
	ctx := r.Context()
	reqs := p.Groups
	if p.All {
		keys, err := s.Store.AllDuplicateGroups(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		reqs = make([]store.MergeRequest, len(keys))
		for i, k := range keys {
			reqs[i] = store.MergeRequest{Group: k}
		}
	} else {
		if len(reqs) == 0 || len(reqs) > maxMergeGroups {
			http.Error(w, "groups must list 1 to 2000 groups, or set all", http.StatusBadRequest)
			return
		}
		for _, g := range reqs {
			if g.Keep != "" && !validSHA(g.Keep) {
				http.Error(w, "invalid sha", http.StatusBadRequest)
				return
			}
		}
	}
	removed, err := s.Store.MergeDuplicates(ctx, reqs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, sha := range removed {
		if err := os.Remove(s.Importer.Paths.BlobFor(sha)); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("delete duplicate blob", slog.String("sha256", sha), slog.Any("err", err))
		}
	}
	slog.Info("merged duplicates", slog.Int("groups", len(reqs)), slog.Int("removed", len(removed)))
	writeJSON(w, http.StatusOK, mergeResult{Removed: len(removed)})
}
