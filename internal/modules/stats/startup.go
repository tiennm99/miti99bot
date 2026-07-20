package stats

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
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

	dividendStatsMigrationKey    = "migration:stock-dividend-command-stats-v1"
	dividendStatsRowMarkerPrefix = dividendStatsMigrationKey + ":row:"
	migrationStatusPrepared      = "prepared"
	migrationStatusCompleted     = "completed"
)

type commandRename struct {
	from string
	to   string
}

var dividendCommandRenames = []commandRename{
	{from: "stock_dividend", to: "stock_cash_dividend"},
	{from: "stock_bonus", to: "stock_share_dividend"},
}

// InitStore performs stats collection startup maintenance. It is safe to call
// every boot: MongoDB indexes and the guarded stats migration are idempotent.
func InitStore(ctx context.Context, statsColl, systemColl storage.Collection) error {
	if mongoColl, ok := storage.MongoCollection(statsColl); ok {
		if err := ensureUsageIndexes(ctx, mongoColl); err != nil {
			return err
		}
	}
	if err := migrateDividendCommandStats(
		ctx,
		storage.Typed[usageEntry](statsColl),
		storage.Typed[systemstate.Record](systemColl),
	); err != nil {
		return fmt.Errorf("stats dividend command migration: %w", err)
	}
	return nil
}

func migrateDividendCommandStats(
	ctx context.Context,
	statsDocs storage.DocStore[usageEntry],
	systemDocs storage.DocStore[systemstate.Record],
) error {
	global, exists, err := getSystemRecord(ctx, systemDocs, dividendStatsMigrationKey)
	if err != nil {
		return fmt.Errorf("read global marker: %w", err)
	}
	if exists && global.Status == migrationStatusCompleted {
		return nil
	}

	for _, rename := range dividendCommandRenames {
		keys, err := statsDocs.List(ctx, rename.from)
		if err != nil {
			return fmt.Errorf("list %s rows: %w", rename.from, err)
		}
		for _, key := range keys {
			userID, ok := usageUserIDForCommandKey(key, rename.from)
			if !ok {
				continue
			}
			if err := migrateDividendStatsRow(ctx, statsDocs, systemDocs, rename, userID); err != nil {
				return err
			}
		}
	}

	markerKeys, err := systemDocs.List(ctx, dividendStatsRowMarkerPrefix)
	if err != nil {
		return fmt.Errorf("list row checkpoints: %w", err)
	}
	for _, markerKey := range markerKeys {
		rename, userID, ok := parseDividendStatsRowMarkerKey(markerKey)
		if !ok {
			continue
		}
		marker, exists, err := getSystemRecord(ctx, systemDocs, markerKey)
		if err != nil {
			return fmt.Errorf("read row checkpoint %s: %w", markerKey, err)
		}
		if !exists || marker.Status != migrationStatusPrepared {
			continue
		}
		if err := migrateDividendStatsRow(ctx, statsDocs, systemDocs, rename, userID); err != nil {
			return err
		}
	}

	now := time.Now().UnixMilli()
	return systemDocs.Put(ctx, dividendStatsMigrationKey, systemstate.Record{
		Kind:        "migration",
		Name:        "stock dividend command stats v1",
		Status:      migrationStatusCompleted,
		CompletedAt: now,
		UpdatedAt:   now,
	})
}

func migrateDividendStatsRow(
	ctx context.Context,
	statsDocs storage.DocStore[usageEntry],
	systemDocs storage.DocStore[systemstate.Record],
	rename commandRename,
	userID int64,
) error {
	markerKey := dividendStatsRowMarkerKey(rename.from, userID)
	marker, markerExists, err := getSystemRecord(ctx, systemDocs, markerKey)
	if err != nil {
		return fmt.Errorf("read %s checkpoint for user %d: %w", rename.from, userID, err)
	}
	if markerExists && marker.Status == migrationStatusCompleted {
		return nil
	}

	sourceKey := usageKey(rename.from, userID)
	targetKey := usageKey(rename.to, userID)
	source, sourceExists, err := getUsageEntry(ctx, statsDocs, sourceKey)
	if err != nil {
		return fmt.Errorf("read source %s: %w", sourceKey, err)
	}
	target, targetExists, err := getUsageEntry(ctx, statsDocs, targetKey)
	if err != nil {
		return fmt.Errorf("read target %s: %w", targetKey, err)
	}

	if !markerExists {
		if !sourceExists {
			return nil
		}
		if source.N < 0 || target.N < 0 || source.N > math.MaxInt64-target.N {
			return fmt.Errorf("count overflow for %s user %d", rename.from, userID)
		}
		now := time.Now().UnixMilli()
		marker = systemstate.Record{
			Kind:      "migration-row",
			Name:      rename.from + " -> " + rename.to,
			Status:    migrationStatusPrepared,
			Count:     target.N + source.N,
			UpdatedAt: now,
		}
		if err := systemDocs.Put(ctx, markerKey, marker); err != nil {
			return fmt.Errorf("prepare %s user %d: %w", rename.from, userID, err)
		}
	}

	if !targetExists {
		target = usageEntry{UserID: userID}
	}
	target.Cmd = rename.to
	target.UserID = userID
	target.N = marker.Count
	target.Deleted = false
	if sourceExists && source.Username != "" {
		target.Username = source.Username
	}
	if userID == 0 {
		target.Username = ""
	}
	if err := statsDocs.Put(ctx, targetKey, target); err != nil {
		return fmt.Errorf("write target %s: %w", targetKey, err)
	}
	if sourceExists {
		if err := statsDocs.Delete(ctx, sourceKey); err != nil {
			return fmt.Errorf("delete source %s: %w", sourceKey, err)
		}
	}

	now := time.Now().UnixMilli()
	marker.Status = migrationStatusCompleted
	marker.CompletedAt = now
	marker.UpdatedAt = now
	if err := systemDocs.Put(ctx, markerKey, marker); err != nil {
		return fmt.Errorf("complete %s user %d: %w", rename.from, userID, err)
	}
	return nil
}

func getUsageEntry(ctx context.Context, docs storage.DocStore[usageEntry], key string) (usageEntry, bool, error) {
	entry, _, err := docs.Get(ctx, key)
	if errors.Is(err, storage.ErrNotFound) {
		return usageEntry{}, false, nil
	}
	return entry, err == nil, err
}

func getSystemRecord(ctx context.Context, docs storage.DocStore[systemstate.Record], key string) (systemstate.Record, bool, error) {
	record, _, err := docs.Get(ctx, key)
	if errors.Is(err, storage.ErrNotFound) {
		return systemstate.Record{}, false, nil
	}
	return record, err == nil, err
}

func usageUserIDForCommandKey(key, command string) (int64, bool) {
	if key == command {
		return 0, true
	}
	prefix := command + ":"
	if !strings.HasPrefix(key, prefix) {
		return 0, false
	}
	userID, err := strconv.ParseInt(strings.TrimPrefix(key, prefix), 10, 64)
	return userID, err == nil && userID != 0
}

func dividendStatsRowMarkerKey(sourceCommand string, userID int64) string {
	return dividendStatsRowMarkerPrefix + sourceCommand + ":" + strconv.FormatInt(userID, 10)
}

func parseDividendStatsRowMarkerKey(key string) (commandRename, int64, bool) {
	remainder := strings.TrimPrefix(key, dividendStatsRowMarkerPrefix)
	if remainder == key {
		return commandRename{}, 0, false
	}
	separator := strings.LastIndexByte(remainder, ':')
	if separator < 1 {
		return commandRename{}, 0, false
	}
	sourceCommand := remainder[:separator]
	userID, err := strconv.ParseInt(remainder[separator+1:], 10, 64)
	if err != nil {
		return commandRename{}, 0, false
	}
	for _, rename := range dividendCommandRenames {
		if rename.from == sourceCommand {
			return rename, userID, true
		}
	}
	return commandRename{}, 0, false
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
