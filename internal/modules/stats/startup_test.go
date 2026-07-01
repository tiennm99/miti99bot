package stats

import (
	"context"
	"errors"
	"testing"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
)

func TestInitStore_RenamesMiscCommandStatsOnce(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	statsColl := provider.Collection("stats")
	systemColl := provider.Collection(systemstate.CollectionName)
	docs := storage.Typed[usageEntry](statsColl)

	seed := map[string]usageEntry{
		usageKey("mstats", 0):       {Cmd: "mstats", N: 2},
		usageKey("mstats", 7):       {Cmd: "mstats", UserID: 7, Username: "alice", N: 3},
		usageKey("ping_stats", 7):   {Cmd: "ping_stats", UserID: 7, Username: "alice", N: 5},
		usageKey("fortytwo", 0):     {Cmd: "fortytwo", N: 1},
		usageKey("the_answer", 0):   {Cmd: "the_answer", N: 4},
		usageKey("fortytwo", 100):   {Cmd: "fortytwo", UserID: 100, Username: "owner", N: 8},
		usageKey("the_answer", 100): {Cmd: "the_answer", UserID: 100, Username: "owner", N: 9},
	}
	for key, entry := range seed {
		if err := docs.Put(ctx, key, entry); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}

	if err := InitStore(ctx, statsColl, systemColl); err != nil {
		t.Fatalf("InitStore first run: %v", err)
	}
	if err := InitStore(ctx, statsColl, systemColl); err != nil {
		t.Fatalf("InitStore second run: %v", err)
	}

	want := map[string]int64{
		usageKey("ping_stats", 0):   2,
		usageKey("ping_stats", 7):   8,
		usageKey("the_answer", 0):   5,
		usageKey("the_answer", 100): 17,
	}
	for key, wantN := range want {
		got, _, err := docs.Get(ctx, key)
		if err != nil {
			t.Fatalf("get %s: %v", key, err)
		}
		if got.N != wantN {
			t.Fatalf("%s N = %d, want %d", key, got.N, wantN)
		}
	}

	for _, key := range []string{
		usageKey("mstats", 0),
		usageKey("mstats", 7),
		usageKey("fortytwo", 0),
		usageKey("fortytwo", 100),
	} {
		if _, _, err := docs.Get(ctx, key); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("old stats key %s still exists: %v", key, err)
		}
	}

	rec, ok, err := systemstate.New(systemColl).Get(ctx, "migration:"+miscCommandRenameMigrationName)
	if err != nil {
		t.Fatalf("migration marker get: %v", err)
	}
	if !ok || rec.Status != "complete" || rec.Count != 14 {
		t.Fatalf("migration marker = %+v, ok=%v; want complete count 14", rec, ok)
	}
}
