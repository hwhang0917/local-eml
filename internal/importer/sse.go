package importer

import "sync"

// Event types, as the SPA reads them off the stream.
const (
	EventStep  = "step"  // a new phase began
	EventItem  = "item"  // one message processed (or failed, with Message set)
	EventError = "error" // the job stopped early
	EventDone  = "done"  // the job finished
)

// hubBuffer is how many events a slow subscriber may fall behind before
// Publish starts dropping for it; see Publish.
const hubBuffer = 64

type Event struct {
	Type      string `json:"type"`
	Phase     string `json:"phase,omitempty"`
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

func (h *Hub) Subscribe(importID string) (<-chan Event, func()) {
	ch := make(chan Event, hubBuffer)
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

// Publish fans ev out to every subscriber without blocking: a subscriber
// whose buffer is full misses the event rather than stalling the job.
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

func (h *Hub) Close(importID string) {
	h.mu.Lock()
	subs := h.subs[importID]
	delete(h.subs, importID)
	h.mu.Unlock()
	for _, c := range subs {
		close(c)
	}
}
