package stats

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
)

const (
	statsCommandUsersIndexName     = "stats_cmd_n_user"
	statsUserCommandsIndexName     = "stats_uid_n_cmd"
	statsUsernameLookupIndexName   = "stats_user_uid"
	deletedStockDividendMarkerKey  = "migration:stats-delete-stock-dividend-v1"
	deletedStockDividendCommand    = "stock_dividend"
	deletedCommandMigrationRetries = 5
)

// InitStore performs stats collection startup maintenance. MongoDB index
// creation and the legacy-command migration are both safe to run every boot.
func InitStore(ctx context.Context, statsColl, systemColl storage.Collection) error {
	if mongoColl, ok := storage.MongoCollection(statsColl); ok {
		if err := ensureUsageIndexes(ctx, mongoColl); err != nil {
			return err
		}
	}
	return markDeletedCommand(ctx, statsColl, systemColl, deletedStockDividendCommand, deletedStockDividendMarkerKey)
}

func markDeletedCommand(ctx context.Context, statsColl, systemColl storage.Collection, command, markerKey string) error {
	system := systemstate.New(systemColl)
	marker, exists, err := system.Get(ctx, markerKey)
	if err != nil {
		return fmt.Errorf("stats deleted-command migration: read marker: %w", err)
	}
	var matched int64
	if mongoColl, ok := storage.MongoCollection(statsColl); ok {
		matched, err = markMongoUsageEntriesDeleted(ctx, mongoColl, command)
	} else {
		matched, err = markDocUsageEntriesDeleted(ctx, storage.Typed[usageEntry](statsColl), command)
	}
	if err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	if !exists {
		marker = systemstate.Record{Kind: "migration", Name: "stats delete " + command + " v1"}
	}
	if marker.CompletedAt == 0 {
		marker.CompletedAt = now
	}
	marker.Status = "completed"
	marker.Count = matched
	marker.UpdatedAt = now
	if err := system.Put(ctx, markerKey, marker); err != nil {
		return fmt.Errorf("stats deleted-command migration: write marker: %w", err)
	}
	return nil
}

func markMongoUsageEntriesDeleted(ctx context.Context, coll *mongo.Collection, command string) (int64, error) {
	filter := bson.M{"cmd": command}
	if _, err := coll.UpdateMany(ctx,
		bson.M{"cmd": command, "deleted": bson.M{"$ne": true}},
		bson.M{
			"$set":         bson.M{"deleted": true},
			"$inc":         bson.M{"version": int64(1)},
			"$currentDate": bson.M{"updatedAt": true},
		},
	); err != nil {
		return 0, fmt.Errorf("stats deleted-command migration: update %s: %w", command, err)
	}
	count, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("stats deleted-command migration: count %s: %w", command, err)
	}
	return count, nil
}

func markDocUsageEntriesDeleted(ctx context.Context, docs storage.DocStore[usageEntry], command string) (int64, error) {
	keys, err := docs.List(ctx, command)
	if err != nil {
		return 0, fmt.Errorf("stats deleted-command migration: list %s: %w", command, err)
	}
	var matched int64
	for _, key := range keys {
		isMatch, err := markUsageEntryDeleted(ctx, docs, key, command)
		if err != nil {
			return 0, err
		}
		if isMatch {
			matched++
		}
	}
	return matched, nil
}

func markUsageEntryDeleted(ctx context.Context, docs storage.DocStore[usageEntry], key, command string) (bool, error) {
	for attempt := 0; attempt < deletedCommandMigrationRetries; attempt++ {
		entry, version, err := docs.Get(ctx, key)
		if errors.Is(err, storage.ErrNotFound) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("stats deleted-command migration: read %s: %w", key, err)
		}
		if entry.Cmd != command {
			return false, nil
		}
		if entry.Deleted {
			return true, nil
		}
		entry.Deleted = true
		if err := docs.PutVersioned(ctx, key, version, entry); err == nil {
			return true, nil
		} else if !errors.Is(err, storage.ErrConflict) {
			return false, fmt.Errorf("stats deleted-command migration: write %s: %w", key, err)
		}
	}
	return false, fmt.Errorf("stats deleted-command migration: write %s: %w", key, storage.ErrConflict)
}

func ensureUsageIndexes(ctx context.Context, coll *mongo.Collection) error {
	models := []mongo.IndexModel{
		{
			Keys:    bson.D{bsonField("cmd", 1), bsonField("n", -1), bsonField("user", 1)},
			Options: options.Index().SetName(statsCommandUsersIndexName),
		},
		{
			Keys:    bson.D{bsonField("uid", 1), bsonField("n", -1), bsonField("cmd", 1)},
			Options: options.Index().SetName(statsUserCommandsIndexName),
		},
		{
			Keys:    bson.D{bsonField("user", 1), bsonField("uid", 1)},
			Options: options.Index().SetName(statsUsernameLookupIndexName),
		},
	}
	if _, err := coll.Indexes().CreateMany(ctx, models); err != nil {
		return fmt.Errorf("stats indexes: %w", err)
	}
	return nil
}
