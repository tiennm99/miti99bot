---
phase: 4
title: "Phase 4: Sticker commands (reply path)"
status: todo
priority: P1
effort: "5h"
dependencies: [1, 2, 3]
---

# Phase 4: Sticker commands (reply path)

## Overview

Per-sticker operations on packs the caller owns: `/addsticker` (existing-sticker source),
`/delsticker`, `/editsticker`, `/ordersticker`. All are driven by replying to a sticker. The
photo source arrives in Phase 5 through the same `/addsticker` handler.

All handlers follow the plan's cross-cutting rules.

## Requirements

- Functional: add an existing static sticker to one of the caller's packs.
- Functional: remove, re-emoji, and reposition a sticker already in one of those packs.
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

// an EXISTING sticker in one of the caller's packs
type ownedSticker struct {
    fileID string
    pack   Pack
}
func (s *state) resolveOwned(ctx context.Context, msg *models.Message, ownerID int64) (ownedSticker, error)
```

`resolveOwned` is the single ownership gate for `/delsticker`, `/editsticker`,
`/ordersticker`, and (Phase 5) `/setpackicon`:

1. Require `msg.ReplyToMessage.Sticker`; else usage error.
2. Require a non-empty `Sticker.SetName`.
3. `listPacks(ownerID)`, then `matchPack(packs, sticker.SetName)` (Phase 2) — a
   case-insensitive comparison against the **stored** `Pack.Name`.
4. No match → the caller does not own it.
5. **Steps 3 and 4 must produce byte-identical reply text.** Distinct messages would let a
   user probe which slugs exist under other accounts. Pack *management* refusals stay uniform
   even though `/newpack` deliberately discloses slug occupancy (see the plan's Accepted
   disclosure section) — the two are different questions.
6. Reject stickers with `IsAnimated` or `IsVideo`. Static-only module; defence in depth.

Note this uses `matchPack`, not a slug re-derived from the live bot username. Phase 2
explains why (plan R8): a BotFather rename would otherwise make every user's own packs refuse
as "not yours" while `/packlist` still listed them.

`models.Sticker.Emoji` is a single string (`models/sticker.go:23`), so `stickerSource.emoji`
from a replied sticker holds at most one element.

### `/addsticker <pack> [emoji...]`

Reply required. Slug from args, ownership from `store.Get`, source from `resolveSource`.
Emoji precedence: explicit args → the replied sticker's emoji → `defaultEmoji`.

`AddStickerToSet{UserID: ownerID, Name: pack.Name, Sticker: InputSticker{Sticker: fileID, Format: "static", EmojiList: emoji}}`.

`UserID` is the pack owner, always the caller — the module never lets a non-owner reach this
call. Take the per-user keylock. On success, increment `Pack.Count` and commit under
`WithoutCancel`. `STICKERS_TOO_MUCH` maps to "this pack is full (120 stickers)".

### `/delsticker`

No arguments. `resolveOwned`, then `DeleteStickerFromSet{Sticker: fileID}`. On success,
decrement `Pack.Count` (floor 0) and commit.

Whether removing the final sticker also destroys the set is **not documented** in the Bot API
docs or the open-source Bot API server, so the plan does not depend on either answer.

The earlier draft probed with `GetStickerSet` afterwards and deleted the local record when
the probe "reported not-found" — but never defined how not-found differs from a failed call,
and its own success criterion (`FailMethod` → "record confirmed removed") specified the
destructive reading. Under that design a 429, a DNS blip, or a SIGTERM-cancelled context
during a routine delete would erase the only record of a live 49-sticker pack, which plan C5
makes unrecoverable without operator help.

Corrected: **do not probe.** Decrement the count and stop. If the set really is gone, the next
command against it returns `STICKERSET_INVALID`, and the shared handler for that (Phase 3)
deletes the record then — a positive signal, per plan rule 4. This is simpler and strictly
safer than probing.

### `/editsticker <emoji...>`

`resolveOwned`, `parseEmoji` (at least one required — an empty `emoji_list` is invalid), then
`SetStickerEmojiList{Sticker: fileID, EmojiList: emoji}`.

Deviates from the approved command preview, which included a `<pack>` argument — see plan O1.

### `/ordersticker <position>`

`resolveOwned`, parse a non-negative integer, reject negatives locally. 0-based, stated in the
usage text. Do **not** bound the upper end locally — Telegram validates against the current
set size and a local copy would go stale. Its error goes through `replyAPIError`.

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
   `<pack> [emoji...]`, none, `<emoji...>`, `<position>`.
4. Tests per the Todo list.

## Todo

- [ ] `resolveSource` sticker branch (photo branch stubbed for Phase 5)
- [ ] `resolveOwned` using `matchPack`, with the 6-step gate and uniform refusal
- [ ] `/addsticker` with emoji precedence and `Count` increment
- [ ] `/delsticker` with `Count` decrement and **no** probe
- [ ] `/editsticker` requiring at least one emoji
- [ ] `/ordersticker` rejecting negatives locally only
- [ ] Register all four with `Parameters` metadata
- [ ] `resolve_test.go`, `sticker_handlers_test.go`

## Success Criteria

- [ ] Each command's happy path asserts the expected method in `RecordingBot.Sent()`
- [ ] Missing reply, non-sticker reply, and empty `set_name` each produce a usage error with zero API calls
- [ ] Foreign-bot set and another user's slug produce **byte-identical** reply text, asserted by comparing the two replies to each other
- [ ] A sticker whose `SetName` differs only in case from the stored `Pack.Name` resolves successfully
- [ ] `/ordersticker -1` rejected locally; `/ordersticker 999` reaches the API
- [ ] `/editsticker` with no emoji rejected locally
- [ ] `/delsticker` makes exactly one API call and never deletes the `Pack` record
- [ ] `/addsticker` and `/delsticker` move `Count` by exactly one, floored at 0
- [ ] Animated/video sticker reply rejected before any API call

## Risk Assessment

**The uniform-refusal requirement is easy to regress.** A later contributor improving the
error copy could split the two messages and reintroduce the disclosure. Mitigation: the test
asserts equality *between the two paths' replies* rather than asserting two fixed strings, so
the intent survives a rewrite of the copy.

**`resolveOwned` costs a `listPacks` per invocation** — one `List` plus up to ten `Get`s
against storage, no API calls. Bounded by `maxPacksPerUser` and cheap relative to the
network round trip these commands already make. Signal it matters: storage latency shows up
in command timings. Response: cache the caller's packs for the duration of one handler, which
is a local change since every command resolves at most once.

**`Count` drift is user-visible but harmless.** Editing a pack through @Stickers desyncs it.
Phase 3 refreshes it whenever a command already holds a `GetStickerSet` response, so it
self-heals without any command paying for a lookup it did not otherwise need.
