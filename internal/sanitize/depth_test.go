package sanitize

import (
	"errors"
	"strings"
	"testing"
)

// Rendering the parsed tree recurses once per level, so the parser's refusal
// of markup nested past its open-element limit is what protects the stack.
// Pin that it stays an error rather than ever becoming a crash.
func TestSanitizeRefusesDeeplyNestedHTML(t *testing.T) {
	deep := strings.Repeat("<span>", 200_000) + "hi"
	if _, err := Sanitize(deep, Options{}); !errors.Is(err, ErrBadMarkup) {
		t.Fatalf("err = %v, want ErrBadMarkup", err)
	}
	if _, err := Sanitize(strings.Repeat("<span>", 100)+"hi", Options{}); err != nil {
		t.Fatalf("ordinary nesting must pass: %v", err)
	}
}
