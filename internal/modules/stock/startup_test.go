package stock

import (
	"context"
	"errors"
	"testing"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
)

type migrationStockPrices struct {
	quotes map[string]float64
	err    error
	calls  int
}

func (f *migrationStockPrices) FetchPrices(_ context.Context, _ []string) (map[string]float64, error) {
	f.calls++
	return f.quotes, f.err
}

func TestInitStoreMigratesLegacyStockBasisIdempotently(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	portfolioColl := provider.Collection(CollectionName)
	systemColl := provider.Collection(systemstate.CollectionName)
	docs := storage.Typed[Portfolio](portfolioColl)
	legacy := NewPortfolio(1)
	legacy.Assets["TCB"] = 100
	legacy.Assets["FPT"] = 20
	if err := docs.Put(ctx, "user:7", legacy); err != nil {
		t.Fatal(err)
	}
	prices := &migrationStockPrices{quotes: map[string]float64{"TCB": 30_000, "FPT": 120_000}}
	if err := InitStore(ctx, portfolioColl, systemColl, prices); err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	got, _, err := docs.Get(ctx, "user:7")
	if err != nil {
		t.Fatal(err)
	}
	if got.CostBasis["TCB"] != 3_000_000 || got.CostBasis["FPT"] != 2_400_000 {
		t.Fatalf("CostBasis = %#v", got.CostBasis)
	}
	if err := InitStore(ctx, portfolioColl, systemColl, prices); err != nil {
		t.Fatalf("InitStore second run: %v", err)
	}
	if prices.calls != 1 {
		t.Fatalf("quote calls = %d, want 1", prices.calls)
	}
	marker, _, err := storage.Typed[systemstate.Record](systemColl).Get(ctx, costBasisMigrationKey)
	if err != nil || marker.Status != "completed" {
		t.Fatalf("marker = %+v, err=%v", marker, err)
	}
}

func TestInitStoreStockRequiresCompleteQuotesBeforeWriting(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	portfolioColl := provider.Collection(CollectionName)
	systemColl := provider.Collection(systemstate.CollectionName)
	docs := storage.Typed[Portfolio](portfolioColl)
	legacy := NewPortfolio(1)
	legacy.Assets["TCB"] = 100
	if err := docs.Put(ctx, "user:7", legacy); err != nil {
		t.Fatal(err)
	}
	if err := InitStore(ctx, portfolioColl, systemColl, &migrationStockPrices{quotes: map[string]float64{}}); err == nil {
		t.Fatal("InitStore succeeded without a complete quote set")
	}
	got, _, _ := docs.Get(ctx, "user:7")
	if len(got.CostBasis) != 0 {
		t.Fatalf("partial migration wrote basis: %#v", got.CostBasis)
	}
	if _, _, err := storage.Typed[systemstate.Record](systemColl).Get(ctx, costBasisMigrationKey); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("marker err = %v, want ErrNotFound", err)
	}
}

func TestStockWeightedAverageBasis(t *testing.T) {
	p := NewPortfolio(1)
	p.AddAsset("TCB", 100)
	if err := p.AddCostBasis("TCB", 2_000_000); err != nil {
		t.Fatal(err)
	}
	p.AddAsset("TCB", 50)
	if err := p.AddCostBasis("TCB", 1_500_000); err != nil {
		t.Fatal(err)
	}
	removed, err := p.RemoveCostBasis("TCB", 60, 150)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1_400_000 || p.CostBasis["TCB"] != 2_100_000 {
		t.Fatalf("removed=%v remaining=%v", removed, p.CostBasis["TCB"])
	}
}

type conflictOnceStockMigrationStore struct {
	Store
	conflicted bool
}

func (s *conflictOnceStockMigrationStore) PutVersioned(ctx context.Context, key string, version int64, p Portfolio) error {
	if !s.conflicted {
		s.conflicted = true
		return storage.ErrConflict
	}
	return s.Store.PutVersioned(ctx, key, version, p)
}

func TestStockMigrationRetriesWriteConflictAndPreservesExistingBasis(t *testing.T) {
	ctx := context.Background()
	base := newStockStore()
	legacy := NewPortfolio(1)
	legacy.Assets["TCB"] = 100
	legacy.Assets["FPT"] = 10
	legacy.CostBasis["TCB"] = 2_500_000
	if err := base.Put(ctx, "user:7", legacy); err != nil {
		t.Fatal(err)
	}
	store := &conflictOnceStockMigrationStore{Store: base}
	count, err := migrateLegacyPortfolio(ctx, store, "user:7", map[string]float64{"FPT": 120_000})
	if err != nil || count != 1 || !store.conflicted {
		t.Fatalf("count=%d conflicted=%v err=%v", count, store.conflicted, err)
	}
	got, _, _ := base.Get(ctx, "user:7")
	if got.CostBasis["TCB"] != 2_500_000 || got.CostBasis["FPT"] != 1_200_000 {
		t.Fatalf("CostBasis=%#v", got.CostBasis)
	}
}

func TestStockMigrationRejectsNoncanonicalLegacySymbol(t *testing.T) {
	p := NewPortfolio(1)
	p.Assets["tcb"] = 100
	if err := inspectLegacyPortfolio(p, map[string]bool{}); err == nil {
		t.Fatal("inspectLegacyPortfolio accepted noncanonical symbol")
	}
}
