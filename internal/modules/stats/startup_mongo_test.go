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
	if err := legacyCounts.Put(ctx, legacyCountPrefix+"ping", legacyCountEntry{N: 2}); err != nil {
		t.Fatalf("legacy count: %v", err)
	}
	if err := legacyUsers.Put(ctx, legacyUserPrefix+"7", legacyUserEntry{Username: "alice", N: 1}); err != nil {
		t.Fatalf("legacy user: %v", err)
	}
	if err := legacyCounts.Put(ctx, legacyPairPrefix+"ping:7", legacyCountEntry{N: 1}); err != nil {
		t.Fatalf("legacy pair: %v", err)
	}

	if err := InitStore(ctx, statsColl, systemColl); err != nil {
		t.Fatalf("InitStore: %v", err)
	}

	usageDocs := storage.Typed[usageEntry](statsColl)
	assertUsageEntry(t, usageDocs, usageKey("ping", 7), usageEntry{Cmd: "ping", UserID: 7, Username: "alice", N: 1})
	assertUsageEntry(t, usageDocs, usageKey("ping", 0), usageEntry{Cmd: "ping", N: 1})

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

	rawDoc := bson.M{}
	err = rawStatsColl.FindOne(ctx, bson.M{"_id": legacyCountPrefix + "ping"}).Decode(&rawDoc)
	if err == nil {
		t.Fatalf("legacy count key still exists: %+v", rawDoc)
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("legacy count lookup err = %v, want ErrNoDocuments", err)
	}
}
