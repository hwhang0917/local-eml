package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hwhang0917/local-eml/internal/mcpconf"
	"github.com/hwhang0917/local-eml/internal/paths"
	"github.com/hwhang0917/local-eml/internal/server"
)

// runMCP is a stdio bridge for agents that start MCP servers as programs
// (Claude Desktop, most desktop clients): it reads JSON-RPC messages from
// stdin, forwards each to the running server's /mcp endpoint, and writes the
// replies to stdout. It reads the agent token from the data folder itself, so
// the agent's config file holds no secret, and it finds the server on
// whichever port it ended up on.
//
// It holds no mail and opens no database: with agent access turned off in
// Settings, every request comes back refused.
func runMCP(args []string) int {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	port := fs.Int("port", 0, "port of the running server (default: find it)")
	_ = fs.Parse(args)

	p, err := paths.Resolve()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	b := &mcpBridge{
		mcpFile: p.MCPFile(),
		port:    *port,
		fixed:   *port != 0,
		client:  &http.Client{Timeout: 5 * time.Minute},
		out:     bufio.NewWriter(os.Stdout),
	}
	return b.run(os.Stdin)
}

type mcpBridge struct {
	mcpFile string
	client  *http.Client

	mu       sync.Mutex // guards port, token, protocol
	port     int
	fixed    bool
	token    string
	protocol string

	outMu sync.Mutex
	out   *bufio.Writer
}

func (b *mcpBridge) run(in io.Reader) int {
	r := bufio.NewReaderSize(in, 1<<20)
	var wg sync.WaitGroup
	for {
		line, err := r.ReadBytes('\n')
		if msg := bytes.TrimSpace(line); len(msg) > 0 {
			wg.Add(1)
			// Requests run side by side, as they would over HTTP, so a long
			// attachment read doesn't hold up a quick search.
			go func(msg []byte) {
				defer wg.Done()
				b.forward(msg)
			}(append([]byte(nil), msg...))
		}
		if err != nil {
			wg.Wait()
			if errors.Is(err, io.EOF) {
				return 0
			}
			fmt.Fprintln(os.Stderr, "read stdin:", err)
			return 1
		}
	}
}

// forward sends one message and writes back whatever answers it. Failures
// become JSON-RPC errors on the request's id, so the agent shows the reason
// ("access is turned off", "not running") instead of hanging.
func (b *mcpBridge) forward(msg []byte) {
	var head struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	_ = json.Unmarshal(msg, &head)

	body, err := b.post(msg, true)
	if err != nil {
		if len(head.ID) == 0 || string(head.ID) == "null" {
			fmt.Fprintln(os.Stderr, "local-eml mcp:", err)
			return
		}
		b.reply(rpcError(head.ID, err.Error()))
		return
	}
	for _, m := range body {
		if head.Method == "initialize" {
			b.noteProtocol(m)
		}
		b.reply(m)
	}
}

func (b *mcpBridge) post(msg []byte, retry bool) ([][]byte, error) {
	port, token, protocol, err := b.target()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d%s", port, server.MCPPath),
		bytes.NewReader(msg))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)
	if protocol != "" {
		req.Header.Set("Mcp-Protocol-Version", protocol)
	}
	resp, err := b.client.Do(req)
	if err != nil {
		// The server may have restarted on another port; look again once.
		if retry && b.forget(false) {
			return b.post(msg, false)
		}
		return nil, errors.New("Local Eml is not running. Start it, then try again")
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized && retry:
		// The token was rotated in Settings; read the new one.
		b.forget(true)
		return b.post(msg, false)
	case resp.StatusCode == http.StatusAccepted:
		return nil, nil
	case resp.StatusCode != http.StatusOK:
		return nil, errors.New(strings.TrimSpace(string(data)))
	}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		return sseData(data), nil
	}
	return [][]byte{bytes.TrimSpace(data)}, nil
}

// target returns where to send and with what token, looking both up the
// first time.
func (b *mcpBridge) target() (port int, token, protocol string, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.token == "" {
		cfg, err := mcpconf.Load(b.mcpFile)
		if err != nil {
			return 0, "", "", fmt.Errorf("read %s: %w", b.mcpFile, err)
		}
		if !cfg.Enabled || cfg.Token == "" {
			return 0, "", "", errors.New("AI agent access is turned off. Turn it on in Local Eml under Settings > AI agents")
		}
		b.token = cfg.Token
	}
	if b.port == 0 {
		b.port = findExisting()
		if b.port == 0 {
			return 0, "", "", errors.New("Local Eml is not running. Start it, then try again")
		}
	}
	return b.port, b.token, b.protocol, nil
}

// forget drops the cached port (or token) so the next target looks again,
// reporting whether there was anything to drop.
func (b *mcpBridge) forget(token bool) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if token {
		b.token = ""
		return true
	}
	if b.fixed || b.port == 0 {
		return false
	}
	b.port = 0
	return true
}

func (b *mcpBridge) noteProtocol(m []byte) {
	var r struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
	}
	if json.Unmarshal(m, &r) == nil && r.Result.ProtocolVersion != "" {
		b.mu.Lock()
		b.protocol = r.Result.ProtocolVersion
		b.mu.Unlock()
	}
}

func (b *mcpBridge) reply(m []byte) {
	b.outMu.Lock()
	defer b.outMu.Unlock()
	// Stdio framing is one message per line; the server's JSON has no raw
	// newlines, but compact it anyway in case.
	var buf bytes.Buffer
	if json.Compact(&buf, m) == nil {
		m = buf.Bytes()
	}
	_, _ = b.out.Write(m)
	_ = b.out.WriteByte('\n')
	_ = b.out.Flush()
}

func rpcError(id json.RawMessage, msg string) []byte {
	out, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": -32000, "message": msg},
	})
	return out
}

// sseData pulls the messages out of a text/event-stream body.
func sseData(body []byte) [][]byte {
	var out [][]byte
	var cur []byte
	flush := func() {
		if len(cur) > 0 {
			out = append(out, cur)
			cur = nil
		}
	}
	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimRight(line, "\r")
		if len(line) == 0 {
			flush()
			continue
		}
		if d, ok := bytes.CutPrefix(line, []byte("data:")); ok {
			if len(cur) > 0 {
				cur = append(cur, '\n')
			}
			cur = append(cur, bytes.TrimPrefix(d, []byte(" "))...)
		}
	}
	flush()
	return out
}
