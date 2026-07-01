package lol

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
)

const (
	// CollectionName is the current MongoDB collection/module key.
	CollectionName = "lol"

	legacyCollectionName = "lolschedule"

	collectionMigrationName = "lol-collection-v1"
	collectionMigrationKey  = "migration:" + collectionMigrationName
)

// InitStore performs lol collection startup maintenance. It is safe to call on
// every boot: the collection rename migration is guarded by the system
// collection and only applies to Mongo-backed storage.
func InitStore(ctx context.Context, provider storage.Provider, systemColl storage.Collection) error {
	oldColl, okOld := storage.MongoCollection(provider.Collection(legacyCollectionName))
	newColl, okNew := storage.MongoCollection(provider.Collection(CollectionName))
	if !okOld || !okNew {
		return nil
	}

	sys := systemstate.New(systemColl)
	if rec, ok, err := sys.Get(ctx, collectionMigrationKey); err != nil {
		return fmt.Errorf("lol collection migration marker get: %w", err)
	} else if ok && rec.Status == "done" {
		return nil
	}

	count, err := oldColl.CountDocuments(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("lol legacy collection count: %w", err)
	}
	if count == 0 {
		if err := oldColl.Drop(ctx); err != nil {
			return fmt.Errorf("lol legacy collection drop: %w", err)
		}
		return markCollectionMigrationDone(ctx, sys, 0)
	}

	cur, err := oldColl.Find(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("lol legacy collection find: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	copied := int64(0)
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			return fmt.Errorf("lol legacy collection decode: %w", err)
		}
		id, ok := doc["_id"]
		if !ok {
			return fmt.Errorf("lol legacy collection document missing _id")
		}
		if _, err := newColl.ReplaceOne(ctx, bson.M{"_id": id}, doc, options.Replace().SetUpsert(true)); err != nil {
			return fmt.Errorf("lol collection copy %v: %w", id, err)
		}
		copied++
	}
	if err := cur.Err(); err != nil {
		return fmt.Errorf("lol legacy collection cursor: %w", err)
	}

	if err := oldColl.Drop(ctx); err != nil {
		return fmt.Errorf("lol legacy collection drop: %w", err)
	}
	return markCollectionMigrationDone(ctx, sys, copied)
}

func markCollectionMigrationDone(ctx context.Context, sys systemstate.Store, count int64) error {
	now := time.Now().UnixMilli()
	if err := sys.Put(ctx, collectionMigrationKey, systemstate.Record{
		Kind:        "migration",
		Name:        collectionMigrationName,
		Status:      "done",
		Count:       count,
		CompletedAt: now,
		UpdatedAt:   now,
	}); err != nil {
		return fmt.Errorf("lol collection migration marker put: %w", err)
	}
	return nil
}
