package server

import (
	"testing"
	"time"
)

func TestDayBound(t *testing.T) {
	day := func(y int, m time.Month, d, hh, mm, ss int) int64 {
		return time.Date(y, m, d, hh, mm, ss, 0, time.Local).Unix()
	}
	cases := []struct {
		name     string
		in       string
		endOfDay bool
		want     int64
	}{
		{"start of day", "2026-07-27", false, day(2026, 7, 27, 0, 0, 0)},
		{"end of day", "2026-07-27", true, day(2026, 7, 27, 23, 59, 59)},
		{"padded input", " 2026-01-01 ", false, day(2026, 1, 1, 0, 0, 0)},
		{"empty is unbounded", "", false, 0},
		{"half-typed is unbounded", "2026-07", false, 0},
		{"garbage is unbounded", "not-a-date", true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dayBound(tc.in, tc.endOfDay); got != tc.want {
				t.Errorf("dayBound(%q, %v) = %d, want %d", tc.in, tc.endOfDay, got, tc.want)
			}
		})
	}
}

// A from/to pair naming the same date must select that whole day, not an
// empty instant.
func TestDayBoundSameDayCoversWholeDay(t *testing.T) {
	from, to := dayBound("2026-07-27", false), dayBound("2026-07-27", true)
	if span := to - from; span != 86399 {
		t.Errorf("same-day span = %ds, want 86399", span)
	}
}
