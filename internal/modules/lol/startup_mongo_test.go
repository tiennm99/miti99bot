package lol

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/tiennm99/miti99bot/internal/storage"
)

func TestInitStore_MongoCreatesMatchCacheTTLIndex(t *testing.T) {
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
	lolColl := provider.Collection(CollectionName)
	rawLolColl, ok := storage.MongoCollection(lolColl)
	if !ok {
		t.Fatal("lol collection is not Mongo-backed")
	}

	fetchedAt := time.Date(2026, 5, 9, 5, 0, 0, 0, time.UTC)
	if _, err := rawLolColl.InsertMany(ctx, []any{
		bson.M{
			"_id":       "matches:2026-05-09T00:00:00Z:2026-05-10T00:00:00Z",
			"version":   int64(1),
			"updatedAt": time.Now().UTC(),
			"ts":        fetchedAt.UnixMilli(),
			"events":    bson.A{},
		},
		bson.M{
			"_id":         "subscribers",
			"version":     int64(1),
			"updatedAt":   time.Now().UTC(),
			"ts":          fetchedAt.UnixMilli(),
			"subscribers": bson.A{},
		},
	}); err != nil {
		t.Fatalf("seed legacy docs: %v", err)
	}

	if err := InitStore(ctx, lolColl); err != nil {
		t.Fatalf("InitStore: %v", err)
	}

	var matchDoc struct {
		FetchedAt time.Time `bson:"fetchedAt"`
	}
	if err := rawLolColl.FindOne(ctx, bson.M{"_id": "matches:2026-05-09T00:00:00Z:2026-05-10T00:00:00Z"}).Decode(&matchDoc); err != nil {
		t.Fatalf("load backfilled match doc: %v", err)
	}
	if !matchDoc.FetchedAt.Equal(fetchedAt) {
		t.Fatalf("backfilled fetchedAt = %s, want %s", matchDoc.FetchedAt, fetchedAt)
	}

	var subscriberDoc bson.M
	if err := rawLolColl.FindOne(ctx, bson.M{"_id": "subscribers"}).Decode(&subscriberDoc); err != nil {
		t.Fatalf("load subscriber doc: %v", err)
	}
	if _, ok := subscriberDoc["fetchedAt"]; ok {
		t.Fatalf("subscriber doc was backfilled with fetchedAt: %#v", subscriberDoc)
	}

	cur, err := rawLolColl.Indexes().List(ctx)
	if err != nil {
		t.Fatalf("list indexes: %v", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var found bson.M
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			t.Fatalf("decode index: %v", err)
		}
		if doc["name"] == matchCacheTTLIndexName {
			found = doc
			break
		}
	}
	if err := cur.Err(); err != nil {
		t.Fatalf("index cursor: %v", err)
	}
	if found == nil {
		t.Fatalf("missing index %s", matchCacheTTLIndexName)
	}
	if got, ok := int64FromBSON(found["expireAfterSeconds"]); !ok || got != int64(matchCacheTTL/time.Second) {
		t.Fatalf("expireAfterSeconds = %v, want %d", found["expireAfterSeconds"], int64(matchCacheTTL/time.Second))
	}

	keyDoc, ok := bsonDoc(found["key"])
	if !ok {
		t.Fatalf("index key is not a document: %#v", found["key"])
	}
	if got, _ := int64FromBSON(keyDoc["fetchedAt"]); got != 1 {
		t.Fatalf("index key fetchedAt = %v, want 1", keyDoc["fetchedAt"])
	}

	partial, ok := bsonDoc(found["partialFilterExpression"])
	if !ok {
		t.Fatalf("partialFilterExpression is not a document: %#v", found["partialFilterExpression"])
	}
	idFilter, ok := bsonDoc(partial["_id"])
	if !ok {
		t.Fatalf("partial _id filter is not a document: %#v", partial["_id"])
	}
	if idFilter["$gte"] != matchCacheKeyPrefix || idFilter["$lt"] != matchCacheKeyUpper {
		t.Fatalf("partial _id filter = %#v, want [%q, %q)", idFilter, matchCacheKeyPrefix, matchCacheKeyUpper)
	}
}

func bsonDoc(v any) (bson.M, bool) {
	switch d := v.(type) {
	case bson.M:
		return d, true
	case map[string]any:
		return bson.M(d), true
	case bson.D:
		out := bson.M{}
		for _, e := range d {
			out[e.Key] = e.Value
		}
		return out, true
	default:
		return nil, false
	}
}

func int64FromBSON(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	default:
		return 0, false
	}
}
