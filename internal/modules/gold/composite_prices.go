package gold

import (
	"context"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/storage"
)

// sjcPriceFetcher is the subset of VNAppMobClient needed by the composite
// fetcher. It is implemented by *VNAppMobClient and by test stubs.
type sjcPriceFetcher interface {
	FetchSJCPrice(ctx context.Context) (buy, sell float64, err error)
}

// compositePriceFetcher prefers VNAppMob SJC prices and falls back to the
// existing XAU/USD-derived chain when SJC is unavailable.
type compositePriceFetcher struct {
	vnappmob sjcPriceFetcher
	fallback priceFetcher
}

// NewCompositePriceFetcherFromEnv builds the production price fetcher using
// env-driven VNAppMob and fallback clients.
func NewCompositePriceFetcherFromEnv(kv storage.KVStore) priceFetcher {
	return &compositePriceFetcher{
		vnappmob: NewVNAppMobClientFromEnv(kv),
		fallback: NewGoldPriceClientFromEnv(),
	}
}

// FetchLuongPrice returns a representative VND/lượng price. It uses the SJC
// mid price when available, otherwise the XAU/USD-derived spot price.
func (f *compositePriceFetcher) FetchLuongPrice(ctx context.Context) (float64, error) {
	buy, sell, err := f.FetchLuongPrices(ctx)
	if err != nil {
		return 0, err
	}
	return (buy + sell) / 2, nil
}

// FetchLuongPrices returns SJC buy/sell VND/lượng when VNAppMob is healthy,
// otherwise the XAU/USD-derived spot price for both sides.
func (f *compositePriceFetcher) FetchLuongPrices(ctx context.Context) (float64, float64, error) {
	buy, sell, err := f.vnappmob.FetchSJCPrice(ctx)
	if err == nil {
		return buy, sell, nil
	}
	log.Warn("vnappmob_sjc_failed", "err", err)
	p, err := f.fallback.FetchLuongPrice(ctx)
	return p, p, err
}

// FetchPrice returns a GoldPrice. When VNAppMob succeeds the struct carries
// SJC buy/sell data and Source "vnappmob-sjc"; otherwise it falls back to the
// XAU/USD chain with Source "xau-fallback".
func (f *compositePriceFetcher) FetchPrice(ctx context.Context) (GoldPrice, error) {
	buy, sell, err := f.vnappmob.FetchSJCPrice(ctx)
	if err == nil {
		mid := (buy + sell) / 2
		return GoldPrice{
			VNDPerLuong: mid,
			Source:      "vnappmob-sjc",
			SJC:         &SJCPrice{Buy: buy, Sell: sell},
		}, nil
	}
	log.Warn("vnappmob_sjc_failed", "err", err)
	p, err := f.fallback.FetchPrice(ctx)
	if err != nil {
		return GoldPrice{}, err
	}
	p.Source = "xau-fallback"
	return p, nil
}
