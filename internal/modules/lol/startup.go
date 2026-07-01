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
	matchCacheKeyPrefix    = "matches:"
	matchCacheKeyUpper     = "matches;"
	matchCacheTTL          = 30 * 24 * time.Hour
)

// InitStore performs LoL collection startup maintenance. It is safe to call
// every boot: MongoDB indexes are created idempotently and memory storage is a
// no-op.
func InitStore(ctx context.Context, lolColl storage.Collection) error {
	if mongoColl, ok := storage.MongoCollection(lolColl); ok {
		if err := backfillMatchCacheFetchedAt(ctx, mongoColl); err != nil {
			return err
		}
		if err := ensureMatchCacheTTLIndex(ctx, mongoColl); err != nil {
			return err
		}
	}
	return nil
}

func backfillMatchCacheFetchedAt(ctx context.Context, coll *mongo.Collection) error {
	filter := bson.D{
		{Key: "_id", Value: matchCacheIDRange()},
		{Key: "fetchedAt", Value: bson.D{{Key: "$exists", Value: false}}},
		{Key: "ts", Value: bson.D{
			{Key: "$type", Value: "number"},
			{Key: "$gt", Value: int64(0)},
		}},
	}
	update := mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "fetchedAt", Value: bson.D{{Key: "$toDate", Value: "$ts"}}},
		}}},
	}
	if _, err := coll.UpdateMany(ctx, filter, update); err != nil {
		return fmt.Errorf("lol match cache fetchedAt backfill: %w", err)
	}
	return nil
}

func ensureMatchCacheTTLIndex(ctx context.Context, coll *mongo.Collection) error {
	model := mongo.IndexModel{
		Keys: bson.D{{Key: "fetchedAt", Value: 1}},
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

func matchCacheIDRange() bson.D {
	return bson.D{
		{Key: "$gte", Value: matchCacheKeyPrefix},
		{Key: "$lt", Value: matchCacheKeyUpper},
	}
}
