package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Mail events tell every open tab that the library changed underneath it, so
// the email list can refresh and new-mail notifications can fire without the
// SPA polling. The stream carries no message content, only "something came
// in"; the SPA reads the list through the normal API.

// Event types on /api/events.
const (
	mailEventReady   = "ready"    // sent once on connect; the SPA catches up then
	mailEventNewMail = "new-mail" // a job added messages to the library
)

// mailEventHeartbeat keeps idle proxies and the browser from timing the
// stream out, and lets a dead client be noticed by the failed write.
const mailEventHeartbeat = 25 * time.Second

// mailFeedBuffer is how far a slow tab may fall behind before it misses an
// event. Missing one is harmless: the next refresh shows everything.
const mailFeedBuffer = 8

type mailEvent struct {
	Type     string `json:"type"`
	Kind     string `json:"kind,omitempty"`
	ImportID string `json:"import_id,omitempty"`
	Added    int    `json:"added,omitempty"`
}

// mailFeed fans mail events out to every connected tab. Its zero value is
// ready to use.
type mailFeed struct {
	mu     sync.Mutex
	subs   map[chan mailEvent]struct{}
	closed bool
}

// subscribe returns a channel of events and a cancel func. The channel is
// closed when the feed shuts down.
func (f *mailFeed) subscribe() (<-chan mailEvent, func()) {
	ch := make(chan mailEvent, mailFeedBuffer)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		close(ch)
		return ch, func() {}
	}
	if f.subs == nil {
		f.subs = map[chan mailEvent]struct{}{}
	}
	f.subs[ch] = struct{}{}
	return ch, func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		if _, ok := f.subs[ch]; ok {
			delete(f.subs, ch)
			close(ch)
		}
	}
}

// close ends every stream, so a graceful HTTP shutdown doesn't wait on tabs
// that would otherwise stay connected forever.
func (f *mailFeed) close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	for ch := range f.subs {
		delete(f.subs, ch)
		close(ch)
	}
}

// CloseStreams ends the long-lived /api/events streams. Register it with
// http.Server.RegisterOnShutdown.
func (s *Server) CloseStreams() {
	s.mail.close()
}

// publish never blocks: a tab whose buffer is full skips this event.
func (f *mailFeed) publish(ev mailEvent) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for ch := range f.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// announceImported tells open tabs how many new messages a finished import
// job added. Exports and jobs that added nothing stay quiet.
func (s *Server) announceImported(id, kind string) {
	if strings.HasSuffix(kind, "-export") {
		return
	}
	imp, err := s.Store.GetImport(context.Background(), id)
	if err != nil {
		return
	}
	added := imp.Processed - imp.Duplicates - imp.Errors
	if added <= 0 {
		return
	}
	s.mail.publish(mailEvent{Type: mailEventNewMail, Kind: kind, ImportID: id, Added: added})
}

// handleMailEvents streams mail events to one tab until it disconnects.
func (s *Server) handleMailEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	ch, cancel := s.mail.subscribe()
	defer cancel()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	writeMailEvent(w, flusher, mailEvent{Type: mailEventReady})

	beat := time.NewTicker(mailEventHeartbeat)
	defer beat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-beat.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			writeMailEvent(w, flusher, ev)
		}
	}
}

func writeMailEvent(w http.ResponseWriter, flusher http.Flusher, ev mailEvent) {
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}
