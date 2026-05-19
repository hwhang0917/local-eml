package importer

import "sync"

type Event struct {
	Type      string `json:"type"` // "start" | "item" | "done" | "error"
	Path      string `json:"path,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
	Duplicate bool   `json:"duplicate,omitempty"`
	Message   string `json:"message,omitempty"`
	Processed int    `json:"processed"`
	Total     int    `json:"total"`
}

type Hub struct {
	mu   sync.Mutex
	subs map[string][]chan Event
}

func NewHub() *Hub {
	return &Hub{subs: map[string][]chan Event{}}
}

// Subscribe returns a buffered channel of events for importID and a cancel
// function the caller must invoke to release the subscription.
func (h *Hub) Subscribe(importID string) (<-chan Event, func()) {
	ch := make(chan Event, 64)
	h.mu.Lock()
	h.subs[importID] = append(h.subs[importID], ch)
	h.mu.Unlock()

	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		list := h.subs[importID]
		for i, c := range list {
			if c != ch {
				continue
			}
			h.subs[importID] = append(list[:i], list[i+1:]...)
			if len(h.subs[importID]) == 0 {
				delete(h.subs, importID)
			}
			// Drain so close is safe, then close.
			select {
			case <-ch:
			default:
			}
			close(ch)
			return
		}
	}
	return ch, cancel
}

// Publish delivers ev to every current subscriber for importID. Slow subscribers
// have events dropped rather than blocking the importer (the DB is the source
// of truth for final state).
func (h *Hub) Publish(importID string, ev Event) {
	h.mu.Lock()
	subs := append([]chan Event{}, h.subs[importID]...)
	h.mu.Unlock()
	for _, c := range subs {
		select {
		case c <- ev:
		default:
		}
	}
}

// Close terminates all subscribers for importID. Safe to call once.
func (h *Hub) Close(importID string) {
	h.mu.Lock()
	subs := h.subs[importID]
	delete(h.subs, importID)
	h.mu.Unlock()
	for _, c := range subs {
		close(c)
	}
}
