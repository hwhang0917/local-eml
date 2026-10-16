package server

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// SyncIntervalSettingKey is the settings row holding the IMAP poll interval in
// seconds. It overrides the LOCAL_EML_SYNC_INTERVAL env var once set from the UI.
const SyncIntervalSettingKey = "imap_sync_interval_seconds"

// Bounds for UI-set intervals: at least a minute to stay polite to remote IMAP
// servers, at most a day. 0 is always allowed and pauses the syncer.
const (
	minSyncInterval = time.Minute
	maxSyncInterval = 24 * time.Hour
)

type syncIntervalPayload struct {
	Seconds int `json:"seconds"`
}

func (s *Server) handleGetSyncInterval(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, syncIntervalPayload{Seconds: int(s.IMAPSyncInterval().Seconds())})
}

func (s *Server) handlePutSyncInterval(w http.ResponseWriter, r *http.Request) {
	var p syncIntervalPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	d := time.Duration(p.Seconds) * time.Second
	if p.Seconds != 0 && (d < minSyncInterval || d > maxSyncInterval) {
		http.Error(w, fmt.Sprintf("seconds must be 0 (off) or between %d and %d",
			int(minSyncInterval.Seconds()), int(maxSyncInterval.Seconds())), http.StatusBadRequest)
		return
	}
	if err := s.Store.SetSetting(r.Context(), SyncIntervalSettingKey, strconv.Itoa(p.Seconds)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.SetIMAPSyncInterval(d)
	writeJSON(w, http.StatusOK, syncIntervalPayload{Seconds: p.Seconds})
}
