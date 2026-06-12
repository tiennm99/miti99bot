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
	diff := currentValue - invested
	pct := 0.0
	if invested > 0 {
		pct = (diff / invested) * 100
	}
	sign := ""
	if diff >= 0 {
		sign = "+"
	}
	return sign + FormatUSD(diff) + " (" + sign + strconv.FormatFloat(pct, 'f', 2, 64) + "%)"
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
