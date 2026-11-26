package server

import (
	"testing"
	"time"
)

func localUnix(y int, m time.Month, d, hh, mm, ss int) int64 {
	return time.Date(y, m, d, hh, mm, ss, 0, time.Local).Unix()
}

func TestDayStart(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int64
	}{
		{"start of day", "2026-07-27", localUnix(2026, 7, 27, 0, 0, 0)},
		{"padded input", " 2026-01-01 ", localUnix(2026, 1, 1, 0, 0, 0)},
		{"empty is unbounded", "", 0},
		{"half-typed is unbounded", "2026-07", 0},
		{"garbage is unbounded", "not-a-date", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dayStart(tc.in); got != tc.want {
				t.Errorf("dayStart(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestDayEnd(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int64
	}{
		{"end of day", "2026-07-27", localUnix(2026, 7, 27, 23, 59, 59)},
		{"empty is unbounded", "", 0},
		{"garbage is unbounded", "not-a-date", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dayEnd(tc.in); got != tc.want {
				t.Errorf("dayEnd(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// A from/to pair naming the same date must select that whole day, not an
// empty instant.
func TestSameDayRangeCoversWholeDay(t *testing.T) {
	from, to := dayStart("2026-07-27"), dayEnd("2026-07-27")
	if span := to - from; span != 86399 {
		t.Errorf("same-day span = %ds, want 86399", span)
	}
}
