package stock

import "testing"

func TestFormatVND(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0 VND"},
		{1, "1 VND"},
		{100, "100 VND"},
		{1000, "1.000 VND"},
		{15_000_000, "15.000.000 VND"},
		{1_234_567_890, "1.234.567.890 VND"},
		{-500, "-500 VND"},
		{-1_500_000, "-1.500.000 VND"},
		// rounding: half away from zero (math.Round)
		{1.5, "2 VND"},
		{-1.5, "-2 VND"},
	}
	for _, c := range cases {
		if got := FormatVND(c.in); got != c.want {
			t.Errorf("FormatVND(%v): got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatStock(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{100, "100"},
		{99.9, "99"}, // floor for safety; we never store fractional shares anyway
	}
	for _, c := range cases {
		if got := FormatStock(c.in); got != c.want {
			t.Errorf("FormatStock(%v): got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatCompactVND(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{999.4, "999"},
		{999.5, "1k"},
		{1_000, "1k"},
		{25_000, "25k"},
		{25_350, "25,35k"},
		{25_351, "25,351k"},
		{999_499, "999,499k"},
		{999_999.4, "999,999k"},
		{999_999.5, "1M"},
		{1_000_000, "1M"},
		{1_234_000, "1,234M"},
		{126_000_000, "126M"},
		{999_999_999.5, "1B"},
		{1_250_000_000, "1,25B"},
		{999_999_999_999.5, "1T"},
		{1_000_000_000_000, "1T"},
		{-25_350, "-25,35k"},
		{-1_250_000_000, "-1,25B"},
		{25_350.5, "25,351k"},
	}
	for _, c := range cases {
		if got := formatCompactVND(c.in); got != c.want {
			t.Errorf("formatCompactVND(%v): got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatPortfolioPositionPnLUsesCompactAmountAndFullPercentage(t *testing.T) {
	if got, want := formatPortfolioPositionPnL(1_250_000, 1_000_000), "+250k (+25.00%)"; got != want {
		t.Fatalf("formatPortfolioPositionPnL: got %q, want %q", got, want)
	}
}

func TestFormatShareQuantity(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.000"},
		{9_007_199_254_740_993, "9.007.199.254.740.993"},
	}
	for _, c := range cases {
		if got := formatShareQuantity(c.in); got != c.want {
			t.Errorf("formatShareQuantity(%d): got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatPnL(t *testing.T) {
	cases := []struct {
		current, invested float64
		want              string
	}{
		{1_500_000, 1_000_000, "+500.000 VND (+50.00%)"},
		{800_000, 1_000_000, "-200.000 VND (-20.00%)"},
		{1_000_000, 1_000_000, "+0 VND (+0.00%)"},
		{500_000, 0, "+500.000 VND (+0.00%)"}, // invested=0 → pct=0, no NaN
	}
	for _, c := range cases {
		if got := FormatPnL(c.current, c.invested); got != c.want {
			t.Errorf("FormatPnL(%v,%v): got %q, want %q", c.current, c.invested, got, c.want)
		}
	}
}
