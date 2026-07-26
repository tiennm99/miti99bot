package lol

import (
	"strings"
	"testing"
	"time"
)

func mkEvent(state, slug, name, t1Code, t2Code, startISO string) ScheduleEvent {
	return ScheduleEvent{
		StartTime: startISO,
		State:     state,
		League:    League{Slug: slug, Name: name},
		Match: Match{
			Teams:    []Team{{Code: t1Code}, {Code: t2Code}},
			Strategy: Strategy{Type: "bestOf", Count: 3},
		},
	}
}

func TestFormatEventLine_Unstarted(t *testing.T) {
	e := mkEvent("unstarted", "lck", "LCK", "T1", "GEN", "2026-05-09T05:00:00Z")
	got := formatEventLine(e)
	if !strings.Contains(got, "🕒") {
		t.Errorf("missing clock emoji: %q", got)
	}
	if !strings.Contains(got, "T1 vs GEN") {
		t.Errorf("missing team labels: %q", got)
	}
	if !strings.Contains(got, "Bo3") {
		t.Errorf("missing Bo3: %q", got)
	}
	// 05:00 UTC == 12:00 ICT.
	if !strings.Contains(got, "12:00") {
		t.Errorf("ICT time wrong; got %q (expected 12:00)", got)
	}
}

func TestFormatEventLine_Completed_BoldsWinner(t *testing.T) {
	winResult := &TeamResult{Outcome: "win", GameWins: 3}
	loseResult := &TeamResult{Outcome: "loss", GameWins: 1}
	e := ScheduleEvent{
		StartTime: "2026-05-09T05:00:00Z",
		State:     "completed",
		League:    League{Slug: "lck", Name: "LCK"},
		Match: Match{
			Teams: []Team{
				{Code: "T1", Result: winResult},
				{Code: "GEN", Result: loseResult},
			},
			Strategy: Strategy{Count: 5},
		},
	}
	got := formatEventLine(e)
	if !strings.Contains(got, "✅") {
		t.Errorf("missing completed emoji: %q", got)
	}
	if !strings.Contains(got, "<b>T1</b>") {
		t.Errorf("winner not bolded: %q", got)
	}
	if !strings.Contains(got, "3–1") {
		t.Errorf("score missing: %q", got)
	}
	if strings.Contains(got, "<b>GEN</b>") {
		t.Errorf("loser should not be bolded: %q", got)
	}
}

// Upstream flips state to "completed" when the broadcast window closes, but
// fills gameWins/outcome from a separate per-game ingestion path. In the gap it
// sends {"outcome": null, "gameWins": 0} for both teams — a shape that must not
// be reported as a real 0–0 draw.
func TestFormatEventLine_CompletedWithoutResults_OmitsScore(t *testing.T) {
	pending := &TeamResult{} // json `{"outcome": null, "gameWins": 0}`
	e := ScheduleEvent{
		StartTime: "2026-07-25T17:30:00Z",
		State:     "completed",
		BlockName: "Week 1",
		League:    League{Slug: "lec", Name: "LEC"},
		Match: Match{
			Teams: []Team{
				{Code: "MKOI", Result: pending},
				{Code: "KC", Result: pending},
			},
			Strategy: Strategy{Type: "bestOf", Count: 3},
		},
	}
	got := formatEventLine(e)
	if strings.Contains(got, "0–0") {
		t.Errorf("fabricated 0–0 score for unscored match: %q", got)
	}
	if strings.Contains(got, "✅") {
		t.Errorf("unscored match should not use the scored-result glyph: %q", got)
	}
	if !strings.Contains(got, "MKOI vs KC") {
		t.Errorf("missing matchup: %q", got)
	}
	if !strings.Contains(got, "Bo3") || !strings.Contains(got, "Week 1") {
		t.Errorf("lost static metadata: %q", got)
	}
}

// A missing result object entirely (no `result` key) is the same class of
// unknown as a null outcome.
func TestFormatEventLine_CompletedNilResult_OmitsScore(t *testing.T) {
	e := mkEvent("completed", "lck", "LCK", "T1", "GEN", "2026-05-09T05:00:00Z")
	got := formatEventLine(e)
	if strings.Contains(got, "0–0") {
		t.Errorf("fabricated 0–0 score for nil-result match: %q", got)
	}
}

// One side declaring an outcome is enough to trust the score, even if the
// other side's result is absent.
func TestFormatEventLine_CompletedPartialResult_KeepsScore(t *testing.T) {
	e := ScheduleEvent{
		StartTime: "2026-05-09T05:00:00Z",
		State:     "completed",
		League:    League{Slug: "lck", Name: "LCK"},
		Match: Match{
			Teams: []Team{
				{Code: "T1", Result: &TeamResult{Outcome: "win", GameWins: 2}},
				{Code: "GEN"},
			},
			Strategy: Strategy{Count: 3},
		},
	}
	got := formatEventLine(e)
	if !strings.Contains(got, "2–0") {
		t.Errorf("score dropped despite a declared outcome: %q", got)
	}
	if !strings.Contains(got, "<b>T1</b>") {
		t.Errorf("winner not bolded: %q", got)
	}
}

func TestFormatEventLine_InProgress(t *testing.T) {
	w := &TeamResult{GameWins: 1}
	e := ScheduleEvent{
		StartTime: "2026-05-09T05:00:00Z",
		State:     "inProgress",
		League:    League{Slug: "lck"},
		Match: Match{
			Teams:    []Team{{Code: "T1", Result: w}, {Code: "GEN", Result: w}},
			Strategy: Strategy{Count: 5},
		},
	}
	got := formatEventLine(e)
	if !strings.Contains(got, "🔴 LIVE") {
		t.Errorf("missing LIVE marker: %q", got)
	}
	if !strings.Contains(got, "1–1") {
		t.Errorf("score missing: %q", got)
	}
}

func TestRenderToday_GroupsByLeagueInOrder(t *testing.T) {
	day := time.Date(2026, 5, 9, 0, 0, 0, 0, IctLocation)
	events := []ScheduleEvent{
		mkEvent("unstarted", "lcs", "LCS", "TL", "C9", "2026-05-09T18:00:00Z"),
		mkEvent("unstarted", "ewc_lol", "Esports World Cup", "GEN", "KC", "2026-05-09T09:00:00Z"),
		mkEvent("unstarted", "lck", "LCK", "T1", "GEN", "2026-05-09T05:00:00Z"),
		mkEvent("unstarted", "lpl", "LPL", "JDG", "BLG", "2026-05-09T08:00:00Z"),
	}
	got := RenderToday(events, day)
	// League order puts international events before regional leagues.
	idxEwc := strings.Index(got, "<b>Esports World Cup</b>")
	idxLck := strings.Index(got, "<b>LCK</b>")
	idxLpl := strings.Index(got, "<b>LPL</b>")
	idxLcs := strings.Index(got, "<b>LCS</b>")
	if idxEwc < 0 || idxLck < 0 || idxLpl < 0 || idxLcs < 0 {
		t.Fatalf("missing league section; got:\n%s", got)
	}
	if idxEwc >= idxLck || idxLck >= idxLpl || idxLpl >= idxLcs {
		t.Errorf("league order wrong: ewc=%d lck=%d lpl=%d lcs=%d\n%s", idxEwc, idxLck, idxLpl, idxLcs, got)
	}
	// Header in ICT.
	if !strings.Contains(got, "LoL — Sat May 9</b> (ICT)") {
		t.Errorf("header wrong: %q", got)
	}
}

func TestRenderToday_EmptyShowsNoMatches(t *testing.T) {
	day := time.Date(2026, 5, 9, 0, 0, 0, 0, IctLocation)
	got := RenderToday(nil, day)
	if !strings.Contains(got, "No major LoL matches today.") {
		t.Errorf("empty render missing major-match empty text: %q", got)
	}
}

func TestRenderDay_CustomEmptyLine(t *testing.T) {
	day := time.Date(2026, 5, 10, 0, 0, 0, 0, IctLocation)
	got := renderDay(nil, day, "No major LoL matches tomorrow.")
	if !strings.Contains(got, "Sun May 10") || !strings.Contains(got, "No major LoL matches tomorrow.") {
		t.Errorf("custom day empty render wrong: %q", got)
	}
}

func TestRenderWeek_GroupsByLeagueAndDay(t *testing.T) {
	from := time.Date(2026, 5, 9, 0, 0, 0, 0, IctLocation)
	to := from.AddDate(0, 0, 7)
	events := []ScheduleEvent{
		mkEvent("unstarted", "lck", "LCK", "T1", "GEN", "2026-05-09T05:00:00Z"),
		mkEvent("unstarted", "lck", "LCK", "DK", "KT", "2026-05-10T05:00:00Z"),
	}
	got := RenderWeek(events, from, to)
	if !strings.Contains(got, "<b>LCK</b>") {
		t.Errorf("missing LCK section: %q", got)
	}
	// Both days should appear under LCK.
	if !strings.Contains(got, "Sat May 9") {
		t.Errorf("missing Sat May 9: %q", got)
	}
	if !strings.Contains(got, "Sun May 10") {
		t.Errorf("missing Sun May 10: %q", got)
	}
}

func TestRenderWeek_CustomEmptyLine(t *testing.T) {
	from := time.Date(2026, 5, 11, 0, 0, 0, 0, IctLocation)
	to := from.AddDate(0, 0, 7)
	got := renderWeek(nil, from, to, "No major LoL matches next week.")
	for _, want := range []string{"Mon May 11", "Sun May 17", "No major LoL matches next week."} {
		if !strings.Contains(got, want) {
			t.Errorf("custom week empty render missing %q in %q", want, got)
		}
	}
}

func TestFilterMajor(t *testing.T) {
	events := []ScheduleEvent{
		{League: League{Slug: "lck"}},
		{League: League{Slug: "lpl"}},
		{League: League{Slug: "tcl"}}, // Turkish league — not in allowlist
		{League: League{Slug: "lja"}}, // Japan academy — not in allowlist
		{League: League{Slug: "msi"}},
		{League: League{Slug: "ewc_lol"}},
	}
	got := FilterMajor(events)
	if len(got) != 4 {
		t.Errorf("filtered count = %d, want 4", len(got))
	}
	for _, e := range got {
		if e.League.Slug == "tcl" || e.League.Slug == "lja" {
			t.Errorf("non-major league leaked: %s", e.League.Slug)
		}
	}
}

func TestFormatEventLine_EscapesUserStrings(t *testing.T) {
	e := ScheduleEvent{
		StartTime: "2026-05-09T05:00:00Z",
		State:     "unstarted",
		BlockName: "<script>",
		League:    League{Slug: "lck"},
		Match: Match{
			Teams: []Team{
				{Name: "Tom & Jerry"},
				{Name: `"Quotes"`},
			},
			Strategy: Strategy{Count: 1},
		},
	}
	got := formatEventLine(e)
	if strings.Contains(got, "<script>") {
		t.Errorf("raw <script> leaked: %q", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("BlockName not escaped: %q", got)
	}
	if strings.Contains(got, "Tom & Jerry") {
		// & should be escaped to &amp;
		t.Errorf("ampersand not escaped: %q", got)
	}
}
