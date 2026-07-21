package storage

import (
	"context"
	"errors"
	"testing"
)

// MongoProvider.Collection re-validates the module name as defense-in-depth.
// Invalid names yield an invalidCollection whose Typed store errors without
// touching the DB — the branch worth locking even without a live Mongo.
func TestMongoProvider_Collection_RejectsInvalidName(t *testing.T) {
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
		// Invalid names short-circuit to invalidCollection before touching the
		// (nil) db, so Typed yields a store whose ops report ErrInvalidModuleName.
		store := Typed[string](p.Collection(name))
		if _, _, err := store.Get(context.Background(), "any-key"); !errors.Is(err, ErrInvalidModuleName) {
			t.Errorf("Collection(%q) store Get → %v, want ErrInvalidModuleName", name, err)
		}
	}
	// Valid names pass validation against a real db in
	// TestMongoProvider_CrossModuleIsolation (provided by Testcontainers or
	// MONGODB_TEST_URL); a nil db here would panic for a valid name.
}

// TestMongoProvider_CrossModuleIsolation verifies collection-per-module
// isolation: the same key written through two module stores yields independent
// values against Testcontainers or the MONGODB_TEST_URL override.
func TestMongoProvider_CrossModuleIsolation(t *testing.T) {
	db, cleanup := mongoLocalSetup(t)
	defer cleanup()

	p := NewMongoProvider(db)
	ctx := context.Background()

	// Mongo payloads must be a struct or map (bson inline cannot hoist a
	// scalar); a wrapper struct is the same shape modules use.
	wordle := Typed[wrappedScalar](p.Collection("wordle"))
	loldle := Typed[wrappedScalar](p.Collection("loldle"))
	if err := wordle.Put(ctx, "shared", wrappedScalar{Date: "from-wordle"}); err != nil {
		t.Fatalf("wordle Put: %v", err)
	}
	if err := loldle.Put(ctx, "shared", wrappedScalar{Date: "from-loldle"}); err != nil {
		t.Fatalf("loldle Put: %v", err)
	}

	gotW, _, _ := wordle.Get(ctx, "shared")
	gotL, _, _ := loldle.Get(ctx, "shared")
	if gotW.Date != "from-wordle" {
		t.Errorf("wordle key leaked: got %q", gotW.Date)
	}
	if gotL.Date != "from-loldle" {
		t.Errorf("loldle key leaked: got %q", gotL.Date)
	}
}
