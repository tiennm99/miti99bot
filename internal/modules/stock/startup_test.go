package stock

import (
	"context"
	"testing"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
)

type oldStockPortfolio struct {
	Currency  map[string]float64 `json:"currency" bson:"currency"`
	Assets    map[string]int64   `json:"assets" bson:"assets"`
	CostBasis map[string]float64 `json:"costBasis" bson:"costBasis"`
	Meta      PortfolioMeta      `json:"meta" bson:"meta"`
}

func TestInitStoreMigratesStockNestedAssetsAndVND(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	portfolioColl := provider.Collection(CollectionName)
	systemColl := provider.Collection(systemstate.CollectionName)
	oldDocs := storage.Typed[oldStockPortfolio](portfolioColl)
	if err := oldDocs.Put(ctx, "user:7", oldStockPortfolio{
		Currency: map[string]float64{"VND": 2_000_000},
		Assets:   map[string]int64{"TCB": 100}, CostBasis: map[string]float64{"TCB": 3_000_000},
		Meta: PortfolioMeta{CreatedAt: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if err := InitStore(ctx, portfolioColl, systemColl); err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	got, err := LoadPortfolio(ctx, storage.Typed[Portfolio](portfolioColl), 7, 9)
	if err != nil {
		t.Fatal(err)
	}
	position := got.Assets["TCB"]
	if got.VND != 2_000_000 || position.Quantity != 100 || position.Base != 3_000_000 || position.DividendCheckedAt <= 0 {
		t.Fatalf("portfolio=%+v", got)
	}
	if err := InitStore(ctx, portfolioColl, systemColl); err != nil {
		t.Fatalf("second InitStore: %v", err)
	}
	marker, exists, err := systemstate.New(systemColl).Get(ctx, assetSchemaMarkerKey)
	if err != nil || !exists || marker.Status != "completed" || marker.Count != 1 {
		t.Fatalf("marker=%+v exists=%v err=%v", marker, exists, err)
	}
}

func TestStockMigrationRejectsMissingBasis(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	coll := provider.Collection(CollectionName)
	if err := storage.Typed[oldStockPortfolio](coll).Put(ctx, "user:7", oldStockPortfolio{
		Currency: map[string]float64{"VND": 1}, Assets: map[string]int64{"TCB": 100}, CostBasis: map[string]float64{},
	}); err != nil {
		t.Fatal(err)
	}
	if err := InitStore(ctx, coll, provider.Collection(systemstate.CollectionName)); err == nil {
		t.Fatal("InitStore accepted legacy holding without basis")
	}
}

type conflictOnceSchemaStore struct {
	storage.DocStore[legacyPortfolio]
	conflicted bool
}

func (s *conflictOnceSchemaStore) PutVersioned(ctx context.Context, key string, version int64, value legacyPortfolio) error {
	if !s.conflicted {
		s.conflicted = true
		return storage.ErrConflict
	}
	return s.DocStore.PutVersioned(ctx, key, version, value)
}

func TestStockSchemaMigrationRetriesConflict(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	coll := provider.Collection(CollectionName)
	if err := storage.Typed[oldStockPortfolio](coll).Put(ctx, "user:7", oldStockPortfolio{
		Currency:  map[string]float64{"VND": 1},
		Assets:    map[string]int64{"TCB": 10},
		CostBasis: map[string]float64{"TCB": 300_000},
	}); err != nil {
		t.Fatal(err)
	}
	docs := storage.Typed[legacyPortfolio](coll)
	store := &conflictOnceSchemaStore{DocStore: docs}
	changed, err := migrateAssetSchema(ctx, store, "user:7", 123)
	if err != nil || !changed || !store.conflicted {
		t.Fatalf("changed=%v conflicted=%v err=%v", changed, store.conflicted, err)
	}
}
