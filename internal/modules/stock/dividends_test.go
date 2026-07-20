package stock

import (
	"math"
	"testing"
)

func TestParsePositiveWhole(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int64
		ok   bool
	}{
		{"1500", 1500, true},
		{"0010", 10, true},
		{"", 0, false},
		{"0", 0, false},
		{"+1", 0, false},
		{"-1", 0, false},
		{"1.5", 0, false},
		{"NaN", 0, false},
		{"9223372036854775808", 0, false},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			got, ok := parsePositiveWhole(tc.raw)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("parsePositiveWhole(%q) = (%d, %v), want (%d, %v)", tc.raw, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestParseShareRatio(t *testing.T) {
	for _, raw := range []string{"4:1", "100:10", "004:02"} {
		ratio, ok := parseShareRatio(raw)
		if !ok || ratio.raw != raw {
			t.Fatalf("parseShareRatio(%q) = %+v, %v", raw, ratio, ok)
		}
	}
	for _, raw := range []string{"", "4", "4:1:1", "0:1", "1:0", "+4:1", "4:-1", "4.0:1", "a:b", "9223372036854775808:1"} {
		if _, ok := parseShareRatio(raw); ok {
			t.Fatalf("parseShareRatio(%q) unexpectedly succeeded", raw)
		}
	}
}

func TestShareDividendEntitlement(t *testing.T) {
	for _, tc := range []struct {
		held  int64
		ratio shareRatio
		want  int64
	}{
		{139, shareRatio{owned: 100, new: 10}, 13},
		{2026, shareRatio{owned: 4, new: 1}, 506},
		{8, shareRatio{owned: 4, new: 1}, 2},
		{8, shareRatio{owned: 100, new: 25}, 2},
		{math.MaxInt64, shareRatio{owned: math.MaxInt64, new: math.MaxInt64}, math.MaxInt64},
	} {
		got, err := shareDividendEntitlement(tc.held, tc.ratio)
		if err != nil || got != tc.want {
			t.Fatalf("shareDividendEntitlement(%d, %+v) = %d, %v; want %d", tc.held, tc.ratio, got, err, tc.want)
		}
	}
	if _, err := shareDividendEntitlement(math.MaxInt64, shareRatio{owned: 1, new: 2}); err == nil {
		t.Fatal("overflowing entitlement unexpectedly succeeded")
	}
}

func TestMinimumHoldingForShare(t *testing.T) {
	for _, tc := range []struct {
		ratio shareRatio
		want  int64
	}{
		{shareRatio{owned: 100, new: 10}, 10},
		{shareRatio{owned: 4, new: 1}, 4},
		{shareRatio{owned: 3, new: 2}, 2},
		{shareRatio{owned: 1, new: math.MaxInt64}, 1},
	} {
		if got := minimumHoldingForShare(tc.ratio); got != tc.want {
			t.Fatalf("minimumHoldingForShare(%+v) = %d, want %d", tc.ratio, got, tc.want)
		}
	}
}

func TestCheckedDividendTotals(t *testing.T) {
	if got, err := cashDividendTotal(139, 1500); err != nil || got != 208500 {
		t.Fatalf("cashDividendTotal = %d, %v", got, err)
	}
	if _, err := cashDividendTotal(math.MaxInt64, 2); err == nil {
		t.Fatal("overflowing cash total unexpectedly succeeded")
	}
	if _, err := checkedHoldingAfterDividend(math.MaxInt64, 1); err == nil {
		t.Fatal("overflowing holding unexpectedly succeeded")
	}
}

func TestCheckedVNDBalanceRequiresExactSum(t *testing.T) {
	if got, err := checkedVNDBalance(1, maxExactFloatInteger-1); err != nil || got != float64(maxExactFloatInteger) {
		t.Fatalf("exact boundary sum = %v, %v", got, err)
	}
	if _, err := checkedVNDBalance(1, maxExactFloatInteger); err == nil {
		t.Fatal("inexact boundary sum unexpectedly succeeded")
	}
}
