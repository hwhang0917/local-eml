package server

import (
	"net/http"
	"time"
)

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.Store.Stats(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// handleStatsCalendar returns per-local-day counts for one month:
// GET /api/stats/calendar?month=YYYY-MM -> {"2026-08-01": 3, ...}.
// Unlike dayBound's silent-zero tolerance for half-typed filter input, a bad
// month here is a client bug and gets a 400.
func (s *Server) handleStatsCalendar(w http.ResponseWriter, r *http.Request) {
	start, err := time.ParseInLocation("2006-01", r.URL.Query().Get("month"), time.Local)
	if err != nil {
		http.Error(w, "month must be YYYY-MM", http.StatusBadRequest)
		return
	}
	end := start.AddDate(0, 1, 0).Add(-time.Second) // same end-of-day convention as dayBound
	counts, err := s.Store.CountEmailsByDay(r.Context(), start.Unix(), end.Unix())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, counts)
}
