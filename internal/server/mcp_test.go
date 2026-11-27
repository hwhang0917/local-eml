package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hwhang0917/local-eml/internal/mcpconf"
	"github.com/hwhang0917/local-eml/internal/store"
)

const mcpLeaseEML = "From: Landlord <landlord@example.com>\r\n" +
	"To: me@example.com\r\n" +
	"Subject: Lease renewal\r\n" +
	"MIME-Version: 1.0\r\n" +
	"Content-Type: multipart/mixed; boundary=BOUND\r\n" +
	"\r\n" +
	"--BOUND\r\n" +
	"Content-Type: text/plain; charset=utf-8\r\n" +
	"\r\n" +
	"Hi, the rent goes up 5% from March. 임대료 인상 안내.\r\n" +
	"--BOUND\r\n" +
	"Content-Type: text/csv; name=terms.csv\r\n" +
	"Content-Disposition: attachment; filename=terms.csv\r\n" +
	"\r\n" +
	"item,amount\r\nrent,1260\r\n" +
	"--BOUND\r\n" +
	"Content-Type: image/png; name=photo.png\r\n" +
	"Content-Disposition: attachment; filename=photo.png\r\n" +
	"Content-Transfer-Encoding: base64\r\n" +
	"\r\n" +
	"iVBORw0KGgo=\r\n" +
	"--BOUND--\r\n"

// newMCPServer seeds a lease thread, a newsletter and a flagged phish, and
// serves the router over real HTTP so the SDK client can talk to it.
func newMCPServer(t *testing.T) (*Server, *httptest.Server, func(string) string) {
	t.Helper()
	s, p := newDriftServer(t)
	s.MCPFile = filepath.Join(p.Base, "mcp.json")
	sha := func(c string) string { return strings.Repeat(c, 64) }
	day := func(d int) time.Time { return time.Date(2026, 3, d, 12, 0, 0, 0, time.UTC) }
	rows := []struct {
		e    store.Email
		body string
		blob string
	}{
		{store.Email{SHA256: sha("a"), ThreadID: "lease@x", Subject: "Lease renewal", FromAddr: "Landlord <landlord@example.com>",
			SentAt: day(2), HasAttachments: true, AttachmentCount: 2}, "rent goes up", mcpLeaseEML},
		{store.Email{SHA256: sha("b"), ThreadID: "lease@x", Subject: "Re: Lease renewal", FromAddr: "me@example.com",
			SentAt: day(5)}, "fine by me", "Subject: Re: Lease renewal\r\n\r\nfine by me\r\n"},
		{store.Email{SHA256: sha("c"), Subject: "Newsletter", FromAddr: "news@example.com", SentAt: day(1)},
			"weekly news", "Subject: Newsletter\r\n\r\nweekly news\r\n"},
		{store.Email{SHA256: sha("d"), Subject: "Lease renewal urgent", FromAddr: "landlord@evil.example", SentAt: day(3),
			Flag: "phishing", HasAttachments: true, AttachmentCount: 2}, "rent goes up", mcpLeaseEML},
	}
	for _, r := range rows {
		row := store.EmailRow{Email: r.e, BodyText: r.body}
		if r.e.HasAttachments {
			row.Attachments = []store.AttachmentMeta{
				{Index: 0, Filename: "terms.csv", ContentType: "text/csv", Size: 20},
				{Index: 1, Filename: "photo.png", ContentType: "image/png", Size: 8},
			}
		}
		if _, err := s.Store.InsertEmail(context.Background(), row); err != nil {
			t.Fatal(err)
		}
		if r.e.Flag != "" {
			if err := s.Store.SetEmailFlag(context.Background(), r.e.SHA256, r.e.Flag); err != nil {
				t.Fatal(err)
			}
		}
		writeBlobAs(t, p, r.e.SHA256, r.blob)
	}
	ts := httptest.NewServer(s.Router())
	t.Cleanup(ts.Close)
	return s, ts, sha
}

func setMCP(t *testing.T, s *Server, enabled bool) mcpInfo {
	t.Helper()
	body := `{"enabled":false}`
	if enabled {
		body = `{"enabled":true}`
	}
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodPut, "/api/mcp", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/mcp: %d %s", rec.Code, rec.Body)
	}
	var info mcpInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	return info
}

func postMCP(t *testing.T, ts *httptest.Server, token string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, ts.URL+MCPPath,
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

type bearerRT struct{ token string }

func (b bearerRT) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func connectMCP(t *testing.T, ts *httptest.Server, token string) *mcp.ClientSession {
	t.Helper()
	c := mcp.NewClient(&mcp.Implementation{Name: "test-agent", Version: "1"}, nil)
	cs, err := c.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   ts.URL + MCPPath,
		HTTPClient: &http.Client{Transport: bearerRT{token}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// call runs a tool and decodes its JSON result into out, returning the error
// text when the tool reported one.
func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any, out any) string {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if res.IsError {
		return text
	}
	if err := json.Unmarshal([]byte(text), out); err != nil {
		t.Fatalf("%s: decode %q: %v", name, text, err)
	}
	return ""
}

func TestMCPOffByDefaultAndTokenRules(t *testing.T) {
	s, ts, _ := newMCPServer(t)

	if code := postMCP(t, ts, ""); code != http.StatusForbidden {
		t.Errorf("off: got %d, want 403", code)
	}
	if _, err := os.Stat(s.MCPFile); !os.IsNotExist(err) {
		t.Error("reading the switch must not create the file")
	}

	info := setMCP(t, s, true)
	if !info.Enabled || !strings.HasPrefix(info.Token, mcpconf.TokenPrefix) {
		t.Fatalf("enable: %+v", info)
	}
	if code := postMCP(t, ts, info.Token); code != http.StatusOK {
		t.Errorf("good token: %d", code)
	}
	for name, tok := range map[string]string{"none": "", "wrong": "nope", "session token": testToken} {
		if code := postMCP(t, ts, tok); code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", name, code)
		}
	}

	// Rotating locks out the old token at once.
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodPost, "/api/mcp/token", nil))
	var rotated mcpInfo
	_ = json.Unmarshal(rec.Body.Bytes(), &rotated)
	if rec.Code != http.StatusOK || rotated.Token == info.Token {
		t.Fatalf("rotate: %d %+v", rec.Code, rotated)
	}
	if code := postMCP(t, ts, info.Token); code != http.StatusUnauthorized {
		t.Errorf("old token after rotate: %d", code)
	}

	// Off hides the token and refuses even the right one; on again keeps it.
	if off := setMCP(t, s, false); off.Token != "" {
		t.Error("token shown while off")
	}
	if code := postMCP(t, ts, rotated.Token); code != http.StatusForbidden {
		t.Errorf("turned off: got %d, want 403", code)
	}
	if on := setMCP(t, s, true); on.Token != rotated.Token {
		t.Error("turning back on should keep the token")
	}

	// The file is owner-only and survives a restart of the server.
	if fi, err := os.Stat(s.MCPFile); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("mcp.json mode: %v %v", fi.Mode(), err)
	}
	fresh := &Server{Store: s.Store, Importer: s.Importer, Token: testToken, MCPFile: s.MCPFile}
	if cfg, _ := fresh.mcpConfig(); !cfg.Enabled || cfg.Token != rotated.Token {
		t.Errorf("reloaded config: %+v", cfg)
	}

	// Cross-site pages can't use it: the loopback guard checks Origin.
	req, _ := http.NewRequest(http.MethodPost, ts.URL+MCPPath, strings.NewReader(`{}`))
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Authorization", "Bearer "+rotated.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-origin: %d", resp.StatusCode)
	}
}

func TestMCPToolsAreReadOnly(t *testing.T) {
	s, ts, _ := newMCPServer(t)
	cs := connectMCP(t, ts, setMCP(t, s, true).Token)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s is not marked read-only", tool.Name)
		}
	}
	want := slices.Sorted(slices.Values(mcpTools))
	if slices.Sort(names); !slices.Equal(names, want) {
		t.Errorf("tools %v, want %v", names, want)
	}
	if init := cs.InitializeResult(); !strings.Contains(init.Instructions, "untrusted") {
		t.Error("instructions should warn that mail is untrusted")
	}
}

func TestMCPSearchReadAndAttachments(t *testing.T) {
	s, ts, sha := newMCPServer(t)
	cs := connectMCP(t, ts, setMCP(t, s, true).Token)

	var found searchOut
	if msg := call(t, cs, "search_emails", map[string]any{"query": "lease"}, &found); msg != "" {
		t.Fatal(msg)
	}
	// The phish matches too but is hidden, like in the library.
	if found.Total != 2 || found.Emails[0].ID != sha("b") || found.Emails[1].ID != sha("a") {
		t.Errorf("search: %+v", found)
	}
	if msg := call(t, cs, "search_emails", map[string]any{"query": "from:news"}, &found); msg != "" || found.Total != 1 {
		t.Errorf("operator search: %q %+v", msg, found)
	}

	var mail readEmailOut
	if msg := call(t, cs, "read_email", map[string]any{"id": strings.ToUpper(sha("a"))}, &mail); msg != "" {
		t.Fatal(msg)
	}
	if !strings.Contains(mail.Body, "임대료 인상") || mail.ThreadSize != 2 || len(mail.AttachmentList) != 2 {
		t.Errorf("read_email: %+v", mail)
	}
	if !mail.AttachmentList[0].Readable || mail.AttachmentList[1].Readable {
		t.Errorf("readable flags: %+v", mail.AttachmentList)
	}

	// Paging a body by characters, not bytes.
	if msg := call(t, cs, "read_email", map[string]any{"id": sha("a"), "offset": 4, "max_chars": 3}, &mail); msg != "" {
		t.Fatal(msg)
	}
	if mail.Body != "the" || !mail.Truncated || mail.BodyOffset != 4 {
		t.Errorf("paged body: %q truncated=%v offset=%d", mail.Body, mail.Truncated, mail.BodyOffset)
	}

	var thread threadOut
	if msg := call(t, cs, "get_thread", map[string]any{"id": sha("b")}, &thread); msg != "" {
		t.Fatal(msg)
	}
	if len(thread.Emails) != 2 || thread.Emails[0].ID != sha("a") {
		t.Errorf("thread: %+v", thread)
	}

	var atts listAttachmentsOut
	if msg := call(t, cs, "list_attachments", map[string]any{"kind": "spreadsheet"}, &atts); msg != "" {
		t.Fatal(msg)
	}
	if atts.Total != 1 || atts.Attachments[0].Filename != "terms.csv" || atts.Attachments[0].EmailID != sha("a") {
		t.Errorf("list_attachments: %+v", atts)
	}
	if msg := call(t, cs, "list_attachments", map[string]any{"kind": "spaceship"}, &atts); !strings.Contains(msg, "kind must be") {
		t.Errorf("bad kind: %q", msg)
	}

	var att readAttachmentOut
	if msg := call(t, cs, "read_attachment", map[string]any{"email_id": sha("a"), "index": 0}, &att); msg != "" {
		t.Fatal(msg)
	}
	if att.Text != "item,amount\nrent,1260" {
		t.Errorf("attachment text: %q", att.Text)
	}
	if msg := call(t, cs, "read_attachment", map[string]any{"email_id": sha("a"), "index": 1}, &att); !strings.Contains(msg, "no text") {
		t.Errorf("image: %q", msg)
	}
	if msg := call(t, cs, "read_attachment", map[string]any{"email_id": sha("a"), "index": 9}, &att); !strings.Contains(msg, "index must be") {
		t.Errorf("out of range: %q", msg)
	}
}

func TestMCPFlaggedMail(t *testing.T) {
	s, ts, sha := newMCPServer(t)
	cs := connectMCP(t, ts, setMCP(t, s, true).Token)

	var mail readEmailOut
	if msg := call(t, cs, "read_email", map[string]any{"id": sha("d")}, &mail); msg != "" {
		t.Fatal(msg)
	}
	if !strings.Contains(mail.Warning, "phishing") || mail.AttachmentList[0].Readable {
		t.Errorf("flagged read_email: %+v", mail)
	}
	var att readAttachmentOut
	if msg := call(t, cs, "read_attachment", map[string]any{"email_id": sha("d"), "index": 0}, &att); !strings.Contains(msg, "flagged") {
		t.Errorf("flagged attachment should be refused, got %q", msg)
	}
	if msg := call(t, cs, "read_email", map[string]any{"id": "../../etc/passwd"}, &mail); !strings.Contains(msg, "no message") {
		t.Errorf("bad id: %q", msg)
	}
}
