package lol

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
)

func TestInitStore_MongoMigratesLegacyCollection(t *testing.T) {
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
	dbName := fmt.Sprintf("miti99bot_lol_test_%d", time.Now().UnixNano())
	db := client.Database(dbName)
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = db.Drop(cleanupCtx)
		_ = client.Disconnect(cleanupCtx)
	}()

	provider := storage.NewMongoProvider(db)
	legacyColl := provider.Collection(legacyCollectionName)
	systemColl := provider.Collection(systemstate.CollectionName)
	legacySubscribers := storage.Typed[subscribersDoc](legacyColl)
	legacyPushDate := storage.Typed[lastPushDoc](legacyColl)
	legacyCache := storage.Typed[cacheRecord](legacyColl)

	if err := legacySubscribers.Put(ctx, subscribersKey, subscribersDoc{Subscribers: []Subscriber{{ChatID: 7}, {ChatID: 8, ThreadID: 3}}}); err != nil {
		t.Fatalf("legacy subscribers: %v", err)
	}
	if err := legacyPushDate.Put(ctx, lastPushDateKey, lastPushDoc{Date: "2026-07-01"}); err != nil {
		t.Fatalf("legacy push date: %v", err)
	}
	if err := legacyCache.Put(ctx, "matches:from:to", cacheRecord{
		Ts:     123,
		Events: []ScheduleEvent{{StartTime: "2026-07-01T01:00:00Z", League: League{Slug: "lck"}}},
	}); err != nil {
		t.Fatalf("legacy cache: %v", err)
	}

	if err := InitStore(ctx, provider, systemColl); err != nil {
		t.Fatalf("InitStore: %v", err)
	}

	newColl := provider.Collection(CollectionName)
	newSubscribers := storage.Typed[subscribersDoc](newColl)
	newPushDate := storage.Typed[lastPushDoc](newColl)
	newCache := storage.Typed[cacheRecord](newColl)

	subs, _, err := newSubscribers.Get(ctx, subscribersKey)
	if err != nil {
		t.Fatalf("new subscribers get: %v", err)
	}
	if len(subs.Subscribers) != 2 || subs.Subscribers[1] != (Subscriber{ChatID: 8, ThreadID: 3}) {
		t.Fatalf("new subscribers = %+v", subs.Subscribers)
	}
	push, _, err := newPushDate.Get(ctx, lastPushDateKey)
	if err != nil {
		t.Fatalf("new push date get: %v", err)
	}
	if push.Date != "2026-07-01" {
		t.Fatalf("new push date = %q", push.Date)
	}
	cached, _, err := newCache.Get(ctx, "matches:from:to")
	if err != nil {
		t.Fatalf("new cache get: %v", err)
	}
	if cached.Ts != 123 || len(cached.Events) != 1 || cached.Events[0].League.Slug != "lck" {
		t.Fatalf("new cache = %+v", cached)
	}

	names, err := db.ListCollectionNames(ctx, bson.M{"name": legacyCollectionName})
	if err != nil {
		t.Fatalf("ListCollectionNames: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("legacy collection still exists: %v", names)
	}

	rec, ok, err := systemstate.New(systemColl).Get(ctx, collectionMigrationKey)
	if err != nil {
		t.Fatalf("migration marker get: %v", err)
	}
	if !ok || rec.Status != "done" || rec.Name != collectionMigrationName || rec.Count != 3 {
		t.Fatalf("migration marker = %+v ok=%v", rec, ok)
	}

	if err := InitStore(ctx, provider, systemColl); err != nil {
		t.Fatalf("second InitStore: %v", err)
	}
}
