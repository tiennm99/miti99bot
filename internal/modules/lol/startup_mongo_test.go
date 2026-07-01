package lol

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/tiennm99/miti99bot/internal/storage"
)

func TestInitStore_MongoCreatesMatchCacheTTLIndex(t *testing.T) {
	const legacyFetchedAtField = "fetchedAt"

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

	legacyFrom := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	legacyTo := legacyFrom.Add(24 * time.Hour)
	legacyMatchID := cacheKey(legacyFrom, legacyTo)
	legacyFetchedAt := time.Date(2026, 6, 1, 2, 3, 4, 0, time.UTC)
	_, err = rawLolColl.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: legacyFetchedAtField, Value: 1}},
		Options: options.Index().
			SetName(matchCacheTTLIndexName).
			SetExpireAfterSeconds(int32(matchCacheTTL / time.Second)).
			SetPartialFilterExpression(bson.D{{Key: "_id", Value: matchCacheIDRange()}}),
	})
	if err != nil {
		t.Fatalf("create legacy ttl index: %v", err)
	}
	_, err = rawLolColl.InsertMany(ctx, []any{
		bson.D{
			{Key: "_id", Value: legacyMatchID},
			{Key: "version", Value: int64(1)},
			{Key: "updatedAt", Value: legacyFetchedAt},
			{Key: "ts", Value: legacyFetchedAt.UnixMilli()},
			{Key: legacyFetchedAtField, Value: legacyFetchedAt},
			{Key: "events", Value: bson.A{}},
		},
		bson.D{
			{Key: "_id", Value: "subscribers"},
			{Key: "version", Value: int64(1)},
			{Key: "updatedAt", Value: legacyFetchedAt},
			{Key: legacyFetchedAtField, Value: legacyFetchedAt},
			{Key: "subscribers", Value: bson.A{}},
		},
	})
	if err != nil {
		t.Fatalf("insert legacy docs: %v", err)
	}

	if err := InitStore(ctx, lolColl); err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if err := InitStore(ctx, lolColl); err != nil {
		t.Fatalf("InitStore second run: %v", err)
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
	if got, _ := int64FromBSON(keyDoc[matchCacheTTLField]); got != 1 {
		t.Fatalf("index key %s = %v, want 1", matchCacheTTLField, keyDoc[matchCacheTTLField])
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

	var matchDoc bson.M
	err = rawLolColl.FindOne(ctx, bson.M{"_id": legacyMatchID}).Decode(&matchDoc)
	if !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("legacy match doc lookup err = %v, want ErrNoDocuments; doc=%#v", err, matchDoc)
	}
	subscriberDoc := rawLolDoc(t, ctx, rawLolColl, "subscribers")
	if _, ok := subscriberDoc[legacyFetchedAtField]; !ok {
		t.Fatalf("non-match doc was deleted or changed unexpectedly: %#v", subscriberDoc)
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

func rawLolDoc(t *testing.T, ctx context.Context, coll *mongo.Collection, id string) bson.M {
	t.Helper()
	var doc bson.M
	if err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(&doc); err != nil {
		t.Fatalf("find raw lol doc %s: %v", id, err)
	}
	return doc
}
