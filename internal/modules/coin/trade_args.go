package coin

import "errors"

var errInvalidUSDAmount = errors.New("coin: invalid USD amount")

type coinValueArgs struct {
	coin  CoinSymbol
	value float64
}

func parseCoinValueArgs(args []string, validValue func(float64) bool, invalidValueErr error) (coinValueArgs, error) {
	if len(args) != 2 {
		return coinValueArgs{}, invalidValueErr
	}
	if coin, err := ResolveCoinSymbol(args[0]); err == nil {
		value, ok := parsePositiveFinite(args[1])
		if !ok || !validValue(value) {
			return coinValueArgs{}, invalidValueErr
		}
		return coinValueArgs{coin: coin, value: value}, nil
	}
	value, ok := parsePositiveFinite(args[0])
	if !ok || !validValue(value) {
		if _, secondOK := parsePositiveFinite(args[1]); secondOK {
			return coinValueArgs{}, ErrUnsupportedCoin
		}
		return coinValueArgs{}, invalidValueErr
	}
	coin, err := ResolveCoinSymbol(args[1])
	if err != nil {
		return coinValueArgs{}, err
	}
	return coinValueArgs{coin: coin, value: value}, nil
}
