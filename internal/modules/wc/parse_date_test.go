package wc

import (
	"strings"
	"testing"
	"time"
)

var refNow = time.Date(2026, 6, 12, 5, 0, 0, 0, time.UTC)

func TestParseScheduleDate(t *testing.T) {
	wantToday := time.Date(2026, 6, 12, 0, 0, 0, 0, IctLocation).UTC()
	if got := ParseScheduleDate("", refNow); !got.OK || !got.Date.Equal(wantToday) {
		t.Fatalf("empty = %+v, want %v", got, wantToday)
	}

	wantFull := time.Date(2026, 7, 15, 0, 0, 0, 0, IctLocation).UTC()
	for _, in := range []string{"15-07-2026", "15/07/2026", "15072026"} {
		got := ParseScheduleDate(in, refNow)
		if !got.OK || !got.Date.Equal(wantFull) {
			t.Fatalf("%q = %+v, want %v", in, got, wantFull)
		}
	}

	if got := ParseScheduleDate("notadate", refNow); got.OK || !strings.Contains(got.Error, "Invalid date") {
		t.Fatalf("invalid = %+v, want error", got)
	}
}

func TestIctWeekStartOf(t *testing.T) {
	// refNow is Fri 2026-06-12 ICT. Monday is 2026-06-08 00:00 ICT.
	want := time.Date(2026, 6, 8, 0, 0, 0, 0, IctLocation).UTC()
	if got := ictWeekStartOf(refNow); !got.Equal(want) {
		t.Fatalf("week start = %v, want %v", got, want)
	}
}
