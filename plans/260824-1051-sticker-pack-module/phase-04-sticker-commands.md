---
phase: 4
title: "Phase 4: Sticker commands (reply path)"
status: todo
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
// source of a NEW sticker: whatever the replied message carries
type stickerSource struct {
    fileID string    // static sticker file_id, or "" when a photo needs uploading
    photo  *photoRef // Phase 5; nil on the sticker path
    emoji  []string  // from the replied sticker; at most one element
}
func (s *state) resolveSource(msg *models.Message) (stickerSource, error)

// an EXISTING sticker in the caller's pack
type ownedSticker struct {
    fileID string
    pack   Pack
}
func (s *state) resolveOwned(ctx context.Context, msg *models.Message, ownerID int64) (ownedSticker, error)
```

`resolveOwned` is the single ownership gate for `/delsticker`, `/editsticker`, `/ordersticker`,
and (Phase 5) `/setpackicon`:

1. Require `msg.ReplyToMessage.Sticker`; else usage error.
2. Require a non-empty `Sticker.SetName`.
3. `getPack(ownerID)` — **one `Get`**. Absent → the caller has no pack.
4. `ownsSet(pack, sticker.SetName)` (Phase 2) — case-insensitive against the **stored**
   `Pack.Name`. False → the sticker is not from the caller's pack.
5. **Steps 3 and 4 must produce byte-identical reply text.** Distinct messages would let a user
   probe whether a given set belongs to someone else. Pack *management* refusals stay uniform
   even though `/newpack` deliberately discloses slug occupancy (plan's Accepted disclosure
   section) — those are different questions.
6. Reject stickers with `IsAnimated` or `IsVideo`. Static-only module; defence in depth.

This replaced a `listPacks` + match design; with one pack it collapses to a single `Get` and a
string comparison. It still compares against the stored `Pack.Name` rather than a slug
re-derived from the live bot username — Phase 2 explains why (plan R8): a BotFather rename would
otherwise make the user's own pack refuse as "not yours" while `/mypack` still displayed it.

`models.Sticker.Emoji` is a single string (`models/sticker.go:23`), so `stickerSource.emoji`
from a replied sticker holds at most one element.

### `/addsticker [emoji...]`

Reply required. Pack from `getPack`, source from `resolveSource`. Emoji precedence: explicit
args → the replied sticker's emoji → `defaultEmoji`.

Every argument is an emoji — there is no pack token to disambiguate, so a stray word is caught
by `parseEmoji` and reported as a usage error rather than silently read as a pack name.

`AddStickerToSet{UserID: ownerID, Name: pack.Name, Sticker: InputSticker{Sticker: fileID, Format: "static", EmojiList: emoji}}`.

`UserID` is the pack owner, always the caller — the module never lets a non-owner reach this
call. Take the per-user keylock. On success, increment `Pack.Count` and commit under
`WithoutCancel`. `STICKERS_TOO_MUCH` maps to "your pack is full (120 stickers)".

### `/delsticker`

No arguments. `resolveOwned`, then `DeleteStickerFromSet{Sticker: fileID}`. On success,
decrement `Pack.Count` (floor 0) and commit.

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

- [ ] `resolveSource` sticker branch (photo branch stubbed for Phase 5)
- [ ] `resolveOwned` using `getPack` + `ownsSet`, with the 6-step gate and uniform refusal
- [ ] `/addsticker` with emoji precedence and `Count` increment
- [ ] `/delsticker` with `Count` decrement and **no** probe
- [ ] `/editsticker` requiring at least one emoji
- [ ] `/ordersticker` rejecting negatives locally only
- [ ] Register all four with `Parameters` metadata
- [ ] `resolve_test.go`, `sticker_handlers_test.go`

## Success Criteria

- [ ] Each command's happy path asserts the expected method in `RecordingBot.Sent()`
- [ ] Missing reply, non-sticker reply, and empty `set_name` each produce a usage error with zero API calls
- [ ] "No pack yet" and "sticker from another bot's set" produce **byte-identical** reply text, asserted by comparing the two replies to each other
- [ ] A sticker whose `SetName` differs only in case from the stored `Pack.Name` resolves successfully
- [ ] `/addsticker` with a non-emoji argument is rejected by `parseEmoji`, not silently reinterpreted
- [ ] `/ordersticker -1` rejected locally; `/ordersticker 999` reaches the API
- [ ] `/editsticker` with no emoji rejected locally
- [ ] `/delsticker` makes exactly one API call and never deletes the `Pack` record
- [ ] `/addsticker` and `/delsticker` move `Count` by exactly one, floored at 0
- [ ] Animated/video sticker reply rejected before any API call

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
