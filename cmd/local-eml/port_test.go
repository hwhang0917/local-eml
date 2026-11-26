package main

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// occupy holds a run of consecutive loopback ports and returns the first.
func occupy(t *testing.T, n int) int {
	t.Helper()
	for try := 0; try < 50; try++ {
		first, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		base := first.Addr().(*net.TCPAddr).Port
		held := []net.Listener{first}
		ok := true
		for i := 1; i < n; i++ {
			ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(base+i))
			if err != nil {
				ok = false
				break
			}
			held = append(held, ln)
		}
		if ok {
			t.Cleanup(func() {
				for _, ln := range held {
					ln.Close()
				}
			})
			return base
		}
		for _, ln := range held {
			ln.Close()
		}
	}
	t.Fatal("could not reserve consecutive ports")
	return 0
}

func never(int) bool { return false }

func TestListenFromSkipsForeignPorts(t *testing.T) {
	base := occupy(t, 2)
	ln, err := listenFrom(base, 5, never)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if got := ln.Addr().(*net.TCPAddr).Port; got != base+2 {
		t.Fatalf("want port %d, got %d", base+2, got)
	}
}

func TestListenFromStopsAtLocalEml(t *testing.T) {
	base := occupy(t, 2)
	isLocalEml := func(p int) bool { return p == base+1 }
	if _, err := listenFrom(base, 5, isLocalEml); !errors.Is(err, errAlreadyRunning) {
		t.Fatalf("want errAlreadyRunning, got %v", err)
	}
}

func TestListenFromSingleAttempt(t *testing.T) {
	base := occupy(t, 1)
	if _, err := listenFrom(base, 1, never); err == nil || errors.Is(err, errAlreadyRunning) {
		t.Fatalf("an explicit port must fail plainly when taken, got %v", err)
	}
}

func TestProbeExisting(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{"local-eml", `{"status":"ok","app":"local-eml","version":"0.18.0"}`, true},
		{"older local-eml", `{"status":"ok","version":"0.17.0"}`, true},
		{"other app", `{"status":"ok","app":"grafana","version":"10"}`, false},
		{"bare ok", `ok`, false},
		{"status only", `{"status":"ok"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			defer ts.Close()
			port := ts.Listener.Addr().(*net.TCPAddr).Port
			if got := probeExisting(port); got != tc.want {
				t.Fatalf("probeExisting = %v, want %v", got, tc.want)
			}
		})
	}
}
