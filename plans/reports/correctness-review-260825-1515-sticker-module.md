# Correctness / Crash-safety / Concurrency Review — internal/modules/sticker

Date: 2026-08-25 · Reviewer lens: correctness, crash-safety, concurrency.
Out of scope by assignment: cross-user security impact, test quality.
Scope: uncommitted `internal/modules/sticker/` (~1.9k LOC non-test) plus the
modified `cmd/server/main.go`, `internal/modules/dispatcher.go`, and the
storage/keylock contracts they rely on.

`go build ./...` clean. `go vet` clean on sticker/storage/modules.
`go test ./internal/modules/sticker/` passes.

## Verdict

The write-ahead intent state machine is sound. Every /newpack interruption point
recovers or refuses; none wedges the user permanently on paths reachable in this
deployment. Two real findings (H-1, H-2) undermine the *durability* half of the
design rather than its logic. The rest is MEDIUM/LOW.

---

## Findings

### H-1 (HIGH) — `commitContext`'s SIGTERM protection is defeated by the shutdown path

`cmd/server/main.go:214-228`, `internal/modules/sticker/state.go:53-59`

`commitContext` uses `context.WithoutCancel(ctx)` + 5s so a post-action commit
survives SIGTERM. That only defends against *context* cancellation. The process
does not wait for it:

```
go func() { b.Start(rootCtx) }()   // main.go:214 — return value never awaited
<-rootCtx.Done()                   // main.go:220
srv.Shutdown(shutdownCtx)          // HTTP only
}                                  // main returns -> defer closeProvider() -> process exits
```

`b.Start` *would* drain correctly — with `WithNotAsyncHandlers` + 1 worker
(`telegram/client.go:28`, lib `defaultWorkers = 1`) its `wg.Wait()` blocks until
the inline handler returns — but `main` never joins that goroutine. On SIGTERM
`main` proceeds as soon as `srv.Shutdown` finishes (immediate with no in-flight
HTTP), runs `defer closeProvider()` (main.go:125), and exits. The detached
commit gets milliseconds, then the Mongo client is disconnected under it.

Failure scenario: deploy lands while user runs `/newpack foo Bar`.
`CreateNewStickerSet` returns 200; `finishNewPack` → `commitPack` → detached
`Put` starts; SIGTERM arrives; process exits before the write lands. Record stays
`Pending:true`. Recoverable (re-run `/newpack foo …` adopts), so not data loss —
but the write-ahead recovery path is exercised routinely rather than rarely, and
the comment's claim ("must not be lost because the process is shutting down") is
false as built.

Same exit kills `adjustCount`'s commit (count silently under-counts, never
self-corrects) and `renamePack`'s commit (stored title diverges from Telegram
permanently).

Fix direction: have `main` wait for the polling goroutine before returning —
e.g. `pollDone := make(chan struct{}); go func(){ b.Start(rootCtx); close(pollDone) }()`
then `select { case <-pollDone: case <-time.After(shutdownGrace): }` before
`srv.Shutdown` / `closeProvider`. Grace must exceed `commitTimeout` (5s).

### H-2 (HIGH) — every reply is sent on the same exhausted context the API work drained

`internal/modules/sticker/state.go:62-65` and all nine handlers.

`handlerContext` = 10s for the whole handler. Nothing reserves a tail for the
reply. `chathelper.Reply` sends with that same ctx, so once the budget is spent
the user gets **no message at all** — success or failure.

Concrete: `/newpack mypack My Pack` replying to a **photo**.
`resolveSource` → `downloadFile` (own 8s client timeout, but also bounded by the
handler ctx) → `toStickerPNG` (CatmullRom on up to 4096×4096) → `UploadStickerFile`.
On a slow link that is 7-9s. `CreateNewStickerSet` then runs on <1-3s and
returns `context.DeadlineExceeded` — which is correctly *not* `createRefused`,
so intent + reservation are kept — and `replyAPIError` then calls
`reply(ctx, …)` on the dead context. User sees silence, has a `Pending` record
and a burned reservation, and no indication what happened. `/mypack` does show
the pending marker, so it is discoverable, not wedged.

The repo already has the fix pattern and this module is the only one not using
it: `chathelper.FetchContext` reserves a 3s reply tail and is used by
`coin/views.go:35`, `gold/handlers.go:29,185`, `stock/stock_events.go:78`,
`monkeyd/tags_command.go:81`.

Fix direction: derive `fetchCtx, cancel := chathelper.FetchContext(ctx)` for the
download/upload/Telegram calls and keep the outer `ctx` for `reply`.

### M-1 (MEDIUM) — post-action cleanup helpers *read* on the cancellable context

`pack_handlers.go:179` (`releaseSlug`), `:397` (`adjustCount`), `:432`
(`dropPackRecordIfSet`), `:464` (`dropPackRecord`).

Each of these runs *after* a confirmed Telegram-side action, and each carefully
wraps its **write** in `commitContext` — but performs the **read** it depends on
with the caller's cancellable `ctx`. A cancelled/expired ctx therefore silently
skips the write.

- `adjustCount:397` — ctx expired right after a successful `AddStickerToSet` →
  `getPack` fails → count increment never persisted. The delta is lost forever
  (the next adjust reads the stale base). Handler falls back to an in-memory
  `pack.Count++` purely for the reply, so the user is told a number that was
  never stored.
- `releaseSlug:179` — ctx expired in the `createRefused` branch
  (`pack_handlers.go:351-354`) → reservation read fails → name stays reserved
  with no pack behind it. The owner can re-reserve (owner check passes), so the
  loss is only to other users' namespace, but it is permanent.
- `dropPackRecord:464` — read fails → record is still deleted (correct: keeping
  it would block `/newpack`) but the slug can never be freed, because the record
  was the only thing that knew the name.

Fix direction: derive the commit context once at the top of each helper and use
it for both the read and the write.

### M-2 (MEDIUM) — emoji: nine valid RGI emoji are refused outright

`emoji.go:130-156` (`isEmojiRune`). Verified by running `parseEmoji` against
each codepoint:

| Input | Result |
|---|---|
| `©️` U+00A9, `®️` U+00AE | refused |
| `〰️` U+3030, `〽️` U+303D | refused |
| `㊗️` U+3297, `㊙️` U+3299 | refused |
| `Ⓜ️` U+24C2 | refused |
| `⤴️` U+2934, `⤵️` U+2935 | refused |

All are in Telegram's emoji keyboard. `™️` U+2122 and `ℹ️` U+2139 are special-cased
but their neighbours are not. `/editsticker ©️` fails with "is not an emoji".

Fix direction: add U+00A9, U+00AE, U+2934-2935, U+3030, U+303D, U+3297, U+3299,
U+24C2 to the singleton/range list (2900-297F would also cover the arrows).

### M-3 (MEDIUM) — emoji: tag-sequence flags shatter, and the refusal prints raw tag characters

`emoji.go:69-102`. `isBinding` covers Mn/Me but tag characters (U+E0020-E007F)
are category **Cf**, so `🏴󠁧󠁢󠁥󠁮󠁧󠁿` (England/Scotland/Wales flags) splits into
`["🏴", "\U000e0067", "\U000e0062", …]`. Verified output:

```
in="🏴\U000e0067\U000e0062\U000e0065\U000e006e\U000e0067\U000e007f"
clusters=["🏴" "\U000e0067" … ] err="\U000e0067" is not an emoji.
```

Two problems: a legitimate emoji is rejected, and `%q` renders an invisible tag
char, so the user is told `"\U000e0067" is not an emoji`.

Fix direction: treat U+E0020-U+E007F as binding, terminating the cluster at
U+E007F (cancel tag).

### M-4 (MEDIUM) — emoji: three inputs pass validation and send an invalid `emoji_list` to Telegram

`emoji.go:69-102`, `:117-128`. Verified:

| Input | Clusters produced | Sent to Telegram |
|---|---|---|
| `😀‍` (trailing ZWJ) | `["😀‍"]` | yes → `STICKER_EMOJI_INVALID` |
| `😀‍🇻🇳` | `["😀‍🇻", "🇳"]` — the ZWJ branch (`emoji.go:83-88`) swallows the first regional indicator, orphaning the second | yes |
| `🇻🇳🇺` (odd RI count) | `["🇻🇳", "🇺"]` — lone RI passes `isEmojiRune` via `emoji.go:154` | yes |

Not a crash: `apiRefusal` maps `STICKER_EMOJI_INVALID` to "Telegram rejected
those emoji", so the user gets a sane message after one wasted API round-trip.
Correctness bug, low impact.

Fix direction: reject a cluster ending in ZWJ; do not classify a lone regional
indicator as emoji; do not let the ZWJ branch consume a regional indicator.

### L-1 (LOW) — `/newpack` does all its expensive work before checking whether the caller already has a pack

`pack_handlers.go:68-99`. `resolveSource` (photo path: GetFile + up to 2 MB
download + resize + `UploadStickerFile`) and `resolver.resolve` (GetMe) run
*before* the lock and before the "you already have a pack" pre-check. A user who
already owns a pack pays a full download+upload and creates a file on Telegram's
servers, then is refused. Wasted work only; no state divergence. Moving the
pre-check above `resolveSource` would also buy back budget for H-2 — but note
the pre-check must stay *after* `lockUser` and *before* `reserveSlug`, which is
the ordering the comment at `:85-91` is defending.

### L-2 (LOW) — `dropPackRecord` is called both inside and outside `lockUser`

Inside: `handleAddSticker` (`sticker_handlers.go:80`), `handleDelSticker` (`:129`),
`handleRenamePack` (`pack_handlers.go:544`).
Outside: `handleEditSticker` (`sticker_handlers.go:172`), `handleOrderSticker`
(`:210`), `handleSetPackIcon` (`setpackicon.go:45`).

`dropPackRecord` is a read → delete → release-slug sequence. Unlocked call sites
could delete a record another handler just committed.

**Not reachable in production today**: dispatch is inline with one worker
(`bot.WithNotAsyncHandlers()`, `defaultWorkers = 1`), the sticker module
registers no cron job, and the dispatcher's detached per-command stats hook
(`dispatcher.go:80-88`) writes only to the `stats` collection. Latent only —
flagging because the inconsistency reads as an oversight rather than a decision.

Same class, same reachability: `handleAddSticker` resolves `pack` at
`sticker_handlers.go:41` *before* `defer s.lockUser(...)()` at `:66`, then uses
`pack.Name` for the API call. `adjustCount` correctly re-reads under the lock,
but the API call itself uses the pre-lock value. `handleRenamePack` likewise
reads at `pack_handlers.go:531` and `Put`s that whole stale record at `:550`,
which would clobber a concurrent `Count` change.

### L-3 (LOW) — `/ordersticker` reports a position Telegram may not have honoured

`sticker_handlers.go:196-222`. Upper bound is deliberately delegated to Telegram
(correct — a local `Count` is advisory). But if `SetStickerPositionInSet` clamps
an out-of-range position instead of erroring, the reply "Moved to position N"
states something false. Consider "Moved." or re-reading the set.

### L-4 (LOW) — `photoFileID` picks by `FileSize`, which may be absent

`photo.go:57-63`. `PhotoSize.FileSize` is optional in the Bot API. If Telegram
omits it for every size, all compare equal to 0 and `Photo[0]` — the *smallest*
thumbnail — is chosen, yielding a blurry sticker. Telegram populates it in
practice. Tie-break on `Width*Height` instead.

### L-5 (LOW) — state divergence on a DB reset / collection drop

`pack_handlers.go:313-320`. `createOrAdopt`'s `err == nil` branch adopts any
existing set under `<slug>_by_<bot>`, and its safety rests entirely on the
reservation table being authoritative. If the module's collection is dropped or
the same bot token is pointed at a fresh Mongo, the reservations vanish while the
Telegram sets do not: the next claimant of a previously-used slug reserves it
cleanly (`created == true`), then adopts the *previous* owner's set, and the
record's `Count = 1` (`:363-365`) will disagree with the real set.

The cross-user impact belongs to the security reviewer; noting it here only as a
Count/ownership divergence and an operational constraint. Fix direction is
operational, not code: never drop this collection while packs exist, or gate
adoption on `created == false`.

### Nit — `apiRefusal` hardcodes `120` instead of `maxStickersPerPack`

`errors.go:76` says "Your pack is full (120 stickers)." while
`sticker_handlers.go:17` defines the const. The const is otherwise referenced
only from tests.

---

## /newpack interruption-point table

Notation: **R** = slug reservation (`slug:<slug>`), **P** = Pack record.
"Next `/newpack <same slug>`" and "Next `/newpack <other slug>`" are the two
recovery entry points. All rows traced against `handleNewPack`
(`pack_handlers.go:45-118`) and its four helpers.

| # | Interruption point | State left behind | Next `/newpack` **same** slug | Next `/newpack` **different** slug | Wedged? |
|---|---|---|---|---|---|
| 1 | Before `reserveSlug` (arg/slug/title/source/GetMe failure, `:50-81`) | none | normal create | normal create | no |
| 2 | Between pre-check (`:92`) and `reserveSlug` write | none | normal create | normal create | no |
| 3 | After R written, before `claimSlug` (`:101-106`) | R only | `reserveSlug` conflict → own → resume (`created=false`), `claimSlug` creates P, create proceeds | R(old) orphaned; new R created; P created for new slug. Old R leaks (owner-held, re-reservable by owner) | no |
| 4 | `claimSlug` fails/answers with `created=true` (`:106-115`) | R released at `:112` | normal create | normal create | no |
| 5 | After P(pending) written, before `GetStickerSet` (`:106→313`) | R + P(pending) | `claimSlug` → `existing.Slug == slug` → resume → `GetStickerSet` missing → create | `resolveStaleIntent` (`:253`): R(old) held by caller, `GetStickerSet(old)` **missing** → `Put(new intent)`, `releaseSlug(old)` → create | no |
| 6 | `GetStickerSet` returns unknown error (`:325-330`) | R + P(pending) unchanged — **deliberately untouched** | retry; succeeds once Telegram answers | as row 5 | no |
| 7 | Between `GetStickerSet`(missing) and `CreateNewStickerSet` (`:337`) | R + P(pending) | resume → create | as row 5 | no |
| 8 | `CreateNewStickerSet` returns a `createRefused` code (`:351-354`) | R released, P dropped | clean retry (same refusal until the cause changes) | clean create | no |
| 9 | `CreateNewStickerSet` returns a non-refusal error (timeout/429/SIGTERM) — **set may or may not exist** (`:355`) | R + P(pending) kept | `GetStickerSet` decides: exists → adopt + commit; missing → create. Both correct | `resolveStaleIntent` probes old name: exists → **adopt old**, tell user "restored"; missing → release old, create new | no |
| 10 | Create succeeded server-side, process dies before `finishNewPack` | R + P(pending); set exists | `GetStickerSet` → exists → `finishNewPack(adopted=true)`, `Count=1` | `resolveStaleIntent` → old set exists → adopt, refuse the new name with "restored" | no |
| 11 | `commitPack` inside `finishNewPack` fails (`:366-369`) | R + P(pending); set exists | as row 10 → adopt + commit | as row 10 | no |
| 12 | Process exits during the detached `commitPack` (**H-1**) | identical to row 11 | as row 10 | as row 10 | no |
| 13 | `resolveStaleIntent` dies between `Put(new intent)` (`:296`) and `releaseSlug(old)` (`:300`) | R(old) orphaned + R(new) + P(new, pending) | resume new slug → create | probes new slug's set | no; old R leaks permanently to other users |
| 14 | P(pending) exists but its R is now held by someone else | P(pending) + foreign R | `claimSlug` → `existing.Slug == slug` → resume → `GetStickerSet` → **if the other holder created it, this adopts their set** | `resolveStaleIntent` → `held.OwnerID != caller` → drop dead intent, proceed cleanly | see L-5 |

Row 14 is only reachable via the L-5 reservation-loss scenario; in normal
operation `reserveSlug` runs before `claimSlug`, so a pending P always implies
the caller held R at the moment P was written, and no code path transfers R
between users (`releaseSlug:187` re-verifies ownership).

**Escape hatch verified**: `handleDelPack` (`delpack_callback.go:22`) does *not*
require `!Pending`, so a user stuck on a pending record can always clear it.
`DeleteStickerSet` on a never-created set returns `STICKERSET_INVALID`, which
`isStickerSetMissing` treats as success (`:180`), dropping P and releasing R.

---

## Verified sound

Checked and found correct; no action needed.

**State machine / ordering**
- Pre-check "already has a finished pack" is inside `lockUser` and before
  `reserveSlug` (`:83-99`). Reversing those two is the name-burning primitive the
  comment describes; the order is right.
- `claimSlug` uses `PutVersioned(…, 0, …)` (create-only), never `Put`. Both
  backends give exactly one winner: Mongo via the version-0/absent filter +
  upsert + `_id` duplicate-key (`mongo_doc_store.go:85-105`); memory via
  `ErrConflict` when the key exists (`memory_provider.go:107-110`).
- `created` is threaded correctly: `reserveSlug` returns `false` when it merely
  resumes an existing reservation (`:165`), so the bail path at `:111` never
  releases a reservation predating the invocation.
- `releaseSlug` re-verifies `held.OwnerID == ownerID` inside the operation
  (`:187`), not at the call site.
- `dropPackRecord` reads before deleting so the slug is still known (`:464-480`),
  and deletes the record before releasing the name — the safe ordering (the
  reverse would free a name while a record still claims it).
- `dropPackRecordIfSet` guards on `ownsSet` (`:440`) so a stale `/delpack`
  confirmation cannot erase a newer pack's record.
- `resolveStaleIntent` re-proves reservation ownership rather than inferring it
  from the pending record (`:258-272`).

**Error classification**
- `isStickerSetMissing` requires both `bot.ErrorBadRequest` **and** the
  `STICKERSET_INVALID` substring (`errors.go:43-46`). `context.DeadlineExceeded`,
  `context.Canceled`, 429, and transport errors all fall through — they never
  authorise a record delete. Verified at all six call sites.
- `createRefused` (`errors.go:113-123`) lists only request-validation codes and
  is kept separate from `apiRefusal` despite the overlap. Every code listed
  genuinely proves nothing was created.
- No path infers absence from a generic failure. `createOrAdopt`'s `default`
  branch (`:325-330`) and `resolveStaleIntent`'s (`:303-307`) both change nothing.

**Count**
- `adjustCount` re-reads inside `lockUser` (`:397`) rather than trusting the
  handler's pre-lock copy.
- Clamped at 0 (`:407-410`); cannot go negative.
- Missing record returns `storage.ErrNotFound` and does **not** recreate the
  record (`:402-405`).
- A failed `AddStickerToSet`/`DeleteStickerFromSet` returns before `adjustCount`,
  so a failed API call never moves the count.
- Pack-full (`STICKERS_TOO_MUCH`) returns via `replyAPIError` with no increment.

**Context**
- `commitContext` used at every post-action commit: `commitPack:418`,
  `dropIntent:381`, `dropPackRecord:471`, `releaseSlug:191`,
  `dropPendingDelete:~200`, and the `/delpack` prompt persist.
- Not used where cancellation should apply: `reserveSlug`'s and `claimSlug`'s
  pre-action writes, `resolveStaleIntent`'s intent replacement, and the pending
  action consume in the callback all use the cancellable ctx. Correct.
- Every `context.WithTimeout` has a matching `cancel`, all `defer`red. No leaks;
  `go vet` agrees.

**Concurrency**
- `keylock.Map` zero value is usable; `defer s.lockUser(id)()` acquires eagerly
  and defers only the unlock — correct idiom at all six call sites.
- `state`'s zero-value `usernameResolver` and `nowFn == nil` are both safe
  (`state.go:47-51`, `setname.go:97-122`), so `New` not initialising them is fine.
- `usernameResolver.resolve` never caches a failure and holds no lock across the
  `GetMe` call.
- Slug races between two *different* users are resolved by store atomicity, not
  by `keylock` (which is per-user and could not help). Correct choice.
- `/delpack` double-press: the pending action is consumed *before*
  `DeleteStickerSet` (`delpack_callback.go:~183`), so a second press finds
  `ErrNotFound`. Serialized dispatch makes it moot anyway.

**Callback safety**
- `models.CallbackQuery.Message` is a **value** (`MaybeInaccessibleMessage`), not
  a pointer, in `go-telegram/bot v1.20.0`. `query.Message.Message` cannot nil-deref;
  the inner `*Message` nil check is the right and sufficient guard.
- Binding check (`chat + message id + non-zero MessageID`) precedes every side
  effect including `clearButton`.
- `parseDeleteCallback` bounds length to 64 and validates hex before use; the id
  is a lookup key only, never an authorisation input.

**Emoji (correct cases, verified by execution)**
ZWJ families `👨‍👩‍👧‍👦`; skin tone + ZWJ `👩🏽‍🚀`; VS16-then-ZWJ `🏳️‍🌈`;
ZWJ-then-VS16 `🏴‍☠️`; keycaps `1️⃣` `#️⃣`; regional-indicator pairs `🇻🇳`;
skin-tone modifiers `👍🏿`; `⭐` (the default emoji) classifies as emoji;
plain `A` is refused. `len(out) > 20` boundary matches Telegram's 1-20.
`parseEmoji` returns `(nil, nil)` for empty input and `/addsticker` falls back
correctly while `/editsticker` refuses — the right asymmetry.

**Image pipeline**
- `decodeBounded` checks `DecodeConfig` before allocating pixels; rejects
  >4096 per side and `<=0` dimensions as a `userError`.
- `scaleToLongEdge` clamps the short edge to ≥1, so 4096×1 → 512×1: no
  zero-dimension image, no divide-by-zero.
- Compression ladder is a fixed 3-element slice — cannot loop forever. The
  `data, err =` reassignment at `image.go:64` clobbers `data` on encode error,
  but the loop unconditionally reassigns it afterwards, and a loop encode error
  returns `(nil, encErr)`. No nil-with-nil-error return.
- `toThumbnailPNG` offsets are always ≥0 because both scaled dimensions are ≤100;
  `draw.Draw`'s `sp` correctly maps `scaled.Bounds().Min`.
- `downloadFile` bounds by `maxSourceBytes+1` on the reader itself rather than
  trusting `Content-Length`, and closes the body.

**Boundaries**
- `slugRe` `^[a-z][a-z0-9_]{2,39}$` = 3-40 chars, matching `minSlugLen`/`maxSlugLen`;
  `__` and trailing `_` are checked separately as the comment states.
- `makeSetName`'s budget cannot go negative for any legal Telegram username (≤32
  chars → suffix 36 → budget 28).
- No `List` calls anywhere in the module; every lookup is a keyed `Get`. No N+1.
- `senderID` rejects `SenderChat != nil`, so anonymous group admins cannot all
  collapse onto `GroupAnonymousBot`'s single user id.

**Error surfacing**
- `replyErr` shows `userError` verbatim and replaces everything else with
  `genericFailure`; `downloadFile` discards the original error entirely rather
  than wrapping, so the bot token in `FileDownloadLink` cannot reach a log via
  `errors.Unwrap`/`%v`. `classify` inspects only error *types*.
- No path returns `nil` where the caller assumes success. The two "log and
  continue" spots (`renamePack` commit `:550-554`, `adjustCount` fallback
  `sticker_handlers.go:87-92`, `:135-141`) both follow a *confirmed* Telegram
  success, so reporting success to the user is accurate about the pack even
  though the stored count/title may lag.

---

## Recommended actions

1. **H-1** — join the polling goroutine in `main` with a grace period > 5s before
   `srv.Shutdown`/`closeProvider`. Without this the whole `commitContext` design
   is decorative.
2. **H-2** — adopt `chathelper.FetchContext` in the sticker handlers so a slow
   photo pipeline cannot swallow the user's reply.
3. **M-1** — use the commit context for the *read* as well in `adjustCount`,
   `releaseSlug`, `dropPackRecord`, `dropPackRecordIfSet`.
4. **M-2/M-3/M-4** — emoji table additions, tag-sequence binding, and the three
   invalid-cluster rejections.
5. **L-1** — move the "already have a pack" pre-check above `resolveSource`
   (keeping it inside `lockUser` and before `reserveSlug`).
6. **L-2** — make `lockUser` coverage uniform across the six `dropPackRecord`
   call sites, and take the lock before the `getPack` whose value feeds the API
   call in `handleAddSticker`/`handleRenamePack`.
7. L-3, L-4, L-5, nit — at author's discretion.

## Unresolved questions

1. Does Telegram permanently reserve a deleted set's short name? The code
   (`pack_handlers.go:455-458`) documents this as unverified and degrades
   gracefully either way, so it is not blocking — but it decides whether
   `releaseSlug` after a `/delpack` is meaningful or purely local bookkeeping.
2. Does `SetStickerPositionInSet` error or clamp on an out-of-range position?
   Determines whether L-3 is a false success message or a non-issue.
3. Is the sticker collection ever dropped or re-pointed in this deployment's
   operational runbook? That is the sole trigger for L-5 / table row 14.
