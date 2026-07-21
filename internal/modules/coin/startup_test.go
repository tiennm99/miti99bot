package coin

import (
	"context"
	"testing"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
)

type oldCoinPortfolio struct {
	USD       float64            `json:"usd" bson:"usd"`
	Assets    map[string]float64 `json:"assets" bson:"assets"`
	CostBasis map[string]float64 `json:"costBasis" bson:"costBasis"`
	Meta      PortfolioMeta      `json:"meta" bson:"meta"`
}

func TestInitStoreMigratesCoinNestedAssets(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	portfolioColl := provider.Collection(CollectionName)
	systemColl := provider.Collection(systemstate.CollectionName)
	if err := storage.Typed[oldCoinPortfolio](portfolioColl).Put(ctx, "user:7", oldCoinPortfolio{
		USD: 500, Assets: map[string]float64{"BTC": 0.25}, CostBasis: map[string]float64{"BTC": 20_000},
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
	position := got.Assets["BTC"]
	if got.USD != 500 || position.Quantity != 0.25 || position.Base != 20_000 || position.DividendCheckedAt <= 0 {
		t.Fatalf("portfolio=%+v", got)
	}
	if err := InitStore(ctx, portfolioColl, systemColl); err != nil {
		t.Fatalf("second InitStore: %v", err)
	}
	marker, exists, err := systemstate.New(systemColl).Get(ctx, assetSchemaMarkerKey)
	if err != nil || !exists || marker.Count != 1 {
		t.Fatalf("marker=%+v exists=%v err=%v", marker, exists, err)
	}
}

func TestCoinMigrationRejectsMissingBasis(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	coll := provider.Collection(CollectionName)
	if err := storage.Typed[oldCoinPortfolio](coll).Put(ctx, "user:7", oldCoinPortfolio{
		Assets: map[string]float64{"BTC": 1}, CostBasis: map[string]float64{},
	}); err != nil {
		t.Fatal(err)
	}
	if err := InitStore(ctx, coll, provider.Collection(systemstate.CollectionName)); err == nil {
		t.Fatal("InitStore accepted legacy holding without basis")
	}
}

func TestCoinSchemaMigrationRejectsMixedAssetShapes(t *testing.T) {
	doc := legacyPortfolio{
		Assets: map[string]AssetPosition{
			"BTC": {Quantity: 1, Base: 10, DividendCheckedAt: 1},
			"ETH": {Quantity: 1, legacyQuantity: true},
		},
		CostBasis: map[string]float64{"ETH": 2_000},
	}
	if _, err := doc.migrate(123); err == nil {
		t.Fatal("migration accepted mixed legacy and nested assets")
	}
}
