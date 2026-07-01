package stats

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
)

func TestInitStore_MongoCreatesIndexesAndMigratesLegacy(t *testing.T) {
	uri := os.Getenv("MONGODB_TEST_URL")
	if uri == "" {
		t.Skip("MONGODB_TEST_URL not set; skipping MongoDB integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := storage.NewMongoClient(ctx, uri)
	if err != nil {
		t.Fatalf("NewMongoClient: %v", err)
	}
	dbName := fmt.Sprintf("miti99bot_stats_test_%d", time.Now().UnixNano())
	db := client.Database(dbName)
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = db.Drop(cleanupCtx)
		_ = client.Disconnect(cleanupCtx)
	}()

	provider := storage.NewMongoProvider(db)
	statsColl := provider.Collection("stats")
	systemColl := provider.Collection(systemstate.CollectionName)
	legacyCounts := storage.Typed[legacyCountEntry](statsColl)
	legacyUsers := storage.Typed[legacyUserEntry](statsColl)
	sys := systemstate.New(systemColl)
	if err := sys.Put(ctx, "migration:stats-command-history-v1", systemstate.Record{
		Kind:      "migration",
		Name:      "stats-command-history-v1",
		Status:    "done",
		Count:     2,
		UpdatedAt: 1,
	}); err != nil {
		t.Fatalf("seed v1 marker: %v", err)
	}
	if err := legacyCounts.Put(ctx, legacyCountPrefix+"ping", legacyCountEntry{N: 2}); err != nil {
		t.Fatalf("legacy count: %v", err)
	}
	if err := legacyCounts.Put(ctx, legacyCountPrefix+"gold_stats", legacyCountEntry{N: 4}); err != nil {
		t.Fatalf("legacy renamed count: %v", err)
	}
	if err := legacyCounts.Put(ctx, legacyCountPrefix+"trade_stats", legacyCountEntry{N: 6}); err != nil {
		t.Fatalf("legacy trade count: %v", err)
	}
	if err := legacyUsers.Put(ctx, legacyUserPrefix+"7", legacyUserEntry{Username: "alice", N: 1}); err != nil {
		t.Fatalf("legacy user: %v", err)
	}
	if err := legacyCounts.Put(ctx, legacyPairPrefix+"ping:7", legacyCountEntry{N: 1}); err != nil {
		t.Fatalf("legacy pair: %v", err)
	}
	if err := legacyCounts.Put(ctx, legacyPairPrefix+"stock_convert:7", legacyCountEntry{N: 3}); err != nil {
		t.Fatalf("legacy deleted pair: %v", err)
	}

	if err := InitStore(ctx, statsColl, systemColl); err != nil {
		t.Fatalf("InitStore: %v", err)
	}

	usageDocs := storage.Typed[usageEntry](statsColl)
	assertUsageEntry(t, usageDocs, usageKey("ping", 7), usageEntry{Cmd: "ping", UserID: 7, Username: "alice", N: 1})
	assertUsageEntry(t, usageDocs, usageKey("ping", 0), usageEntry{Cmd: "ping", N: 1})
	assertUsageEntry(t, usageDocs, usageKey("gold_portfolio", 0), usageEntry{Cmd: "gold_portfolio", N: 4})
	assertUsageEntry(t, usageDocs, usageKey("stock_portfolio", 0), usageEntry{Cmd: "stock_portfolio", N: 6})
	assertUsageEntry(t, usageDocs, usageKey("stock_convert", 7), usageEntry{Cmd: "stock_convert", UserID: 7, Username: "alice", N: 3, Deleted: true})

	rawStatsColl, ok := storage.MongoCollection(statsColl)
	if !ok {
		t.Fatal("stats collection is not Mongo-backed")
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
	for _, name := range []string{"stats_cmd_n_user", "stats_uid_n_cmd", "stats_user_updated_at"} {
		if !found[name] {
			t.Fatalf("missing index %s; indexes=%v", name, found)
		}
	}

	rec, ok, err := sys.Get(ctx, commandHistoryMigrationKey)
	if err != nil || !ok {
		t.Fatalf("command history marker ok=%v err=%v", ok, err)
	}
	if rec.Status != "done" || rec.Count != 3 {
		t.Fatalf("command history marker = %+v, want done count 3", rec)
	}

	rawDoc := bson.M{}
	err = rawStatsColl.FindOne(ctx, bson.M{"_id": legacyCountPrefix + "ping"}).Decode(&rawDoc)
	if err == nil {
		t.Fatalf("legacy count key still exists: %+v", rawDoc)
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("legacy count lookup err = %v, want ErrNoDocuments", err)
	}
}
