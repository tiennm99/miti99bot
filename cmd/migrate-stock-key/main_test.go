package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/tiennm99/miti99bot/internal/storage"
)

// The in-memory provider key-prefixes by module name, mirroring how the
// DynamoDB provider partitions by pk — so it faithfully exercises the
// cross-partition copy without needing a live table.
func TestMigrate_CopiesSkipsAndPreservesSource(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	src := provider.For("trading")
	dst := provider.For("stock")

	mustPut(t, ctx, src, "user:7", `{"currency":{"VND":1500000}}`)
	mustPut(t, ctx, src, "user:42", `{"currency":{"VND":1}}`)
	mustPut(t, ctx, src, "sym:TCB", `{"symbol":"TCB"}`)
	// Destination already holds a *fresher* user:42 — migration must not clobber it.
	mustPut(t, ctx, dst, "user:42", `{"currency":{"VND":99999}}`)

	// Dry run writes nothing new.
	var dry bytes.Buffer
	if err := migrate(ctx, src, dst, "trading", "stock", false, &dry); err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if got := getString(t, ctx, dst, "user:42"); got != `{"currency":{"VND":99999}}` {
		t.Fatalf("dry run mutated dst user:42: %q", got)
	}
	if _, err := dst.Get(ctx, "user:7"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("dry run created dst user:7, want ErrNotFound, got %v", err)
	}

	// Apply.
	var live bytes.Buffer
	if err := migrate(ctx, src, dst, "trading", "stock", true, &live); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// New keys copied byte-for-byte.
	if got := getString(t, ctx, dst, "user:7"); got != `{"currency":{"VND":1500000}}` {
		t.Errorf("user:7 not copied correctly: %q", got)
	}
	if got := getString(t, ctx, dst, "sym:TCB"); got != `{"symbol":"TCB"}` {
		t.Errorf("sym:TCB not copied correctly: %q", got)
	}
	// Pre-existing destination key left untouched (skip, not overwrite).
	if got := getString(t, ctx, dst, "user:42"); got != `{"currency":{"VND":99999}}` {
		t.Errorf("user:42 was clobbered: %q", got)
	}
	// Source rows remain in place (rollback safety).
	if got := getString(t, ctx, src, "user:7"); got != `{"currency":{"VND":1500000}}` {
		t.Errorf("source user:7 was modified/removed: %q", got)
	}
}

func mustPut(t *testing.T, ctx context.Context, kv storage.KVStore, key, val string) {
	t.Helper()
	if err := kv.Put(ctx, key, []byte(val)); err != nil {
		t.Fatalf("put %s: %v", key, err)
	}
}

func getString(t *testing.T, ctx context.Context, kv storage.KVStore, key string) string {
	t.Helper()
	raw, err := kv.Get(ctx, key)
	if err != nil {
		t.Fatalf("get %s: %v", key, err)
	}
	return string(raw)
}
