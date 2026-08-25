---
phase: 3
title: "Phase 3: Pack lifecycle commands"
status: done
priority: P1
effort: "7h"
dependencies: [1, 2]
---

# Phase 3: Pack lifecycle commands

## Overview

`/newpack`, `/mypack`, `/renamepack`, `/delpack` (+ confirm callback). These own creation and
destruction of the user's single pack, and they carry the plan's two hardest correctness
problems: surviving partial failure, and making an irreversible delete safe to confirm.

All handlers follow the plan's cross-cutting rules (explicit deadline, `WithoutCancel`
commits, `senderID`, positive error classification).

## Requirements

- Functional: create the caller's pack and persist its record such that no interruption can
  permanently strand the slug.
- Functional: show, rename, and delete that pack, never another user's.
- Functional: a second `/newpack` while a pack exists is refused with a clear next step.
- Functional: `/delpack` confirmation is bound to invoker, chat, message, and a TTL.
- Functional: `/mypack` makes zero API calls.
- Non-functional: a store record must never claim a pack that does not exist, and a live pack
  must never lose its record because of a transient error.

## Architecture

### Factory

Mirrors `internal/modules/coin/coin.go`. `state` holds `store`, `pending` (a second typed view
for delete confirmations — the pattern is idiomatic here; `loldle` and `lol` each build three
views over one collection), `resolver`, `locks keylock.Map`, `nowFn`.

Registry key is `sticker`; command names stay unprefixed — the registry keys commands by
`cmd.Name` independent of module name (`registry.go:176`), which is why `misc` ships `/ff`.

`handlerTimeout = 10 * time.Second` is a package constant; every handler opens with it.

> **Superseded during implementation — see plan.md, "post-implementation review:
> global slug reservation".** Step 5 below adopts an existing set on the strength
> of a `Pending` record for this owner and slug. Review proved that insufficient:
> the pack record is keyed by owner, so a user with *no* pack who types someone
> else's slug produces identical evidence and took over their pack. The shipped
> code adds a create-only global reservation (`slug:<slug>` → ownerID) written
> before Telegram is touched, and adopts only when it names the caller. Steps 4-7
> read as implemented **except** that a reservation check precedes step 4, and
> step 5's "abort, delete the pending record" on an unknown error is wrong for
> the same reason rule 4 exists — the shipped code keeps both the intent and the
> reservation unless the refusal is positively classified.

### `/newpack <pack> <title...>` — write-ahead intent

`<pack>` appears here and nowhere else in the module. It fixes the permanent share URL, and
Telegram has no rename-short-name method, so it cannot be corrected later.

An earlier draft did "create on Telegram, then write the store", accepting that an interruption
stranded the slug forever. Two review findings killed that: the commit ran on `rootCtx`, which
SIGTERM cancels, so **every deploy** during a `/newpack` stranded a slug; and the
"cannot adopt" rule was not forced by the API's missing owner field (plan C5). The bot does not
need the API to name the owner — it needs its own record of who asked.

1. `senderID(msg)`; `defer s.locks.Acquire(...)()`.
2. Parse slug and title (1–64 chars).
3. `makeSetName(slug, username)`.
4. **`PutVersioned(ctx, key, 0, Pack{Pending: true, …})`** — the create-only primitive
   (`doc_store.go:33-37`; Mongo gives a linearizable single-winner via duplicate-key,
   `mongo_doc_store.go:87-105`). This *is* the one-pack quota — no separate counter exists.
   On `ErrConflict`, read the record:
   - confirmed → "you already have a pack (`<slug>`). Use /delpack first." Stop.
   - `Pending` with the **same** slug → this is our own interrupted attempt; resume at step 5.
   - `Pending` with a **different** slug → an earlier attempt was interrupted. **Probe
     `GetStickerSet(oldName)` before doing anything.**
     - The old set **exists** → the earlier attempt got as far as creating it. Adopt the old
       set, commit it, and tell the user they already have a pack (`<oldSlug>`) and must
       `/delpack` first if they want the new name. Do **not** overwrite.
     - The old set is **missing** (`isStickerSetMissing`) → nothing was created; overwrite the
       pending record with the new slug and continue.
     - **Any other error** → unknown; abort without touching the record.

     Overwriting unconditionally would orphan a created-but-uncommitted set permanently: the
     set exists and is owned by the user, but adoption keys on the pending slug matching, so
     `/newpack <oldSlug>` would afterwards report "taken" with no route back. The probe is what
     makes the different-slug branch safe.

   Use `Put` nowhere here — it is a 5-attempt Get→PutVersioned loop
   (`mongo_doc_store.go:120-142`) that silently overwrites.
5. `GetStickerSet(name)`:
   - **succeeds** → the set exists. We hold a `Pending` record for this owner and slug, so this
     is our own interrupted attempt: **adopt it**, jump to step 7.
   - **`isStickerSetMissing`** → the slug is free; proceed to step 6.
   - **any other error** → unknown. Abort, delete the pending record, reply generic failure.
     Never guess (plan rule 4).
6. `CreateNewStickerSet{UserID, Name, Title, Stickers: []InputSticker{{Sticker: fileID, Format: "static", EmojiList: emoji}}}`.
   No top-level `sticker_format` — it moved to `InputSticker.Format` in Bot API 7.2 (C7).
   On `PACK_SHORT_NAME_OCCUPIED`, another user of this bot holds the slug: delete the pending
   record and ask for a different one.
7. Commit: `Put(context.WithoutCancel(ctx), key, Pack{Pending: false, Count: 1, …})`.
   Reply with the title and `https://t.me/addstickers/<name>`.

Re-running `/newpack` with the same slug after any interruption completes the operation instead
of reporting it taken. That is the plan's "interrupted `/newpack` can be completed by re-running"
criterion.

### `/mypack` — zero API calls

`getPack(senderID)`. One `Get`. Renders slug, title, `Count`, and the share link, or a short
"you don't have a pack yet — `/newpack <name> <title>`" when absent. A `Pending` record renders
with an "(incomplete — re-run /newpack)" marker rather than being hidden, so a stranded attempt
is visible and fixable.

This replaces the multi-pack `/packlist`, which issued one `GetStickerSet` per pack. Under plan
C2 each call is bounded only by the library's 60s `http.Client` timeout
(`bot.go:17-18,75-77`), so ten of them on a serialized dispatcher was a ~10-minute bot-wide
freeze from one argument-free public command. That failure mode no longer exists.

### `/renamepack <title...>`

`getPack(senderID)`; absent → "you don't have a pack yet". `SetStickerSetTitle{Name, Title}`,
then commit the new `Title` under `WithoutCancel`.

The reply must state the share link is unchanged **and name the route to a different one**:
`/delpack` then `/newpack <new-slug>`. This matters more now that the command takes no slug — a
user typing "rename" with only a title is even likelier to expect the URL to follow, and it
never can. Pointing at the real path turns a dead end into an answer.

Required elements of the reply: the new title, the unchanged link, and the delete-and-recreate
route with its cost stated (the stickers do not come along).

**Reverse gap:** if the API succeeds and the commit fails, `/mypack` shows a title Telegram no
longer has. Cosmetic, self-heals on the next successful rename. Documented, not mitigated.

### `/delpack` — no argument, bound confirmation

An earlier draft put the slug in the callback data and re-checked ownership from
`CallbackQuery.From.ID`. That defends against *other* users pressing the button and nothing
else: the payload never expired, was not bound to a chat or message, and lived in scrollback
forever. Three reviewers flagged it independently, and the `stock` module the draft cited as its
model already solves it properly.

With one pack per user the payload needs no slug at all — but it still needs everything else:

1. `/delpack` resolves the caller's pack, then writes a pending action:
   `pendingDelete{ID, OwnerID, Slug, ChatID, MessageID, ExpiresAt}` with
   `pendingDeleteTTL = 10 * time.Minute`. (`stock/pending_dividend.go:12-16,26-34` uses 24h for
   a non-destructive action; a destructive one earns a shorter window.)

   The confirm prompt must state all four consequences before the tap, since the command itself
   names nothing:

   - the pack title being deleted;
   - **the sticker count that will be lost** (`Pack.Count`);
   - the exact share link that will stop working;
   - that both are permanent.

   `/delpack` is the sanctioned way to change a pack URL, so this prompt is the last point at
   which a user learns the stickers do not survive the change. Understating it here is how
   someone loses 47 stickers expecting a rename.
2. Callback data is `sticker_pack:d:<opaque id>` — comfortably inside the 64-byte cap (C3).
3. The callback handler:
   - returns early when `update.CallbackQuery == nil`;
   - resolves the pending action; absent → "this confirmation expired or was already used"
     (`stock/dividend_callback.go:66-80` is the model);
   - checks `query.From.ID == action.OwnerID` — identity from `From.ID`, **never** the payload;
   - checks the chat/message binding and `ExpiresAt`;
   - guards `query.Message.Message` for nil — it is a `MaybeInaccessibleMessage`
     (`models/message.go:17-21`), nil for messages Telegram marks inaccessible.
     `stock/dividend_callback.go:82-83` already guards exactly this. Phase 1's panic barrier is
     the backstop, not an excuse to skip the guard;
   - deletes the pending action **before** calling `DeleteStickerSet` (single-use);
   - `DeleteStickerSet{Name}`, then `store.Delete` under `WithoutCancel`;
   - `AnswerCallbackQuery` and clear the button via `EditMessageReplyMarkup` with empty markup,
     using `action.ChatID`/`action.MessageID` (`dividend_callback.go:26-33`).

**Reverse gap:** if `DeleteStickerSet` succeeds and `store.Delete` fails, a phantom record
survives — and under one-pack-per-user that is worse than before, because it blocks `/newpack`
entirely rather than consuming one of ten slots. Mitigation: any command receiving
`isStickerSetMissing` from the API deletes the record on the spot, so the phantom clears on
first contact and `/newpack` works again.

### Error mapping

`replyAPIError` matches **MTProto code substrings**, never human text (plan rule 4 / R3). Only
`PACK_SHORT_NAME_OCCUPIED`, `PACK_SHORT_NAME_INVALID`, and `STICKER_EMOJI_INVALID` are rewritten
into prose by the Bot API server; everything else arrives as `Bad Request: <CODE>`.

| Match | Reply |
|---|---|
| `PACK_SHORT_NAME_OCCUPIED` / "already occupied" | slug taken, pick another |
| `PACK_SHORT_NAME_INVALID` / "invalid sticker set name" | slug rejected by Telegram |
| `PACK_TITLE_INVALID` | title rejected by Telegram |
| `STICKERSET_INVALID` | your pack no longer exists (and delete the record) |
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
3. `/mypack` first — no mutations, no API calls, easiest to verify.
4. `/newpack` with the write-ahead state machine.
5. `/renamepack`.
6. `pending_delete.go`, `/delpack`, and the callback.
7. Tests per the Todo list, using Phase 1's `StubMethod` / `FailMethodCode`.

## Todo

- [x] `state.go` with store, pending view, resolver, locks, nowFn, `handlerTimeout`
- [x] `sticker.go` factory registering the module's commands + callback prefix (9 as shipped, once phases 4-5 landed)
- [x] `errors.go`: `replyAPIError` code table + `isStickerSetMissing`
- [x] `/mypack` reading `Count`, marking a pending record, zero API calls
- [x] `/newpack` steps 1-7 incl. `PutVersioned` intent, three `ErrConflict` branches, adoption
- [x] Different-slug pending branch probes `GetStickerSet(oldName)` before overwriting
- [x] `/renamepack` reply: new title, unchanged link, and the /delpack + /newpack route
- [x] `/delpack` confirm prompt: title, sticker count, link, permanence
- [x] `pending_delete.go` with TTL, chat/message binding, opaque id
- [x] `/delpack` naming the pack in its confirm prompt
- [x] `delpack_callback.go` with expiry, binding, nil-message guard, single-use
- [x] Record self-heal on `isStickerSetMissing` across commands
- [x] Tests per the success criteria

## Success Criteria

- [x] `/mypack` records **zero** entries in `RecordingBot.Sent()`
- [x] A second `/newpack` with a confirmed pack present is refused, names the existing slug, and makes zero API calls
- [x] Interrupted `/newpack` (pending record, same slug, set exists) completes on re-run and does not report the slug taken
- [x] Interrupted `/newpack` with a *different* slug where the old set **exists** adopts the old set and refuses the new slug, leaving nothing orphaned
- [x] Interrupted `/newpack` with a *different* slug where the old set is **missing** replaces the pending record and proceeds
- [x] `/newpack` where `GetStickerSet` fails with a non-missing error aborts and never calls `CreateNewStickerSet`, **keeping** the pending record and the reservation — superseded, see the note at the top of this file. Deleting them on an unknown error is what strands a slug: the set may exist, and re-running is how the user recovers.
- [x] `/newpack` uses `PutVersioned(…, 0, …)`; the create path never calls `Put` for a new record
- [x] `/renamepack` with no pack replies "you don't have a pack yet" and makes zero API calls
- [x] `/delpack` confirm after `ExpiresAt` is refused as expired, with no `DeleteStickerSet`
- [x] `/delpack` confirm from a different `From.ID` is refused, with no `DeleteStickerSet`
- [x] `/delpack` confirm with a nil `CallbackQuery.Message.Message` is handled without panic
- [x] Pressing the same confirm twice deletes once; the second press reports already-used
- [x] Callback data is asserted ≤ 64 bytes
- [x] A command receiving `STICKERSET_INVALID` deletes the stale record, unblocking `/newpack`
- [x] Title of 65 chars rejected locally, before any API call
- [x] `/delpack` confirm text contains the pack title, the sticker count, and the share link
- [x] `/renamepack` reply names the `/delpack` + `/newpack` route

## Risk Assessment

**The write-ahead state machine is the most intricate logic in the plan**, and one-pack-per-user
adds a branch rather than removing one: `ErrConflict` now means three different things
(confirmed pack, own pending same-slug, own pending different-slug). Its correctness rests on
one property — a `Pending` record for an owner means *that owner* asked for *that name*, and
only this bot can create `*_by_<bot_username>` names. If either half stops holding, adoption
becomes unsafe. Signal: a user reports adopting a pack they did not create. Response: disable
adoption (step 5 becomes "slug taken") and fall back to the documented orphan gap — a one-line
change, deliberately.

**A stranded pending record now blocks the user entirely.** With ten slots it cost one; with one
pack it blocks `/newpack` until resolved. Mitigated by making it visible in `/mypack` with a
re-run hint, and by the different-slug overwrite branch in step 4 so a user is never wedged by
a name they no longer want.

**`/delpack` remains irreversible on Telegram's side.** The TTL and bindings reduce accidental
confirmation; they cannot undo a deliberate one. Do not add a `--force` bypass.
