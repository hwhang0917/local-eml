package server

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
)

// maxBulkSHAs bounds one bulk request. A page lists at most 500 rows; the
// headroom is for clients that select across a couple of pages.
const maxBulkSHAs = 2000

const (
	bulkRead   = "read"
	bulkUnread = "unread"
	bulkDelete = "delete"
)

type bulkPayload struct {
	Action  string   `json:"action"`
	SHA256s []string `json:"sha256s"`
	// Threads widens each sha to its whole conversation, for rows the
	// library shows grouped.
	Threads bool `json:"threads"`
}

type bulkResult struct {
	Affected int64 `json:"affected"`
}

// handleBulkEmails applies one action to many messages: the library's
// checkbox selection. Shas that no longer exist are skipped, not errors.
func (s *Server) handleBulkEmails(w http.ResponseWriter, r *http.Request) {
	var p bulkPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if p.Action != bulkRead && p.Action != bulkUnread && p.Action != bulkDelete {
		http.Error(w, `action must be "read", "unread" or "delete"`, http.StatusBadRequest)
		return
	}
	if len(p.SHA256s) == 0 || len(p.SHA256s) > maxBulkSHAs {
		http.Error(w, "sha256s must list 1 to 2000 messages", http.StatusBadRequest)
		return
	}
	for _, sha := range p.SHA256s {
		if !validSHA(sha) {
			http.Error(w, "invalid sha", http.StatusBadRequest)
			return
		}
	}

	ctx := r.Context()
	shas := p.SHA256s
	if p.Threads {
		var err error
		if shas, err = s.Store.ExpandThreads(ctx, shas); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	var n int64
	switch p.Action {
	case bulkRead, bulkUnread:
		var err error
		if n, err = s.Store.SetEmailsRead(ctx, shas, p.Action == bulkRead); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case bulkDelete:
		deleted, err := s.Store.DeleteEmailsBySHA(ctx, shas)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Rows go first so a failed file removal leaves an unreferenced blob,
		// never a row pointing at nothing.
		for _, sha := range deleted {
			if err := os.Remove(s.Importer.Paths.BlobFor(sha)); err != nil && !errors.Is(err, os.ErrNotExist) {
				slog.Warn("delete email blob", slog.String("sha256", sha), slog.Any("err", err))
			}
		}
		n = int64(len(deleted))
		slog.Info("bulk deleted emails", slog.Int64("count", n))
	}
	writeJSON(w, http.StatusOK, bulkResult{Affected: n})
}
