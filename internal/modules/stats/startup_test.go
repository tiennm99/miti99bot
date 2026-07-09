package stats

import (
	"context"
	"errors"
	"testing"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
)

func TestInitStore_RenamesLolNextWeekStatsOnce(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	statsColl := provider.Collection("stats")
	systemColl := provider.Collection(systemstate.CollectionName)
	docs := storage.Typed[usageEntry](statsColl)

	seeds := map[string]usageEntry{
		usageKey(oldLolNextWeekCommand, 0): {Cmd: oldLolNextWeekCommand, N: 2},
		usageKey(oldLolNextWeekCommand, 7): {
			Cmd:      oldLolNextWeekCommand,
			UserID:   7,
			Username: "alice",
			N:        3,
		},
		usageKey(newLolNextWeekCommand, 7): {
			Cmd:      newLolNextWeekCommand,
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

	anon, _, err := docs.Get(ctx, usageKey(newLolNextWeekCommand, 0))
	if err != nil {
		t.Fatalf("new anonymous stats: %v", err)
	}
	if anon.Cmd != newLolNextWeekCommand || anon.N != 2 || anon.UserID != 0 {
		t.Fatalf("new anonymous stats = %+v, want cmd %q count 2", anon, newLolNextWeekCommand)
	}

	user, _, err := docs.Get(ctx, usageKey(newLolNextWeekCommand, 7))
	if err != nil {
		t.Fatalf("new user stats: %v", err)
	}
	if user.Cmd != newLolNextWeekCommand || user.UserID != 7 || user.Username != "alice" || user.N != 8 {
		t.Fatalf("new user stats = %+v, want merged count 8", user)
	}

	for _, key := range []string{usageKey(oldLolNextWeekCommand, 0), usageKey(oldLolNextWeekCommand, 7)} {
		if _, _, err := docs.Get(ctx, key); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("old key %s err = %v, want ErrNotFound", key, err)
		}
	}

	rec, ok, err := systemstate.New(systemColl).Get(ctx, renameLolNextWeekStatsKey)
	if err != nil {
		t.Fatalf("migration marker: %v", err)
	}
	if !ok || rec.Status != "complete" || rec.Count != 5 {
		t.Fatalf("migration marker = %+v ok=%v, want complete count 5", rec, ok)
	}
}

func TestInitStore_MarksLegacyWheelOfNamesStatsDeletedOnce(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	statsColl := provider.Collection("stats")
	systemColl := provider.Collection(systemstate.CollectionName)
	docs := storage.Typed[usageEntry](statsColl)

	seeds := map[string]usageEntry{
		usageKey(deletedLegacyWheelOfNamesCommand, 0): {Cmd: deletedLegacyWheelOfNamesCommand, N: 2},
		usageKey(deletedLegacyWheelOfNamesCommand, 7): {
			Cmd:      deletedLegacyWheelOfNamesCommand,
			UserID:   7,
			Username: "alice",
			N:        3,
		},
		usageKey("wheelofnames", 7): {
			Cmd:      "wheelofnames",
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

	for _, key := range []string{usageKey(deletedLegacyWheelOfNamesCommand, 0), usageKey(deletedLegacyWheelOfNamesCommand, 7)} {
		entry, _, err := docs.Get(ctx, key)
		if err != nil {
			t.Fatalf("legacy stats %s: %v", key, err)
		}
		if !entry.Deleted {
			t.Fatalf("legacy stats %s = %+v, want deleted", key, entry)
		}
	}

	active, _, err := docs.Get(ctx, usageKey("wheelofnames", 7))
	if err != nil {
		t.Fatalf("active wheelofnames stats: %v", err)
	}
	if active.Deleted {
		t.Fatalf("active wheelofnames stats = %+v, want not deleted", active)
	}

	rec, ok, err := systemstate.New(systemColl).Get(ctx, deleteLegacyWheelOfNamesStatsKey)
	if err != nil {
		t.Fatalf("migration marker: %v", err)
	}
	if !ok || rec.Status != "complete" || rec.Count != 5 {
		t.Fatalf("migration marker = %+v ok=%v, want complete count 5", rec, ok)
	}
}
