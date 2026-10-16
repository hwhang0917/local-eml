package risk

import (
	"strings"
	"testing"
)

// Markup nested a few hundred thousand levels deep would exhaust the stack in
// the recursive walks; Assess must give up on it instead of taking the
// process down.
func TestAssessSurvivesDeeplyNestedHTML(t *testing.T) {
	deep := strings.Repeat("<span>", 200_000) + `<a href="http://1.2.3.4/x">bank.example</a>`
	env := msg(t, "From: a@example.com", deep)
	if got := Assess(env); len(got) != 0 {
		t.Fatalf("deep markup must be skipped, got %v", got)
	}
}
