package gold

import (
	"math"
	"strconv"
	"strings"
)

func FormatVND(n float64) string {
	if math.IsNaN(n) || math.IsInf(n, 0) || n > float64(math.MaxInt64) || n < float64(math.MinInt64) {
		return "invalid VND"
	}
	rounded := int64(math.Round(n))
	abs := strconv.FormatInt(absInt64(rounded), 10)
	var sb strings.Builder
	if rounded < 0 {
		sb.WriteByte('-')
	}
	for i := 0; i < len(abs); i++ {
		if i > 0 && (len(abs)-i)%3 == 0 {
			sb.WriteByte('.')
		}
		sb.WriteByte(abs[i])
	}
	sb.WriteString(" VND")
	return sb.String()
}

func FormatLuong(n float64) string {
	s := strconv.FormatFloat(n, 'f', 4, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-0" {
		return "0"
	}
	return s
}

func FormatPnL(currentValue, invested float64) string {
	diff := currentValue - invested
	pct := 0.0
	if invested > 0 {
		pct = (diff / invested) * 100
	}
	sign := ""
	if diff >= 0 {
		sign = "+"
	}
	return sign + FormatVND(diff) + " (" + sign + strconv.FormatFloat(pct, 'f', 2, 64) + "%)"
}

func absInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
