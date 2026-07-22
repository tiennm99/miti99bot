package coin

import (
	"math"
	"strconv"
	"strings"
)

func FormatUSD(n float64) string {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return "invalid USD"
	}
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	whole := int64(math.Floor(n))
	cents := int64(math.Round((n - float64(whole)) * 100))
	if cents == 100 {
		whole++
		cents = 0
	}
	return sign + "$" + groupDigits(strconv.FormatInt(whole, 10)) + "." + twoDigits(cents)
}

// formatCompactUSD renders a position-table amount with at most three scaled
// fractional digits. The coin portfolio makes USD implicit and omits "$".
func formatCompactUSD(n float64) string {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return "invalid USD"
	}
	// Use the full formatter's cent rounding at the base/k boundary so an
	// amount displayed as 1,000.00 is promoted to 1k instead.
	if math.Round(math.Abs(n)*100)/100 < 1_000 {
		return strings.Replace(FormatUSD(n), "$", "", 1)
	}

	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	suffixes := [...]string{"k", "M", "B", "T"}
	divisor := 1_000.0
	suffixIndex := 0
	for suffixIndex < len(suffixes)-1 && n >= divisor*1_000 {
		divisor *= 1_000
		suffixIndex++
	}

	scaled := math.Round(n/divisor*1_000) / 1_000
	if scaled >= 1_000 && suffixIndex < len(suffixes)-1 {
		divisor *= 1_000
		suffixIndex++
		scaled = math.Round(n/divisor*1_000) / 1_000
	}
	amount := strings.TrimRight(strings.TrimRight(strconv.FormatFloat(scaled, 'f', 3, 64), "0"), ".")
	return sign + amount + suffixes[suffixIndex]
}

func FormatCoinQty(n float64) string {
	s := strconv.FormatFloat(n, 'f', 8, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-0" {
		return "0"
	}
	return s
}

func FormatPnLUSD(currentValue, invested float64) string {
	return formatPnLUSD(currentValue, invested, FormatUSD)
}

func formatPortfolioPositionPnLUSD(currentValue, invested float64) (string, string) {
	return formatPnLUSDParts(currentValue, invested, formatCompactUSD)
}

func formatPnLUSD(currentValue, invested float64, formatAmount func(float64) string) string {
	amount, percentage := formatPnLUSDParts(currentValue, invested, formatAmount)
	return amount + " (" + percentage + ")"
}

func formatPnLUSDParts(currentValue, invested float64, formatAmount func(float64) string) (string, string) {
	diff := currentValue - invested
	pct := 0.0
	if invested > 0 {
		pct = (diff / invested) * 100
	}
	sign := ""
	if diff >= 0 {
		sign = "+"
	}
	return sign + formatAmount(diff), sign + strconv.FormatFloat(pct, 'f', 2, 64) + "%"
}

func groupDigits(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			sb.WriteByte(',')
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

func twoDigits(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}
