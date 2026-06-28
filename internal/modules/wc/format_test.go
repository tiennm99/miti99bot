package wc

import (
	"strings"
	"testing"
	"time"
)

func intPtr(v int) *int {
	return &v
}

func mkMatch(status, home, away, startISO string) Match {
	return Match{
		ID:      1,
		UTCDate: startISO,
		Status:  status,
		Stage:   "GROUP_STAGE",
		Group:   "GROUP_A",
		Venue:   "Estadio Azteca",
		HomeTeam: Team{
			TLA: home,
		},
		AwayTeam: Team{
			TLA: away,
		},
	}
}

func TestFormatMatchLine_Scheduled(t *testing.T) {
	got := formatMatchLine(mkMatch("TIMED", "MEX", "RSA", "2026-06-12T13:00:00Z"))
	for _, want := range []string{"20:00", "MEX vs RSA", "Group A", "Estadio Azteca"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestFormatMatchLine_FinishedBoldsWinner(t *testing.T) {
	m := mkMatch("FINISHED", "MEX", "RSA", "2026-06-12T13:00:00Z")
	m.Score = Score{
		Winner:   "HOME_TEAM",
		FullTime: ScoreValue{Home: intPtr(2), Away: intPtr(0)},
	}
	got := formatMatchLine(m)
	for _, want := range []string{"FT", "<b>MEX</b>", "2-0", "RSA"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestFormatMatchLine_LiveScore(t *testing.T) {
	m := mkMatch("IN_PLAY", "CAN", "SUI", "2026-06-13T13:00:00Z")
	m.Score = Score{FullTime: ScoreValue{Home: intPtr(1), Away: intPtr(1)}}
	got := formatMatchLine(m)
	if !strings.Contains(got, "LIVE CAN 1-1 SUI") {
		t.Fatalf("live line = %q", got)
	}
}

func TestRenderToday_Empty(t *testing.T) {
	day := time.Date(2026, 6, 12, 0, 0, 0, 0, IctLocation)
	got := RenderToday(nil, day)
	if !strings.Contains(got, "World Cup") || !strings.Contains(got, "No matches today") {
		t.Fatalf("empty render = %q", got)
	}
}

func TestRenderWeek_GroupsByDay(t *testing.T) {
	from := time.Date(2026, 6, 12, 0, 0, 0, 0, IctLocation)
	matches := []Match{
		mkMatch("TIMED", "MEX", "RSA", "2026-06-12T13:00:00Z"),
		mkMatch("TIMED", "CAN", "SUI", "2026-06-13T13:00:00Z"),
	}
	got := RenderWeek(matches, from, addDays(from, 7))
	for _, want := range []string{"Fri Jun 12", "Sat Jun 13", "MEX vs RSA", "CAN vs SUI"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}
