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
