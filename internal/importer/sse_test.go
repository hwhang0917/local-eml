package importer

import (
	"testing"
	"time"
)

func TestHubDeliversToSubscriber(t *testing.T) {
	h := NewHub()
	ch, cancel := h.Subscribe("a")
	defer cancel()
	h.Publish("a", Event{Type: "start"})
	select {
	case ev := <-ch:
		if ev.Type != "start" {
			t.Errorf("got %v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestHubCloseEndsSubscriber(t *testing.T) {
	h := NewHub()
	ch, _ := h.Subscribe("a")
	h.Close("a")
	if _, ok := <-ch; ok {
		t.Error("channel should be closed")
	}
}

func TestHubMultipleSubscribers(t *testing.T) {
	h := NewHub()
	a, ca := h.Subscribe("x")
	b, cb := h.Subscribe("x")
	defer ca()
	defer cb()
	h.Publish("x", Event{Type: "start"})
	for _, ch := range []<-chan Event{a, b} {
		select {
		case ev := <-ch:
			if ev.Type != "start" {
				t.Errorf("got %v", ev)
			}
		case <-time.After(time.Second):
			t.Fatal("subscriber missed event")
		}
	}
}

func TestHubDropsWhenSubscriberIsSlow(t *testing.T) {
	h := NewHub()
	_, cancel := h.Subscribe("y")
	defer cancel()
	// Buffer is 64; publishing 1000 should not block.
	done := make(chan struct{})
	go func() {
		for range 1000 {
			h.Publish("y", Event{Type: "item"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publish blocked on slow subscriber")
	}
}

func TestHubNoSubscribersIsNoop(t *testing.T) {
	h := NewHub()
	h.Publish("nobody", Event{Type: "item"})
	h.Close("nobody")
}
