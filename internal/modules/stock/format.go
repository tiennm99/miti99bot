// Package stock is a paper-stock module for VN stocks. It keeps a per-user
// live portfolio in KV and prices positions at the current market price.
package stock

import (
	"math"
	"strconv"
	"strings"
)

// FormatVND renders an integer-rounded amount with dot-thousands separators
// and a "VND" suffix (e.g. 15000000 -> "15.000.000 VND"). Manual to avoid
// locale-dependent formatting.
func FormatVND(n float64) string {
	return formatVNDNumber(n) + " VND"
}

// formatVNDNumber renders a VND amount without its currency suffix. Portfolio
// tables use this because their title declares the currency once.
func formatVNDNumber(n float64) string {
	return formatGroupedInteger(int64(math.Round(n)))
}

// formatCompactVND renders a position-table amount using the smallest suitable
// financial suffix. Values are first rounded to the nearest VND, matching the
// module's full VND formatter.
func formatCompactVND(n float64) string {
	rounded := math.Round(n)
	abs := math.Abs(rounded)
	if abs < 1_000 {
		return formatVNDNumber(rounded)
	}

	suffixes := [...]string{"k", "M", "B", "T"}
	divisor := 1_000.0
	suffixIndex := 0
	for suffixIndex < len(suffixes)-1 && abs >= divisor*1_000 {
		divisor *= 1_000
		suffixIndex++
	}

	scaled := math.Round(abs/divisor*1_000) / 1_000
	if scaled >= 1_000 && suffixIndex < len(suffixes)-1 {
		divisor *= 1_000
		suffixIndex++
		scaled = math.Round(abs/divisor*1_000) / 1_000
	}

	result := strings.TrimRight(strings.TrimRight(strconv.FormatFloat(scaled, 'f', 3, 64), "0"), ".")
	result = strings.Replace(result, ".", ",", 1)
	if rounded < 0 {
		result = "-" + result
	}
	return result + suffixes[suffixIndex]
}

func formatGroupedInteger(n int64) string {
	abs := strconv.FormatInt(absInt64(n), 10)
	var sb strings.Builder
	if n < 0 {
		sb.WriteByte('-')
	}
	for i := 0; i < len(abs); i++ {
		if i > 0 && (len(abs)-i)%3 == 0 {
			sb.WriteByte('.')
		}
		sb.WriteByte(abs[i])
	}
	return sb.String()
}

// FormatStock renders an integer share quantity (always whole shares).
func FormatStock(n float64) string {
	return strconv.FormatInt(int64(math.Floor(n)), 10)
}

// formatShareQuantity renders an exact whole-share quantity with Vietnamese
// dot-thousands separators, without converting the int64 value through float64.
func formatShareQuantity(n int64) string {
	raw := strconv.FormatInt(n, 10)
	digitStart := 0
	if raw[0] == '-' {
		digitStart = 1
	}

	var sb strings.Builder
	sb.Grow(len(raw) + (len(raw)-digitStart-1)/3)
	if digitStart == 1 {
		sb.WriteByte('-')
	}
	for i := digitStart; i < len(raw); i++ {
		if i > digitStart && (len(raw)-i)%3 == 0 {
			sb.WriteByte('.')
		}
		sb.WriteByte(raw[i])
	}
	return sb.String()
}

// FormatPnL renders a signed VND delta + percentage line, e.g.
// "+1.234 VND (+12.34%)" or "-500.000 VND (-5.00%)". When invested is zero
// the percentage is reported as 0.00 to avoid division-by-zero.
func FormatPnL(currentValue, invested float64) string {
	return formatPnL(currentValue, invested, FormatVND)
}

func formatPortfolioPnL(currentValue, invested float64) string {
	return formatPnL(currentValue, invested, formatVNDNumber)
}

func formatPortfolioPositionPnL(currentValue, invested float64) string {
	return formatPnL(currentValue, invested, formatCompactVND)
}

func formatPnL(currentValue, invested float64, formatAmount func(float64) string) string {
	diff := currentValue - invested
	pct := 0.0
	if invested > 0 {
		pct = (diff / invested) * 100
	}
	sign := ""
	if diff >= 0 {
		sign = "+"
	}
	return sign + formatAmount(diff) + " (" + sign + strconv.FormatFloat(pct, 'f', 2, 64) + "%)"
}

func absInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
