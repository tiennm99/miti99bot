package stats

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/testutil/mongotest"
)

var mongoTests mongotest.Manager

func TestMain(m *testing.M) {
	os.Exit(mongoTests.Run(m))
}

func TestInitStore_MongoCreatesIndexes(t *testing.T) {
	ctx, statsColl, systemColl := setupMongoStatsTest(t)

	rawStatsColl, ok := storage.MongoCollection(statsColl)
	if !ok {
		t.Fatal("stats collection is not Mongo-backed")
	}

	if err := InitStore(ctx, statsColl, systemColl); err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if err := InitStore(ctx, statsColl, systemColl); err != nil {
		t.Fatalf("InitStore second run: %v", err)
	}

	cur, err := rawStatsColl.Indexes().List(ctx)
	if err != nil {
		t.Fatalf("list indexes: %v", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	found := map[string]bool{}
	for cur.Next(ctx) {
		var doc struct {
			Name string `bson:"name"`
		}
		if err := cur.Decode(&doc); err != nil {
			t.Fatalf("decode index: %v", err)
		}
		found[doc.Name] = true
	}
	if err := cur.Err(); err != nil {
		t.Fatalf("index cursor: %v", err)
	}
	for _, name := range []string{statsCommandUsersIndexName, statsUserCommandsIndexName, statsUsernameLookupIndexName} {
		if !found[name] {
			t.Fatalf("missing index %s; indexes=%v", name, found)
		}
	}
}

func TestInitStore_MongoMigratesDividendStatsIdempotently(t *testing.T) {
	ctx, statsColl, systemColl := setupMongoStatsTest(t)
	docs := storage.Typed[usageEntry](statsColl)
	seedUsageEntries(t, docs, map[string]usageEntry{
		usageKey("stock_bonus", 0):          {Cmd: "stock_bonus", N: 4},
		usageKey("stock_bonus", 7):          {Cmd: "stock_bonus", UserID: 7, Username: "alice", N: 5},
		usageKey("stock_share_dividend", 7): {Cmd: "stock_share_dividend", UserID: 7, Username: "alice", N: 2},
	})
	if err := InitStore(ctx, statsColl, systemColl); err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if err := InitStore(ctx, statsColl, systemColl); err != nil {
		t.Fatalf("InitStore second run: %v", err)
	}
	assertUsageEntry(t, docs, usageKey("stock_share_dividend", 0), 4, "")
	assertUsageEntry(t, docs, usageKey("stock_share_dividend", 7), 7, "alice")
}

func setupMongoStatsTest(t *testing.T) (context.Context, storage.Collection, storage.Collection) {
	t.Helper()

	uri := mongoTests.URI(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	client, err := storage.NewMongoClient(ctx, uri)
	if err != nil {
		t.Fatalf("NewMongoClient: %v", err)
	}
	dbName := fmt.Sprintf("miti99bot_stats_test_%d", time.Now().UnixNano())
	db := client.Database(dbName)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = db.Drop(cleanupCtx)
		_ = client.Disconnect(cleanupCtx)
	})

	provider := storage.NewMongoProvider(db)
	return ctx, provider.Collection("stats"), provider.Collection("system")
}
