package storage

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryKVStore_Versioned(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryKVStore()

	// Absent key reports ErrNotFound + version 0.
	if _, v, err := s.GetVersioned(ctx, "k"); !errors.Is(err, ErrNotFound) || v != 0 {
		t.Fatalf("GetVersioned absent: got (v=%d, %v), want (0, ErrNotFound)", v, err)
	}

	// Create when absent (expectedVersion 0).
	if err := s.PutVersioned(ctx, "k", 0, []byte("v1")); err != nil {
		t.Fatalf("PutVersioned create: %v", err)
	}
	val, ver, err := s.GetVersioned(ctx, "k")
	if err != nil || string(val) != "v1" || ver != 1 {
		t.Fatalf("after create: got (%q, v=%d, %v), want (v1, 1, nil)", val, ver, err)
	}

	// Create over existing conflicts.
	if err := s.PutVersioned(ctx, "k", 0, []byte("v2")); !errors.Is(err, ErrConflict) {
		t.Errorf("create over existing: got %v, want ErrConflict", err)
	}

	// Swap with matching version succeeds and bumps version.
	if err := s.PutVersioned(ctx, "k", 1, []byte("v2")); err != nil {
		t.Fatalf("swap matching version: %v", err)
	}
	if _, ver, _ := s.GetVersioned(ctx, "k"); ver != 2 {
		t.Errorf("version after swap = %d, want 2", ver)
	}

	// Stale version conflicts.
	if err := s.PutVersioned(ctx, "k", 1, []byte("v3")); !errors.Is(err, ErrConflict) {
		t.Errorf("stale version: got %v, want ErrConflict", err)
	}

	// Swap on a missing key conflicts.
	if err := s.PutVersioned(ctx, "missing", 1, []byte("x")); !errors.Is(err, ErrConflict) {
		t.Errorf("swap missing key: got %v, want ErrConflict", err)
	}

	// A plain Put bumps the version, so a versioned writer holding the old
	// version sees a conflict.
	if err := s.Put(ctx, "k", []byte("v4")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := s.PutVersioned(ctx, "k", 2, []byte("v5")); !errors.Is(err, ErrConflict) {
		t.Errorf("versioned write after plain Put: got %v, want ErrConflict", err)
	}
}
