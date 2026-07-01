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

const miscCommandRenameMigrationName = "stats-command-renames-misc-20260701"

type commandRename struct {
	old string
	new string
}

var miscCommandRenames = []commandRename{
	{old: "mstats", new: "ping_stats"},
	{old: "fortytwo", new: "the_answer"},
}

// InitStore performs stats collection startup maintenance. It is safe to call
// every boot: MongoDB indexes are created idempotently and one-time migrations
// are guarded by the shared system collection.
func InitStore(ctx context.Context, statsColl, systemColl storage.Collection) error {
	if mongoColl, ok := storage.MongoCollection(statsColl); ok {
		if err := ensureUsageIndexes(ctx, mongoColl); err != nil {
			return err
		}
	}
	if err := runMiscCommandRenameMigration(ctx, statsColl, systemColl); err != nil {
		return err
	}
	return nil
}

func runMiscCommandRenameMigration(ctx context.Context, statsColl, systemColl storage.Collection) error {
	sys := systemstate.New(systemColl)
	key := "migration:" + miscCommandRenameMigrationName
	if rec, ok, err := sys.Get(ctx, key); err != nil {
		return fmt.Errorf("stats migration state %s: %w", miscCommandRenameMigrationName, err)
	} else if ok && rec.Status == "complete" {
		return nil
	}

	count, err := migrateCommandStats(ctx, storage.Typed[usageEntry](statsColl), miscCommandRenames)
	if err != nil {
		return err
	}

	now := time.Now().UTC().UnixMilli()
	if err := sys.Put(ctx, key, systemstate.Record{
		Kind:        "migration",
		Name:        miscCommandRenameMigrationName,
		Status:      "complete",
		Count:       count,
		CompletedAt: now,
		UpdatedAt:   now,
	}); err != nil {
		return fmt.Errorf("stats migration mark complete %s: %w", miscCommandRenameMigrationName, err)
	}
	return nil
}

func migrateCommandStats(ctx context.Context, docs storage.DocStore[usageEntry], renames []commandRename) (int64, error) {
	renameByOld := make(map[string]string, len(renames))
	for _, r := range renames {
		renameByOld[r.old] = r.new
	}

	keys, err := docs.List(ctx, "")
	if err != nil {
		return 0, fmt.Errorf("stats command rename list: %w", err)
	}

	var moved int64
	for _, key := range keys {
		src, _, err := docs.Get(ctx, key)
		if err != nil {
			return moved, fmt.Errorf("stats command rename get %s: %w", key, err)
		}
		if src.Cmd == "" || src.Deleted {
			continue
		}
		newCmd, ok := renameByOld[src.Cmd]
		if !ok {
			continue
		}

		dstKey := usageKey(newCmd, src.UserID)
		dst, _, err := docs.Get(ctx, dstKey)
		switch {
		case errors.Is(err, storage.ErrNotFound):
			dst = usageEntry{}
		case err != nil:
			return moved, fmt.Errorf("stats command rename get %s: %w", dstKey, err)
		}

		dst.Cmd = newCmd
		dst.N += src.N
		dst.Deleted = false
		if src.UserID != 0 {
			dst.UserID = src.UserID
			if dst.Username == "" {
				dst.Username = src.Username
			}
		} else {
			dst.UserID = 0
			dst.Username = ""
		}
		if err := docs.Put(ctx, dstKey, dst); err != nil {
			return moved, fmt.Errorf("stats command rename put %s: %w", dstKey, err)
		}
		if err := docs.Delete(ctx, key); err != nil {
			return moved, fmt.Errorf("stats command rename delete %s: %w", key, err)
		}
		moved += src.N
	}
	return moved, nil
}

func ensureUsageIndexes(ctx context.Context, coll *mongo.Collection) error {
	models := []mongo.IndexModel{
		{
			Keys:    bson.D{bsonField("cmd", 1), bsonField("n", -1), bsonField("user", 1)},
			Options: options.Index().SetName("stats_cmd_n_user"),
		},
		{
			Keys:    bson.D{bsonField("uid", 1), bsonField("n", -1), bsonField("cmd", 1)},
			Options: options.Index().SetName("stats_uid_n_cmd"),
		},
		{
			Keys:    bson.D{bsonField("user", 1), bsonField("updatedAt", -1)},
			Options: options.Index().SetName("stats_user_updated_at"),
		},
	}
	if _, err := coll.Indexes().CreateMany(ctx, models); err != nil {
		return fmt.Errorf("stats indexes: %w", err)
	}
	return nil
}
