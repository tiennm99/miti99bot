package storage

import (
	"context"
	"errors"
	"testing"
)

// MongoProvider.For re-validates the module name as defense-in-depth. Invalid
// names return invalidStore without touching the DB — the branch worth locking
// even without a live Mongo (mirrors the firestore provider test).
func TestMongoProvider_For_RejectsInvalidName(t *testing.T) {
	p := &MongoProvider{db: nil}
	bogus := []string{
		"",
		"with spaces",
		"WITHCAPS",
		"path/traversal",
		"../etc/passwd",
		"way-too-long-for-our-32-char-limit-x",
		"with:colon",
	}
	for _, name := range bogus {
		store := p.For(name)
		if _, err := store.Get(context.Background(), "any-key"); !errors.Is(err, ErrInvalidModuleName) {
			t.Errorf("For(%q).Get → %v, want ErrInvalidModuleName", name, err)
		}
	}
}

// TestMongoProvider_CrossModuleIsolation verifies collection-per-module
// isolation: the same key written through two module stores yields independent
// values. Gated on MONGODB_TEST_URL.
func TestMongoProvider_CrossModuleIsolation(t *testing.T) {
	_, db, cleanup := mongoLocalSetup(t, "wordle")
	defer cleanup()

	p := NewMongoProvider(db)
	ctx := context.Background()

	wordle := p.For("wordle")
	loldle := p.For("loldle")
	if err := wordle.Put(ctx, "shared", []byte("from-wordle")); err != nil {
		t.Fatalf("wordle Put: %v", err)
	}
	if err := loldle.Put(ctx, "shared", []byte("from-loldle")); err != nil {
		t.Fatalf("loldle Put: %v", err)
	}

	gotW, _ := wordle.Get(ctx, "shared")
	gotL, _ := loldle.Get(ctx, "shared")
	if string(gotW) != "from-wordle" {
		t.Errorf("wordle key leaked: got %q", gotW)
	}
	if string(gotL) != "from-loldle" {
		t.Errorf("loldle key leaked: got %q", gotL)
	}
	// Canonical names pass validation (not invalidStore); this also covers the
	// valid-name branch of For against a real database.
	for _, name := range []string{"misc", "demo-mod", "x", "a1_b-2"} {
		if _, ok := p.For(name).(invalidStore); ok {
			t.Errorf("For(%q) returned invalidStore; expected validation to pass", name)
		}
	}
}
