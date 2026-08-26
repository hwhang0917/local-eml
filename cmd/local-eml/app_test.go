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
