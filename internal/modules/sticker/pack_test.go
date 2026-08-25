package sticker

import (
	"context"
	"testing"

	"github.com/tiennm99/miti99bot/internal/storage"
)

func newTestStore(t *testing.T) PackStore {
	t.Helper()
	return storage.Typed[Pack](storage.NewMemoryProvider().Collection("sticker"))
}

// Pack is persisted with its fields hoisted to the document root, so a bson tag
// colliding with a reserved root field would panic at startup. Typed panics on
// collision; constructing the store is the assertion.
func TestPack_NoReservedFieldCollision(t *testing.T) {
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("Pack collides with a reserved storage field: %v", rec)
		}
	}()
	_ = newTestStore(t)
}

// The key is the owner ID alone, which is what makes the lookup itself the
// ownership check: there is no key shape that reads another user's pack.
func TestGetPack_IsolatesOwners(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	want := Pack{Slug: "alpha", Name: "alpha_by_bot", Title: "Alpha", OwnerID: 1, Count: 3}
	if err := store.Put(ctx, packKey(1), want); err != nil {
		t.Fatalf("put: %v", err)
	}

	got, found, err := getPack(ctx, store, 1)
	if err != nil || !found {
		t.Fatalf("getPack(owner 1) = (%+v, %v, %v), want found", got, found, err)
	}
	if got.Slug != want.Slug || got.Count != want.Count {
		t.Errorf("getPack(owner 1) = %+v, want %+v", got, want)
	}

	other, found, err := getPack(ctx, store, 2)
	if err != nil {
		t.Fatalf("getPack(owner 2) error: %v", err)
	}
	if found {
		t.Errorf("getPack(owner 2) returned owner 1's pack: %+v", other)
	}
}

// A user who has never run /newpack is the normal case, not an error worth
// propagating to every caller.
func TestGetPack_MissingIsNotAnError(t *testing.T) {
	got, found, err := getPack(context.Background(), newTestStore(t), 404)
	if err != nil {
		t.Fatalf("getPack(unknown) error: %v", err)
	}
	if found {
		t.Errorf("getPack(unknown) found %+v, want not found", got)
	}
}

func TestShareLink(t *testing.T) {
	if got, want := shareLink("mypack_by_bot"), "https://t.me/addstickers/mypack_by_bot"; got != want {
		t.Errorf("shareLink = %q, want %q", got, want)
	}
}
