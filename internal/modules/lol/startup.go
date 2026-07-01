package lol

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/tiennm99/miti99bot/internal/storage"
)

const (
	matchCacheTTLIndexName = "lol_match_cache_ttl"
	matchCacheTTLField     = "updatedAt"
	matchCacheKeyPrefix    = "matches:"
	matchCacheKeyUpper     = "matches;"
	matchCacheTTL          = 30 * 24 * time.Hour
)

// InitStore performs LoL collection startup maintenance. It is safe to call
// every boot: MongoDB indexes are created idempotently and memory storage is a
// no-op.
func InitStore(ctx context.Context, lolColl storage.Collection) error {
	if mongoColl, ok := storage.MongoCollection(lolColl); ok {
		if err := deleteMatchCacheDocs(ctx, mongoColl); err != nil {
			return err
		}
		if err := ensureMatchCacheTTLIndex(ctx, mongoColl); err != nil {
			return err
		}
	}
	return nil
}

func ensureMatchCacheTTLIndex(ctx context.Context, coll *mongo.Collection) error {
	if err := dropConflictingMatchCacheTTLIndex(ctx, coll); err != nil {
		return err
	}
	model := mongo.IndexModel{
		Keys: bson.D{{Key: matchCacheTTLField, Value: 1}},
		Options: options.Index().
			SetName(matchCacheTTLIndexName).
			SetExpireAfterSeconds(int32(matchCacheTTL / time.Second)).
			SetPartialFilterExpression(bson.D{{Key: "_id", Value: matchCacheIDRange()}}),
	}
	if _, err := coll.Indexes().CreateOne(ctx, model); err != nil {
		return fmt.Errorf("lol match cache ttl index: %w", err)
	}
	return nil
}

func dropConflictingMatchCacheTTLIndex(ctx context.Context, coll *mongo.Collection) error {
	cur, err := coll.Indexes().List(ctx)
	if err != nil {
		return fmt.Errorf("list lol indexes: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			return fmt.Errorf("decode lol index: %w", err)
		}
		if doc["name"] != matchCacheTTLIndexName {
			continue
		}
		if matchCacheTTLIndexMatches(doc) {
			return nil
		}
		if err := coll.Indexes().DropOne(ctx, matchCacheTTLIndexName); err != nil {
			return fmt.Errorf("drop old lol match cache ttl index: %w", err)
		}
		return nil
	}
	if err := cur.Err(); err != nil {
		return fmt.Errorf("iterate lol indexes: %w", err)
	}
	return nil
}

func matchCacheTTLIndexMatches(doc bson.M) bool {
	if !singleAscendingKey(doc["key"], matchCacheTTLField) {
		return false
	}
	if got, ok := int64BSON(doc["expireAfterSeconds"]); !ok || got != int64(matchCacheTTL/time.Second) {
		return false
	}
	partial, ok := bsonDocument(doc["partialFilterExpression"])
	if !ok {
		return false
	}
	idFilter, ok := bsonDocument(partial["_id"])
	if !ok {
		return false
	}
	return idFilter["$gte"] == matchCacheKeyPrefix && idFilter["$lt"] == matchCacheKeyUpper
}

func deleteMatchCacheDocs(ctx context.Context, coll *mongo.Collection) error {
	if _, err := coll.DeleteMany(ctx, bson.D{{Key: "_id", Value: matchCacheIDRange()}}); err != nil {
		return fmt.Errorf("delete old lol match cache docs: %w", err)
	}
	return nil
}

func matchCacheIDRange() bson.D {
	return bson.D{
		{Key: "$gte", Value: matchCacheKeyPrefix},
		{Key: "$lt", Value: matchCacheKeyUpper},
	}
}

func singleAscendingKey(v any, field string) bool {
	doc, ok := bsonDocument(v)
	if !ok || len(doc) != 1 {
		return false
	}
	got, ok := int64BSON(doc[field])
	return ok && got == 1
}

func bsonDocument(v any) (bson.M, bool) {
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

func int64BSON(v any) (int64, bool) {
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
