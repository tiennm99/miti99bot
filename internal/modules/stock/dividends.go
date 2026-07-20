package stock

import (
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
)

var errDividendOverflow = errors.New("dividend calculation overflows")

const maxExactFloatInteger = int64(1 << 53)

type shareRatio struct {
	owned int64
	new   int64
	raw   string
}

func parsePositiveWhole(raw string) (int64, bool) {
	if raw == "" {
		return 0, false
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func parseShareRatio(raw string) (shareRatio, bool) {
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return shareRatio{}, false
	}
	owned, ok := parsePositiveWhole(parts[0])
	if !ok {
		return shareRatio{}, false
	}
	newShares, ok := parsePositiveWhole(parts[1])
	if !ok {
		return shareRatio{}, false
	}
	return shareRatio{owned: owned, new: newShares, raw: raw}, true
}

func shareDividendEntitlement(held int64, ratio shareRatio) (int64, error) {
	if held <= 0 || ratio.owned <= 0 || ratio.new <= 0 {
		return 0, errors.New("invalid dividend inputs")
	}
	product := new(big.Int).Mul(big.NewInt(held), big.NewInt(ratio.new))
	result := product.Quo(product, big.NewInt(ratio.owned))
	if !result.IsInt64() {
		return 0, errDividendOverflow
	}
	return result.Int64(), nil
}

func minimumHoldingForShare(ratio shareRatio) int64 {
	return (ratio.owned-1)/ratio.new + 1
}

func checkedHoldingAfterDividend(held, newShares int64) (int64, error) {
	if newShares < 0 || held > math.MaxInt64-newShares {
		return 0, errDividendOverflow
	}
	return held + newShares, nil
}

func cashDividendTotal(held, vndPerShare int64) (int64, error) {
	if held <= 0 || vndPerShare <= 0 || held > math.MaxInt64/vndPerShare {
		return 0, errDividendOverflow
	}
	total := held * vndPerShare
	if total > maxExactFloatInteger {
		return 0, errDividendOverflow
	}
	return total, nil
}

func checkedVNDBalance(balance float64, credit int64) (float64, error) {
	if credit <= 0 || math.IsNaN(balance) || math.IsInf(balance, 0) {
		return 0, errDividendOverflow
	}
	result := balance + float64(credit)
	if math.IsNaN(result) || math.IsInf(result, 0) || result > float64(maxExactFloatInteger) {
		return 0, errDividendOverflow
	}

	exactResult := new(big.Rat).SetFloat64(balance)
	exactResult.Add(exactResult, new(big.Rat).SetInt64(credit))
	representedResult := new(big.Rat).SetFloat64(result)
	if exactResult.Cmp(representedResult) != 0 {
		return 0, errDividendOverflow
	}
	return result, nil
}
