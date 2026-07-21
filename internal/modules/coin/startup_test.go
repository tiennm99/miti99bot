package coin

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
)

type migrationCoinPrices struct {
	quotes map[string]float64
	err    error
	calls  int
}

func (f *migrationCoinPrices) FetchUSD(_ context.Context, coin CoinSymbol) (CoinPrice, error) {
	f.calls++
	if f.err != nil {
		return CoinPrice{}, f.err
	}
	return CoinPrice{Symbol: coin.Symbol, USD: f.quotes[coin.Symbol], Source: "test"}, nil
}

func TestInitStoreMigratesLegacyCoinBasisIdempotently(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	portfolioColl := provider.Collection(CollectionName)
	systemColl := provider.Collection(systemstate.CollectionName)
	docs := storage.Typed[Portfolio](portfolioColl)
	legacy := NewPortfolio(1)
	legacy.Assets["BTC"] = 0.25
	legacy.Assets["ETH"] = 2
	if err := docs.Put(ctx, "user:7", legacy); err != nil {
		t.Fatal(err)
	}
	prices := &migrationCoinPrices{quotes: map[string]float64{"BTC": 100_000, "ETH": 3_000}}
	if err := InitStore(ctx, portfolioColl, systemColl, prices); err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	got, _, err := docs.Get(ctx, "user:7")
	if err != nil {
		t.Fatal(err)
	}
	if got.CostBasis["BTC"] != 25_000 || got.CostBasis["ETH"] != 6_000 {
		t.Fatalf("CostBasis = %#v", got.CostBasis)
	}
	if err := InitStore(ctx, portfolioColl, systemColl, prices); err != nil {
		t.Fatalf("InitStore second run: %v", err)
	}
	if prices.calls != 2 {
		t.Fatalf("quote calls = %d, want one per symbol on first run", prices.calls)
	}
}

func TestInitStoreCoinRequiresCompleteQuotesBeforeWriting(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	portfolioColl := provider.Collection(CollectionName)
	systemColl := provider.Collection(systemstate.CollectionName)
	docs := storage.Typed[Portfolio](portfolioColl)
	legacy := NewPortfolio(1)
	legacy.Assets["BTC"] = 1
	if err := docs.Put(ctx, "user:7", legacy); err != nil {
		t.Fatal(err)
	}
	if err := InitStore(ctx, portfolioColl, systemColl, &migrationCoinPrices{quotes: map[string]float64{}}); err == nil {
		t.Fatal("InitStore succeeded without a valid quote")
	}
	got, _, _ := docs.Get(ctx, "user:7")
	if len(got.CostBasis) != 0 {
		t.Fatalf("partial migration wrote basis: %#v", got.CostBasis)
	}
	if _, _, err := storage.Typed[systemstate.Record](systemColl).Get(ctx, costBasisMigrationKey); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("marker err = %v, want ErrNotFound", err)
	}
}

func TestCoinWeightedAverageBasisAndDustExit(t *testing.T) {
	p := NewPortfolio(1)
	p.AddAsset("BTC", 0.3)
	if err := p.AddCostBasis("BTC", 12_000); err != nil {
		t.Fatal(err)
	}
	removed, err := p.RemoveCostBasis("BTC", 0.1, 0.3, true)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(removed-4_000) > 1e-9 || math.Abs(p.CostBasis["BTC"]-8_000) > 1e-9 {
		t.Fatalf("removed=%v remaining=%v", removed, p.CostBasis["BTC"])
	}
	removed, err = p.RemoveCostBasis("BTC", 0.2, 0.2, false)
	if err != nil || math.Abs(removed-8_000) > 1e-9 {
		t.Fatalf("full exit removed=%v err=%v", removed, err)
	}
	if _, exists := p.CostBasis["BTC"]; exists {
		t.Fatal("full exit retained basis")
	}
}

func TestCoinMigrationScansRowsAddedAfterCompletionMarker(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	portfolioColl := provider.Collection(CollectionName)
	systemColl := provider.Collection(systemstate.CollectionName)
	prices := &migrationCoinPrices{quotes: map[string]float64{"BTC": 100_000}}
	if err := InitStore(ctx, portfolioColl, systemColl, prices); err != nil {
		t.Fatal(err)
	}
	docs := storage.Typed[Portfolio](portfolioColl)
	legacy := NewPortfolio(1)
	legacy.Assets["BTC"] = 0.5
	if err := docs.Put(ctx, "user:8", legacy); err != nil {
		t.Fatal(err)
	}
	if err := InitStore(ctx, portfolioColl, systemColl, prices); err != nil {
		t.Fatal(err)
	}
	got, _, err := docs.Get(ctx, "user:8")
	if err != nil || got.CostBasis["BTC"] != 50_000 {
		t.Fatalf("portfolio=%+v err=%v", got, err)
	}
}

func TestCoinMigrationHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	provider := storage.NewMemoryProvider()
	docs := storage.Typed[Portfolio](provider.Collection(CollectionName))
	legacy := NewPortfolio(1)
	legacy.Assets["BTC"] = 1
	if err := docs.Put(context.Background(), "user:7", legacy); err != nil {
		t.Fatal(err)
	}
	prices := &migrationCoinPrices{err: context.Canceled}
	if err := InitStore(ctx, provider.Collection(CollectionName), provider.Collection(systemstate.CollectionName), prices); !errors.Is(err, context.Canceled) {
		t.Fatalf("InitStore err=%v, want context.Canceled", err)
	}
}
