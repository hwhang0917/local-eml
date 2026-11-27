package server

import (
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hwhang0917/local-eml/internal/mcpconf"
)

// MCPPath is where AI agents connect (MCP over streamable HTTP). It sits
// outside /api: it has its own token and never accepts the browser's
// session cookie, so a web page can't reach it through the user's sign-in.
const MCPPath = "/mcp"

// mcpState caches the agent switch and token; the file is the source of
// truth and only this server writes it while running.
type mcpState struct {
	mu     sync.Mutex
	loaded bool
	cfg    mcpconf.Config
}

func (s *Server) mcpConfig() (mcpconf.Config, error) {
	s.mcp.mu.Lock()
	defer s.mcp.mu.Unlock()
	if !s.mcp.loaded {
		if s.MCPFile == "" {
			return mcpconf.Config{}, nil
		}
		c, err := mcpconf.Load(s.MCPFile)
		if err != nil {
			return mcpconf.Config{}, err
		}
		s.mcp.cfg, s.mcp.loaded = c, true
	}
	return s.mcp.cfg, nil
}

// updateMCPConfig applies change to the stored config and saves it.
func (s *Server) updateMCPConfig(change func(*mcpconf.Config) error) (mcpconf.Config, error) {
	if _, err := s.mcpConfig(); err != nil {
		return mcpconf.Config{}, err
	}
	s.mcp.mu.Lock()
	defer s.mcp.mu.Unlock()
	if s.MCPFile == "" {
		return mcpconf.Config{}, fmt.Errorf("agent settings are not configured")
	}
	next := s.mcp.cfg
	if err := change(&next); err != nil {
		return mcpconf.Config{}, err
	}
	if err := mcpconf.Save(s.MCPFile, next); err != nil {
		return mcpconf.Config{}, err
	}
	s.mcp.cfg = next
	return next, nil
}

// mcpHandler serves the MCP endpoint. It is stateless with plain JSON
// replies: every tool is a single read, so there is nothing to stream or
// resume, and a stateless server needs no session cleanup.
func (s *Server) mcpHandler() http.Handler {
	srv := s.NewMCPServer()
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	return s.requireMCPToken(h)
}

// requireMCPToken lets a request through only while agent access is on and
// it carries the MCP token as a bearer header.
func (s *Server) requireMCPToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg, err := s.mcpConfig()
		if err != nil {
			slog.Error("read mcp settings", "err", err)
			http.Error(w, "could not read agent settings", http.StatusInternalServerError)
			return
		}
		if !cfg.Enabled || cfg.Token == "" {
			http.Error(w, "AI agent access is turned off. Turn it on in Local Eml under Settings > AI agents.",
				http.StatusForbidden)
			return
		}
		h := r.Header.Get("Authorization")
		got, ok := strings.CutPrefix(h, bearerPrefix)
		if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(cfg.Token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="local-eml"`)
			http.Error(w, "unauthorized: copy the agent token from Local Eml under Settings > AI agents",
				http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// mcpInfo is what the Settings page shows.
type mcpInfo struct {
	Enabled bool   `json:"enabled"`
	Token   string `json:"token"`
	URL     string `json:"url"`
	// Executable is this binary's path, for the stdio config that runs
	// `local-eml mcp`.
	Executable string `json:"executable"`
	// WSL is set when the server runs inside WSL, where a Windows agent has
	// to start the bridge through wsl.exe.
	WSL   bool     `json:"wsl"`
	Tools []string `json:"tools"`
}

func (s *Server) mcpInfo(cfg mcpconf.Config) mcpInfo {
	exe, _ := os.Executable()
	info := mcpInfo{
		Enabled:    cfg.Enabled,
		URL:        fmt.Sprintf("http://127.0.0.1:%d%s", s.Port, MCPPath),
		Executable: exe,
		WSL:        runningInWSL(),
		Tools:      mcpToolNames(),
	}
	// The token is shown only while access is on: off means no agent should
	// be configured, and the page has nothing to copy.
	if cfg.Enabled {
		info.Token = cfg.Token
	}
	return info
}

// GET /api/mcp
func (s *Server) handleGetMCP(w http.ResponseWriter, _ *http.Request) {
	cfg, err := s.mcpConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, s.mcpInfo(cfg))
}

// PUT /api/mcp  {"enabled": bool}
//
// The first time access is turned on a token is minted. Turning it off keeps
// the token, so turning it back on doesn't break agents already set up; the
// endpoint refuses everything while off either way.
func (s *Server) handlePutMCP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Enabled == nil {
		http.Error(w, "enabled is required", http.StatusBadRequest)
		return
	}
	cfg, err := s.updateMCPConfig(func(c *mcpconf.Config) error {
		c.Enabled = *body.Enabled
		if c.Enabled && c.Token == "" {
			t, err := mcpconf.NewToken()
			if err != nil {
				return err
			}
			c.Token = t
		}
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("ai agent access changed", "enabled", cfg.Enabled)
	writeJSON(w, http.StatusOK, s.mcpInfo(cfg))
}

// POST /api/mcp/token replaces the token. Agents using the old one are
// refused from the next request on.
func (s *Server) handleRotateMCPToken(w http.ResponseWriter, _ *http.Request) {
	cfg, err := s.updateMCPConfig(func(c *mcpconf.Config) error {
		if !c.Enabled {
			return errMCPOff
		}
		t, err := mcpconf.NewToken()
		if err != nil {
			return err
		}
		c.Token = t
		return nil
	})
	if err == errMCPOff {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("ai agent token rotated")
	writeJSON(w, http.StatusOK, s.mcpInfo(cfg))
}

var errMCPOff = fmt.Errorf("turn AI agent access on first")

// runningInWSL detects Windows Subsystem for Linux from the kernel release,
// which says "microsoft" there (WSL1 and WSL2 alike).
func runningInWSL() bool {
	b, err := os.ReadFile("/proc/sys/kernel/osrelease")
	return err == nil && strings.Contains(strings.ToLower(string(b)), "microsoft")
}
