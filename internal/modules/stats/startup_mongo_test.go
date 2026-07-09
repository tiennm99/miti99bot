package stats

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
)

func TestInitStore_MongoCreatesIndexes(t *testing.T) {
	ctx, statsColl, systemColl := setupMongoStatsTest(t)

	rawStatsColl, ok := storage.MongoCollection(statsColl)
	if !ok {
		t.Fatal("stats collection is not Mongo-backed")
	}

	if err := InitStore(ctx, statsColl, systemColl); err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if err := InitStore(ctx, statsColl, systemColl); err != nil {
		t.Fatalf("InitStore second run: %v", err)
	}

	cur, err := rawStatsColl.Indexes().List(ctx)
	if err != nil {
		t.Fatalf("list indexes: %v", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	found := map[string]bool{}
	for cur.Next(ctx) {
		var doc struct {
			Name string `bson:"name"`
		}
		if err := cur.Decode(&doc); err != nil {
			t.Fatalf("decode index: %v", err)
		}
		found[doc.Name] = true
	}
	if err := cur.Err(); err != nil {
		t.Fatalf("index cursor: %v", err)
	}
	for _, name := range []string{statsCommandUsersIndexName, statsUserCommandsIndexName, statsUsernameLookupIndexName} {
		if !found[name] {
			t.Fatalf("missing index %s; indexes=%v", name, found)
		}
	}
}

func TestInitStore_MongoMergesLegacyWheelOfNamesBetaStats(t *testing.T) {
	ctx, statsColl, systemColl := setupMongoStatsTest(t)
	docs := storage.Typed[usageEntry](statsColl)

	seeds := map[string]usageEntry{
		usageKey(legacyWheelOfNamesBetaCommand, 0): {Cmd: legacyWheelOfNamesBetaCommand, N: 2},
		usageKey(legacyWheelOfNamesBetaCommand, 7): {
			Cmd:      legacyWheelOfNamesBetaCommand,
			UserID:   7,
			Username: "alice",
			N:        3,
			Deleted:  true,
		},
		usageKey(currentWheelOfNamesCommand, 7): {
			Cmd:      currentWheelOfNamesCommand,
			UserID:   7,
			Username: "alice",
			N:        5,
		},
	}
	for key, entry := range seeds {
		if err := docs.Put(ctx, key, entry); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}

	if err := InitStore(ctx, statsColl, systemColl); err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if err := InitStore(ctx, statsColl, systemColl); err != nil {
		t.Fatalf("InitStore second run: %v", err)
	}

	anon, _, err := docs.Get(ctx, usageKey(currentWheelOfNamesCommand, 0))
	if err != nil {
		t.Fatalf("merged anonymous wheelofnames stats: %v", err)
	}
	if anon.N != 2 || anon.Deleted || len(anon.MergedFrom) != 0 {
		t.Fatalf("merged anonymous stats = %+v, want visible count 2", anon)
	}
	alice, _, err := docs.Get(ctx, usageKey(currentWheelOfNamesCommand, 7))
	if err != nil {
		t.Fatalf("merged alice wheelofnames stats: %v", err)
	}
	if alice.N != 8 || alice.Deleted || len(alice.MergedFrom) != 0 {
		t.Fatalf("merged alice stats = %+v, want visible count 8", alice)
	}
	for _, key := range []string{usageKey(legacyWheelOfNamesBetaCommand, 0), usageKey(legacyWheelOfNamesBetaCommand, 7)} {
		if _, _, err := docs.Get(ctx, key); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("legacy key %s err = %v, want ErrNotFound", key, err)
		}
	}
	if got := renderStats(ctx, newCounter(statsColl), ""); !strings.Contains(got, "/wheelofnames: 10") || strings.Contains(got, "wheelofnamesbeta") {
		t.Fatalf("top commands after Mongo migration = %q, want merged visible /wheelofnames count 10 only", got)
	}
}

func setupMongoStatsTest(t *testing.T) (context.Context, storage.Collection, storage.Collection) {
	t.Helper()

	uri := os.Getenv("MONGODB_TEST_URL")
	if uri == "" {
		t.Skip("MONGODB_TEST_URL not set; skipping MongoDB integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	client, err := storage.NewMongoClient(ctx, uri)
	if err != nil {
		t.Fatalf("NewMongoClient: %v", err)
	}
	dbName := fmt.Sprintf("miti99bot_stats_test_%d", time.Now().UnixNano())
	db := client.Database(dbName)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = db.Drop(cleanupCtx)
		_ = client.Disconnect(cleanupCtx)
	})

	provider := storage.NewMongoProvider(db)
	return ctx, provider.Collection("stats"), provider.Collection(systemstate.CollectionName)
}
