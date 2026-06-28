package storage

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestPrefixed_RoundTrip(t *testing.T) {
	ctx := context.Background()
	base := NewMemoryKVStore()
	a := Prefixed(base, "modA")
	b := Prefixed(base, "modB")

	if err := a.Put(ctx, "score", []byte("10")); err != nil {
		t.Fatalf("a.Put: %v", err)
	}
	if err := b.Put(ctx, "score", []byte("20")); err != nil {
		t.Fatalf("b.Put: %v", err)
	}

	got, err := a.Get(ctx, "score")
	if err != nil {
		t.Fatalf("a.Get: %v", err)
	}
	if string(got) != "10" {
		t.Errorf("a.Get score = %q, want %q", got, "10")
	}

	got, err = b.Get(ctx, "score")
	if err != nil {
		t.Fatalf("b.Get: %v", err)
	}
	if string(got) != "20" {
		t.Errorf("b.Get score = %q, want %q", got, "20")
	}
}

func TestPrefixed_ListStripsPrefix(t *testing.T) {
	ctx := context.Background()
	base := NewMemoryKVStore()
	mod := Prefixed(base, "wordle")

	for _, k := range []string{"u:1", "u:2", "session:abc"} {
		if err := mod.Put(ctx, k, []byte("x")); err != nil {
			t.Fatalf("Put %q: %v", k, err)
		}
	}

	got, err := mod.List(ctx, "u:")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"u:1", "u:2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List u: = %v, want %v", got, want)
	}
}

func TestPrefixed_NotFoundPropagates(t *testing.T) {
	ctx := context.Background()
	base := NewMemoryKVStore()
	mod := Prefixed(base, "modA")

	if _, err := mod.Get(ctx, "missing"); err != ErrNotFound {
		t.Errorf("Get missing = %v, want ErrNotFound", err)
	}
}

func TestPrefixed_VersionedDelegatesWithPrefixedKey(t *testing.T) {
	ctx := context.Background()
	base := NewMemoryKVStore()
	mod := Prefixed(base, "gold")

	vs, ok := mod.(VersionedStore)
	if !ok {
		t.Fatal("Prefixed store does not implement VersionedStore")
	}
	if err := vs.PutVersioned(ctx, "user:1", 0, []byte("v1")); err != nil {
		t.Fatalf("PutVersioned create: %v", err)
	}

	// The write must land under the prefixed key on the inner store.
	got, err := base.Get(ctx, "gold:user:1")
	if err != nil {
		t.Fatalf("inner Get: %v", err)
	}
	if string(got) != "v1" {
		t.Errorf("inner value = %q, want %q", got, "v1")
	}

	// GetVersioned reads through the prefix and returns the inner version.
	val, ver, err := vs.GetVersioned(ctx, "user:1")
	if err != nil || string(val) != "v1" || ver != 1 {
		t.Fatalf("GetVersioned: got (%q, v=%d, %v), want (v1, 1, nil)", val, ver, err)
	}

	// Stale version must surface the inner store's conflict.
	if err := vs.PutVersioned(ctx, "user:1", 99, []byte("v2")); !errors.Is(err, ErrConflict) {
		t.Errorf("stale version: got %v, want ErrConflict", err)
	}
	if err := vs.PutVersioned(ctx, "user:1", 1, []byte("v2")); err != nil {
		t.Errorf("matching version: %v", err)
	}
}

// plainStore hides the inner store's versioned methods so the wrapper's
// missing-capability path can be exercised.
type plainStore struct {
	KVStore
}

func TestPrefixed_VersionedUnsupportedInner(t *testing.T) {
	mod := Prefixed(&plainStore{KVStore: NewMemoryKVStore()}, "gold")
	vs := mod.(VersionedStore)

	err := vs.PutVersioned(context.Background(), "user:1", 0, []byte("v1"))
	if !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("got %v, want errors.ErrUnsupported", err)
	}
	if errors.Is(err, ErrConflict) {
		t.Error("missing versioned capability must not be reported as a retryable conflict")
	}
}

func TestPrefixed_PanicsOnEmptyPrefix(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Prefixed(_, \"\") did not panic")
		}
	}()
	Prefixed(NewMemoryKVStore(), "")
}
