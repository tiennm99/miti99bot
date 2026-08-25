package sticker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/storage"
)

const (
	newpackUsage = "Reply to a sticker with: /newpack <pack> <title...>\nEg: /newpack mypack My Pack"
	// slugTaken answers a name held by anyone other than the caller. It says
	// only "taken", never who holds it — the plan accepts that pack names are
	// enumerable (share links are public), but there is no reason to confirm
	// which are backed by real packs any more precisely than Telegram already
	// does.
	slugTaken     = "That pack name is taken. Pick a different one."
	noPackYet     = "You don't have a pack yet. Reply to a sticker with /newpack <pack> <title...> to create one."
	pendingMarker = "\n\n⚠️ This pack is incomplete — re-run the same /newpack command to finish it."
)

// handleNewPack creates the caller's pack.
//
// The hard part is not the API call, it is surviving an interruption. The slug
// fixes a permanent public URL, so an attempt that dies between "Telegram
// created the set" and "we wrote it down" would strand that slug forever: the
// set exists, nobody's record points at it, and the user cannot recreate it.
//
// The fix is a write-ahead record, claimed before Telegram is called, in two
// parts that answer two different questions:
//
//   - a global reservation on the name — "who claimed this name?" (reserveSlug)
//   - an owner-keyed pending Pack — "does this user have a pack?" (claimSlug)
//
// Both are needed. The pending record alone proves only that this caller *asked
// for* the name, which a user naming someone else's pack also does; treating it
// as proof of ownership was a pack-takeover hole. The reservation is what makes
// "this set is mine to adopt" a fact rather than an assumption.
func (s *state) handleNewPack(ctx context.Context, b *bot.Bot, update *models.Update) error {
	ctx, cancel := handlerContext(ctx)
	defer cancel()

	msg := update.Message
	ownerID, err := senderID(msg)
	if err != nil {
		return reply(ctx, b, msg, senderRefusal)
	}

	args := commandArgs(msg)
	if len(args) < 2 {
		return reply(ctx, b, msg, newpackUsage)
	}
	slug := strings.ToLower(args[0])
	if err := validateSlug(slug); err != nil {
		return replyErr(ctx, b, msg, "sticker_newpack_slug", err)
	}
	title := strings.TrimSpace(strings.Join(args[1:], " "))
	if title == "" || len([]rune(title)) > maxTitleLen {
		return reply(ctx, b, msg, fmt.Sprintf("Give a title of 1-%d characters.", maxTitleLen))
	}

	// The lock covers everything below, including the media leg. It is per-user,
	// so a slow download delays only the user who sent it.
	defer s.lockUser(ownerID)()

	// Refuse a caller who already has a finished pack BEFORE reserving anything,
	// and before doing any expensive work.
	//
	// Reservations are permanent and global, so writing one and *then*
	// discovering the caller is not entitled to a pack handed every user an
	// unlimited name-burning primitive: each refused /newpack claimed a name for
	// everyone else, at the cost of one message and zero API calls. The order of
	// this check and reserveSlug is the whole defence.
	//
	// It also runs before resolveSource, which downloads, resamples and
	// re-uploads an image. Answering "you already have a pack" is a single store
	// read; making a user who cannot create a pack pay for the full media
	// pipeline first was free work for anyone who wanted to spend the bot's CPU.
	if existing, found, err := getPack(ctx, s.store, ownerID); err != nil {
		log.Error("sticker_newpack_precheck", "err", err)
		return reply(ctx, b, msg, genericFailure)
	} else if found && !existing.Pending {
		return reply(ctx, b, msg, fmt.Sprintf(
			"You already have a pack (%s). Use /delpack first if you want a different one.\n%s",
			existing.Slug, shareLink(existing.Name)))
	}

	source, err := s.resolveSource(ctx, b, ownerID, msg)
	if err != nil {
		return replyErr(ctx, b, msg, "sticker_newpack_source", err)
	}

	username, err := s.resolver.resolve(ctx, b)
	if err != nil {
		log.Error("sticker_newpack_username", "err", err)
		return reply(ctx, b, msg, genericFailure)
	}
	setName, err := makeSetName(slug, username)
	if err != nil {
		return replyErr(ctx, b, msg, "sticker_newpack_setname", err)
	}

	created, done, err := s.reserveSlug(ctx, b, msg, ownerID, slug)
	if err != nil || done {
		return err
	}

	claimed, done, err := s.claimSlug(ctx, b, msg, ownerID, slug, setName, title)
	if err != nil || done {
		// Nothing downstream is holding this name: release it rather than
		// leaving a permanent claim with no pack behind it. Only what this
		// invocation created — never a reservation we merely resumed.
		if created {
			s.releaseSlug(ctx, ownerID, slug)
		}
		return err
	}

	return s.createOrAdopt(ctx, b, msg, claimed, source, created)
}

// reserveSlug claims a pack name for ownerID, globally and permanently.
//
// Pack records are keyed by owner, so they answer "does this user have a pack"
// and nothing else. That is not enough to make adoption safe: a user with no
// pack who names someone else's slug produces exactly the same evidence as a
// user resuming their own interrupted attempt — a pending record naming a set
// that exists. Adopting on that evidence let anyone take over any pack whose
// public link they could guess, and every share link is public.
//
// A create-only write on the name itself supplies the missing fact. The first
// claimant is the only user who can ever adopt a set under that name, so by the
// time createOrAdopt sees an existing set, "it is ours" has actually been
// proven rather than assumed.
//
// created reports whether this call wrote the reservation, so a caller that
// bails can release exactly what it made and never a reservation it merely
// resumed. done == true means the caller was already answered and the handler
// must stop.
func (s *state) reserveSlug(ctx context.Context, b *bot.Bot, msg *models.Message, ownerID int64, slug string) (created, done bool, err error) {
	err = s.slugs.PutVersioned(ctx, slugKey(slug), 0, SlugReservation{
		Slug:      slug,
		OwnerID:   ownerID,
		CreatedAt: s.now().UnixMilli(),
	})
	if err == nil {
		return true, false, nil
	}
	if !errors.Is(err, storage.ErrConflict) {
		log.Error("sticker_newpack_reserve", "err", err)
		return false, true, reply(ctx, b, msg, genericFailure)
	}

	held, found, getErr := getSlugReservation(ctx, s.slugs, slug)
	if getErr != nil || !found {
		// Conflict but unreadable: treat the name as unavailable rather than
		// guessing. Guessing the other way is the takeover.
		log.Error("sticker_newpack_reserve_read", "err", getErr)
		return false, true, reply(ctx, b, msg, slugTaken)
	}
	if held.OwnerID != ownerID {
		return false, true, reply(ctx, b, msg, slugTaken)
	}
	// Our own reservation from an earlier attempt: carry on and resume it, but
	// do not claim we created it — releasing it on a later bail would discard a
	// claim that predates this command.
	return false, false, nil
}

// releaseSlug frees a reservation whose pack was never created, or was deleted.
//
// Only ever called with a positive signal that the name is not in use — never on
// an unknown error, which would hand the name to whoever asks next while the set
// may still exist.
//
// It verifies ownership itself rather than trusting call sites. A bare
// delete-by-name is a cross-user primitive, and this module has already been
// bitten once by an ownership check that lived in the caller instead of the
// operation.
func (s *state) releaseSlug(ctx context.Context, ownerID int64, slug string) {
	// The ownership read runs on the detached context too, not just the delete.
	// Splitting them meant a cancelled request (SIGTERM, or the handler
	// deadline) failed the read and returned before the delete — leaving a
	// reservation with no pack and no set behind it, which no code path can
	// ever reach again. A cleanup that only half-survives shutdown is worse
	// than one that does not run at all.
	commitCtx, cancel := commitContext(ctx)
	defer cancel()

	held, found, err := getSlugReservation(commitCtx, s.slugs, slug)
	if err != nil {
		log.Error("sticker_release_slug_read", "slug", slug, "err", err)
		return
	}
	if !found {
		return
	}
	if held.OwnerID != ownerID {
		log.Error("sticker_release_slug_refused", "slug", slug, "holder", held.OwnerID, "caller", ownerID)
		return
	}
	if err := s.slugs.Delete(commitCtx, slugKey(slug)); err != nil && !errors.Is(err, storage.ErrNotFound) {
		log.Error("sticker_release_slug", "slug", slug, "err", err)
	}
}

// claimSlug writes the write-ahead intent record, or interprets the conflict
// when the caller already has one.
//
// done == true means the caller was already answered and the handler must stop.
func (s *state) claimSlug(ctx context.Context, b *bot.Bot, msg *models.Message, ownerID int64, slug, setName, title string) (Pack, bool, error) {
	intent := Pack{
		Slug:      slug,
		Name:      setName,
		Title:     title,
		OwnerID:   ownerID,
		Pending:   true,
		CreatedAt: s.now().UnixMilli(),
	}

	// PutVersioned with expectedVersion 0 is create-only, and Mongo resolves it
	// with a duplicate-key error, so exactly one writer wins. This record *is*
	// the one-pack-per-user quota — there is no separate counter to keep in
	// sync. Put would silently overwrite and must not be used here.
	err := s.store.PutVersioned(ctx, packKey(ownerID), 0, intent)
	if err == nil {
		return intent, false, nil
	}
	if !errors.Is(err, storage.ErrConflict) {
		log.Error("sticker_newpack_claim", "err", err)
		return Pack{}, true, reply(ctx, b, msg, genericFailure)
	}

	existing, found, getErr := getPack(ctx, s.store, ownerID)
	if getErr != nil || !found {
		log.Error("sticker_newpack_reread", "err", getErr)
		return Pack{}, true, reply(ctx, b, msg, genericFailure)
	}

	switch {
	case !existing.Pending:
		return Pack{}, true, reply(ctx, b, msg, fmt.Sprintf(
			"You already have a pack (%s). Use /delpack first if you want a different one.\n%s",
			existing.Slug, shareLink(existing.Name)))

	case existing.Slug == slug:
		// Our own interrupted attempt for this exact name: resume it.
		return existing, false, nil

	default:
		// An earlier attempt was interrupted under a *different* name. Probing
		// first is what makes this safe: if that set was already created,
		// overwriting the record here would orphan it permanently — the set
		// would exist, owned by this user, with adoption keyed on a slug the
		// record no longer holds.
		return s.resolveStaleIntent(ctx, b, msg, existing, intent)
	}
}

// resolveStaleIntent decides what to do with a pending record for a slug the
// caller is no longer asking for.
func (s *state) resolveStaleIntent(ctx context.Context, b *bot.Bot, msg *models.Message, existing, intent Pack) (Pack, bool, error) {
	// The old name is only adoptable if this caller reserved it too. They
	// normally did — the pending record came from their own earlier run through
	// reserveSlug — but adoption is the dangerous operation in this module, so
	// it is re-proven rather than inferred from the pending record.
	held, found, err := getSlugReservation(ctx, s.slugs, existing.Slug)
	if err != nil {
		log.Error("sticker_newpack_stale_reservation", "err", err)
		return Pack{}, true, reply(ctx, b, msg, genericFailure)
	}
	if !found || held.OwnerID != intent.OwnerID {
		// Someone else holds the old name, or it was released. Either way this
		// caller cannot adopt under it; drop the dead intent and let them
		// proceed with the name they actually asked for.
		if putErr := s.store.Put(ctx, packKey(intent.OwnerID), intent); putErr != nil {
			log.Error("sticker_newpack_replace_intent", "err", putErr)
			return Pack{}, true, reply(ctx, b, msg, genericFailure)
		}
		return intent, false, nil
	}

	_, err = b.GetStickerSet(ctx, &bot.GetStickerSetParams{Name: existing.Name})
	switch {
	case err == nil:
		// The old set exists — adopt it rather than stranding it.
		adopted := existing
		adopted.Pending = false
		if adopted.Count == 0 {
			adopted.Count = 1
		}
		if commitErr := s.commitPack(ctx, adopted); commitErr != nil {
			log.Error("sticker_newpack_adopt_commit", "err", commitErr)
			return Pack{}, true, reply(ctx, b, msg, genericFailure)
		}
		return Pack{}, true, reply(ctx, b, msg, fmt.Sprintf(
			"You already have a pack (%s) from an earlier attempt — it has been restored.\n%s\nUse /delpack first if you want a different name.",
			adopted.Slug, shareLink(adopted.Name)))

	case isStickerSetMissing(err):
		// Nothing was created under the old name; take over the record. A
		// positive "no such set" is what makes releasing the old reservation
		// safe — otherwise an abandoned name would be held against every other
		// user forever, with no set behind it.
		if putErr := s.store.Put(ctx, packKey(intent.OwnerID), intent); putErr != nil {
			log.Error("sticker_newpack_replace_intent", "err", putErr)
			return Pack{}, true, reply(ctx, b, msg, genericFailure)
		}
		s.releaseSlug(ctx, intent.OwnerID, existing.Slug)
		return intent, false, nil

	default:
		// Unknown failure: change nothing (plan rule 4).
		log.Error("sticker_newpack_probe", "err", err)
		return Pack{}, true, reply(ctx, b, msg, genericFailure)
	}
}

// createOrAdopt performs the Telegram-side creation for a claimed intent, or
// adopts a set an interrupted attempt already created.
//
// freshReservation reports that *this* invocation first claimed the name, which
// is what disqualifies adoption: see the err == nil branch.
func (s *state) createOrAdopt(ctx context.Context, b *bot.Bot, msg *models.Message, pack Pack, source stickerSource, freshReservation bool) error {
	_, err := b.GetStickerSet(ctx, &bot.GetStickerSetParams{Name: pack.Name})
	switch {
	case err == nil:
		// A set exists under this name. Adopting it grants full control -
		// DeleteStickerSet, DeleteStickerFromSet and SetStickerSetTitle are all
		// keyed by set name alone, with no owner scoping - so this branch must
		// prove the set is the caller's own interrupted attempt, not merely
		// assume it.
		//
		// The reservation proves it only if it outlived the set. It does not
		// always: the reservation lives in our store and the set lives at
		// Telegram, and the store can be wiped (a restart on the in-memory
		// backend does exactly that) while every pack survives. Then the
		// evidence a takeover produces and the evidence a real resume produces
		// are once again identical.
		//
		// freshReservation separates them without any new state. A genuine
		// interrupted attempt reserved the name *before* creating the set, so it
		// re-enters here having found its own reservation, never having made
		// one. Claiming the name for the first time therefore proves the set
		// under it is somebody else's.
		if freshReservation {
			log.Error("sticker_newpack_adopt_refused", "slug", pack.Slug, "owner", pack.OwnerID)
			// Leave nothing behind. The intent records an attempt that is not
			// going to happen, and the reservation names a set this caller has
			// just been refused — holding either would deny the name to the
			// set's real owner, who after a store wipe has to re-register it
			// exactly as this caller tried to.
			s.dropIntent(ctx, pack.OwnerID)
			s.releaseSlug(ctx, pack.OwnerID, pack.Slug)
			return reply(ctx, b, msg, slugTaken)
		}
		return s.finishNewPack(ctx, b, msg, pack, true)

	case isStickerSetMissing(err):
		// Free: create it.

	default:
		// Unknown. Keep both the intent and the reservation: the set may exist,
		// and re-running is how the user recovers. Destroying either here is
		// what strands a slug (plan rule 4).
		log.Error("sticker_newpack_lookup", "err", err)
		return reply(ctx, b, msg, genericFailure)
	}

	emoji := source.emoji
	if len(emoji) == 0 {
		emoji = []string{defaultEmoji}
	}
	_, err = b.CreateNewStickerSet(ctx, &bot.CreateNewStickerSetParams{
		UserID: pack.OwnerID,
		Name:   pack.Name,
		Title:  pack.Title,
		Stickers: []models.InputSticker{{
			Sticker:   source.fileID,
			Format:    stickerFormatStatic,
			EmojiList: emoji,
		}},
	})
	if err != nil {
		// Only a refusal that proves nothing was created lets us undo the
		// claim. On anything else the create may have succeeded server-side, so
		// both the intent and the reservation stay and a re-run adopts the set.
		if createRefused(err) {
			s.dropIntent(ctx, pack.OwnerID)
			s.releaseSlug(ctx, pack.OwnerID, pack.Slug)
		}
		return replyAPIError(ctx, b, msg, "sticker_newpack_create", err)
	}
	return s.finishNewPack(ctx, b, msg, pack, false)
}

// finishNewPack commits the confirmed record and replies with the share link.
func (s *state) finishNewPack(ctx context.Context, b *bot.Bot, msg *models.Message, pack Pack, adopted bool) error {
	pack.Pending = false
	if pack.Count == 0 {
		pack.Count = 1
	}
	if err := s.commitPack(ctx, pack); err != nil {
		log.Error("sticker_newpack_commit", "err", err)
		return reply(ctx, b, msg, genericFailure)
	}
	prefix := "Created"
	if adopted {
		prefix = "Finished an earlier attempt at"
	}
	return reply(ctx, b, msg, fmt.Sprintf("%s %s.\n%s\n\nAdd more with /addsticker while replying to a sticker.",
		prefix, pack.Title, shareLink(pack.Name)))
}

// dropIntent removes a write-ahead record whose creation never happened, so the
// user is not left holding a slug for a set that does not exist.
func (s *state) dropIntent(ctx context.Context, ownerID int64) {
	commitCtx, cancel := commitContext(ctx)
	defer cancel()
	if err := s.store.Delete(commitCtx, packKey(ownerID)); err != nil && !errors.Is(err, storage.ErrNotFound) {
		log.Error("sticker_drop_intent", "user", ownerID, "err", err)
	}
}

// adjustCount applies a delta to the caller's sticker count and commits.
//
// It re-reads the record rather than trusting the copy the handler resolved
// earlier: that read happened *before* the per-user lock was taken, so writing
// a count derived from it would clobber any change made in between. Reading and
// writing both inside the lock is what makes the lock mean anything.
//
// Returns the committed record so the reply can quote the new count.
func (s *state) adjustCount(ctx context.Context, ownerID, delta int64) (Pack, error) {
	// Detached: the sticker has already been added or removed at Telegram by
	// the time this runs, so the read-modify-write that records it must not be
	// abandoned because the request context expired mid-flight.
	commitCtx, cancel := commitContext(ctx)
	defer cancel()

	pack, found, err := getPack(commitCtx, s.store, ownerID)
	if err != nil {
		return Pack{}, err
	}
	if !found {
		// The record vanished under us — nothing to update, and nothing that
		// justifies recreating it.
		return Pack{}, storage.ErrNotFound
	}
	pack.Count += int(delta)
	if pack.Count < 0 {
		// Count is advisory and drifts when a pack is edited through @Stickers.
		// It must never go negative.
		pack.Count = 0
	}
	return pack, s.commitPack(commitCtx, pack)
}

// commitPack writes a record that reflects a completed Telegram-side action, on
// a context detached from the request. See commitContext.
func (s *state) commitPack(ctx context.Context, pack Pack) error {
	commitCtx, cancel := commitContext(ctx)
	defer cancel()
	return s.store.Put(commitCtx, packKey(pack.OwnerID), pack)
}

// dropPackRecordIfSet deletes the caller's pack record only when it still names
// setName.
//
// dropPackRecord addresses a record by owner, which is right when the caller is
// acting on the pack they just resolved. It is wrong for a deferred action like
// a /delpack confirmation, which can be pressed after the record has moved on to
// a different pack — deleting by owner then destroys the record of a set that is
// very much alive.
func (s *state) dropPackRecordIfSet(ctx context.Context, ownerID int64, setName string) {
	// Detached like the drop it guards: a cancelled read here would skip a
	// cleanup that the set's confirmed deletion has already made mandatory.
	commitCtx, cancel := commitContext(ctx)
	defer cancel()

	pack, found, err := getPack(commitCtx, s.store, ownerID)
	if err != nil {
		log.Error("sticker_drop_record_check", "user", ownerID, "err", err)
		return
	}
	if !found {
		return
	}
	if !ownsSet(pack, setName) {
		// The record already points at a different pack; leave it alone.
		return
	}
	s.dropPackRecord(commitCtx, ownerID)
}

// dropPackRecord deletes a pack record and frees the name it held.
//
// Every call site reaches here on a positive "this set is gone" signal — either
// isStickerSetMissing, or a confirmed DeleteStickerSet — so the name genuinely
// has no pack behind it any more and must return to the pool. Keeping it would
// shrink the global namespace permanently and let a /newpack + /delpack loop
// burn one name per cycle.
//
// If Telegram reserves deleted short names on its side (plan R11, unverified),
// releasing here is simply a no-op in practice: the next claimant reserves the
// name locally, then CreateNewStickerSet refuses with PACK_SHORT_NAME_OCCUPIED,
// which createRefused releases again. Either way the user gets a correct answer.
//
// Callers must never reach this on a transient failure.
func (s *state) dropPackRecord(ctx context.Context, ownerID int64) {
	// Read before deleting: the record is the only thing that knows which name
	// this owner held. The read shares the delete's detached context — on the
	// request context it would fail during shutdown while the delete below
	// still succeeded, stranding the name permanently.
	commitCtx, cancel := commitContext(ctx)
	defer cancel()

	pack, found, err := getPack(commitCtx, s.store, ownerID)
	if err != nil {
		log.Error("sticker_drop_record_read", "user", ownerID, "err", err)
		// Still drop the record — leaving it would block /newpack — but the
		// name cannot be freed without knowing it.
	}

	if delErr := s.store.Delete(commitCtx, packKey(ownerID)); delErr != nil && !errors.Is(delErr, storage.ErrNotFound) {
		log.Error("sticker_drop_record", "user", ownerID, "err", delErr)
		return
	}

	if err == nil && found && pack.Slug != "" {
		s.releaseSlug(commitCtx, ownerID, pack.Slug)
	}
}

// handleMyPack shows the caller's pack. Makes zero API calls: the count lives
// on the record, which is the whole reason it is stored.
func (s *state) handleMyPack(ctx context.Context, b *bot.Bot, update *models.Update) error {
	ctx, cancel := handlerContext(ctx)
	defer cancel()

	msg := update.Message
	ownerID, err := senderID(msg)
	if err != nil {
		return reply(ctx, b, msg, senderRefusal)
	}

	pack, found, err := getPack(ctx, s.store, ownerID)
	if err != nil {
		log.Error("sticker_mypack", "err", err)
		return reply(ctx, b, msg, genericFailure)
	}
	if !found {
		return reply(ctx, b, msg, noPackYet)
	}

	text := fmt.Sprintf("%s (%s)\n%d sticker(s)\n%s", pack.Title, pack.Slug, pack.Count, shareLink(pack.Name))
	if pack.Pending {
		// Showing an unfinished attempt beats hiding it: the user is blocked
		// from /newpack until it resolves, and re-running the same command is
		// what resolves it.
		text += pendingMarker
	}
	return reply(ctx, b, msg, text)
}

// handleRenamePack changes the pack's display title. The share link cannot
// follow — Telegram has no rename-short-name method.
func (s *state) handleRenamePack(ctx context.Context, b *bot.Bot, update *models.Update) error {
	ctx, cancel := handlerContext(ctx)
	defer cancel()

	msg := update.Message
	ownerID, err := senderID(msg)
	if err != nil {
		return reply(ctx, b, msg, senderRefusal)
	}

	title := strings.TrimSpace(commandArgText(msg))
	if title == "" || len([]rune(title)) > maxTitleLen {
		return reply(ctx, b, msg, fmt.Sprintf("Usage: /renamepack <title...>\nGive a title of 1-%d characters.", maxTitleLen))
	}

	pack, found, err := getPack(ctx, s.store, ownerID)
	if err != nil {
		log.Error("sticker_renamepack_load", "err", err)
		return reply(ctx, b, msg, genericFailure)
	}
	if !found || pack.Pending {
		return reply(ctx, b, msg, noPackYet)
	}

	defer s.lockUser(ownerID)()

	if _, err := b.SetStickerSetTitle(ctx, &bot.SetStickerSetTitleParams{Name: pack.Name, Title: title}); err != nil {
		if isStickerSetMissing(err) {
			s.dropPackRecord(ctx, ownerID)
		}
		return replyAPIError(ctx, b, msg, "sticker_renamepack", err)
	}

	pack.Title = title
	if err := s.commitPack(ctx, pack); err != nil {
		// The rename already happened on Telegram's side; only our copy of the
		// title is stale, and the next successful rename fixes it.
		log.Error("sticker_renamepack_commit", "err", err)
	}

	// Naming the delete-and-recreate route turns a dead end into an answer.
	// A user who typed "rename" with only a title is likely expecting the URL
	// to follow, and it never can.
	return reply(ctx, b, msg, fmt.Sprintf(
		"Renamed to %s.\nThe link is unchanged: %s\n\nTo get a different link you have to /delpack and then /newpack under a new name — the stickers do not come along.",
		title, shareLink(pack.Name)))
}
