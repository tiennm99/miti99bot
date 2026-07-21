package coin

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
	"github.com/tiennm99/miti99bot/internal/testutil/mongotest"
)

var coinMongoTests mongotest.Manager

func TestMain(m *testing.M) { os.Exit(coinMongoTests.Run(m)) }

func TestInitStoreRemovesCoinDividendCursorInMongoDB(t *testing.T) {
	uri := coinMongoTests.URI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	client, err := storage.NewMongoClient(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	db := client.Database(fmt.Sprintf("miti99bot_coin_cleanup_test_%d", time.Now().UnixNano()))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = db.Drop(cleanupCtx)
		_ = client.Disconnect(cleanupCtx)
	})
	provider := storage.NewMongoProvider(db)
	coinColl := provider.Collection(CollectionName)
	legacyDocs := storage.Typed[dividendCursorCleanupPortfolio](coinColl)
	if err := legacyDocs.Put(ctx, "user:7", dividendCursorCleanupPortfolio{
		USD: 5,
		Assets: map[string]dividendCursorCleanupPosition{
			"BTC": {Quantity: 0.25, Base: 20_000, DividendCheckedAt: int64Pointer(123)},
		},
		Meta: PortfolioMeta{Invested: 10, CreatedAt: 1},
	}); err != nil {
		t.Fatal(err)
	}
	rawCollection, ok := storage.MongoCollection(coinColl)
	if !ok {
		t.Fatal("coin collection is not MongoDB")
	}
	if !mongoCoinDividendCursorExists(t, ctx, rawCollection) {
		t.Fatal("test setup did not persist the stale dividend cursor")
	}
	_, versionBefore, _ := legacyDocs.Get(ctx, "user:7")
	if err := InitStore(ctx, coinColl, provider.Collection(systemstate.CollectionName)); err != nil {
		t.Fatal(err)
	}
	if err := InitStore(ctx, coinColl, provider.Collection(systemstate.CollectionName)); err != nil {
		t.Fatal(err)
	}
	_, versionAfter, err := storage.Typed[Portfolio](coinColl).Get(ctx, "user:7")
	if err != nil || versionAfter != versionBefore+1 {
		t.Fatalf("version before=%d after=%d err=%v", versionBefore, versionAfter, err)
	}
	if mongoCoinDividendCursorExists(t, ctx, rawCollection) {
		t.Fatal("stale dividend cursor remains")
	}
}

func mongoCoinDividendCursorExists(t *testing.T, ctx context.Context, collection *mongo.Collection) bool {
	t.Helper()
	var raw bson.Raw
	if err := collection.FindOne(ctx, bson.M{"_id": "user:7"}).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	return raw.Lookup("assets", "BTC", "dividendCheckedAt").Type != 0
}
