package coin

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
)

const (
	CollectionName            = "coin"
	costBasisMigrationKey     = "migration:coin-cost-basis-v1"
	costBasisMigrationRetries = 5
)

type MigrationPriceFetcher interface {
	FetchUSD(context.Context, CoinSymbol) (CoinPrice, error)
}

// InitStore seeds legacy coin holdings at current market prices and verifies
// the invariant again on every startup, even after the audit marker exists.
func InitStore(ctx context.Context, portfolioColl, systemColl storage.Collection, prices MigrationPriceFetcher) error {
	docs := storage.Typed[Portfolio](portfolioColl)
	system := systemstate.New(systemColl)
	marker, markerExists, err := system.Get(ctx, costBasisMigrationKey)
	if err != nil {
		return fmt.Errorf("coin cost basis migration: read marker: %w", err)
	}
	keys, err := docs.List(ctx, "user:")
	if err != nil {
		return fmt.Errorf("coin cost basis migration: list portfolios: %w", err)
	}
	missing := map[string]bool{}
	for _, key := range keys {
		p, _, err := docs.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("coin cost basis migration: read %s: %w", key, err)
		}
		if err := inspectLegacyPortfolio(p, missing); err != nil {
			return fmt.Errorf("coin cost basis migration: %s: %w", key, err)
		}
	}

	quotes := map[string]float64{}
	for _, symbol := range sortedMissingSymbols(missing) {
		coin, err := ResolveCoinSymbol(symbol)
		if err != nil {
			return fmt.Errorf("coin cost basis migration: resolve %s: %w", symbol, err)
		}
		price, err := prices.FetchUSD(ctx, coin)
		if err != nil {
			return fmt.Errorf("coin cost basis migration: fetch %s quote: %w", symbol, err)
		}
		if !isPositiveFinite(price.USD) {
			return fmt.Errorf("coin cost basis migration: no valid quote for %s", symbol)
		}
		quotes[symbol] = price.USD
	}

	var migrated int64
	for index, key := range keys {
		count, err := migrateLegacyPortfolio(ctx, docs, key, quotes)
		if err != nil {
			return err
		}
		migrated += int64(count)
		if count > 0 {
			log.Info("coin cost basis migrated", "portfolio", index+1, "total", len(keys), "positions", count)
		}
	}
	now := time.Now().UnixMilli()
	if markerExists && marker.Status == "completed" && migrated == 0 {
		return nil
	}
	if !markerExists {
		marker = systemstate.Record{Kind: "migration", Name: "coin cost basis v1", CompletedAt: now}
	}
	marker.Status = "completed"
	marker.Count += migrated
	marker.UpdatedAt = now
	if err := system.Put(ctx, costBasisMigrationKey, marker); err != nil {
		return fmt.Errorf("coin cost basis migration: write marker: %w", err)
	}
	return nil
}

func inspectLegacyPortfolio(p Portfolio, missing map[string]bool) error {
	for symbol, basis := range p.CostBasis {
		if !isPositiveFinite(basis) {
			return fmt.Errorf("%s has invalid cost basis", symbol)
		}
		if p.Assets[symbol] <= 0 {
			return fmt.Errorf("%s has cost basis without a holding", symbol)
		}
	}
	for symbol, qty := range p.Assets {
		if !isPositiveFinite(qty) {
			if qty == 0 {
				continue
			}
			return fmt.Errorf("%s has invalid quantity", symbol)
		}
		coin, err := ResolveCoinSymbol(symbol)
		if err != nil || coin.Symbol != symbol {
			return fmt.Errorf("%q is not a canonical coin symbol", symbol)
		}
		if _, ok := p.CostBasis[symbol]; !ok {
			missing[symbol] = true
		}
	}
	return nil
}

func migrateLegacyPortfolio(ctx context.Context, docs storage.DocStore[Portfolio], key string, quotes map[string]float64) (int, error) {
	for attempt := 0; attempt < costBasisMigrationRetries; attempt++ {
		p, version, err := docs.Get(ctx, key)
		if err != nil {
			return 0, fmt.Errorf("coin cost basis migration: read %s: %w", key, err)
		}
		missing := map[string]bool{}
		if err := inspectLegacyPortfolio(p, missing); err != nil {
			return 0, fmt.Errorf("coin cost basis migration: %s: %w", key, err)
		}
		if len(missing) == 0 {
			return 0, nil
		}
		if p.CostBasis == nil {
			p.CostBasis = map[string]float64{}
		}
		for symbol := range missing {
			quote := quotes[symbol]
			basis := p.Assets[symbol] * quote
			if !isPositiveFinite(quote) || !isPositiveFinite(basis) {
				return 0, fmt.Errorf("coin cost basis migration: no cached valid quote for %s", symbol)
			}
			p.CostBasis[symbol] = basis
		}
		if err := docs.PutVersioned(ctx, key, version, p); err == nil {
			return len(missing), nil
		} else if !errors.Is(err, storage.ErrConflict) {
			return 0, fmt.Errorf("coin cost basis migration: write %s: %w", key, err)
		}
	}
	return 0, fmt.Errorf("coin cost basis migration: write %s: %w", key, storage.ErrConflict)
}

func sortedMissingSymbols(missing map[string]bool) []string {
	symbols := make([]string, 0, len(missing))
	for symbol := range missing {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	return symbols
}
