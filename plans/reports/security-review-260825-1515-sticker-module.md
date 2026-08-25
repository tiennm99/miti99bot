# Security review — sticker module (uncommitted)

Lens: security / abuse resistance only. Style, naming, test coverage out of scope.
Method: traced attacker-controlled inputs (command args, replied message, callback payload,
image bytes) through every store write and every Telegram call that names a set. Third pass,
after the takeover and name-burning fixes.

Verified environment facts used below:
- `internal/telegram/client.go:26-31` — `WithNotAsyncHandlers()`, single worker: updates are
  processed strictly one at a time, inline on the polling goroutine.
- `cmd/server/main.go:266-280` — provider auto-detect: `MONGO_URL` unset ⇒ **memory backend**,
  announced with `log.Warn` only. `"sticker": sticker.New` is registered unconditionally
  (`cmd/server/main.go:95`).
- `go-telegram/bot@v1.21.0/raw_request.go:78-81` — the library redacts the token inside
  `*url.Error.URL` for API-call failures.
- `models.CallbackQuery.Message` is a **value** `MaybeInaccessibleMessage` holding `*Message`,
  so `query.Message.Message` cannot nil-panic.

---

## HIGH — adoption is authorised by a record that is less durable than the object it protects

`internal/modules/sticker/pack_handlers.go:313-321` (adopt branch), `:138-166` (reserveSlug),
`cmd/server/main.go:266-280` (backend selection).

The reservation is the *only* evidence that an existing Telegram set belongs to the caller —
`GetStickerSet` exposes no owner. The reservation lives in the module's store; the sticker set
lives on Telegram forever. Any event that empties the store while the sets survive re-opens the
exact cross-user takeover the reservation was added to close, with no code change.

Exploitation (memory backend variant — reachable by omitting `MONGO_URL`, which only logs a Warn):

1. Victim V: `/newpack cool My Pack` → set `cool_by_<bot>` created, share link is public by design.
2. Bot restarts (deploy, OOM, VM reboot). Memory store is empty; Telegram set untouched.
3. Attacker A (any user, no pack) replies to any sticker with `/newpack cool Whatever`.
   - `reserveSlug` → no reservation exists → created for A.
   - `claimSlug` → no pack record for A → pending intent written.
   - `createOrAdopt` → `GetStickerSet("cool_by_<bot>")` returns **nil error** →
     `finishNewPack(..., adopted=true)` → A's record now owns V's set.
4. A gains, via calls that carry no owner scoping at all:
   - `/delpack` → `DeleteStickerSet{Name}` — **destroys V's pack permanently** (no user_id param).
   - `/delsticker` replying to any sticker from V's public pack — `resolveOwned` passes because
     `pack.Name == st.SetName`; `DeleteStickerFromSet{Sticker}` takes only the file id.
   - `/renamepack` → `SetStickerSetTitle{Name,Title}` — also unscoped.
   V is simultaneously locked out: V's `/newpack cool` answers `slugTaken`, `/mypack` says no pack.

Same primitive without the memory backend: collection dropped, `MONGO_DATABASE` changed, module
renamed, or a Mongo restore from a backup older than the newest sets.

Why the existing guards do not stop it: every guard (reservation owner check, `created` scoping,
`ownsSet`, uniform refusals) reasons entirely inside the local store. When the store is empty the
guards are all *satisfied*, and the adopt branch is by design the path that turns "a set exists
under a name I hold" into ownership.

Fix direction (cheap and precise, no new state): adoption should require that the reservation
**pre-dated this invocation**. `reserveSlug` already computes exactly that as `created`; plumb it
into `createOrAdopt` and, when `created == true` and `GetStickerSet` succeeds, refuse with
`slugTaken` (plus release the just-made reservation) instead of adopting. A genuine interrupted
attempt always re-enters with `created == false` (its reservation was written by the earlier run),
and a set cannot exist for a reservation first written microseconds ago in this same handler — so
this has no false negatives, and the store-wipe path can no longer adopt anything.
Secondly: refuse to build/register this module on a non-durable provider (or `log.Fatal` when
`KV_PROVIDER=memory` and sticker is enabled) — the module creates permanent, globally visible
Telegram objects and must not run on a store documented as "data lost on restart".

## MEDIUM — cleanup helpers read on the request context but write on a detached one; slugs leak permanently

`internal/modules/sticker/pack_handlers.go:179` (`releaseSlug` → `getSlugReservation(ctx, …)`),
`:464` (`dropPackRecord` → `getPack(ctx, …)`), `:432` (`dropPackRecordIfSet` → `getPack(ctx, …)`).

`commitContext` (`state.go:59-61`) exists precisely because SIGTERM cancels `rootCtx` mid-handler.
It is applied to the `Delete`/`Put` calls in these helpers but **not** to the reads that decide what
to delete. The reads therefore fail exactly in the situation the detached write was designed for.

Scenario (ordinary deploy, no attacker needed): user presses the `/delpack` confirm button;
`DeleteStickerSet` succeeds; SIGTERM lands (or the 10s `handlerTimeout` expires — the handler has
already made 2-4 API calls by then).
- In `dropPackRecordIfSet`, `getPack` fails → returns early → the record survives naming a set that
  no longer exists. Self-heals on the next command via `STICKERSET_INVALID`, so this half is benign.
- In `dropPackRecord` (reached from any `isStickerSetMissing` path), `getPack` fails, the pack record
  is deleted anyway on the detached context, and `pack.Slug` is never known, so the reservation is
  never released. Result: a `slug:` document with no pack and no set behind it, held against every
  other user **forever** — nothing in the module can free it (`releaseSlug` needs both owner and
  slug, and the only record of the slug was just deleted). Manual DB surgery is the only recovery.
- `handleNewPack:111-113` has the same shape: a deadline-exceeded bail calls `releaseSlug` with the
  dead context, so the "release only what this invocation created" repair silently no-ops.

Fix direction: derive the commit context once at the top of `releaseSlug` / `dropPackRecord` /
`dropPackRecordIfSet` and use it for the read as well as the write. These reads are part of the
commit, not part of serving the request.

## MEDIUM — uncancellable image work stalls every user of the bot

`internal/modules/sticker/image.go:35` (`maxDecodeDimension = 4096`), `:63-79` (the fallback ladder,
which re-scales from the **full-size** source `img` on every rung), reached from `/addsticker`,
`/newpack` and `/setpackicon`.

`toStickerPNG` takes no context and checks none, so `handlerTimeout` bounds nothing here, and
handlers are strictly serialized (one worker), so this is a whole-bot stall, not a per-user one.
Measured on this box (ARM64, Go 1.27) with a 4096×4096 PNG of random 8×8 blocks — 1,278,612 bytes,
comfortably under the 2 MiB `maxSourceBytes` cap, and its 512px downscale is per-pixel noise, so
every PNG encode overshoots `softMaxStickerBytes` and the full ladder runs:

```
decode                175 ms
512 DefaultCompression 553 ms  → 787,362 B  (> 512 KiB, ladder continues)
512 BestCompression     41 ms  → 773,200 B
448 BestCompression    522 ms  → 568,571 B
384 BestCompression    513 ms  → 411,425 B
320 BestCompression    475 ms  → 280,071 B
TOTAL                 2.28 s   of uninterruptible CPU per message
```

One user resending that image faster than every 2.3s keeps the single dispatch goroutine saturated;
all other users' commands queue behind it. Peak live memory is also ~64 MB for the decoded source
alone (as the comment at `image.go:29-34` acknowledges).

Fix direction: scale the ladder rungs from the already-downscaled 512 image instead of `img` (drops
three of the four expensive 4096²→N CatmullRom passes); lower `maxDecodeDimension` to ~1536-2048
(the target is 512px, so nothing above that adds quality); optionally take `ctx` and bail between
rungs.

## LOW — `/newpack` pays for the image before the check that refuses the caller

`internal/modules/sticker/pack_handlers.go:68` (`resolveSource`) runs before the lock, before the
"you already have a pack" pre-check at `:92`, and before `reserveSlug`. A user who already owns a
pack can make the bot download up to 2 MB, run the full conversion above, and call
`UploadStickerFile` on every `/newpack`, only to be refused by a single store read that could have
run first. Not a new primitive (the same work is legitimately available via `/addsticker`), and the
ordering is what keeps name-burning closed, so this is cost, not a hole. Moving the cheap
`getPack` pre-check above `resolveSource` preserves the reserve-after-precheck invariant and removes
the free work.

## LOW — `handleRenamePack` commits a record it read before taking the lock

`internal/modules/sticker/pack_handlers.go:531-551`: `getPack` runs at `:531`, the lock is taken at
`:540`, and `commitPack` at `:550` writes the whole document (`Count`, `Pending`, `Name`, …) from
that pre-lock read. `adjustCount:388-395` documents exactly why that is wrong and re-reads inside
the lock; rename does not. `handleDelPack:32-83` likewise reads and writes the pending record with
no lock at all. Neither is exploitable today — `WithNotAsyncHandlers` + one worker means no two
handlers ever interleave — so the locks are currently decorative and these are latent regressions
that surface the day async handlers or a second replica are introduced.

---

## Attacked and held

Callback path (`delpack_callback.go`), payload fully attacker-chosen:
- **Address someone else's confirmation** — held: the store lookup key is
  `pendingDeleteKey(query.From.ID)` (`:108`); the payload id is never used to select *whose* action
  loads, only compared for equality at `:144`.
- **Press a bystander's button in a group** — held: `msg.Chat.ID != action.ChatID || msg.ID !=
  action.MessageID` (`:137`) is evaluated *before* any side effect, so the bystander cannot even
  strip the victim's keyboard; `clearButton` only runs after that binding passes.
- **Replay / double-press / two live confirmations** — held: deterministic per-user key
  (`pending_delete.go:64`) so a second `/delpack` supersedes the first, and the action is consumed
  with `pending.Delete` *before* `DeleteStickerSet` (`:159-167`).
- **Stale press after `/delpack` + `/newpack`** — held: `dropPackRecordIfSet` (`pack_handlers.go:431`)
  re-checks `ownsSet` so a stale confirmation cannot erase the record of the *new* live pack.
- **Forwarded copy of the prompt / inaccessible message** — held: `query.Message.Message == nil`
  guard at `:125` before any use, plus the message-id binding.
- **Malformed payload** — held: `parseDeleteCallback` bounds length to 64, requires the prefix, and
  requires lowercase hex; ids are 12 random bytes from `crypto/rand`.
- **Anonymous / bot senders** — held for the command side: `senderID` rejects `IsBot` and
  `SenderChat != nil` (`sender.go:28-38`), so the shared GroupAnonymousBot identity can never own or
  delete a pack; `query.From.ID == 0` is rejected on the callback side.

Reservation lifecycle — every `slug:` create/delete site enumerated:
create at `reserveSlug:139` only; delete at `handleNewPack:112` (only when `created`),
`resolveStaleIntent:300` (only after a positive `STICKERSET_INVALID` on the old name),
`createOrAdopt:353` (only when `createRefused`), `dropPackRecord:479` (only after a confirmed
`DeleteStickerSet` or a positive `STICKERSET_INVALID`). All four funnel through `releaseSlug`,
which re-verifies the holder itself (`:187`) rather than trusting the caller, so a delete-by-name
cross-user primitive does not exist. Unknown/transient errors change nothing
(`createOrAdopt:325-331`, `resolveStaleIntent:303-307`) — verified by
`TestNewPack_UnknownLookupErrorAborts` and `TestNewPack_ResumedReservationSurvivesABail`.
The only leak I could construct is the cancelled-context one filed as MEDIUM above.

- **Name burning at zero API cost** — held: the existing-pack pre-check precedes `reserveSlug`
  (`:92-101`) and the `created` flag stops a bail from releasing a resumed reservation. Sustained
  burning also needs one Telegram account per name, since one pack per user is enforced by the
  create-only `PutVersioned(packKey, 0, …)` and `/delpack` returns the name to the pool.
- **Key-space collision across the three views over one collection** — held:
  `slugRe = ^[a-z][a-z0-9_]{2,39}$` cannot emit `:` or a leading digit, so `"slug:"+slug` is
  disjoint from decimal `packKey` and from `"pending-delete:"+decimal`; `storage.validateKey`
  additionally rejects `/`, `.`/`..` and `__ns__`, and `provider.Collection("sticker")` isolates the
  module (`registry.go:151`). Callback prefixes are conflict-checked bidirectionally
  (`registry.go:219`).
- **Mutating another user's stickers via a replied sticker** — held: `resolveOwned` compares the
  *stored* `Pack.Name` against `Sticker.SetName`, which is authored by Telegram and not settable by
  the sender; a copied sticker lands in the copier's own set with a new file id, and the original
  message still carries the victim's `set_name`.
- **Enumeration** — held: `slugTaken` (`pack_handlers.go:23`) is byte-identical to the
  `PACK_SHORT_NAME_OCCUPIED` mapping in `apiRefusal` (`errors.go:62`), so "reserved in this bot" and
  "occupied on Telegram" are indistinguishable; `notOwnedRefusal` is the single answer for no-pack,
  pending-pack, and foreign-set. Raw Telegram descriptions never reach a reply — `replyAPIError`
  maps or genericises.
- **Bot-token leakage** — held on every path I could reach: the download path discards the original
  error entirely (`errDownloadFailed`, `download.go:36-83`, no `%w` of the transport error) and logs
  only `classify(err)`, which inspects types and never formats the error; API-call failures have the
  token redacted inside `url.Error.URL` by the library itself; user replies echo only `userError`
  text (`state.go:95-101`). Replies are sent with no `ParseMode`, so a 64-char attacker-chosen title
  echoed in `/mypack` and the delete prompt cannot inject markup either.
- **Error classification** — held: `isStickerSetMissing` requires both `bot.ErrorBadRequest` and the
  `STICKERSET_INVALID` code; `createRefused` is a separate positive-only list of
  request-validation codes and is used only to authorise undoing an intent + reservation. No path
  infers absence from a generic failure.
- **Bounds ordering** — held except as noted in LOW: MIME allowlist and `FileSize` are checked before
  any byte is fetched (`photo.go:64,74`), `GetFile.FileSize` is re-checked server-side, the body is
  read through `LimitReader(max+1)` with an explicit overflow check, `DecodeConfig` bounds dimensions
  before any pixel buffer exists, and the 20-emoji cap is enforced before the API call.

---

## Recommended actions

1. Gate adoption on `created == false` (reservation pre-dated this `/newpack`), and refuse to run
   this module on a non-durable store. — HIGH
2. Use `commitContext` for the reads inside `releaseSlug` / `dropPackRecord` /
   `dropPackRecordIfSet`. — MEDIUM
3. Scale the fallback ladder from the 512px image and lower `maxDecodeDimension`. — MEDIUM
4. Move the existing-pack pre-check above `resolveSource` in `/newpack`. — LOW
5. Re-read inside the lock in `handleRenamePack` (match `adjustCount`), or state in the module doc
   that serialization is guaranteed by the dispatcher and the locks are belt-and-braces. — LOW

## Unresolved

- `go test ./internal/modules/sticker/...` failed once on its first (uncached) run — tail showed two
  `sticker_newpack_lookup … upstream is unhappy` ERROR lines then `FAIL` — and then passed 25+
  consecutive runs including `-count=8` and `-race`, and under 8-way CPU load. Not reproduced, not a
  security finding, but flagging it for whoever owns the tests: the suspects are
  `TestNewPack_UnknownLookupErrorAborts` / `TestNewPack_ResumedReservationSurvivesABail`.
- Whether Telegram permanently reserves the short name of a deleted set (plan R11) is still
  unverified. The code handles both answers correctly, so this is an open fact, not a defect.
