package stats

import (
	"context"
	"errors"
	"strings"
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

func TestInitStore_MergesLegacyWheelOfNamesBetaStatsOnce(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	statsColl := provider.Collection("stats")
	systemColl := provider.Collection(systemstate.CollectionName)
	docs := storage.Typed[usageEntry](statsColl)

	seeds := map[string]usageEntry{
		usageKey(legacyWheelOfNamesBetaCommand, 0): {Cmd: legacyWheelOfNamesBetaCommand, N: 2},
		usageKey(legacyWheelOfNamesBetaCommand, 7): {
			Cmd:      legacyWheelOfNamesBetaCommand,
			UserID:   7,
			Username: "alice",
			N:        3,
		},
		usageKey(legacyWheelOfNamesBetaCommand, 8): {
			Cmd:      legacyWheelOfNamesBetaCommand,
			UserID:   8,
			Username: "bob",
			N:        4,
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
	if anon.Cmd != currentWheelOfNamesCommand || anon.N != 2 || anon.UserID != 0 || anon.Deleted || len(anon.MergedFrom) != 0 {
		t.Fatalf("merged anonymous stats = %+v, want visible count 2", anon)
	}

	alice, _, err := docs.Get(ctx, usageKey(currentWheelOfNamesCommand, 7))
	if err != nil {
		t.Fatalf("merged alice wheelofnames stats: %v", err)
	}
	if alice.Cmd != currentWheelOfNamesCommand || alice.UserID != 7 || alice.Username != "alice" || alice.N != 8 || alice.Deleted || len(alice.MergedFrom) != 0 {
		t.Fatalf("merged alice stats = %+v, want visible count 8", alice)
	}

	bob, _, err := docs.Get(ctx, usageKey(currentWheelOfNamesCommand, 8))
	if err != nil {
		t.Fatalf("merged bob wheelofnames stats: %v", err)
	}
	if bob.Cmd != currentWheelOfNamesCommand || bob.UserID != 8 || bob.Username != "bob" || bob.N != 4 || bob.Deleted || len(bob.MergedFrom) != 0 {
		t.Fatalf("merged bob stats = %+v, want visible count 4", bob)
	}

	renderedTopCommands := renderStats(ctx, newCounter(statsColl), "")
	if !strings.Contains(renderedTopCommands, "/wheelofnames: 14") || strings.Contains(renderedTopCommands, "wheelofnamesbeta") {
		t.Fatalf("top commands after migration = %q, want merged visible /wheelofnames count 14 only", renderedTopCommands)
	}
	renderedCommandUsers := renderStats(ctx, newCounter(statsColl), "cmd wheelofnames")
	if !strings.Contains(renderedCommandUsers, "@alice: 8") || !strings.Contains(renderedCommandUsers, "@bob: 4") {
		t.Fatalf("wheelofnames users after migration = %q, want merged users", renderedCommandUsers)
	}

	for _, key := range []string{
		usageKey(legacyWheelOfNamesBetaCommand, 0),
		usageKey(legacyWheelOfNamesBetaCommand, 7),
		usageKey(legacyWheelOfNamesBetaCommand, 8),
	} {
		if _, _, err := docs.Get(ctx, key); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("legacy key %s err = %v, want ErrNotFound", key, err)
		}
	}

	rec, ok, err := systemstate.New(systemColl).Get(ctx, renameWheelOfNamesBetaStatsKey)
	if err != nil {
		t.Fatalf("migration marker: %v", err)
	}
	if !ok || rec.Status != "complete" || rec.Count != 9 {
		t.Fatalf("migration marker = %+v ok=%v, want complete count 9", rec, ok)
	}
}

func TestInitStore_RetriesPartiallyMergedWheelOfNamesBetaStats(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	statsColl := provider.Collection("stats")
	systemColl := provider.Collection(systemstate.CollectionName)
	docs := storage.Typed[usageEntry](statsColl)

	alreadyMergedKey := usageKey(legacyWheelOfNamesBetaCommand, 7)
	seeds := map[string]usageEntry{
		alreadyMergedKey: {
			Cmd:      legacyWheelOfNamesBetaCommand,
			UserID:   7,
			Username: "alice",
			N:        3,
		},
		usageKey(currentWheelOfNamesCommand, 7): {
			Cmd:        currentWheelOfNamesCommand,
			UserID:     7,
			Username:   "alice",
			N:          8,
			MergedFrom: []string{alreadyMergedKey},
		},
		usageKey(legacyWheelOfNamesBetaCommand, 9): {
			Cmd:      legacyWheelOfNamesBetaCommand,
			UserID:   9,
			Username: "carol",
			N:        6,
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

	alice, _, err := docs.Get(ctx, usageKey(currentWheelOfNamesCommand, 7))
	if err != nil {
		t.Fatalf("alice wheelofnames stats: %v", err)
	}
	if alice.N != 8 || len(alice.MergedFrom) != 0 {
		t.Fatalf("alice stats = %+v, want count still 8 with cleanup marker removed", alice)
	}
	carol, _, err := docs.Get(ctx, usageKey(currentWheelOfNamesCommand, 9))
	if err != nil {
		t.Fatalf("carol wheelofnames stats: %v", err)
	}
	if carol.N != 6 || carol.Deleted || len(carol.MergedFrom) != 0 {
		t.Fatalf("carol stats = %+v, want visible count 6", carol)
	}
	for _, key := range []string{alreadyMergedKey, usageKey(legacyWheelOfNamesBetaCommand, 9)} {
		if _, _, err := docs.Get(ctx, key); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("legacy key %s err = %v, want ErrNotFound", key, err)
		}
	}
	if got := renderStats(ctx, newCounter(statsColl), "cmd wheelofnames"); !strings.Contains(got, "@alice: 8") || !strings.Contains(got, "@carol: 6") {
		t.Fatalf("wheelofnames users after retry = %q, want no duplicate alice count and visible carol count", got)
	}
}
