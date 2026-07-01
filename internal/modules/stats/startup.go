package stats

import (
	"context"
	"errors"
	"fmt"
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
	legacyCountPrefix = "count:"
	legacyUserPrefix  = "user:"
	legacyPairPrefix  = "pair:"

	usageMigrationName = "stats-usage-v2"
	usageMigrationKey  = "migration:" + usageMigrationName

	commandHistoryMigrationName = "stats-command-history-v2"
	commandHistoryMigrationKey  = "migration:" + commandHistoryMigrationName
)

type commandRename struct {
	Old string
	New string
}

var commandRenames = []commandRename{
	{Old: "trade_topup", New: "stock_topup"},
	{Old: "trade_buy", New: "stock_buy"},
	{Old: "trade_sell", New: "stock_sell"},
	{Old: "trade_income_stock", New: "stock_bonus"},
	{Old: "trade_income_vnd", New: "stock_dividend"},
	{Old: "trade_stats", New: "stock_portfolio"},
	{Old: "lolschedule", New: "lol"},
	{Old: "lolschedule_week", New: "lol_this_week"},
	{Old: "lolschedule_subscribe", New: "lol_subscribe"},
	{Old: "lolschedule_unsubscribe", New: "lol_unsubscribe"},
	{Old: "wc_week", New: "wc_this_week"},
	{Old: "gold_stats", New: "gold_portfolio"},
	{Old: "coin_stats", New: "coin_portfolio"},
	{Old: "stock_stats", New: "stock_portfolio"},
	{Old: "stock_income_stock", New: "stock_bonus"},
	{Old: "stock_income_vnd", New: "stock_dividend"},
}

var deletedCommandNames = []string{
	"lolschedule_today",
	"trade_income_events",
	"trade_convert",
	"stock_income_events",
	"wc_today",
	"stock_convert",
}

type legacyCountEntry struct {
	N int64 `json:"n" bson:"n"`
}

type legacyUserEntry struct {
	Username string `json:"username" bson:"username"`
	N        int64  `json:"n" bson:"n"`
}

// InitStore performs stats collection startup maintenance. It is safe to call
// every boot: indexes are idempotent and legacy migration is guarded by a
// system collection marker.
func InitStore(ctx context.Context, statsColl, systemColl storage.Collection) error {
	if mongoColl, ok := storage.MongoCollection(statsColl); ok {
		if err := ensureUsageIndexes(ctx, mongoColl); err != nil {
			return err
		}
	}
	if err := migrateLegacyUsage(ctx, statsColl, systemColl); err != nil {
		return err
	}
	return migrateCommandHistory(ctx, statsColl, systemColl)
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

func migrateLegacyUsage(ctx context.Context, statsColl, systemColl storage.Collection) error {
	sys := systemstate.New(systemColl)
	if rec, ok, err := sys.Get(ctx, usageMigrationKey); err != nil {
		return fmt.Errorf("stats legacy migration marker get: %w", err)
	} else if ok && rec.Status == "done" {
		return nil
	}

	legacyCounts := storage.Typed[legacyCountEntry](statsColl)
	legacyUsers := storage.Typed[legacyUserEntry](statsColl)
	usageDocs := storage.Typed[usageEntry](statsColl)

	usernames, userKeys, err := loadLegacyUsernames(ctx, legacyUsers)
	if err != nil {
		return err
	}
	countTotals, countKeys, err := loadLegacyCounts(ctx, legacyCounts)
	if err != nil {
		return err
	}
	pairs, pairKeys, pairSums, err := loadLegacyPairs(ctx, legacyCounts, usernames)
	if err != nil {
		return err
	}

	written := int64(0)
	for _, entry := range pairs {
		if err := usageDocs.Put(ctx, usageKey(entry.Cmd, entry.UserID), entry); err != nil {
			return fmt.Errorf("stats legacy pair put %s:%d: %w", entry.Cmd, entry.UserID, err)
		}
		written++
	}

	for cmd, total := range countTotals {
		anonymous := total - pairSums[cmd]
		if anonymous <= 0 {
			continue
		}
		if err := usageDocs.Put(ctx, usageKey(cmd, 0), usageEntry{Cmd: cmd, N: anonymous}); err != nil {
			return fmt.Errorf("stats legacy anonymous put %s: %w", cmd, err)
		}
		written++
	}

	for _, key := range append(append(countKeys, pairKeys...), userKeys...) {
		if err := legacyCounts.Delete(ctx, key); err != nil {
			return fmt.Errorf("stats legacy delete %s: %w", key, err)
		}
	}

	now := nowMillis()
	if err := sys.Put(ctx, usageMigrationKey, systemstate.Record{
		Kind:        "migration",
		Name:        usageMigrationName,
		Status:      "done",
		Count:       written,
		CompletedAt: now,
		UpdatedAt:   now,
	}); err != nil {
		return fmt.Errorf("stats legacy migration marker put: %w", err)
	}
	return nil
}

func migrateCommandHistory(ctx context.Context, statsColl, systemColl storage.Collection) error {
	sys := systemstate.New(systemColl)
	if rec, ok, err := sys.Get(ctx, commandHistoryMigrationKey); err != nil {
		return fmt.Errorf("stats command history migration marker get: %w", err)
	} else if ok && rec.Status == "done" {
		return nil
	}

	docs := storage.Typed[usageEntry](statsColl)
	changed := int64(0)
	for _, rename := range commandRenames {
		n, err := migrateCommandRename(ctx, docs, rename)
		if err != nil {
			return err
		}
		changed += n
	}
	for _, cmd := range deletedCommandNames {
		n, err := markCommandDeleted(ctx, docs, cmd)
		if err != nil {
			return err
		}
		changed += n
	}

	now := nowMillis()
	if err := sys.Put(ctx, commandHistoryMigrationKey, systemstate.Record{
		Kind:        "migration",
		Name:        commandHistoryMigrationName,
		Status:      "done",
		Count:       changed,
		CompletedAt: now,
		UpdatedAt:   now,
	}); err != nil {
		return fmt.Errorf("stats command history migration marker put: %w", err)
	}
	return nil
}

func migrateCommandRename(ctx context.Context, docs storage.DocStore[usageEntry], rename commandRename) (int64, error) {
	keys, err := docs.List(ctx, rename.Old)
	if err != nil {
		return 0, fmt.Errorf("stats command rename list %s: %w", rename.Old, err)
	}
	changed := int64(0)
	for _, key := range keys {
		if !usageKeyBelongsToCommand(key, rename.Old) {
			continue
		}
		entry, _, err := docs.Get(ctx, key)
		if err != nil {
			return changed, fmt.Errorf("stats command rename get %s: %w", key, err)
		}
		if entry.Cmd != "" && entry.Cmd != rename.Old {
			continue
		}
		entry.Cmd = rename.Old
		targetKey := usageKey(rename.New, entry.UserID)
		target, _, err := docs.Get(ctx, targetKey)
		switch {
		case errors.Is(err, storage.ErrNotFound):
			target = usageEntry{
				Cmd:      rename.New,
				UserID:   entry.UserID,
				Username: entry.Username,
			}
		case err != nil:
			return changed, fmt.Errorf("stats command rename target get %s: %w", targetKey, err)
		}

		target.Cmd = rename.New
		target.UserID = entry.UserID
		if entry.UserID == 0 {
			target.Username = ""
		} else if entry.Username != "" {
			target.Username = entry.Username
		}
		target.N += entry.N
		target.Deleted = false
		if err := docs.Put(ctx, targetKey, target); err != nil {
			return changed, fmt.Errorf("stats command rename put %s: %w", targetKey, err)
		}
		if err := docs.Delete(ctx, key); err != nil {
			return changed, fmt.Errorf("stats command rename delete %s: %w", key, err)
		}
		changed++
	}
	return changed, nil
}

func markCommandDeleted(ctx context.Context, docs storage.DocStore[usageEntry], cmd string) (int64, error) {
	keys, err := docs.List(ctx, cmd)
	if err != nil {
		return 0, fmt.Errorf("stats command delete list %s: %w", cmd, err)
	}
	changed := int64(0)
	for _, key := range keys {
		if !usageKeyBelongsToCommand(key, cmd) {
			continue
		}
		entry, _, err := docs.Get(ctx, key)
		if err != nil {
			return changed, fmt.Errorf("stats command delete get %s: %w", key, err)
		}
		if entry.Cmd != "" && entry.Cmd != cmd {
			continue
		}
		entry.Cmd = cmd
		if entry.Deleted {
			continue
		}
		entry.Deleted = true
		if err := docs.Put(ctx, key, entry); err != nil {
			return changed, fmt.Errorf("stats command delete put %s: %w", key, err)
		}
		changed++
	}
	return changed, nil
}

func usageKeyBelongsToCommand(key, cmd string) bool {
	return key == cmd || strings.HasPrefix(key, cmd+":")
}

func loadLegacyUsernames(ctx context.Context, users storage.DocStore[legacyUserEntry]) (map[int64]string, []string, error) {
	keys, err := users.List(ctx, legacyUserPrefix)
	if err != nil {
		return nil, nil, fmt.Errorf("stats legacy users list: %w", err)
	}
	usernames := make(map[int64]string, len(keys))
	for _, key := range keys {
		id, err := strconv.ParseInt(strings.TrimPrefix(key, legacyUserPrefix), 10, 64)
		if err != nil {
			continue
		}
		entry, _, err := users.Get(ctx, key)
		if err != nil {
			return nil, nil, fmt.Errorf("stats legacy user get %s: %w", key, err)
		}
		usernames[id] = entry.Username
	}
	return usernames, keys, nil
}

func loadLegacyCounts(ctx context.Context, counts storage.DocStore[legacyCountEntry]) (map[string]int64, []string, error) {
	keys, err := counts.List(ctx, legacyCountPrefix)
	if err != nil {
		return nil, nil, fmt.Errorf("stats legacy counts list: %w", err)
	}
	totals := make(map[string]int64, len(keys))
	for _, key := range keys {
		cmd := strings.TrimPrefix(key, legacyCountPrefix)
		if cmd == "" {
			continue
		}
		entry, _, err := counts.Get(ctx, key)
		if err != nil {
			return nil, nil, fmt.Errorf("stats legacy count get %s: %w", key, err)
		}
		totals[cmd] = entry.N
	}
	return totals, keys, nil
}

func loadLegacyPairs(ctx context.Context, counts storage.DocStore[legacyCountEntry], usernames map[int64]string) ([]usageEntry, []string, map[string]int64, error) {
	keys, err := counts.List(ctx, legacyPairPrefix)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("stats legacy pairs list: %w", err)
	}
	pairs := make([]usageEntry, 0, len(keys))
	pairSums := make(map[string]int64)
	for _, key := range keys {
		cmd, userID, ok := parseLegacyPairKey(key)
		if !ok {
			continue
		}
		entry, _, err := counts.Get(ctx, key)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("stats legacy pair get %s: %w", key, err)
		}
		pairs = append(pairs, usageEntry{
			Cmd:      cmd,
			UserID:   userID,
			Username: usernames[userID],
			N:        entry.N,
		})
		pairSums[cmd] += entry.N
	}
	return pairs, keys, pairSums, nil
}

func parseLegacyPairKey(key string) (string, int64, bool) {
	rest := strings.TrimPrefix(key, legacyPairPrefix)
	idx := strings.LastIndexByte(rest, ':')
	if idx <= 0 || idx == len(rest)-1 {
		return "", 0, false
	}
	id, err := strconv.ParseInt(rest[idx+1:], 10, 64)
	if err != nil {
		return "", 0, false
	}
	return rest[:idx], id, true
}

func nowMillis() int64 {
	return time.Now().UnixMilli()
}
