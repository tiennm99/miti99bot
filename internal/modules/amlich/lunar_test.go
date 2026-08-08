package amlich

import "testing"

// knownDates anchors the algorithm to independently published Vietnamese
// calendar facts: Tết dates, leap-month starts, and two national holidays
// whose lunar dates are widely documented.
var knownDates = []struct {
	name                            string
	solarDay, solarMonth, solarYear int
	lunarDay, lunarMonth, lunarYear int
	leap                            bool
}{
	{"Tết Ất Tỵ", 29, 1, 2025, 1, 1, 2025, false},
	{"Tết Giáp Thìn", 10, 2, 2024, 1, 1, 2024, false},
	{"Tết Quý Mão", 22, 1, 2023, 1, 1, 2023, false},
	{"Tết Bính Ngọ", 17, 2, 2026, 1, 1, 2026, false},
	{"start of leap month 2, Quý Mão", 22, 3, 2023, 1, 2, 2023, true},
	{"start of leap month 4, Canh Tý", 23, 5, 2020, 1, 4, 2020, true},
	{"Quốc khánh 1945 — 26/7 Ất Dậu", 2, 9, 1945, 26, 7, 1945, false},
	{"30/4/1975 — 20/3 Ất Mão", 30, 4, 1975, 20, 3, 1975, false},
}

func TestSolarToLunar_KnownDates(t *testing.T) {
	for _, tc := range knownDates {
		t.Run(tc.name, func(t *testing.T) {
			d, m, y, leap := solarToLunar(tc.solarDay, tc.solarMonth, tc.solarYear)
			if d != tc.lunarDay || m != tc.lunarMonth || y != tc.lunarYear || leap != tc.leap {
				t.Errorf("solarToLunar(%d/%d/%d) = %d/%d/%d leap=%v, want %d/%d/%d leap=%v",
					tc.solarDay, tc.solarMonth, tc.solarYear,
					d, m, y, leap, tc.lunarDay, tc.lunarMonth, tc.lunarYear, tc.leap)
			}
		})
	}
}

func TestLunarToSolar_KnownDates(t *testing.T) {
	for _, tc := range knownDates {
		t.Run(tc.name, func(t *testing.T) {
			d, m, y, err := lunarToSolar(tc.lunarDay, tc.lunarMonth, tc.lunarYear, tc.leap)
			if err != nil {
				t.Fatalf("lunarToSolar(%d/%d/%d leap=%v): %v",
					tc.lunarDay, tc.lunarMonth, tc.lunarYear, tc.leap, err)
			}
			if d != tc.solarDay || m != tc.solarMonth || y != tc.solarYear {
				t.Errorf("lunarToSolar(%d/%d/%d leap=%v) = %d/%d/%d, want %d/%d/%d",
					tc.lunarDay, tc.lunarMonth, tc.lunarYear, tc.leap,
					d, m, y, tc.solarDay, tc.solarMonth, tc.solarYear)
			}
		})
	}
}

// TestSolarLunarRoundTrip converts every day of 1950–2050 solar→lunar→solar.
// Identity across a century (including ~37 leap months) rules out whole
// classes of boundary bugs without needing external truth per day.
func TestSolarLunarRoundTrip(t *testing.T) {
	start := jdFromDate(1, 1, 1950)
	end := jdFromDate(31, 12, 2050)
	for jd := start; jd <= end; jd++ {
		solarDay, solarMonth, solarYear := jdToDate(jd)
		lunarDay, lunarMonth, lunarYear, leap := solarToLunar(solarDay, solarMonth, solarYear)
		if lunarDay < 1 || lunarDay > 30 || lunarMonth < 1 || lunarMonth > 12 {
			t.Fatalf("solarToLunar(%d/%d/%d) out of range: %d/%d/%d",
				solarDay, solarMonth, solarYear, lunarDay, lunarMonth, lunarYear)
		}
		gotDay, gotMonth, gotYear, err := lunarToSolar(lunarDay, lunarMonth, lunarYear, leap)
		if err != nil {
			t.Fatalf("round trip %d/%d/%d → %d/%d/%d leap=%v: %v",
				solarDay, solarMonth, solarYear, lunarDay, lunarMonth, lunarYear, leap, err)
		}
		if gotDay != solarDay || gotMonth != solarMonth || gotYear != solarYear {
			t.Fatalf("round trip %d/%d/%d → %d/%d/%d leap=%v → %d/%d/%d",
				solarDay, solarMonth, solarYear, lunarDay, lunarMonth, lunarYear, leap,
				gotDay, gotMonth, gotYear)
		}
	}
}

func TestLunarToSolar_LeapMonth2025RoundTrip(t *testing.T) {
	// Ất Tỵ 2025 has a leap month 6.
	solarDay, solarMonth, solarYear, err := lunarToSolar(1, 6, 2025, true)
	if err != nil {
		t.Fatalf("lunarToSolar(1/6 nhuận/2025): %v", err)
	}
	d, m, y, leap := solarToLunar(solarDay, solarMonth, solarYear)
	if d != 1 || m != 6 || y != 2025 || !leap {
		t.Errorf("round trip of 1/6 nhuận/2025 via %d/%d/%d = %d/%d/%d leap=%v",
			solarDay, solarMonth, solarYear, d, m, y, leap)
	}
}

func TestLunarToSolar_RejectsImpossibleLeapMonth(t *testing.T) {
	// 2023's leap month is 2 — a leap month 3 does not exist.
	if _, _, _, err := lunarToSolar(1, 3, 2023, true); err == nil {
		t.Error("lunarToSolar(1/3 nhuận/2023) accepted; 2023's leap month is 2")
	}
	// 2024 has no leap month at all.
	if _, _, _, err := lunarToSolar(1, 2, 2024, true); err == nil {
		t.Error("lunarToSolar(1/2 nhuận/2024) accepted; 2024 has no leap month")
	}
}

func TestLunarToSolar_RejectsDay30OfShortMonth(t *testing.T) {
	// Locate a 29-day month using the module itself: lunar 2024 has no leap
	// month, so months 1..12 are consecutive and day-1 starts are adjacent.
	// The astronomy is anchored by the known-date tests above.
	found := false
	for m := 1; m <= 11; m++ {
		d1, m1, y1, err := lunarToSolar(1, m, 2024, false)
		if err != nil {
			t.Fatalf("lunarToSolar(1/%d/2024): %v", m, err)
		}
		d2, m2, y2, err := lunarToSolar(1, m+1, 2024, false)
		if err != nil {
			t.Fatalf("lunarToSolar(1/%d/2024): %v", m+1, err)
		}
		if jdFromDate(d2, m2, y2)-jdFromDate(d1, m1, y1) != 29 {
			continue
		}
		found = true
		if _, _, _, err := lunarToSolar(30, m, 2024, false); err == nil {
			t.Errorf("lunarToSolar(30/%d/2024) accepted; month has only 29 days", m)
		}
		if _, _, _, err := lunarToSolar(29, m, 2024, false); err != nil {
			t.Errorf("lunarToSolar(29/%d/2024) rejected: %v", m, err)
		}
		break
	}
	if !found {
		t.Fatal("no 29-day month found in lunar year 2024 — month-length computation is broken")
	}
}

func TestCanChiYear(t *testing.T) {
	for year, want := range map[int]string{
		2024: "Giáp Thìn",
		2025: "Ất Tỵ",
		2026: "Bính Ngọ",
		1945: "Ất Dậu",
		1975: "Ất Mão",
	} {
		if got := canChiYear(year); got != want {
			t.Errorf("canChiYear(%d) = %q, want %q", year, got, want)
		}
	}
}
