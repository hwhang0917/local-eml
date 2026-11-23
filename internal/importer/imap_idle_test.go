package importer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeIdleConn replays a script of Wait outcomes, then blocks until ctx ends.
type fakeIdleConn struct {
	noIdle bool
	steps  []idleStep
	closed bool
}

type idleStep struct {
	changed bool
	err     error
}

func (f *fakeIdleConn) SupportsIdle() bool { return !f.noIdle }

func (f *fakeIdleConn) Wait(ctx context.Context, _ time.Duration) (bool, error) {
	if len(f.steps) == 0 {
		<-ctx.Done()
		return false, nil
	}
	s := f.steps[0]
	f.steps = f.steps[1:]
	return s.changed, s.err
}

func (f *fakeIdleConn) Close() error { f.closed = true; return nil }

// recorder collects what the watcher reported, safely across goroutines.
type recorder struct {
	mu      sync.Mutex
	changes int
	states  []IdleState
}

func (r *recorder) change() { r.mu.Lock(); r.changes++; r.mu.Unlock() }
func (r *recorder) state(s IdleState) {
	r.mu.Lock()
	r.states = append(r.states, s)
	r.mu.Unlock()
}
func (r *recorder) snapshot() (int, []IdleState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.changes, append([]IdleState(nil), r.states...)
}

func newTestWatcher(r *recorder, dial func(IMAPConfig) (idleConn, error)) *IdleWatcher {
	return &IdleWatcher{
		OnChange: r.change,
		OnState:  r.state,
		MinWait:  time.Millisecond,
		MaxWait:  4 * time.Millisecond,
		dial:     dial,
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestIdleWatcherUnsupportedServerFallsBack(t *testing.T) {
	r := &recorder{}
	conn := &fakeIdleConn{noIdle: true}
	w := newTestWatcher(r, func(IMAPConfig) (idleConn, error) { return conn, nil })

	err := w.Run(context.Background())
	if !errors.Is(err, ErrIdleUnsupported) {
		t.Fatalf("err = %v, want ErrIdleUnsupported", err)
	}
	if !conn.closed {
		t.Error("connection left open")
	}
	changes, states := r.snapshot()
	if changes != 0 {
		t.Errorf("changes = %d, want 0", changes)
	}
	if got := states[len(states)-1]; got != IdleUnsupported {
		t.Errorf("last state = %q, want unsupported", got)
	}
}

func TestIdleWatcherCatchesUpThenReportsEachChange(t *testing.T) {
	r := &recorder{}
	conn := &fakeIdleConn{steps: []idleStep{{changed: true}, {changed: false}, {changed: true}}}
	w := newTestWatcher(r, func(IMAPConfig) (idleConn, error) { return conn, nil })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	// One catch-up on connect, then the two pushed changes; the timed-out
	// round in between is a quiet restart.
	waitFor(t, func() bool { n, _ := r.snapshot(); return n == 3 })
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if _, states := r.snapshot(); states[0] != IdleConnecting || states[1] != IdleLive {
		t.Errorf("states = %v, want connecting then live", states)
	}
}

func TestIdleWatcherReconnectsAfterDropAndFailedDial(t *testing.T) {
	r := &recorder{}
	var mu sync.Mutex
	dials := 0
	w := newTestWatcher(r, func(IMAPConfig) (idleConn, error) {
		mu.Lock()
		defer mu.Unlock()
		dials++
		switch dials {
		case 1:
			return &fakeIdleConn{steps: []idleStep{{err: errors.New("connection reset")}}}, nil
		case 2:
			return nil, errors.New("dial refused")
		default:
			return &fakeIdleConn{}, nil
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	// Connected twice, so two catch-ups; the refused dial in between none.
	waitFor(t, func() bool { n, _ := r.snapshot(); return n == 2 })
	cancel()
	<-done

	_, states := r.snapshot()
	retries := 0
	for _, s := range states {
		if s == IdleRetrying {
			retries++
		}
	}
	if retries != 2 {
		t.Errorf("retrying entered %d times, want 2 (states %v)", retries, states)
	}
	if states[len(states)-1] != IdleLive {
		t.Errorf("last state = %q, want live", states[len(states)-1])
	}
}

func TestJitterStaysWithinAFifth(t *testing.T) {
	d := 10 * time.Second
	for range 200 {
		got := jitter(d)
		if got < 8*time.Second || got > 12*time.Second {
			t.Fatalf("jitter(%v) = %v, outside ±20%%", d, got)
		}
	}
}
