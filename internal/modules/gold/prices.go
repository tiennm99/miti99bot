package gold

import "errors"

// ErrNoGoldPrice reports that an upstream returned no usable price; handlers
// show it as "No gold price available." rather than a generic fetch failure.
var ErrNoGoldPrice = errors.New("gold: no price available")

// GoldPrice is one SJC gold quote from VNAppMob. VNDPerLuong is the buy/sell
// mid price.
type GoldPrice struct {
	VNDPerLuong float64
	Source      string    // always "vnappmob-sjc"
	SJC         *SJCPrice // buy/sell quotes behind VNDPerLuong
}

// SJCPrice holds VNAppMob SJC buy/sell quotes per lượng (VND).
type SJCPrice struct {
	Buy  float64
	Sell float64
}
