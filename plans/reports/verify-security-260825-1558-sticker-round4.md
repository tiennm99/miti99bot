# Adversarial verification — sticker packs, round 4

Scope: round-4 fixes on `feature/sticker-pack-module` (whole module lives in one
commit `d831747`, so round-3→4 cannot be isolated by git; current state reviewed).
Method: source read + four constructed attacks executed against the real handlers
in a throwaway copy of the repo, plus a 45s fuzz of the emoji splitter and an
image-pipeline probe. `go vet`, `go test ./...` (25 pkgs), `golangci-lint run` all
clean on the branch as committed. No repo file was modified.

Verdict: **DO_NOT_MERGE** — change 1 does not close the takeover it was written for.

---

## CRITICAL 1 — cross-user pack takeover still reachable; `freshReservation` is a per-invocation fact, not durable proof

`internal/modules/sticker/pack_handlers.go:352` (guard), `:365-374` (the branch that
defeats it), `:289-305` (a second adopt branch that never consults the guard).

The guard's premise (`:329`, `:340-351`) is "a genuine interrupted attempt always
re-enters having found its own reservation, never having made one". True. The
converse it relies on — "an attacker naming a foreign set can only ever be the one
who made the reservation" — is false, because the module deliberately **keeps** a
fresh reservation whenever the lookup is inconclusive:

```go
default:
    // Unknown. Keep both the intent and the reservation: the set may exist,
    // and re-running is how the user recovers.
    log.Error("sticker_newpack_lookup", "err", err)
```

One inconclusive `GetStickerSet` converts the attacker's fresh reservation into a
resumed one. The next invocation has `freshReservation == false` and adopts.

**Precondition (all routes):** the victim's slug reservation is absent while the set
lives at Telegram. This is the exact state `docs/sticker-packs.md:122-127` documents
("what a restart does when no database is configured") and promises is safe:
"`/newpack` reports the name as taken rather than adopting a set it can no longer
prove is yours." With `KV_PROVIDER` auto-detect (`cmd/server/main.go:259-279`), any
deploy without `MONGO_URL` re-enters this state on **every restart**.

**Route A — two ordinary commands, no crash, no store error** (executed, reproduced):

1. Attacker sends `/newpack <victimslug> X`. `getStickerSet` answers 429 / 5xx /
   deadline-exceeded — not `STICKERSET_INVALID` — so the `default` branch keeps the
   attacker's intent *and* reservation. User sees "Something went wrong."
2. Attacker sends the identical command again. `reserveSlug` conflicts, holder is the
   attacker → `created=false` → `createOrAdopt(..., freshReservation=false)` → set
   exists → **adopted**.

Observed reply: `Finished an earlier attempt at Mine. https://t.me/addstickers/mypack_by_testbot`,
record `{Slug:mypack Name:mypack_by_testbot OwnerID:999 Pending:false Count:1}`.
The attacker now holds `/delpack` (irreversible `DeleteStickerSet`), `/renamepack`,
`/setpackicon`, `/delsticker` over the victim's set — all keyed by set name with no
owner scoping.

Step 1 is attacker-inducible, not luck: Telegram 429s are per-bot and any user can
provoke them, and after changes 3+4 the tail budget left for `GetStickerSet` is
~3 s (`FetchContext` reserve), so a slow media leg alone produces
`context deadline exceeded` → same `default` branch.

**Route B — `resolveStaleIntent` never checks the guard at all** (executed, reproduced).
With intent + reservation for `<victimslug>` surviving (e.g. a crash at the refusal
point, before `dropIntent`), the attacker runs `/newpack <anyothername>`;
`claimSlug` → `resolveStaleIntent` re-proves only *who holds* the old reservation,
finds the victim's set, and adopts at `:292-304`. Reply: "You already have a pack
(mypack) from an earlier attempt — it has been restored."

**Route C — partial refusal cleanup** (executed, reproduced). `:359-361` is two
independent, error-swallowing writes. If `releaseSlug` fails or the process dies
between them, the attacker keeps the reservation and the next attempt adopts (Route A
step 2 without needing step 1).

**Not exploitable (checked):** `dropIntent` is always keyed to `pack.OwnerID`, which
is always the caller (`claimSlug` builds the intent, or `getPack(caller)` returns it);
`releaseSlug` re-verifies the holder at `:204`. **User A cannot drop user B's intent
or reservation.** That part of round 4 holds.

**Fix (prototyped and verified).** Replace the per-invocation inference with durable
positive evidence: a `Probed bool` on `SlugReservation`, set only when
`GetStickerSet` positively answers `STICKERSET_INVALID` for a reservation this owner
holds, written *before* `CreateNewStickerSet`, and required by **both** adopt
branches. Fails closed: if the flag write fails, abort before creating.

```go
// createOrAdopt, err == nil branch
if freshReservation || !s.probedClear(ctx, pack.OwnerID, pack.Slug) { ...refuse... }

// createOrAdopt, isStickerSetMissing branch, before CreateNewStickerSet
if !s.markProbed(ctx, pack.OwnerID, pack.Slug) { return reply(ctx, b, msg, genericFailure) }

// resolveStaleIntent
case err == nil && !held.Probed:  // refuse: release old reservation, take the new intent
```

With this applied, all four attack routes refuse and the entire existing suite stays
green — one fixture needs updating (`seedInterrupted` in `pack_handlers_test.go:394`
must seed `Probed: true`, since it models a post-probe interrupted attempt).

---

## MEDIUM 2 — the slug-ownership check still runs *after* the media pipeline

`pack_handlers.go:94` (`resolveSource`) precedes `:109` (`reserveSlug`).

Change 3's comment (`:81-84`) says making a user who cannot create a pack pay for the
full media pipeline "was free work for anyone who wanted to spend the bot's CPU" —
but that is still exactly what happens when the slug belongs to someone else.
Executed: `/newpack <slug-held-by-another-user>` replying to a photo issued `getFile`
and the file download before any slug check; on a decodable image it also resamples
and `UploadStickerFile`s. Methods recorded: `[getFile x.jpg sendMessage]`.

Impact, single dispatch worker (`WithNotAsyncHandlers`, one worker): each such message
occupies the bot for the whole leg, and the attacker never acquires a pack so the new
`Pending` precheck never starts refusing them — the loop is unbounded. Measured
`toStickerPNG` cost on the single worker: 0.71 s for a flat 4096×4096 source (360 KB
on the wire) and 3.7 s for a noisy one (the ladder rungs). `mediaContext` does not
bound this — `toStickerPNG` takes no context and cannot be interrupted. Wasted
`UploadStickerFile` calls also burn the bot's API quota, which is the 429 that
Finding 1 step 1 needs.

Fix: a read-only `getSlugReservation` before `resolveSource` — refuse when held by
another user. Read-only, so it does not reintroduce the round-1 name-burning DoS
(the create-only `reserveSlug` write stays where it is).

---

## MEDIUM 3 — detached commit contexts are not actually protected at shutdown

`state.go:53-61`, `cmd/server/main.go:220-227`, `:125`.

Change 2 extends `context.WithoutCancel` to the reads, on the stated grounds that "a
commit that records a completed Telegram-side action must not be lost because the
process is shutting down". `main` does not honour that: on SIGTERM it cancels
`rootCtx`, calls `srv.Shutdown` on the health server (returns in ms with no live
connections), then returns — running `defer closeProvider()`, which disconnects Mongo.
Nothing waits for the in-flight inline handler. The detached context survives
cancellation but the process does not wait for the write, so a deploy can still lose
the commit that these comments promise is safe.

Fix: track in-flight dispatch with a `sync.WaitGroup` (or a drain deadline) before
`closeProvider`, or downgrade the comments to "best effort".

---

## LOW 4 — `downloadTimeout` is now dead

`download.go:25,39` vs `state.go:78-80`. `FetchContext` reserves 3 s of a 10 s
handler, so `mediaCtx` is ≤ ~7 s and the 8 s `http.Client.Timeout` can never bind.
The comment still calls it "this module's own ceiling". Either lower it to match the
real budget or say it is a backstop for a caller with no deadline.

## LOW 5 — `lockUser`'s stated rationale is not true for this module

`state.go:82-84` claims "the cron scheduler and the detached per-command stats hook
run concurrently with them, so this is load-bearing". Grepped: `keylock` here is
`state`-local, the sticker module registers no cron jobs (`sticker.go:21-88`), and the
stats hook (`dispatcher.go:82-90`) touches only the stats collection. Nothing else
acquires these keys, and all sticker handlers run inline on the single dispatch
goroutine. Keep the lock (cheap, correct if dispatch ever goes async) but fix the
claim. Corollary: change 3 cannot deadlock or block the worker — verified, see below.

## LOW 6 — nested `commitContext` re-arms the budget

`pack_handlers.go:483` passes an already-detached ctx into `dropPackRecord:520`, which
derives another. `context.WithoutCancel` drops the parent deadline, so the inner
helper gets a fresh 5 s, and `dropPackRecord` → `releaseSlug` adds a third. A
`/delpack` confirm can therefore spend ~15 s in detached cleanup. No leak (every
`cancel` is deferred) and every op is bounded; just not the 5 s the constant implies.

## LOW 7 — recording bot truncates instead of rejecting oversized bodies

`testutil/recording_bot.go:195` reads through `io.LimitReader(r.Body, 8<<20)`; a body
above the cap is silently truncated and then fails `ParseMultipartForm` as a confusing
"bad multipart form: unexpected EOF" rather than a size error. No current test is near
the cap. Reading one extra byte and reporting "body too large" would match the
module's own `downloadFile:75` pattern.

## LOW 8 — duplicated refusal text

`pack_handlers.go:89-91` and `:248-250` build the same "You already have a pack" reply
independently. One helper; they will drift.

---

## Attacked and held

- **Cross-user destruction of state.** `releaseSlug` re-reads and compares the holder
  (`:204`) and `dropIntent` is always keyed by the caller's own id. Probed both adopt
  refusal paths: user A cannot drop user B's intent or reservation. Held.
- **Deadlock / worker starvation from change 3.** `WithNotAsyncHandlers` +
  one worker (`internal/telegram/client.go:26-30`) means all sticker handlers run
  serially on one goroutine; the `keylock.Map` is `state`-local; no cron, no hook, no
  detached goroutine touches it; no handler nests a second `lockUser`. Held (the lock
  is uncontended today).
- **`Pending` record behaviour after moving the precheck.** The precheck refuses only
  `found && !existing.Pending`, so a pending user still falls through to the same
  `claimSlug` resume path as before. No behaviour change. Held.
- **Change 5, extreme aspect ratios.** Executed 4096×1, 1×4096, 4096×3, 1×1,
  4096×4096: outputs 512×1, 1×512, 512×1, 512×512, 512×512 — no zero dimension, long
  edge exactly 512, and the ladder's targets still derive from the *original* bounds
  so the aspect is identical to the one-step version. Thumbnail path stays 100×100.
  Held.
- **Change 6, emoji clustering.** 45 s / 526k-exec fuzz over valid UTF-8: no panic, no
  infinite loop, no empty cluster, and nothing dropped except leading/trailing ZWJ and
  whitespace. Spot-checked the singleton table against the standard non-block emoji
  set (©, ®, ‼, ⁉, ™, ℹ, Ⓜ, ⤴, ⤵, 〰, 〽, ㊗, ㊙) — complete. Held.
- **Change 7, other modules' tests.** 28 files reference `NewRecordingBot`; full
  `go test ./...` green. The library always sends `multipart/form-data` with a
  boundary header and only omits the body for nil-param methods
  (`raw_request.go:29-72`), so "empty body ⇒ skip parse, non-empty ⇒ must parse" is
  the correct split, and replacing `r.Body` after buffering works because the boundary
  comes from the unchanged header. Held.
- **`/delpack` callback authorisation.** Lookup keyed by presser, owner compared,
  chat+message binding before any side effect, action consumed before the destructive
  call. Held.
- **Token leakage.** `classify` inspects error *types* only; `errDownloadFailed`
  replaces every download error; `replyErr` echoes only `userError`. Held.

## Unresolved questions

1. Is production on Mongo or on the auto-detected memory backend? Finding 1 is routine
   after any restart on memory, and needs data loss on Mongo. It should be fixed either
   way, but this decides whether it blocks the deploy or only the config.
2. `docs/sticker-packs.md:122-127` states the wiped-store refusal as a guarantee. It is
   currently only true for the first attempt; the doc needs no change once Finding 1 is
   fixed, but it should not ship as-is.
