package amlich

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "regenerate golden files")

// goldenTable renders every lunar year in the supported range as one line:
// "YYYY L n1 n2 ... nN" — L is the leap month number (0 = none) and n* are the
// month lengths in chronological order starting at tháng 1, with the leap
// month in sequence (12 entries for a normal year, 13 for a leap year).
func goldenTable(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for year := minLunarYear; year <= maxLunarYear; year++ {
		type monthStart struct {
			jd, month int
			leap      bool
		}
		var starts []monthStart
		for m := 1; m <= 12; m++ {
			dd, mm, yy, err := lunarToSolar(1, m, year, false)
			if err != nil {
				t.Fatalf("lunarToSolar(1, %d, %d): %v", m, year, err)
			}
			starts = append(starts, monthStart{jdFromDate(dd, mm, yy), m, false})
			if dd, mm, yy, err := lunarToSolar(1, m, year, true); err == nil {
				starts = append(starts, monthStart{jdFromDate(dd, mm, yy), m, true})
			}
		}
		sort.Slice(starts, func(i, j int) bool { return starts[i].jd < starts[j].jd })
		// The last month of year Y ends where tháng 1 of Y+1 begins. For
		// maxLunarYear this evaluates the engine one year past the supported
		// range, which the astronomy handles fine — only the bot commands
		// enforce the bounds.
		dd, mm, yy, err := lunarToSolar(1, 1, year+1, false)
		if err != nil {
			t.Fatalf("lunarToSolar(1, 1, %d): %v", year+1, err)
		}
		nextYearStart := jdFromDate(dd, mm, yy)
		leapMonth := 0
		for _, s := range starts {
			if s.leap {
				leapMonth = s.month
			}
		}
		if want := 12; (leapMonth == 0 && len(starts) != want) || (leapMonth != 0 && len(starts) != want+1) {
			t.Fatalf("year %d: %d month starts with leap month %d", year, len(starts), leapMonth)
		}
		fmt.Fprintf(&b, "%d %d", year, leapMonth)
		for i, s := range starts {
			end := nextYearStart
			if i+1 < len(starts) {
				end = starts[i+1].jd
			}
			if length := end - s.jd; length != 29 && length != 30 {
				t.Fatalf("year %d month %d (leap=%v): length %d days", year, s.month, s.leap, length)
			} else {
				fmt.Fprintf(&b, " %d", length)
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// TestGoldenTable compares the engine's full 1800–2199 month structure against
// the committed golden file. The round-trip test proves the two conversion
// directions agree with each other; this pins the verified boundary placement
// itself, so a future engine change cannot shift months while staying
// self-consistent. Regenerate deliberately with:
//
//	go test ./internal/modules/amlich/ -run TestGoldenTable -update
func TestGoldenTable(t *testing.T) {
	got := goldenTable(t)
	golden := filepath.Join("testdata", "lunar-years-1800-2199.txt")
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden file: %v", err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden file (regenerate with -update): %v", err)
	}
	if got == string(want) {
		return
	}
	gotLines := strings.Split(got, "\n")
	wantLines := strings.Split(string(want), "\n")
	for i := 0; i < len(gotLines) && i < len(wantLines); i++ {
		if gotLines[i] != wantLines[i] {
			t.Fatalf("golden mismatch at line %d:\n got: %s\nwant: %s", i+1, gotLines[i], wantLines[i])
		}
	}
	t.Fatalf("golden mismatch: got %d lines, want %d", len(gotLines), len(wantLines))
}
