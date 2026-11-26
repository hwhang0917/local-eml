package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSplitFrom(t *testing.T) {
	for _, c := range []struct{ in, name, addr string }{
		{"Alice <Alice@Example.com>", "Alice", "alice@example.com"},
		{"alice@example.com", "", "alice@example.com"},
		{`"Kim, Minsu" <minsu@x.kr>`, "Kim, Minsu", "minsu@x.kr"},
		// net/mail rejects the unquoted dot; the <> fallback still finds the address.
		{"J. Doe <j@x.com>", "J. Doe", "j@x.com"},
		{"not an address", "", "not an address"},
	} {
		name, addr := splitFrom(c.in)
		if name != c.name || addr != c.addr {
			t.Errorf("splitFrom(%q) = %q, %q; want %q, %q", c.in, name, addr, c.name, c.addr)
		}
	}
}

// The SPA iterates every list without a null check, so an empty library must
// report [] rather than null.
func TestStats_EmptyLibraryListsAreEmptyNotNull(t *testing.T) {
	s := newTestStore(t)
	st, err := s.Stats(context.Background(), 0)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "null") {
		t.Errorf("stats JSON has a null list: %s", b)
	}
}
