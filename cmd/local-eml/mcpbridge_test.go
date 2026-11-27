package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hwhang0917/local-eml/internal/mcpconf"
	"github.com/hwhang0917/local-eml/internal/server"
)

// fakeMCP stands in for the server's /mcp: it checks the bearer token,
// answers requests with their method echoed, and accepts notifications.
func fakeMCP(t *testing.T, token *atomic.Value, sse bool) int {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != server.MCPPath || r.Header.Get("Authorization") != "Bearer "+token.Load().(string) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var m struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &m)
		if len(m.ID) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		reply := `{"jsonrpc":"2.0","id":` + string(m.ID) + `,"result":{"method":"` + m.Method + `"}}`
		if sse {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "event: message\ndata: "+reply+"\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{\n  \"jsonrpc\": \"2.0\","+reply[len(`{"jsonrpc":"2.0",`):]+"\n")
	}))
	t.Cleanup(ts.Close)
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(ts.URL, "http://"))
	n, _ := strconv.Atoi(port)
	return n
}

func runBridge(t *testing.T, file string, port int, input string) []map[string]any {
	t.Helper()
	var out bytes.Buffer
	b := &mcpBridge{mcpFile: file, port: port, fixed: true, client: http.DefaultClient, out: bufio.NewWriter(&out)}
	if code := b.run(strings.NewReader(input)); code != 0 {
		t.Fatalf("exit %d", code)
	}
	var msgs []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("output line is not one JSON message: %q", line)
		}
		msgs = append(msgs, m)
	}
	return msgs
}

func TestMCPBridgeForwardsOneMessagePerLine(t *testing.T) {
	for _, sse := range []bool{false, true} {
		file := filepath.Join(t.TempDir(), "mcp.json")
		_ = mcpconf.Save(file, mcpconf.Config{Enabled: true, Token: "tok1"})
		var tok atomic.Value
		tok.Store("tok1")
		port := fakeMCP(t, &tok, sse)

		msgs := runBridge(t, file, port,
			`{"jsonrpc":"2.0","id":1,"method":"initialize"}`+"\n"+
				`{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n\n"+
				`{"jsonrpc":"2.0","id":"x","method":"tools/list"}`)
		if len(msgs) != 2 {
			t.Fatalf("sse=%v: got %d replies, want 2: %v", sse, len(msgs), msgs)
		}
	}
}

func TestMCPBridgeRereadsRotatedToken(t *testing.T) {
	file := filepath.Join(t.TempDir(), "mcp.json")
	_ = mcpconf.Save(file, mcpconf.Config{Enabled: true, Token: "old"})
	var tok atomic.Value
	tok.Store("new")
	port := fakeMCP(t, &tok, false)

	// The bridge cached "old"; the server now wants "new", which is on disk.
	var out bytes.Buffer
	b := &mcpBridge{mcpFile: file, port: port, fixed: true, client: http.DefaultClient,
		out: bufio.NewWriter(&out), token: "old"}
	_ = mcpconf.Save(file, mcpconf.Config{Enabled: true, Token: "new"})
	b.run(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"))
	if !strings.Contains(out.String(), `"result"`) {
		t.Errorf("after rotation: %s", out.String())
	}
}

func TestMCPBridgeReportsWhyItCannotConnect(t *testing.T) {
	file := filepath.Join(t.TempDir(), "mcp.json")
	msgs := runBridge(t, file, 1, `{"jsonrpc":"2.0","id":7,"method":"initialize"}`+"\n")
	if len(msgs) != 1 || !strings.Contains(msgs[0]["error"].(map[string]any)["message"].(string), "turned off") {
		t.Errorf("off: %v", msgs)
	}
	if msgs[0]["id"].(float64) != 7 {
		t.Errorf("error should answer the request id: %v", msgs[0])
	}

	_ = mcpconf.Save(file, mcpconf.Config{Enabled: true, Token: "t"})
	// Port 1 on loopback has nothing listening.
	msgs = runBridge(t, file, 1, `{"jsonrpc":"2.0","id":8,"method":"initialize"}`+"\n")
	if len(msgs) != 1 || !strings.Contains(msgs[0]["error"].(map[string]any)["message"].(string), "not running") {
		t.Errorf("not running: %v", msgs)
	}
}
