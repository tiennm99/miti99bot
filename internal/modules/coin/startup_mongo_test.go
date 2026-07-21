package coin

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

func TestInitStoreMigratesCoinBasisInMongoDB(t *testing.T) {
	ctx, portfolioColl, systemColl := setupMongoCoinTest(t)
	if err := storage.Typed[oldCoinPortfolio](portfolioColl).Put(ctx, "user:7", oldCoinPortfolio{
		USD: 500, Assets: map[string]float64{"BTC": 0.25}, CostBasis: map[string]float64{"BTC": 25_000},
		Meta: PortfolioMeta{CreatedAt: 1},
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
	if err != nil || got.USD != 500 || got.Assets["BTC"].Quantity != 0.25 || got.Assets["BTC"].Base != 25_000 {
		t.Fatalf("portfolio=%+v err=%v", got, err)
	}
	rawColl, _ := storage.MongoCollection(portfolioColl)
	var raw bson.M
	if err := rawColl.FindOne(ctx, bson.M{"_id": "user:7"}).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	for _, legacyField := range []string{"costBasis", "tickers"} {
		if _, exists := raw[legacyField]; exists {
			t.Fatalf("legacy field %q remains in %#v", legacyField, raw)
		}
	}
}

func setupMongoCoinTest(t *testing.T) (context.Context, storage.Collection, storage.Collection) {
	t.Helper()
	uri := mongoTests.URI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	client, err := storage.NewMongoClient(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	db := client.Database(fmt.Sprintf("miti99bot_coin_test_%d", time.Now().UnixNano()))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = db.Drop(cleanupCtx)
		_ = client.Disconnect(cleanupCtx)
	})
	provider := storage.NewMongoProvider(db)
	return ctx, provider.Collection(CollectionName), provider.Collection(systemstate.CollectionName)
}
