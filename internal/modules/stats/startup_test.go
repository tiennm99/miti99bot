package stats

import (
	"context"
	"errors"
	"testing"

	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
)

func TestInitStoreMigratesDividendCommandStats(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	statsColl := provider.Collection("stats")
	systemColl := provider.Collection(systemstate.CollectionName)
	statsDocs := storage.Typed[usageEntry](statsColl)
	systemDocs := storage.Typed[systemstate.Record](systemColl)

	seedUsageEntries(t, statsDocs, map[string]usageEntry{
		usageKey("stock_dividend", 0):       {Cmd: "stock_dividend", N: 10},
		usageKey("stock_cash_dividend", 0):  {Cmd: "stock_cash_dividend", N: 3},
		usageKey("stock_dividend", 7):       {Cmd: "stock_dividend", UserID: 7, Username: "alice", N: 4},
		usageKey("stock_cash_dividend", 7):  {Cmd: "stock_cash_dividend", UserID: 7, Username: "old-alice", N: 2},
		usageKey("stock_bonus", 0):          {Cmd: "stock_bonus", N: 8},
		usageKey("stock_bonus", 9):          {Cmd: "stock_bonus", UserID: 9, Username: "bob", N: 5},
		usageKey("stock_share_dividend", 9): {Cmd: "stock_share_dividend", UserID: 9, Username: "previous", N: 1},
	})

	if err := InitStore(ctx, statsColl, systemColl); err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	assertUsageEntry(t, statsDocs, usageKey("stock_cash_dividend", 0), 13, "")
	assertUsageEntry(t, statsDocs, usageKey("stock_cash_dividend", 7), 6, "alice")
	assertUsageEntry(t, statsDocs, usageKey("stock_share_dividend", 0), 8, "")
	assertUsageEntry(t, statsDocs, usageKey("stock_share_dividend", 9), 6, "bob")
	for _, key := range []string{
		usageKey("stock_dividend", 0), usageKey("stock_dividend", 7),
		usageKey("stock_bonus", 0), usageKey("stock_bonus", 9),
	} {
		if _, _, err := statsDocs.Get(ctx, key); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("source %s still exists or get failed: %v", key, err)
		}
	}
	global, _, err := systemDocs.Get(ctx, dividendStatsMigrationKey)
	if err != nil || global.Status != migrationStatusCompleted {
		t.Fatalf("global marker = %+v, %v", global, err)
	}
	markerKeys, err := systemDocs.List(ctx, dividendStatsRowMarkerPrefix)
	if err != nil || len(markerKeys) != 4 {
		t.Fatalf("row markers = %v, %v", markerKeys, err)
	}

	if err := InitStore(ctx, statsColl, systemColl); err != nil {
		t.Fatalf("InitStore second run: %v", err)
	}
	assertUsageEntry(t, statsDocs, usageKey("stock_cash_dividend", 0), 13, "")
	assertUsageEntry(t, statsDocs, usageKey("stock_share_dividend", 9), 6, "bob")
}

func TestDividendStatsMigrationResumesPreparedCheckpointWithoutSource(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	statsDocs := storage.Typed[usageEntry](provider.Collection("stats"))
	systemDocs := storage.Typed[systemstate.Record](provider.Collection(systemstate.CollectionName))
	markerKey := dividendStatsRowMarkerKey("stock_dividend", 7)

	if err := statsDocs.Put(ctx, usageKey("stock_cash_dividend", 7), usageEntry{
		Cmd: "stock_cash_dividend", UserID: 7, Username: "alice", N: 12,
	}); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	if err := systemDocs.Put(ctx, markerKey, systemstate.Record{
		Kind: "migration-row", Name: "stock_dividend -> stock_cash_dividend", Status: migrationStatusPrepared, Count: 12,
	}); err != nil {
		t.Fatalf("seed marker: %v", err)
	}

	if err := migrateDividendCommandStats(ctx, statsDocs, systemDocs); err != nil {
		t.Fatalf("migrateDividendCommandStats: %v", err)
	}
	assertUsageEntry(t, statsDocs, usageKey("stock_cash_dividend", 7), 12, "alice")
	marker, _, err := systemDocs.Get(ctx, markerKey)
	if err != nil || marker.Status != migrationStatusCompleted {
		t.Fatalf("row marker = %+v, %v", marker, err)
	}
}

type faultDocStore[T any] struct {
	storage.DocStore[T]
	fail func(op, id string, value T) error
}

func (s *faultDocStore[T]) Put(ctx context.Context, id string, value T) error {
	if s.fail != nil {
		if err := s.fail("put", id, value); err != nil {
			return err
		}
	}
	return s.DocStore.Put(ctx, id, value)
}

func (s *faultDocStore[T]) Delete(ctx context.Context, id string) error {
	var zero T
	if s.fail != nil {
		if err := s.fail("delete", id, zero); err != nil {
			return err
		}
	}
	return s.DocStore.Delete(ctx, id)
}

func TestDividendStatsMigrationRetriesEveryWriteBoundaryWithoutDuplication(t *testing.T) {
	for _, boundary := range []string{"prepare", "target", "delete", "complete"} {
		t.Run(boundary, func(t *testing.T) {
			ctx := context.Background()
			provider := storage.NewMemoryProvider()
			baseStats := storage.Typed[usageEntry](provider.Collection("stats"))
			baseSystem := storage.Typed[systemstate.Record](provider.Collection(systemstate.CollectionName))
			seedUsageEntries(t, baseStats, map[string]usageEntry{
				usageKey("stock_dividend", 7):      {Cmd: "stock_dividend", UserID: 7, Username: "alice", N: 5},
				usageKey("stock_cash_dividend", 7): {Cmd: "stock_cash_dividend", UserID: 7, Username: "alice", N: 7},
			})

			failed := false
			statsDocs := &faultDocStore[usageEntry]{DocStore: baseStats}
			systemDocs := &faultDocStore[systemstate.Record]{DocStore: baseSystem}
			forced := errors.New("forced boundary failure")
			statsDocs.fail = func(op, id string, _ usageEntry) error {
				if failed {
					return nil
				}
				if boundary == "target" && op == "put" && id == usageKey("stock_cash_dividend", 7) ||
					boundary == "delete" && op == "delete" && id == usageKey("stock_dividend", 7) {
					failed = true
					return forced
				}
				return nil
			}
			systemDocs.fail = func(op, id string, record systemstate.Record) error {
				if failed || op != "put" || id != dividendStatsRowMarkerKey("stock_dividend", 7) {
					return nil
				}
				if boundary == "prepare" && record.Status == migrationStatusPrepared ||
					boundary == "complete" && record.Status == migrationStatusCompleted {
					failed = true
					return forced
				}
				return nil
			}

			if err := migrateDividendCommandStats(ctx, statsDocs, systemDocs); err == nil {
				t.Fatal("migration unexpectedly succeeded before injected failure")
			}
			statsDocs.fail = nil
			systemDocs.fail = nil
			if err := migrateDividendCommandStats(ctx, statsDocs, systemDocs); err != nil {
				t.Fatalf("retry migration: %v", err)
			}
			assertUsageEntry(t, baseStats, usageKey("stock_cash_dividend", 7), 12, "alice")
			if _, _, err := baseStats.Get(ctx, usageKey("stock_dividend", 7)); !errors.Is(err, storage.ErrNotFound) {
				t.Fatalf("source remains after retry: %v", err)
			}
		})
	}
}

func seedUsageEntries(t *testing.T, docs storage.DocStore[usageEntry], entries map[string]usageEntry) {
	t.Helper()
	for key, entry := range entries {
		if err := docs.Put(context.Background(), key, entry); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}
}

func assertUsageEntry(t *testing.T, docs storage.DocStore[usageEntry], key string, wantN int64, wantUsername string) {
	t.Helper()
	entry, _, err := docs.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("get %s: %v", key, err)
	}
	if entry.N != wantN || entry.Username != wantUsername || entry.Deleted {
		t.Fatalf("entry %s = %+v, want n=%d username=%q deleted=false", key, entry, wantN, wantUsername)
	}
}
