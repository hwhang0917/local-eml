package store

import "testing"

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
