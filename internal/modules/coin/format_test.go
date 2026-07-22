package coin

import (
	"math"
	"testing"
)

func TestFormatCompactUSD(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "$0.00"},
		{999.99, "$999.99"},
		{999.994, "$999.99"},
		{999.999, "$1k"},
		{1_000, "$1k"},
		{25_350, "$25.35k"},
		{25_351, "$25.351k"},
		{999_499, "$999.499k"},
		{999_999.4, "$999.999k"},
		{999_999.5, "$1M"},
		{1_000_000, "$1M"},
		{1_234_000, "$1.234M"},
		{126_000_000, "$126M"},
		{999_999_999.5, "$1B"},
		{1_250_000_000, "$1.25B"},
		{999_999_999_999.5, "$1T"},
		{1_000_000_000_000, "$1T"},
		{-25_350, "-$25.35k"},
		{-999.999, "-$1k"},
		{-1_250_000_000, "-$1.25B"},
		{math.NaN(), "invalid USD"},
		{math.Inf(1), "invalid USD"},
	}
	for _, test := range tests {
		if got := formatCompactUSD(test.in); got != test.want {
			t.Errorf("formatCompactUSD(%v): got %q, want %q", test.in, got, test.want)
		}
	}
}

func TestFormatPortfolioPositionPnLUSDUsesCompactAmountAndFullPercentage(t *testing.T) {
	tests := []struct {
		current  float64
		invested float64
		want     string
	}{
		{1_250_000, 1_000_000, "+$250k (+25.00%)"},
		{750_000, 1_000_000, "-$250k (-25.00%)"},
	}
	for _, test := range tests {
		if got := formatPortfolioPositionPnLUSD(test.current, test.invested); got != test.want {
			t.Errorf("formatPortfolioPositionPnLUSD(%v, %v): got %q, want %q", test.current, test.invested, got, test.want)
		}
	}
}

func TestExportedUSDFormattersRemainFullPrecision(t *testing.T) {
	if got, want := FormatUSD(1_234_567.89), "$1,234,567.89"; got != want {
		t.Fatalf("FormatUSD: got %q, want %q", got, want)
	}
	if got, want := FormatPnLUSD(1_250_000, 1_000_000), "+$250,000.00 (+25.00%)"; got != want {
		t.Fatalf("FormatPnLUSD: got %q, want %q", got, want)
	}
}
