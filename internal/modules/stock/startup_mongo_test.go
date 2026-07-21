package stock

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
	"github.com/tiennm99/miti99bot/internal/testutil/mongotest"
)

var mongoTests mongotest.Manager

func TestMain(m *testing.M) {
	os.Exit(mongoTests.Run(m))
}

func TestInitStoreMigratesStockBasisInMongoDB(t *testing.T) {
	ctx, portfolioColl, systemColl := setupMongoStockTest(t)
	if err := storage.Typed[oldStockPortfolio](portfolioColl).Put(ctx, "user:7", oldStockPortfolio{
		Currency: map[string]float64{"VND": 500_000}, Assets: map[string]int64{"TCB": 100},
		CostBasis: map[string]float64{"TCB": 3_000_000}, Meta: PortfolioMeta{CreatedAt: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if err := InitStore(ctx, portfolioColl, systemColl); err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if err := InitStore(ctx, portfolioColl, systemColl); err != nil {
		t.Fatalf("InitStore second run: %v", err)
	}
	got, _, err := storage.Typed[Portfolio](portfolioColl).Get(ctx, "user:7")
	if err != nil || got.VND != 500_000 || got.Assets["TCB"].Quantity != 100 || got.Assets["TCB"].Base != 3_000_000 {
		t.Fatalf("portfolio=%+v err=%v", got, err)
	}
	rawColl, _ := storage.MongoCollection(portfolioColl)
	var raw bson.M
	if err := rawColl.FindOne(ctx, bson.M{"_id": "user:7"}).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	for _, legacyField := range []string{"currency", "costBasis", "tickers"} {
		if _, exists := raw[legacyField]; exists {
			t.Fatalf("legacy field %q remains in %#v", legacyField, raw)
		}
	}
}

func setupMongoStockTest(t *testing.T) (context.Context, storage.Collection, storage.Collection) {
	t.Helper()
	uri := mongoTests.URI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	client, err := storage.NewMongoClient(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	db := client.Database(fmt.Sprintf("miti99bot_stock_test_%d", time.Now().UnixNano()))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = db.Drop(cleanupCtx)
		_ = client.Disconnect(cleanupCtx)
	})
	provider := storage.NewMongoProvider(db)
	return ctx, provider.Collection(CollectionName), provider.Collection(systemstate.CollectionName)
}
