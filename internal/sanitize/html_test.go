package sanitize

import (
	"strings"
	"testing"
)

func TestStripsScriptTag(t *testing.T) {
	out, err := Sanitize(`<p>hi <script>alert(1)</script></p>`, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(out), "<script") {
		t.Errorf("script not stripped: %s", out)
	}
	if !strings.Contains(out, "hi") {
		t.Errorf("safe content removed: %s", out)
	}
}

func TestStripsEventHandlers(t *testing.T) {
	out, err := Sanitize(`<img src="cid:foo" onload="alert(1)">`, Options{CIDBaseURL: "/cid/"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(out), "onload") {
		t.Errorf("onload not stripped: %s", out)
	}
}

func TestStripsJavascriptURL(t *testing.T) {
	out, err := Sanitize(`<a href="javascript:alert(1)">click</a>`, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(out), "javascript:") {
		t.Errorf("javascript: URL not stripped: %s", out)
	}
}

func TestRewritesCIDToBaseURL(t *testing.T) {
	out, err := Sanitize(`<img src="cid:image001.png">`,
		Options{CIDBaseURL: "/api/emails/abc/cid/"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "/api/emails/abc/cid/image001.png") {
		t.Errorf("cid not rewritten: %s", out)
	}
}

func TestRewritesCIDCaseInsensitive(t *testing.T) {
	out, err := Sanitize(`<img src="CID:Image.PNG">`,
		Options{CIDBaseURL: "/api/emails/x/cid/"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "/api/emails/x/cid/Image.PNG") {
		t.Errorf("uppercase CID not rewritten: %s", out)
	}
}

func TestBlocksRemoteImagesByDefault(t *testing.T) {
	out, err := Sanitize(`<img src="https://tracker.example.com/p.gif" alt="x">`, Options{})
	if err != nil {
		t.Fatal(err)
	}
	// Anchor with a leading space so we don't match the substring inside
	// `data-original-src="..."`.
	if strings.Contains(out, ` src="https://tracker.example.com/p.gif"`) {
		t.Errorf("remote URL stayed in src attribute: %s", out)
	}
	if !strings.Contains(out, ` src="data:image/gif;base64,`) {
		t.Errorf("src should be replaced with placeholder data URL: %s", out)
	}
	if !strings.Contains(out, `data-remote-blocked="1"`) {
		t.Errorf("missing remote-blocked marker: %s", out)
	}
	if !strings.Contains(out, `data-original-src="https://tracker.example.com/p.gif"`) {
		t.Errorf("missing original-src (needed for 'Load remote images' toggle): %s", out)
	}
}

func TestAllowsRemoteImagesWhenOptedIn(t *testing.T) {
	out, err := Sanitize(`<img src="https://example.com/x.png">`,
		Options{ShowRemote: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "example.com/x.png") {
		t.Errorf("opt-in image dropped: %s", out)
	}
	if strings.Contains(out, "data-remote-blocked") {
		t.Errorf("opt-in image should not be marked blocked: %s", out)
	}
}

func TestPreservesSafeStructure(t *testing.T) {
	in := `<table><tr><td style="color:red">cell</td></tr></table>`
	out, err := Sanitize(in, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<table") || !strings.Contains(out, "<td") {
		t.Errorf("table structure dropped: %s", out)
	}
	if !strings.Contains(out, "color:red") {
		t.Errorf("inline style dropped: %s", out)
	}
}
