---
phase: 4
title: "Phase 4: Sticker commands (reply path)"
status: done
priority: P1
effort: "4h"
dependencies: [1, 2, 3]
---

# Phase 4: Sticker commands (reply path)

## Overview

Per-sticker operations on the caller's pack: `/addsticker` (existing-sticker source),
`/delsticker`, `/editsticker`, `/ordersticker`. All are driven by replying to a sticker. The
photo source arrives in Phase 5 through the same `/addsticker` handler.

None of these takes a pack argument. Under one-pack-per-user there is nothing to name.

All handlers follow the plan's cross-cutting rules.

## Requirements

- Functional: add an existing static sticker to the caller's pack.
- Functional: remove, re-emoji, and reposition a sticker already in that pack.
- Non-functional: ownership is checked before any API call, and the refusal text is identical
  for "another user's pack" and "another bot's pack".
- Non-functional: no transient error may delete a live pack's record.

## Architecture

### Shared resolution

```go
// source of a NEW sticker: whatever the replied message carries. Resolution
// always ends in a file_id usable as InputSticker.Sticker.
type stickerSource struct {
    fileID string   // static sticker file_id, or a freshly uploaded one (Phase 5)
    emoji  []string // from the replied sticker; at most one element
}
func (s *state) resolveSource(ctx context.Context, b *bot.Bot, ownerID int64, msg *models.Message) (stickerSource, error)

// an EXISTING sticker in the caller's pack
type ownedSticker struct {
    fileID string
    pack   Pack
}
func (s *state) resolveOwned(ctx context.Context, msg *models.Message, ownerID int64) (ownedSticker, error)
```

`resolveSource` takes `ctx`, `b`, and `ownerID` from the start even though the sticker branch
uses none of them. Phase 5's photo branch needs all three (`GetFile`, an HTTP download,
`UploadStickerFile{UserID}`), and it lives **inside this function**. Declaring the full
signature now means Phase 5 adds a branch instead of rewriting every call site. There is no
`photoRef` field: the photo path resolves to a `fileID` like every other path.

Its gate:

1. Require `msg.ReplyToMessage`; else usage error.
2. Sticker branch — reject `IsAnimated`, `IsVideo`, **and `Type != "regular"`**
   (`models/sticker.go:16`). A mask or custom-emoji sticker is static yet invalid for a
   regular set, so the `IsAnimated || IsVideo` pair alone does not close this. The static-only
   gate belongs here, on the path that can actually receive a non-static sticker — not only in
   `resolveOwned`, which by construction only ever sees stickers already in a static pack.
3. Otherwise (Phase 5) the photo/document branch; until then, a usage error.

`resolveOwned` is the single ownership gate for `/delsticker`, `/editsticker`, `/ordersticker`,
and (Phase 5) `/setpackicon`:

1. Require `msg.ReplyToMessage.Sticker`; else usage error.
2. Require a non-empty `Sticker.SetName`.
3. Reject `IsAnimated`, `IsVideo`, or `Type != "regular"`. Static-only module; defence in
   depth. **Before** the store read, so a malformed reply costs nothing and the "rejected
   before any API call" criterion holds trivially.
4. `getPack(ownerID)` — **one `Get`**. Absent → the caller has no pack.
5. `ownsSet(pack, sticker.SetName)` (Phase 2) — case-insensitive against the **stored**
   `Pack.Name`. False → the sticker is not from the caller's pack.
6. **Steps 4 and 5 must produce byte-identical reply text.** Distinct messages would let a user
   probe whether a given set belongs to someone else. Pack *management* refusals stay uniform
   even though `/newpack` deliberately discloses slug occupancy (plan's Accepted disclosure
   section) — those are different questions.

This replaced a `listPacks` + match design; with one pack it collapses to a single `Get` and a
string comparison. It still compares against the stored `Pack.Name` rather than a slug
re-derived from the live bot username — Phase 2 explains why (plan R8): a BotFather rename would
otherwise make the user's own pack refuse as "not yours" while `/mypack` still displayed it.

`models.Sticker.Emoji` is a single string (`models/sticker.go:23`), so `stickerSource.emoji`
from a replied sticker holds at most one element.

### `/addsticker [emoji...]`

Reply required. Pack from `getPack`, source from `resolveSource`. Emoji precedence: explicit
args → the replied sticker's emoji → `defaultEmoji`.

**No pack yet** → "you don't have a pack yet — `/newpack <name> <title>`", zero API calls.
This reply is deliberately **not** the uniform `resolveOwned` refusal, and the difference is
not an oversight: `resolveOwned` is uniform because it answers a question about a set the
caller named, which may be someone else's. `/addsticker` answers only "do *you* have a pack",
about the caller's own state, and discloses nothing about anyone else. Being helpful here
costs no privacy. A `Pending` record counts as no usable pack — same reply, plus the
`/mypack` re-run hint from Phase 3.

Every argument is an emoji — there is no pack token to disambiguate, so a stray word is caught
by `parseEmoji` and reported as a usage error rather than silently read as a pack name.

`AddStickerToSet{UserID: ownerID, Name: pack.Name, Sticker: InputSticker{Sticker: fileID, Format: "static", EmojiList: emoji}}`.

`UserID` is the pack owner, always the caller — the module never lets a non-owner reach this
call. Take the per-user keylock. On success, increment `Pack.Count` and commit under
`WithoutCancel`. `STICKERS_TOO_MUCH` maps to "your pack is full (120 stickers)".

**Unverified premise — settle before writing this handler.** The whole sticker-source path
assumes `AddStickerToSet` accepts a `file_id` for a sticker that lives in a set this bot did
not create. The Bot API documents `InputSticker.sticker` as accepting "a file_id as a String
to send a file that already exists on the Telegram servers", and says nothing further — but
unlike C1–C11 this was never checked against the live API, and every other API assumption in
this plan was. If Telegram rejects cross-set reuse, this path collapses into Phase 5's
machinery (`GetFile` → download → `UploadStickerFile` → use the returned `file_id`), which
inverts the 4→5 dependency and is much better known before the handler is written than after.
One live call settles it (plan R12).

### `/delsticker`

No arguments. `resolveOwned`, then `DeleteStickerFromSet{Sticker: fileID}`. On success,
decrement `Pack.Count` (floor 0) and commit under `WithoutCancel`.

**Take the per-user keylock**, exactly as `/addsticker` does. Both run the same
read-modify-write on `Pack.Count`, so locking one and not the other would be a half-measure
that only looks safe. `/editsticker` and `/ordersticker` write nothing and take no lock.
Under the lock, the commit uses plain `Put` — the lock is what makes the read-modify-write
safe, so the `PutVersioned` reasoning from Phase 3 (which guards *creation*, not updates)
does not apply.

Whether removing the final sticker also destroys the set is **not documented** in the Bot API
docs or the open-source Bot API server, so the plan depends on neither answer.

An earlier draft probed with `GetStickerSet` afterwards and deleted the local record when the
probe "reported not-found" — but never defined how not-found differs from a failed call, and its
own success criterion (`FailMethod` → "record confirmed removed") specified the destructive
reading. Under that design a 429, a DNS blip, or a SIGTERM-cancelled context during a routine
delete would erase the only record of a live pack. With one pack per user that is strictly
worse than it was: the user loses their pack *and* is blocked from `/newpack` until the phantom
clears.

Corrected: **do not probe.** Decrement the count and stop. If the set really is gone, the next
command returns `STICKERSET_INVALID`, and the shared handler for that (Phase 3) deletes the
record then — a positive signal, per plan rule 4. Simpler and strictly safer.

**Aftermath of a `Count: 0` pack — state the recovery route.** Not probing means that if
Telegram *did* destroy the set, the user is left holding a record for a pack that no longer
exists. `/mypack` makes zero API calls by design, so it cannot notice; `/newpack` refuses
because a record exists. The user is not wedged — `/delpack` calls `DeleteStickerSet`, gets
`STICKERSET_INVALID`, and Phase 3's self-heal drops the record, freeing `/newpack` — but
nothing in the plan told them that, and this phase is what creates the situation. So: when
the decrement lands on 0, the reply says the pack is now empty, that Telegram may have
removed it, and that `/delpack` clears it if `/addsticker` reports the pack is gone.

This is also why the success criterion below is scoped rather than absolute. `/delsticker`
must never delete the `Pack` record on a **transient or unknown** error — that is finding R7,
the whole reason the probe was removed. It must still delete it on a **positive**
`STICKERSET_INVALID`, which is Phase 3's cross-command self-heal and the mechanism that
unwedges the user. An unqualified "never deletes the record" would forbid the fix.

### `/editsticker <emoji...>`

`resolveOwned`, `parseEmoji` (at least one required — an empty `emoji_list` is invalid), then
`SetStickerEmojiList{Sticker: fileID, EmojiList: emoji}`.

### `/ordersticker <position>`

`resolveOwned`, parse a non-negative integer, reject negatives locally. 0-based, stated in the
usage text. Do **not** bound the upper end locally — Telegram validates against the current set
size and a local copy would go stale. Its error goes through `replyAPIError`.

`SetStickerPositionInSet{Sticker: fileID, Position: pos}`.

## Related Code Files

- Create: `internal/modules/sticker/resolve.go`, `sticker_handlers.go`, and their tests
- Modify: `internal/modules/sticker/sticker.go` (register four commands)
- Reference: `internal/modules/util/handlers_test.go:108-118` — an existing test synthesizing
  `ReplyToMessage` with a `models.Sticker{FileID, FileUniqueID, SetName, Emoji}`. Exactly the
  fixture shape every test here needs.
- Reference: `internal/testutil/update_builders.go` (`NewPrivateMessage`, `NewGroupMessage`)

## Implementation Steps

1. `resolve.go` with both helpers and the deliberately uniform not-owned reply.
2. The four handlers in `sticker_handlers.go`, each opening with `handlerTimeout`.
3. Register with `Parameters` per `docs/command-parameter-conventions.md`:
   `[emoji...]`, none, `<emoji...>`, `<position>`.
4. Tests per the Todo list.

## Todo

- [x] Settle the `file_id`-reuse premise against the live API before writing `/addsticker`
- [x] `resolveSource` with the full `(ctx, b, ownerID, msg)` signature, sticker branch only
- [x] `resolveSource` static-only gate: `IsAnimated`, `IsVideo`, `Type != "regular"`
- [x] `resolveOwned` using `getPack` + `ownsSet`, with the 6-step gate and uniform refusal
- [x] `/addsticker` with emoji precedence and `Count` increment
- [x] `/addsticker` no-pack and pending-pack replies, zero API calls
- [x] `/delsticker` with `Count` decrement, keylock, and **no** probe
- [x] `/delsticker` empty-pack reply naming the `/delpack` recovery route
- [x] `/editsticker` requiring at least one emoji
- [x] `/ordersticker` rejecting negatives locally only
- [x] Register all four with `Parameters` metadata
- [x] `resolve_test.go`, `sticker_handlers_test.go`

## Success Criteria

- [x] Each command's happy path asserts the expected method in `RecordingBot.Sent()`
- [x] Missing reply, non-sticker reply, and empty `set_name` each produce a usage error with zero API calls
- [x] "No pack yet" and "sticker from another bot's set" produce **byte-identical** reply text, asserted by comparing the two replies to each other
- [x] A sticker whose `SetName` differs only in case from the stored `Pack.Name` resolves successfully
- [x] `/addsticker` with a non-emoji argument is rejected by `parseEmoji`, not silently reinterpreted
- [x] `/ordersticker -1` rejected locally; `/ordersticker 999` reaches the API
- [x] `/editsticker` with no emoji rejected locally
- [x] `/delsticker` makes exactly one API call, and keeps the `Pack` record on a transient or unknown error
- [x] `/delsticker` receiving `STICKERSET_INVALID` **does** delete the record (Phase 3 self-heal), unblocking `/newpack`
- [x] `/addsticker` and `/delsticker` move `Count` by exactly one, floored at 0
- [x] A `/delsticker` that lands on `Count: 0` names `/delpack` in its reply
- [x] `/addsticker` with no pack, and with a `Pending` pack, each reply with zero API calls
- [x] `/addsticker` on a full pack maps `STICKERS_TOO_MUCH` to the "pack is full" reply (via Phase 1 `FailMethodCode`)
- [x] Animated, video, **and mask/custom-emoji** (`Type != "regular"`) replies rejected before any API call, on both the source and owned paths

## Risk Assessment

**The uniform-refusal requirement is easy to regress.** A later contributor improving the error
copy could split the two messages and reintroduce the disclosure. Mitigation: the test asserts
equality *between the two paths' replies* rather than asserting two fixed strings, so the intent
survives a rewrite of the copy.

**`resolveOwned` now costs a single `Get`** — no `List`, no fan-out, no API call. This is the
one place the one-pack revision made a correctness-critical path cheaper as well as simpler,
and it removes the caching follow-up the multi-pack version needed.

**`Count` drift is user-visible but harmless.** Editing the pack through @Stickers desyncs it.
Phase 3 refreshes it whenever a command already holds a `GetStickerSet` response, so it
self-heals without any command paying for a lookup it did not otherwise need.
