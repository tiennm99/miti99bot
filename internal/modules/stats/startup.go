package stats

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
)

const (
	statsCommandUsersIndexName   = "stats_cmd_n_user"
	statsUserCommandsIndexName   = "stats_uid_n_cmd"
	statsUsernameLookupIndexName = "stats_user_uid"

	renameLolNextWeekStatsKey = "stats:command-rename:lol_nextweek-to-lol_next_week"
	oldLolNextWeekCommand     = "lol_nextweek"
	newLolNextWeekCommand     = "lol_next_week"

	renameWheelOfNamesBetaStatsKey = "stats:command-rename:wheelofnamesbeta-to-wheelofnames"
	legacyWheelOfNamesBetaCommand  = "wheelofnamesbeta"
	currentWheelOfNamesCommand     = "wheelofnames"
)

// InitStore performs stats collection startup maintenance. It is safe to call
// every boot: MongoDB indexes are created idempotently and one-time command
// rename migrations are guarded by the shared system collection.
func InitStore(ctx context.Context, statsColl, systemColl storage.Collection) error {
	if mongoColl, ok := storage.MongoCollection(statsColl); ok {
		if err := ensureUsageIndexes(ctx, mongoColl); err != nil {
			return err
		}
	}
	if err := migrateCommandRename(ctx, statsColl, systemColl, oldLolNextWeekCommand, newLolNextWeekCommand, renameLolNextWeekStatsKey); err != nil {
		return err
	}
	if err := migrateCommandRename(ctx, statsColl, systemColl, legacyWheelOfNamesBetaCommand, currentWheelOfNamesCommand, renameWheelOfNamesBetaStatsKey); err != nil {
		return err
	}
	return nil
}

func migrateCommandRename(ctx context.Context, statsColl, systemColl storage.Collection, oldCmd, newCmd, markerKey string) error {
	state := systemstate.New(systemColl)
	if rec, ok, err := state.Get(ctx, markerKey); err != nil {
		return fmt.Errorf("stats command rename marker %s: %w", markerKey, err)
	} else if ok && rec.Status == "complete" {
		return nil
	}

	docs := storage.Typed[usageEntry](statsColl)
	keys, err := docs.List(ctx, oldCmd)
	if err != nil {
		return fmt.Errorf("stats command rename list %s: %w", oldCmd, err)
	}

	var moved int64
	for _, key := range keys {
		if key != oldCmd && !strings.HasPrefix(key, oldCmd+":") {
			continue
		}
		entry, _, err := docs.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("stats command rename get %s: %w", key, err)
		}
		if entry.Cmd != oldCmd {
			continue
		}

		targetKey := usageKey(newCmd, entry.UserID)
		target, _, err := docs.Get(ctx, targetKey)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("stats command rename get target %s: %w", targetKey, err)
		}
		if errors.Is(err, storage.ErrNotFound) {
			target = usageEntry{}
		}
		if slices.Contains(target.MergedFrom, key) {
			if err := docs.Delete(ctx, key); err != nil {
				return fmt.Errorf("stats command rename delete already-merged old %s: %w", key, err)
			}
			continue
		}

		target.Cmd = newCmd
		target.UserID = entry.UserID
		if entry.UserID == 0 {
			target.Username = ""
		} else if entry.Username != "" {
			target.Username = entry.Username
		}
		target.N += entry.N
		target.Deleted = false
		target.MergedFrom = append(target.MergedFrom, key)
		if err := docs.Put(ctx, targetKey, target); err != nil {
			return fmt.Errorf("stats command rename put target %s: %w", targetKey, err)
		}
		if err := docs.Delete(ctx, key); err != nil {
			return fmt.Errorf("stats command rename delete old %s: %w", key, err)
		}
		moved += entry.N
	}

	if err := clearCommandMergeMarkers(ctx, docs, newCmd); err != nil {
		return err
	}

	now := time.Now().UTC().UnixMilli()
	if err := state.Put(ctx, markerKey, systemstate.Record{
		Kind:        "migration",
		Name:        markerKey,
		Status:      "complete",
		Count:       moved,
		CompletedAt: now,
		UpdatedAt:   now,
	}); err != nil {
		return fmt.Errorf("stats command rename marker put %s: %w", markerKey, err)
	}
	return nil
}

func clearCommandMergeMarkers(ctx context.Context, docs storage.DocStore[usageEntry], cmd string) error {
	keys, err := docs.List(ctx, cmd)
	if err != nil {
		return fmt.Errorf("stats command rename cleanup list %s: %w", cmd, err)
	}
	for _, key := range keys {
		if key != cmd && !strings.HasPrefix(key, cmd+":") {
			continue
		}
		entry, _, err := docs.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("stats command rename cleanup get %s: %w", key, err)
		}
		if entry.Cmd != cmd || len(entry.MergedFrom) == 0 {
			continue
		}
		entry.MergedFrom = nil
		if err := docs.Put(ctx, key, entry); err != nil {
			return fmt.Errorf("stats command rename cleanup put %s: %w", key, err)
		}
	}
	return nil
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
