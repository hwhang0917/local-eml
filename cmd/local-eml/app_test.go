package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestTouchHealth(t *testing.T) {
	lastSeen := &atomic.Int64{}
	h := touchHealth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), lastSeen)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/emails", nil))
	if lastSeen.Load() != 0 {
		t.Fatal("non-healthz request must not touch lastSeen")
	}

	before := time.Now().UnixNano()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if lastSeen.Load() < before {
		t.Fatal("healthz request must stamp lastSeen")
	}
}

func TestServeArgs(t *testing.T) {
	if got := serveArgs(defaultPort); len(got) != 1 || got[0] != "serve" {
		t.Fatalf("default port must register bare serve, got %v", got)
	}
	if got := serveArgs(9000); len(got) != 3 || got[1] != "--port" || got[2] != "9000" {
		t.Fatalf("custom port must be passed through, got %v", got)
	}
}
