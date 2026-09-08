package storage

import (
	"context"
	"errors"
	"testing"
)

type testPayload struct {
	Name  string `json:"name" bson:"name"`
	Count int    `json:"count" bson:"count"`
}

func memStore(module string) DocStore[testPayload] {
	return Typed[testPayload](NewMemoryProvider().Collection(module))
}

func TestMemoryDocStore_PutGetRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := memStore("coin")
	if err := s.Put(ctx, "k", testPayload{Name: "a", Count: 3}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, version, err := s.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "a" || got.Count != 3 {
		t.Fatalf("got %+v", got)
	}
	if version != 1 {
		t.Fatalf("version = %d, want 1", version)
	}
}

func TestMemoryDocStore_GetMissing(t *testing.T) {
	if _, _, err := memStore("coin").Get(context.Background(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get missing = %v, want ErrNotFound", err)
	}
}

func TestMemoryDocStore_PutBumpsVersion(t *testing.T) {
	ctx := context.Background()
	s := memStore("coin")
	_ = s.Put(ctx, "k", testPayload{Count: 1})
	_ = s.Put(ctx, "k", testPayload{Count: 2})
	_, version, _ := s.Get(ctx, "k")
	if version != 2 {
		t.Fatalf("version = %d, want 2", version)
	}
}

func TestMemoryDocStore_PutVersionedCAS(t *testing.T) {
	ctx := context.Background()
	s := memStore("coin")
	// create
	if err := s.PutVersioned(ctx, "k", 0, testPayload{Count: 1}); err != nil {
		t.Fatalf("create: %v", err)
	}
	// second create must conflict
	if err := s.PutVersioned(ctx, "k", 0, testPayload{Count: 9}); !errors.Is(err, ErrConflict) {
		t.Fatalf("double create = %v, want ErrConflict", err)
	}
	// stale expected version conflicts
	if err := s.PutVersioned(ctx, "k", 99, testPayload{Count: 9}); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale CAS = %v, want ErrConflict", err)
	}
	// fresh version succeeds
	if err := s.PutVersioned(ctx, "k", 1, testPayload{Count: 2}); err != nil {
		t.Fatalf("fresh CAS: %v", err)
	}
}

func TestMemoryDocStore_DeleteIdempotent(t *testing.T) {
	ctx := context.Background()
	s := memStore("coin")
	_ = s.Put(ctx, "k", testPayload{})
	if err := s.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := s.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete missing must be nil, got %v", err)
	}
}

func TestMemoryDocStore_ListPrefix(t *testing.T) {
	ctx := context.Background()
	s := memStore("coin")
	_ = s.Put(ctx, "game:1", testPayload{})
	_ = s.Put(ctx, "game:2", testPayload{})
	_ = s.Put(ctx, "stats:1", testPayload{})
	keys, err := s.List(ctx, "game:")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 2 || keys[0] != "game:1" || keys[1] != "game:2" {
		t.Fatalf("List game: = %v", keys)
	}
}

func TestMemoryDocStore_ValueIsolation(t *testing.T) {
	ctx := context.Background()
	s := Typed[testPayloadWithSlice](NewMemoryProvider().Collection("coin"))
	in := testPayloadWithSlice{Items: []string{"x"}}
	_ = s.Put(ctx, "k", in)
	in.Items[0] = "mutated" // mutating the caller copy must not affect stored state
	got, _, _ := s.Get(ctx, "k")
	if got.Items[0] != "x" {
		t.Fatalf("stored value aliased caller slice: %v", got.Items)
	}
}

type testPayloadWithSlice struct {
	Items []string `json:"items" bson:"items"`
}

func TestCheckReservedFields(t *testing.T) {
	type bad struct {
		Version int `bson:"version"`
	}
	if err := checkReservedFields[bad](); err == nil {
		t.Fatal("expected reserved-field collision error for bson:\"version\"")
	}
	if err := checkReservedFields[testPayload](); err != nil {
		t.Fatalf("clean struct flagged: %v", err)
	}
	// Non-struct payloads are skipped, not rejected.
	if err := checkReservedFields[string](); err != nil {
		t.Fatalf("string payload flagged: %v", err)
	}
}

// Scan is the batched read: keys and values together, in key order, so a
// caller never follows List with a Get per key.
func TestMemoryDocStore_ScanReturnsValuesInKeyOrder(t *testing.T) {
	ctx := context.Background()
	s := memStore("alias")
	_ = s.Put(ctx, "cheese", testPayload{Name: "second", Count: 2})
	_ = s.Put(ctx, "boo", testPayload{Name: "first", Count: 1})
	_ = s.Put(ctx, "other", testPayload{Name: "third", Count: 3})

	docs, err := s.Scan(ctx, "")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	want := []Doc[testPayload]{
		{ID: "boo", Val: testPayload{Name: "first", Count: 1}},
		{ID: "cheese", Val: testPayload{Name: "second", Count: 2}},
		{ID: "other", Val: testPayload{Name: "third", Count: 3}},
	}
	if len(docs) != len(want) {
		t.Fatalf("Scan returned %d docs, want %d: %+v", len(docs), len(want), docs)
	}
	for i := range want {
		if docs[i] != want[i] {
			t.Errorf("docs[%d] = %+v, want %+v", i, docs[i], want[i])
		}
	}
}

func TestMemoryDocStore_ScanPrefix(t *testing.T) {
	ctx := context.Background()
	s := memStore("coin")
	_ = s.Put(ctx, "game:1", testPayload{Name: "a"})
	_ = s.Put(ctx, "game:2", testPayload{Name: "b"})
	_ = s.Put(ctx, "stats:1", testPayload{Name: "c"})

	docs, err := s.Scan(ctx, "game:")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(docs) != 2 || docs[0].ID != "game:1" || docs[1].ID != "game:2" {
		t.Fatalf("Scan game: = %+v", docs)
	}
	if docs[0].Val.Name != "a" || docs[1].Val.Name != "b" {
		t.Errorf("Scan lost payloads: %+v", docs)
	}
}

// An empty collection yields no docs and no error — the picker path answers
// with nothing rather than treating it as a failure.
func TestMemoryDocStore_ScanEmpty(t *testing.T) {
	docs, err := memStore("alias").Scan(context.Background(), "")
	if err != nil || len(docs) != 0 {
		t.Fatalf("Scan on empty store = %+v, err %v", docs, err)
	}
}
