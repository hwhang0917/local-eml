package server

import (
	"net/http"
	"slices"

	"github.com/hwhang0917/local-eml/internal/store"
)

// handleListAttachments backs the attachment browser: every attachment of
// every visible message, one page at a time. Opening or downloading one goes
// through the per-message attachment endpoint, so quarantine and inline
// rules stay in one place.
func (s *Server) handleListAttachments(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind := q.Get("kind")
	if !slices.Contains(store.AttachmentKinds, kind) {
		kind = "" // unknown kinds mean "all", like a junk date filter
	}
	opts := store.AttachmentListOptions{
		Name:   q.Get("q"),
		Kind:   kind,
		Sender: q.Get("sender"),
		From:   dayBound(q.Get("from"), false),
		To:     dayBound(q.Get("to"), true),
		Sort:   q.Get("sort"),
		Order:  q.Get("order"),
		Limit:  store.ClampPageSize(intParam(q.Get("limit"), store.DefaultPageSize)),
		Offset: intParam(q.Get("offset"), 0),
	}
	list, err := s.Store.ListAttachments(r.Context(), opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total":       list.Total,
		"total_bytes": list.TotalBytes,
		"kinds":       list.Kinds,
		"limit":       opts.Limit,
		"offset":      opts.Offset,
		"items":       list.Items,
	})
}
