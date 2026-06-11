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

func TestPrefixed_CompareAndSwapDelegatesWithPrefixedKey(t *testing.T) {
	ctx := context.Background()
	base := NewMemoryKVStore()
	mod := Prefixed(base, "gold")

	cas, ok := mod.(CompareAndSwapStore)
	if !ok {
		t.Fatal("Prefixed store does not implement CompareAndSwapStore")
	}
	if err := cas.CompareAndSwap(ctx, "user:1", nil, []byte("v1")); err != nil {
		t.Fatalf("CompareAndSwap create: %v", err)
	}

	// The write must land under the prefixed key on the inner store.
	got, err := base.Get(ctx, "gold:user:1")
	if err != nil {
		t.Fatalf("inner Get: %v", err)
	}
	if string(got) != "v1" {
		t.Errorf("inner value = %q, want %q", got, "v1")
	}

	// Stale expected must surface the inner store's conflict.
	if err := cas.CompareAndSwap(ctx, "user:1", []byte("stale"), []byte("v2")); !errors.Is(err, ErrConflict) {
		t.Errorf("CompareAndSwap stale: got %v, want ErrConflict", err)
	}
	if err := cas.CompareAndSwap(ctx, "user:1", []byte("v1"), []byte("v2")); err != nil {
		t.Errorf("CompareAndSwap matching: %v", err)
	}
}

// plainStore hides the inner store's CompareAndSwap method so the wrapper's
// missing-capability path can be exercised.
type plainStore struct {
	KVStore
}

func TestPrefixed_CompareAndSwapUnsupportedInner(t *testing.T) {
	mod := Prefixed(&plainStore{KVStore: NewMemoryKVStore()}, "gold")
	cas := mod.(CompareAndSwapStore)

	err := cas.CompareAndSwap(context.Background(), "user:1", nil, []byte("v1"))
	if !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("got %v, want errors.ErrUnsupported", err)
	}
	if errors.Is(err, ErrConflict) {
		t.Error("missing CAS capability must not be reported as a retryable conflict")
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
