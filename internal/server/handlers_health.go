package server

import (
	"net/http"
)

// handleHealth doubles as the SPA's heartbeat. The version lets a page that
// outlived a binary swap (self-update, reinstall) notice and reload itself.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": s.Version})
}
