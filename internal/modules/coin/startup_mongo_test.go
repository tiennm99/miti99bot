package coin

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

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
	docs := storage.Typed[Portfolio](portfolioColl)
	legacy := NewPortfolio(1)
	legacy.Assets["BTC"] = 0.25
	if err := docs.Put(ctx, "user:7", legacy); err != nil {
		t.Fatal(err)
	}
	prices := &migrationCoinPrices{quotes: map[string]float64{"BTC": 100_000}}
	if err := InitStore(ctx, portfolioColl, systemColl, prices); err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if err := InitStore(ctx, portfolioColl, systemColl, prices); err != nil {
		t.Fatalf("InitStore second run: %v", err)
	}
	got, _, err := docs.Get(ctx, "user:7")
	if err != nil || got.CostBasis["BTC"] != 25_000 {
		t.Fatalf("portfolio=%+v err=%v", got, err)
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
