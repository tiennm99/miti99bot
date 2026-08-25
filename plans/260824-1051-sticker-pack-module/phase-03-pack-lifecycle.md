---
phase: 3
title: "Phase 3: Pack lifecycle commands"
status: todo
priority: P1
effort: "8h"
dependencies: [1, 2]
---

# Phase 3: Pack lifecycle commands

## Overview

`/newpack`, `/packlist`, `/renamepack`, `/delpack` (+ confirm callback). These own pack
creation and destruction, and they carry the plan's two hardest correctness problems:
surviving partial failure, and making an irreversible delete safe to confirm.

All handlers follow the plan's cross-cutting rules (explicit deadline, `WithoutCancel`
commits, `senderID`, positive error classification).

## Requirements

- Functional: create a sticker set on behalf of the caller and persist its record such that
  no interruption can permanently strand the slug.
- Functional: list, rename, and delete the caller's own packs, never another user's.
- Functional: `/delpack` confirmation is bound to invoker, chat, message, and a TTL.
- Functional: `/packlist` makes zero API calls.
- Non-functional: a store record must never claim a pack that does not exist, and a live
  pack must never lose its record because of a transient error.

## Architecture

### Factory

Mirrors `internal/modules/coin/coin.go`. `state` holds `store`, `pending` (a second typed
view for delete confirmations), `resolver`, `locks keylock.Map`, `nowFn`. Registry key is
`sticker`; command names stay unprefixed — the registry keys commands by `cmd.Name`
independent of module name (`registry.go:176`), which is why `misc` can ship `/ff`.

`handlerTimeout = 10 * time.Second` is a package constant; every handler opens with it.

### `/newpack <pack> <title...>` — write-ahead intent

The earlier draft did "create on Telegram, then write the store", and accepted that an
interruption strands the slug forever. Two review findings killed that: the commit ran on
`rootCtx`, which SIGTERM cancels, so **every deploy** during a `/newpack` stranded a slug;
and the "cannot adopt" rule was not actually forced by the API's missing owner field (plan
C5). The bot does not need the API to name the owner — it needs its own record of who asked.

1. `senderID(msg)`; `defer s.locks.Acquire(...)()`.
2. Parse slug and title (1–64 chars).
3. Quota check via `listPacks`.
4. `buildSetName(slug, username)`.
5. **`PutVersioned(ctx, key, 0, Pack{Pending: true, …})`** — the create-only primitive
   (`doc_store.go:33-37`; Mongo gives a linearizable single-winner via duplicate-key,
   `mongo_doc_store.go:87-105`). `ErrConflict` means this owner already has a record for
   this slug: if it is `Pending`, resume at step 6 (this is the retry path); if confirmed,
   reply "you already have that pack".
   Use `Put` nowhere here — it is a 5-attempt Get→PutVersioned loop
   (`mongo_doc_store.go:120-142`) that silently overwrites.
6. `GetStickerSet(name)`:
   - **succeeds** → the set exists. We hold a `Pending` record for this owner and slug, so
     this is our own interrupted attempt: **adopt it**, jump to step 8.
     If we did *not* hold a pending record we would not have reached here — step 5 would
     have created one — so adoption is only ever of our own intent.
   - **`isStickerSetMissing`** → the slug is free; proceed to step 7.
   - **any other error** → unknown. Abort, delete the pending record, reply generic failure.
     Never guess (plan rule 4).
7. `CreateNewStickerSet{UserID, Name, Title, Stickers: []InputSticker{{Sticker: fileID, Format: "static", EmojiList: emoji}}}`.
   No top-level `sticker_format` — it moved to `InputSticker.Format` in Bot API 7.2 (C7).
   On `PACK_SHORT_NAME_OCCUPIED`, another user of this bot holds the slug: delete the
   pending record and ask for a different slug.
8. Commit: `Put(context.WithoutCancel(ctx), key, Pack{Pending: false, Count: 1, …})`.
   Reply with the title and `https://t.me/addstickers/<name>`.

Re-running `/newpack` with the same slug after any interruption now completes the operation
instead of reporting it taken. That is the plan's "interrupted `/newpack` can be completed by
re-running" success criterion.

A pending record does consume a quota slot until resolved. Acceptable at
`maxPacksPerUser = 10`; a stale-pending sweep is a follow-up, not this phase.

### `/packlist` — zero API calls

`listPacks(senderID)` rendered with `chathelper.MonospaceTable` (slug, title, `Count`, link).
Pending records render with a "(incomplete — re-run /newpack)" marker rather than being
hidden, so a user can see and fix a stranded attempt.

The earlier design issued one `GetStickerSet` per pack. Under plan C2 each call is bounded
only by the library's 60s `http.Client` timeout (`bot.go:17-18,75-77`), so ten of them on a
serialized dispatcher is a ~10-minute bot-wide freeze from one argument-free public command —
strictly worse than the photo pipeline the plan had been treating as its main risk. Counts
now come from `Pack.Count` (Phase 2).

### `/renamepack <pack> <title...>`

Ownership via `store.Get(packKey(sender, slug))`. `SetStickerSetTitle{Name, Title}`, then
commit the new `Title` under `WithoutCancel`. The reply must state the share link is
unchanged — the Telegram short name is permanent.

**Reverse gap (was undocumented):** if the API succeeds and the commit fails, `/packlist`
shows a title Telegram no longer has. Cosmetic only, self-heals on the next successful
rename. Documented rather than mitigated.

### `/delpack <pack>` — bound confirmation

The earlier design put the slug in the callback data and re-checked ownership from
`CallbackQuery.From.ID`. That defends against *other* users pressing the button and nothing
else: the payload never expires, is not bound to a chat or message, and lives in scrollback
forever. Alice cancels, recreates `memes` months later with 100 stickers, taps the stale
button, and it is destroyed with no fresh intent. Three reviewers flagged it independently,
and the `stock` module the draft cited as its model already solves this properly.

Follow `stock` fully, not half of it:

1. `/delpack` validates ownership, then writes a pending action:
   `pendingDelete{ID, OwnerID, Slug, ChatID, MessageID, ExpiresAt}` with
   `pendingDeleteTTL = 10 * time.Minute` (`stock/pending_dividend.go:12-16,26-34` uses 24h
   for a non-destructive action; a destructive one earns a shorter window).
2. Callback data is `sticker_pack:d:<opaque id>` — an id, not a slug. Well under the 64-byte
   cap (C3), and it decouples the payload from slug length entirely.
3. The callback handler:
   - returns early when `update.CallbackQuery == nil`;
   - resolves the pending action; absent → answer "this confirmation expired or was already
     used" (`stock/dividend_callback.go:66-80` is the model);
   - checks `query.From.ID == action.OwnerID` — identity from `From.ID`, **never** the payload;
   - checks the chat/message binding and `ExpiresAt`;
   - guards `query.Message.Message` for nil — it is a `MaybeInaccessibleMessage`
     (`models/message.go:17-21`) and is nil for messages Telegram marks inaccessible.
     `stock/dividend_callback.go:82-83` already guards exactly this. Phase 1's panic barrier
     is the backstop, not the excuse to skip the guard;
   - deletes the pending action **before** calling `DeleteStickerSet` (single-use);
   - `DeleteStickerSet{Name}`, then `store.Delete` under `WithoutCancel`;
   - `AnswerCallbackQuery` and clear the button via `EditMessageReplyMarkup` with empty
     markup, using `action.ChatID`/`action.MessageID` (`dividend_callback.go:26-33`).

**Reverse gap (was undocumented):** if `DeleteStickerSet` succeeds and `store.Delete` fails,
a phantom record survives — consuming a quota slot, rendering in `/packlist`, and erroring on
every operation. This is worse than an orphan set, because the user cannot see why they are
at their limit. Mitigation: `/packlist` and every command that receives
`isStickerSetMissing` from the API deletes the offending record on the spot, so a phantom
self-heals on first contact.

### Error mapping

`replyAPIError` matches **MTProto code substrings**, never human text (plan rule 4 / R3).
Only `PACK_SHORT_NAME_OCCUPIED`, `PACK_SHORT_NAME_INVALID`, and `STICKER_EMOJI_INVALID` are
rewritten into prose by the Bot API server; everything else arrives as `Bad Request: <CODE>`.

| Match | Reply |
|---|---|
| `PACK_SHORT_NAME_OCCUPIED` / "already occupied" | slug taken, pick another |
| `PACK_SHORT_NAME_INVALID` / "invalid sticker set name" | slug rejected by Telegram |
| `PACK_TITLE_INVALID` | title rejected by Telegram |
| `STICKERSET_INVALID` | that pack no longer exists (and delete the record) |
| `STICKERS_TOO_MUCH` | pack is full (120 stickers) |
| `STICKER_EMOJI_INVALID` / "invalid sticker emojis" | emoji rejected |
| `too many emoji specified` | at most 20 emoji per sticker |
| anything else | generic failure; raw error to the dispatcher log |

## Related Code Files

- Create: `internal/modules/sticker/sticker.go`, `state.go`, `pack_handlers.go`,
  `pending_delete.go`, `delpack_callback.go`, `errors.go`, and their tests
- Reference: `internal/modules/coin/coin.go`, `coin/handlers.go:51` (keylock idiom)
- Reference: `internal/modules/stock/pending_dividend.go:12-34,47-78`,
  `stock/dividend_callback.go:17-33,66-83,99-102`, `stock/dividend_notifications.go:303-320`
- Reference: `internal/storage/doc_store.go:33-41`, `mongo_doc_store.go:87-142`

## Implementation Steps

1. `state.go`, `sticker.go` wiring four commands + the `sticker_pack:` callback.
2. `errors.go` with `replyAPIError` and `isStickerSetMissing`.
3. `/packlist` first — no mutations, no API calls, easiest to verify.
4. `/newpack` with the write-ahead state machine.
5. `/renamepack`.
6. `pending_delete.go`, `/delpack`, and the callback.
7. Tests per the Todo list, using Phase 1's `StubMethod` / `FailMethodCode`.

## Todo

- [ ] `state.go` with store, pending view, resolver, locks, nowFn, `handlerTimeout`
- [ ] `sticker.go` factory registering 4 commands + callback prefix
- [ ] `errors.go`: `replyAPIError` code table + `isStickerSetMissing`
- [ ] `/packlist` reading `Count`, marking pending records, zero API calls
- [ ] `/newpack` steps 1-8 incl. `PutVersioned` intent and adoption
- [ ] `/renamepack` with permanent-link note and `WithoutCancel` commit
- [ ] `pending_delete.go` with TTL, chat/message binding, opaque id
- [ ] `/delpack` emitting the bound confirm keyboard
- [ ] `delpack_callback.go` with expiry, binding, nil-message guard, single-use
- [ ] Record self-heal on `isStickerSetMissing` across commands
- [ ] Tests per the success criteria

## Success Criteria

- [ ] `/packlist` records **zero** entries in `RecordingBot.Sent()`
- [ ] Interrupted `/newpack` (pending record present, set exists) completes on re-run and does not report the slug taken
- [ ] `/newpack` where `GetStickerSet` fails with a non-missing error aborts, deletes the pending record, and never calls `CreateNewStickerSet`
- [ ] `/newpack` uses `PutVersioned(…, 0, …)`; a second create attempt surfaces `ErrConflict` rather than overwriting
- [ ] Foreign-pack access replies with the ownership error and records zero API calls
- [ ] Quota exceeded at 11 packs blocks before any API call
- [ ] `/delpack` confirm after `ExpiresAt` is refused as expired, with no `DeleteStickerSet`
- [ ] `/delpack` confirm from a different `From.ID` is refused, with no `DeleteStickerSet`
- [ ] `/delpack` confirm with a nil `CallbackQuery.Message.Message` is handled without panic
- [ ] Pressing the same confirm twice deletes once; the second press reports already-used
- [ ] Callback data is asserted ≤ 64 bytes
- [ ] A command receiving `STICKERSET_INVALID` deletes the stale record
- [ ] Title of 65 chars rejected locally, before any API call

## Risk Assessment

**The write-ahead state machine is the most intricate logic in the plan.** Its correctness
rests on one property: a `Pending` record for `(owner, slug)` means *this owner* asked for
*this name*, and only this bot can create `*_by_<bot_username>` names. If either half stops
holding, adoption becomes unsafe. Signal: a user reports adopting a pack they did not create.
Response: disable adoption (step 6 becomes "slug taken") and fall back to the documented
orphan gap — a one-line change, deliberately.

**A pending record consumes quota until resolved.** A user who abandons an interrupted
`/newpack` sees 9 usable slots. Visible in `/packlist` with its marker, and re-running the
command clears it. A sweep for pending records older than an hour is a follow-up.

**`/delpack` remains irreversible on Telegram's side.** The TTL and bindings reduce accidental
confirmation; they cannot undo a deliberate one. Do not add a `--force` bypass.
