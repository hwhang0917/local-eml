package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hwhang0917/local-eml/internal/server"
)

// probeExisting reports whether a local-eml server already answers on port —
// app mode reuses it instead of racing a second instance onto the same DB.
// Any other program on the port, even one with its own /healthz, is not one.
func probeExisting(port int) bool {
	c := &http.Client{Timeout: time.Second}
	resp, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var h struct{ Status, App, Version string }
	if json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&h) != nil || h.Status != "ok" {
		return false
	}
	// Versions before the app field reported only status and version.
	return h.App == server.HealthApp || (h.App == "" && h.Version != "")
}

// findExisting returns the port of a local-eml server in the range serve
// falls back through, or 0 when none answers.
func findExisting() int {
	for port := defaultPort; port < defaultPort+portAttempts; port++ {
		if probeExisting(port) {
			return port
		}
	}
	return 0
}

// openWindow opens url in a chromeless --app window when a chromium-family
// browser is around, else falls back to the default browser.
func openWindow(url string) {
	if bin := chromiumBin(); bin != "" {
		if exec.Command(bin, "--app="+url).Start() == nil {
			return
		}
	}
	openDefault(url)
}

func chromiumBin() string {
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		// An unset variable would leave a relative path, which Stat resolves
		// against the current folder: whatever msedge.exe happens to sit
		// there would be run.
		for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
			if dir := os.Getenv(env); filepath.IsAbs(dir) {
				candidates = append(candidates, filepath.Join(dir, `Microsoft\Edge\Application\msedge.exe`))
			}
		}
		candidates = append(candidates, "msedge", "chrome")
	case "darwin":
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	default:
		candidates = []string{"google-chrome", "chromium", "chromium-browser", "brave-browser", "microsoft-edge"}
	}
	for _, c := range candidates {
		if strings.ContainsAny(c, `/\`) {
			if _, err := os.Stat(c); err == nil {
				return c
			}
			continue
		}
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

func openDefault(url string) {
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		_ = exec.Command("open", url).Start()
	default:
		_ = exec.Command("xdg-open", url).Start()
	}
}
