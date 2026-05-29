package importer

import (
	"context"
	"sync"
)

// Canceller tracks in-flight jobs so callers can abort them by ID. The same
// instance is shared between importer and exporter jobs since jobs of either
// flavor live in the imports table and the SSE Hub.
type Canceller struct {
	mu    sync.Mutex
	jobs  map[string]context.CancelFunc
}

func NewCanceller() *Canceller {
	return &Canceller{jobs: map[string]context.CancelFunc{}}
}

// Register wraps ctx with a cancel func and stores it under id. The returned
// context is cancelled either when Cancel(id) is called or when the caller
// runs the returned release func (do this once the job finishes).
func (c *Canceller) Register(ctx context.Context, id string) (context.Context, func()) {
	jobCtx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	c.jobs[id] = cancel
	c.mu.Unlock()

	release := func() {
		c.mu.Lock()
		delete(c.jobs, id)
		c.mu.Unlock()
		cancel()
	}
	return jobCtx, release
}

// Cancel triggers the registered cancel for id. Returns true if a job was
// running under that id.
func (c *Canceller) Cancel(id string) bool {
	c.mu.Lock()
	cancel, ok := c.jobs[id]
	delete(c.jobs, id)
	c.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}
