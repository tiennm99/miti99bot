// Package sticker lets any user create and manage one personal Telegram
// sticker pack through the bot. The pack is created on behalf of the calling
// user, named "<slug>_by_<bot_username>", and stays bot-manageable because the
// bot created it.
//
// One pack per user is the central simplification: no command except /newpack
// takes a pack argument, because there is only ever one pack to act on.
package sticker

import (
	"context"
	"errors"
	"strconv"

	"github.com/tiennm99/miti99bot/internal/storage"
)

// Pack is the single bot-created sticker set owned by a Telegram user.
//
// The record is keyed by owner ID alone, which makes the lookup itself the
// ownership check: there is no way to read a pack without naming its owner.
type Pack struct {
	Slug      string `bson:"slug"`      // chosen at creation, fixes the permanent URL
	Name      string `bson:"name"`      // Telegram set name, "<slug>_by_<botname>"
	Title     string `bson:"title"`     // display title, mutable via /renamepack
	OwnerID   int64  `bson:"ownerId"`   // Telegram user the set belongs to
	Count     int    `bson:"count"`     // stickers in the set; keeps /mypack API-free
	Pending   bool   `bson:"pending"`   // write-ahead intent; see newpack's state machine
	CreatedAt int64  `bson:"createdAt"` // unix millis
}

// PackStore is the module's view over its collection.
type PackStore = storage.DocStore[Pack]

// SlugReservation records which user claimed a pack name, globally.
//
// Pack records are keyed by owner, so they can only answer "does *this* user
// have a pack" — they cannot answer "who holds this name". Without that second
// question the module cannot tell its own interrupted attempt from a set
// belonging to someone else, because both look identical from the caller's
// side: a pending record naming a set that exists. Adopting on that evidence
// alone let any user take over any pack whose public link they could guess.
//
// The reservation is the missing half. It is claimed with a create-only write
// before Telegram is touched, so the first claimant of a name is the only user
// who can ever adopt a set under it.
type SlugReservation struct {
	Slug      string `bson:"slug"`
	OwnerID   int64  `bson:"ownerId"`
	CreatedAt int64  `bson:"createdAt"`
}

// SlugStore is the third typed view over the module's collection.
type SlugStore = storage.DocStore[SlugReservation]

// slugKey namespaces reservations away from the owner-keyed Pack records.
// Pack keys are decimal owner IDs, so the prefix cannot collide.
func slugKey(slug string) string { return slugPrefix + slug }

const slugPrefix = "slug:"

// getSlugReservation reads a name's reservation. Missing is not an error — it
// is the normal state for an unclaimed name.
func getSlugReservation(ctx context.Context, store SlugStore, slug string) (SlugReservation, bool, error) {
	r, _, err := store.Get(ctx, slugKey(slug))
	if errors.Is(err, storage.ErrNotFound) {
		return SlugReservation{}, false, nil
	}
	if err != nil {
		return SlugReservation{}, false, err
	}
	return r, true, nil
}

// packKey is the storage key for a user's pack: the owner ID and nothing else.
//
// One pack per user makes the slug unnecessary as a key component, which is
// what removes the prefix scan the multi-pack design needed. The module calls
// List nowhere.
func packKey(ownerID int64) string { return strconv.FormatInt(ownerID, 10) }

// getPack reads the caller's pack. A missing record is not an error — it is the
// normal state for a user who has never run /newpack — so it reports found
// rather than returning storage.ErrNotFound for every caller to translate.
func getPack(ctx context.Context, store PackStore, ownerID int64) (Pack, bool, error) {
	p, _, err := store.Get(ctx, packKey(ownerID))
	if errors.Is(err, storage.ErrNotFound) {
		return Pack{}, false, nil
	}
	if err != nil {
		return Pack{}, false, err
	}
	return p, true, nil
}

// shareLink is the public URL of a pack. It is fixed at creation and cannot be
// changed: Telegram exposes no method to rename a set's short name.
func shareLink(setName string) string {
	return "https://t.me/addstickers/" + setName
}
