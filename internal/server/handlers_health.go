package server

import (
	"net/http"
)

// HealthApp is the app name /healthz reports.
const HealthApp = "local-eml"

// handleHealth doubles as the SPA's heartbeat. The version lets a page that
// outlived a binary swap (self-update, reinstall) notice and reload itself.
// The app name lets a launcher tell a local-eml server from another program
// that happens to hold the port.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "app": HealthApp, "version": s.Version})
}
