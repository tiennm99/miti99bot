---
title: "Sticker packs module"
description: "internal/modules/sticker — public, one-pack-per-user Telegram sticker set management via single-shot reply commands using @Stickers command names"
status: partial
priority: P2
effort: ""
tags: ["sticker", "telegram-bot", "module"]
created: 2026-08-24
branch: main
blockedBy: []
blocks: []
---

# Sticker packs module

## Overview

New module `internal/modules/sticker` letting **any** user create and manage **one**
personal Telegram sticker pack through the bot. The pack is created on behalf of the calling
user (`user_id`), named `<slug>_by_<bot_username>`, and stays bot-manageable because the bot
created it.

One pack per user is the central simplification: no command except `/newpack` takes a pack
argument, because there is only ever one pack to act on.

Command names mirror @Stickers (`/newpack`, `/addsticker`, …) but each command is
**single-shot**: one message carrying its arguments, optionally replying to a sticker or
photo. Commands are **unprefixed**, like the `misc` module (`/ff`, `/random`).

Phase 1 fixes two shared-code gaps this module would otherwise expose. They are
prerequisites, not incidental work.

## Goals

| # | Goal | Priority |
|---|------|----------|
| 1 | Any user can create and fill their personal sticker pack without leaving the chat | P1 |
| 2 | No command but `/newpack` requires naming a pack | P1 |
| 3 | Ownership is enforced structurally — a user can never mutate another's pack | P1 |
| 4 | No sticker command can stall the bot for other users beyond a bounded deadline | P1 |
| 5 | A partial failure never permanently strands a user's pack | P1 |
| 6 | Command names and semantics recognisable to @Stickers users | P2 |
| 7 | Enabling the module is an explicit operator decision | P2 |

## Accepted scope

| Decision | Value |
|---|---|
| Visibility | `VisibilityPublic` — every user manages their own packs |
| Pack model | **One pack per user.** The slug is chosen at creation and never used as an argument again |
| Inputs | Existing static stickers (reply) + photos/image documents (reply) |
| Interaction | Single-shot reply + args; no conversation state, no `/cancel` |
| Naming | @Stickers command names, no module prefix |

## Command surface

All `VisibilityPublic`. `Parameters` follows `docs/command-parameter-conventions.md`.

| Command | Parameters | Reply required | API calls |
|---|---|---|---|
| `/newpack` | `<pack> <title...>` | yes (sticker/photo) | `GetStickerSet`, `UploadStickerFile`*, `CreateNewStickerSet` |
| `/addsticker` | `[emoji...]` | yes (sticker/photo) | `UploadStickerFile`*, `AddStickerToSet` |
| `/delsticker` | — | yes (sticker in own pack) | `DeleteStickerFromSet` |
| `/editsticker` | `<emoji...>` | yes (sticker in own pack) | `SetStickerEmojiList` |
| `/ordersticker` | `<position>` | yes (sticker in own pack) | `SetStickerPositionInSet` |
| `/setpackicon` | — | yes (sticker in own pack) | `GetFile`, `SetStickerSetThumbnail` |
| `/renamepack` | `<title...>` | no | `SetStickerSetTitle` |
| `/delpack` | — | no (inline confirm) | `DeleteStickerSet` |
| `/mypack` | — | no | **none** — count comes from the store |

`*` only on the photo path.

### Slug is a name, not an address

`<pack>` survives on `/newpack` alone, where it fixes the permanent share URL
`t.me/addstickers/<slug>_by_<bot>`. Telegram has no rename-short-name method, so that choice
is unfixable afterwards — which is exactly why it stays user-chosen rather than derived from
a user ID (which would publish the owner's numeric Telegram ID forever) or generated opaquely.

Every other command resolves the caller's single pack from the store, or from the replied
sticker's `set_name` matched against it. This supersedes the earlier multi-pack design in
which `<pack>` was an argument to `/addsticker`, `/renamepack`, and `/delpack`.

### Dropped from @Stickers

`/stats`, `/top`, `/packstats`, `/packtop`, `/topbypack`, `/packusagetop` report sticker
**usage counts**, which the Bot API does not expose. `/stats` is also already owned by the
`stats` module. `/newanimated`, `/newvideo`, `/newemojipack`, `/newmasks` are outside the
accepted static-only scope. `/cancel` is meaningless without conversation state.

## Architecture constraints (verified against this repo and the live API)

Each was checked against source, not assumed. C1–C8 survived adversarial review; C5 was
corrected.

### C1 — Handlers are globally serialized

`internal/telegram/client.go:27` passes `bot.WithNotAsyncHandlers()`, and the library's
`defaultWorkers = 1` (`bot.go:20`) with no `WithWorkers` override. `process_update.go:26-28`
runs the handler inline. One slow handler stalls **every** user.

Caveat found in review: "one update at a time" is not "one goroutine". The cron scheduler
(`cmd/server/main.go:163`) and the detached per-command stats hook
(`internal/modules/dispatcher.go:80-84`) both run concurrently with handlers. Neither
touches pack state today, but the per-user keylock is therefore **not** redundant.

### C2 — No per-update deadline, and the library's own ceiling is 60s

Handler ctx is `rootCtx` (`cmd/server/main.go:107,214`), which has no deadline, so
`chathelper.FetchContext` (`chathelper.go:107-113`) returns a bare `WithCancel` that bounds
nothing. The only remaining ceiling is the library's shared
`http.Client{Timeout: time.Minute}` (`bot.go:17-18,75-77`).

Consequence: **every** handler needs its own explicit deadline, not just the photo path.
Ten sequential API calls under a 60s per-call ceiling is a ~10-minute bot-wide freeze.

### C3 — Callback data caps at 64 bytes

`internal/modules/stock/pending_dividend.go:16-17` enforces `maxDividendCallbackBytes = 64`
against Telegram's limit. `/delpack` carries an opaque pending-action id, not a slug, so the
budget is comfortable.

### C4 — Callback prefix conflicts are checked bidirectionally

`internal/modules/registry.go:216-219`. `sticker_pack:` does not overlap `stock_div:`, the
only existing prefix.

### C5 — The API cannot prove ownership, so the bot must record intent before acting

`getStickerSet` returns only `name`, `title`, `sticker_type`, `stickers`, `thumbnail`
(`models/sticker_set.go:4-10`) — no owner field.

The earlier draft concluded "therefore orphaned sets can never be adopted". Review showed
that does not follow: the bot does not need the API to name the owner, it needs **its own
record of who asked for that name**. Since only this bot can create `*_by_<bot_username>`
sets, a write-ahead intent record makes an existing set attributable. See R2 and Phase 3.

### C6 — Raw bytes cannot ride on `AddStickerToSet`

`build_request_form.go:105` handles `attach://` only for `[]models.InputSticker`. The single
`InputSticker` in `AddStickerToSetParams` (`methods_params.go:905-909`) falls through to
`addFormFieldDefault`, and `StickerAttachment` is `json:"-"` (`models/sticker.go:40`), so it
is silently dropped. Photo path must be `UploadStickerFile` (which accepts
`*models.InputFileUpload`, `build_request_form.go:87`) → use the returned `File.FileID`.

### C7 — Library is current for stickers

Bot API is at 10.3 (2026-08-24); last sticker changes were Bot API 7.2 (2024-03-31).
`v1.20.0` has `InputSticker.Format`, no top-level `sticker_format` on
`CreateNewStickerSetParams`, and `ReplaceStickerInSet`. No known gap.

### C8 — Telegram limits (confirmed against official docs)

| Limit | Value |
|---|---|
| Set name | 1–64 chars, letters/digits/underscore, begins with a letter, no consecutive underscores, ends `_by_<bot_username>` (case-insensitive) |
| Static sticker image | PNG or WEBP; one side **exactly** 512px, other ≤512px |
| Static sticker file size | **Not documented.** The widely-repeated 512 KB figure appears in no current official page (R4) |
| Set thumbnail | PNG/WEBP, exactly 100×100, ≤128 KB |
| Stickers per set | 120 regular/mask, 200 custom emoji |
| Emoji per sticker | 1–20 |
| Set title | 1–64 chars |
| Sets per bot | **Not documented** — no known ceiling |

### C9 — There is no panic barrier on the update path

`rg "recover()"` finds four sites: `testutil/mongotest`, `server/log_middleware.go:50`,
`monkeyd/export_job.go:52`, `cron/scheduler.go:67`. **None on the command or callback path.**
`internal/modules/dispatcher.go:167` carries a stale comment promising "our `recover()` in
webhook.go"; `internal/telegram/webhook.go` contains only `DeleteWebhook`.

With C1, a panic in any handler terminates the process. Phase 1 closes this.

### C10 — `MODULES` is not opt-in

`internal/modules/registry.go:107-116` expands an empty list to every registered factory,
and its own comment calls that the documented contract (`.env.example:16` ships `MODULES=`
empty; `compose.yml:14` says "empty = all modules"). Adding a `factories()` entry **is** the
enablement. Phase 6 sets `MODULES` explicitly before merging.

### C11 — `RecordingBot` cannot return structured results

`internal/testutil/recording_bot.go:178-195` answers every non-message-producing method with
`{"ok":true,"result":true}`. `GetStickerSet`, `GetFile`, and `UploadStickerFile` decode into
structs, so under the current harness they can only ever **error**. `FailMethod`
(`:99-112`) emits no `error_code`, so library errors in tests never take the
`ErrorBadRequest` shape production emits. Phase 1 extends the harness.

## Phases

| # | Phase | Status |
|---|-------|--------|
| 1 | [Phase 1: Shared prerequisites](./phase-01-shared-prerequisites.md) | Done |
| 2 | [Phase 2: Store, set names, emoji parsing](./phase-02-store-setname-emoji.md) | Done |
| 3 | [Phase 3: Pack lifecycle commands](./phase-03-pack-lifecycle.md) | Done |
| 4 | [Phase 4: Sticker commands (reply path)](./phase-04-sticker-commands.md) | Done |
| 5 | [Phase 5: Photo pipeline and pack icon](./phase-05-photo-pipeline.md) | Done |
| 6 | [Phase 6: Wiring, menu, docs](./phase-06-wiring-docs.md) | Partial — code + docs done; live smoke and deployed MODULES pending |

## Dependencies

Phase 1 blocks everything (shared code + test harness). Phase 2 blocks 3, 4, 5. Phase 3
blocks 4 and 5. Phase 5 depends on Phase 4's `/addsticker` handler. Phase 6 last. No
cross-plan dependencies — the only other plan is completed and touches disjoint files.

## Cross-cutting rules

These apply to every handler in Phases 3–5. Stated once here rather than repeated.

1. **Explicit deadline.** Every handler opens with
   `ctx, cancel := context.WithTimeout(ctx, handlerTimeout)` (`handlerTimeout = 10s`,
   package constant). Required by C2 — nothing else bounds a call.
2. **Durable writes survive shutdown.** Store writes that commit a completed Telegram-side
   action use `context.WithoutCancel(ctx)` plus a short timeout, mirroring the existing
   idiom at `dispatcher.go:78-84`. `rootCtx` is cancelled by SIGTERM mid-handler, so a
   plain `ctx` write fails on every deploy (R2).
3. **One sender helper.** `senderID(msg) (int64, error)` rejects a nil `From`, `From.IsBot`,
   and any message carrying `SenderChat`. Anonymous group admins share a single
   `GroupAnonymousBot` id, so without this all anonymous admins across all groups share one
   pack namespace and one quota (R6).
4. **Positive error classification only.** `isStickerSetMissing(err)` is
   `errors.Is(err, bot.ErrorBadRequest) && strings.Contains(err.Error(), "STICKERSET_INVALID")`.
   Any other error is "unknown — abort with no side effects". Never infer "absent" from a
   generic failure (R3, R7).
5. **Errors from the download path never escape raw.** They are converted to a sentinel at
   the boundary, discarding the original (R5).

## New dependency

`golang.org/x/image` — for `draw.CatmullRom` resampling. Stdlib decodes JPEG/PNG and encodes
PNG but ships no scaler, and stickers need an exact 512px long edge. Note it becomes the
repo's first *direct* `golang.org/x/*` requirement; all current ones are indirect.

## Abuse surface

Public module creating durable Telegram-side objects on a single-threaded dispatcher (C1):

- One pack per user, enforced by a create-only write before `CreateNewStickerSet`. This is
  the quota; there is no separate counter to keep.
- `handlerTimeout = 10s` on every handler — the primary bound (C2).
- `/mypack` makes **zero** API calls; the count lives on the `Pack` record. A single `Get`,
  no `List`, no per-pack fan-out.
- Photo source rejected above 2 MB; decoded dimensions capped at 4096×4096.
- Slug alphabet `^[a-z][a-z0-9_]{2,39}$`, no `__`, no trailing `_`.
- `internal/keylock` per-user serialization — genuinely load-bearing, not decorative,
  because crons and the detached stats hook run concurrently with handlers (C1 caveat).
- Anonymous/bot senders refused outright (cross-cutting rule 3).

## Risks

| # | Risk | Signal it broke | Response |
|---|---|---|---|
| R1 | A handler stalls the bot for all users (C1) | Reply latency spikes for unrelated commands | `handlerTimeout` caps it at 10s; if still felt, lower it, then consider offloading the photo path (Phase 5 R1) |
| R11 | A deleted slug may not be reclaimable | `/newpack <old-slug>` after `/delpack` reports the slug taken | Unknown at decision time; official docs are silent and community reports lean toward short names staying reserved. Does **not** block the URL-change path, which needs a *different* name. Settled empirically in Phase 6 smoke; if reserved, say so in `/delpack`'s confirm text |
| R10 | A user wants two packs and cannot have one | Requests for a second pack | Accepted by design. Reversing it means restoring `<pack>` arguments across four commands — a deliberate, not incidental, change |
| R2 | Partial failure strands a pack | User reports a slug reported taken that `/mypack` does not show | Write-ahead intent + `WithoutCancel` commits (Phase 3). Reverse gaps for `/delpack` and `/renamepack` documented in Phase 3 |
| R3 | Error-code matching drifts | Users see the generic reply where a specific one was expected | Match MTProto **codes**, never human text; confirm empirically in Phase 6 smoke |
| R4 | The 512 KB static-sticker ceiling may not be real | Uploads succeed above it, or fail below it | Client-side ceiling only; never stated as spec in user-facing text |
| R5 | Bot token leaks through a transport error | Any log line containing `api.telegram.org/file/bot` | Sentinel conversion at the download boundary + a test asserting the error text is clean (Phase 5) |
| R6 | Ownership collapses for anonymous senders | Two users see each other's packs | Cross-cutting rule 3 refuses them before any store access |
| R7 | A transient error deletes a live pack's record | `/mypack` reports no pack though the user can still open theirs via link | Rule 4 — destructive store deletes require positive `STICKERSET_INVALID` |
| R8 | Bot username changes in BotFather | Every pack refuses as "not yours" after restart | Ownership matches stored `Pack.Name` against `Sticker.SetName`; username only builds *new* names (Phase 2) |
| R9 | `golang.org/x/image` resampling disappoints | Visibly soft or aliased stickers in smoke | Swap `CatmullRom` for `ApproxBiLinear`; one line, one file |
| R12 | `AddStickerToSet` may reject a `file_id` from a set this bot did not create | `/addsticker` on a sticker from any other pack fails while the photo path works | The one API assumption in this plan never checked against the live API — the docs allow a `file_id` in `InputSticker.sticker` but are silent on provenance. Settle with one live call before writing Phase 4's handler. If rejected, `/addsticker`'s sticker path routes through Phase 5's `GetFile` → download → `UploadStickerFile`, which makes Phase 4 depend on Phase 5 rather than the reverse — cheap to know first, expensive to discover after |

## Accepted disclosure

`/newpack` answers "is this slug taken?" for any slug, which reveals that *some* user of this
bot owns it. This is accepted, not solved: `t.me/addstickers/<slug>_by_<bot>` is publicly
probeable without the bot, so the command adds no information an attacker lacks. The earlier
draft claimed no disclosure while shipping this probe — the claim was wrong and is removed.
Pack *management* refusals remain deliberately uniform (Phase 4), because those would
otherwise disclose which of the caller's own guesses correspond to real packs.

## Success Criteria

- [ ] A non-admin user can reply to a sticker with `/newpack mypack My Pack` and receive a working `t.me/addstickers/mypack_by_<bot>` link
- [ ] `/addsticker 😂` on a photo produces a valid static sticker with a 512px long edge and correct aspect ratio, with no pack named
- [ ] Managing a pack the caller does not own fails with an ownership error and makes zero API calls
- [ ] The not-owned reply text is byte-identical whether the set is another user's or another bot's
- [ ] `/mypack` shows slug, title, count, and link for the caller's own pack, making no API calls
- [ ] A second `/newpack` while a pack exists is refused, naming the existing pack and pointing at `/delpack`
- [ ] `/delpack` takes no argument and requires inline confirmation bound to invoker, chat, message, and a TTL
- [ ] `/delpack`'s confirm prompt states the title, sticker count, link, and permanence before the tap
- [ ] `/renamepack`'s reply names the `/delpack` + `/newpack` route to a different URL
- [ ] Every handler is bounded by an explicit deadline; no handler can exceed `handlerTimeout`
- [ ] A panic in any module handler is contained and logged, and does not terminate the process
- [ ] An interrupted `/newpack` can be completed by re-running the same command
- [ ] Anonymous group admins and bot senders are refused before any store or API access
- [ ] No log line or error string contains the file-download URL
- [ ] Module is enabled only by an explicit `MODULES` entry, verified in a deployed environment
- [ ] `go test ./...`, `go vet ./...`, `gofmt -l .`, and `golangci-lint run` all clean

## Open Questions

None. Both prior questions were resolved by the one-pack-per-user revision: `/editsticker`
takes no pack argument because no command but `/newpack` does (former O1), and the
per-user pack limit is one (former O2).

## Design Revisions

### 2026-08-25 — /delpack confirmations must prove authority, not disprove it

Removing adoption closed the takeover class, but the same DeleteStickerSet
primitive stayed reachable through a stale confirmation. The under-lock re-check
was written as a blocklist — it refused only a *pending* record still naming the
set — and fell through on the two states that mattered: no record at all, and a
record that had moved on. Reproduced with ordinary commands and no attacker:
prompt, pack disappears at Telegram, self-heal frees the name, another user
claims it, first user presses, their pack is destroyed.

Inverted to an allowlist: delete only when a confirmed record still names this
exact set. A guard phrased as "which states do I refuse" cannot fail closed
against a state nobody enumerated. Dropping a pack record also clears any
outstanding confirmation, so a dead prompt is gone rather than merely refused.

This also closes the reservation leak on the confirmed-delete path, since the
moved-on case no longer reaches Telegram at all.

Fixed alongside: resuming an interrupted `/newpack` silently discarded a retyped
title and reported success with the old one; a test named for freeing a dead
name never asserted it; and `lockUser`'s comment justified the lock with cron
and stats-hook contention that does not exist — the map is state-local and this
module registers neither.

### 2026-08-25 — pack adoption removed

Verification found the round-4 guard defeated by the module's own recovery
branch: an inconclusive `GetStickerSet` keeps the reservation, which turns a
*fresh* claim into a *resumed* one, so two ordinary `/newpack` commands took
over a stranger's pack. `resolveStaleIntent` had a second adopt path that never
consulted the guard at all. Reproduced against the real handlers.

That is the fourth consecutive failure of the same mechanism, and the reason is
structural rather than a bug that can be patched. Adoption needs to prove "this
set is mine to finish" from local state, and local state is exactly what a
restart on the in-memory backend erases while the packs at Telegram survive.
Once the proof is gone the honest and the malicious case are indistinguishable.

Adoption is therefore removed entirely — both branches. `/newpack` refuses any
name a set already occupies, for everyone, whatever the records say.

Implementing it surfaced a second hole the option did not cover: a *pending*
record is not evidence either, and `/delpack` deletes by set name, which
Telegram authorises for every set this bot created. Keeping a refused intent so
the user could `/delpack` it would hand over a way to destroy the pack we had
just refused to adopt. `/delpack` now clears a pending record locally and calls
Telegram only for a confirmed one.

Accepted cost: a crash between creating a set and recording it strands that set
permanently. Documented rather than mitigated, because every mitigation is the
mechanism that just failed four times.

### 2026-08-25 — three-lens review pass (security, correctness, tests)

Three independent reviewers ran against the finished module. What they changed:

- **Adoption no longer trusts the reservation alone.** The reservation proves
  ownership only while it outlives the sets it guards, and it does not: it lives
  in our store, the packs live at Telegram, and the in-memory backend is
  selected silently whenever `MONGO_URL` is unset. After any wipe the original
  takeover was reachable again. `createOrAdopt` now refuses to adopt when *this*
  invocation first claimed the name — a genuine interrupted attempt always finds
  its own reservation waiting, so there are no false negatives.
- **Post-action cleanup reads moved onto the detached context.** They wrote via
  `commitContext` but read on the request context, so a cancelled request failed
  the read and skipped the release while still deleting the pack record —
  stranding a name permanently. Found independently by two reviewers.
- **Emoji clustering.** Nine valid emoji were refused outright; tag-sequence
  flags shattered; and three inputs (trailing ZWJ, a joiner before a flag, an
  odd regional-indicator count) passed validation and would have sent an
  `emoji_list` Telegram rejects.
- **A reply tail is now reserved** from the handler budget via the existing
  `chathelper.FetchContext`, which every other data-fetching module already
  used. A slow photo `/newpack` could spend the whole 10s before Telegram was
  called, and then send its error reply on a dead context — the user saw nothing.
- **The compression ladder resamples the scaled image, not the source.** It was
  paying a full-size resample per rung: measured 1.99s versus 74ms for the three
  rungs, on a dispatcher that runs handlers one at a time.
- **Tests.** Mutation testing refuted the previous round's non-vacuity claim in
  three places. The `created`-flag release machinery had no coverage in either
  direction; two emoji assertions passed against a handler that never called
  Telegram; and nothing pinned module registration — the whole module could be
  removed from `factories()` with the suite still green.

Left open deliberately: the shutdown path never joins the polling goroutine, so
`context.WithoutCancel` does not actually survive process exit. That is
`cmd/server/main.go`, outside this module and affecting every module's commits.

### 2026-08-25 — one pack per user

The accepted scope originally chose "named packs, multiple per user, addressed by slug
argument". The user revised it to one pack per user, with every command operating on that
pack implicitly.

Removed by the revision:

- `<pack>` arguments on `/addsticker`, `/renamepack`, and `/delpack`
- `/packlist` (replaced by `/mypack`, singular) and its `List` + per-pack `Get` fan-out
- `maxPacksPerUser` as a tunable — the limit is one
- The separate default-pack command and per-user prefs record that a multi-pack default
  would have required

Red-team findings this revision resolves outright rather than mitigates:

- Finding 4 (`/packlist` unbounded: 60s client timeout x 10 calls plus an N+1) — `/mypack`
  is a single `Get` and makes no API calls at all
- Former open question O2 (`maxPacksPerUser` sizing) — no longer a choice

Unaffected: Phase 1 (panic barrier, test harness) and Phase 5 (photo pipeline, token-leak
sentinel) need no changes. The write-ahead intent machinery, anonymous-sender rule,
`handlerTimeout`, and bot-rename resilience all carry over unchanged.

Trade-off accepted: a user who wants a memes pack and a reactions pack separately cannot
have both (R10).

### 2026-08-25 — `/delpack` as the URL-change path

`/delpack` was reviewed as a destructive convenience. It is in fact the **only** mechanism for
changing a pack's URL, because Telegram exposes no rename-short-name method. That reframing
changed three things without adding a command:

- `/renamepack`'s reply now names the delete-and-recreate route instead of only stating that
  the link cannot change — turning a dead end into an answer.
- `/delpack`'s confirm prompt must state the pack title, the sticker count being destroyed, the
  exact link being surrendered, and that both are permanent. It is the last point at which a
  user learns stickers do not survive a URL change.
- A `/repack <newslug>` migration command was considered and **rejected for this plan**: copying
  a full pack is up to ~121 sequential API calls, which blows `handlerTimeout` and stalls the
  bot for every user under C1. It is viable only after Phase 5's goroutine offload lands, and is
  recorded here as a follow-up rather than scoped in.

The @Stickers command set was mapped exhaustively against the one-pack model; all nine of our
commands are either direct adaptations or (for `/mypack`) a justified addition, and every
dropped @Stickers command has a stated reason.

**Bug found during this pass** (Phase 3, `/newpack`): the different-slug pending branch
overwrote the pending record unconditionally, which permanently orphans a set that was created
before an interruption. It now probes `GetStickerSet(oldName)` first and adopts rather than
overwrites when the old set exists.


### 2026-08-25 — post-implementation review: global slug reservation

An independent review of the implemented module found a **pack takeover** in
`/newpack`, and it traces back to this plan, not only to the code.

Phase 3 step 5 said: `GetStickerSet` succeeds → "we hold a `Pending` record for
this owner and slug, so this is our own interrupted attempt: adopt it". The
plan's own risk section stated the supporting invariant as *"a `Pending` record
for an owner means that owner asked for that name"*. That is true and **not
sufficient**. Asking is not creating. `PutVersioned` is create-only per *owner
key* (`packKey` is the owner ID alone); nothing reserved a name globally. So a
user with no pack who typed another user's slug produced byte-identical evidence
to a genuine resumed attempt — and adopted their pack, then could `/delpack` it.
Every share link is public, so slugs are trivially enumerable.

The plan pre-authorised one response to this signal ("disable adoption — a
one-line change"), which would have closed the hole by dropping the
"an interrupted `/newpack` can be completed by re-running" success criterion.
The user chose the stronger fix instead:

**A global slug reservation.** `SlugReservation{Slug, OwnerID}` is written
create-only under `slug:<slug>` *before* Telegram is touched. Adoption is
allowed only when the reservation names the caller. The first claimant of a name
is the only user who can ever adopt a set under it, so "this set is mine" became
a proven fact rather than an assumption — and both success criteria survive.

Consequent changes:

- `reserveSlug` runs before `claimSlug`; a name held by anyone else replies
  "that pack name is taken" with **zero** API calls.
- `resolveStaleIntent` re-proves the *old* slug's reservation before adopting
  under it, and releases it on a positive "no such set".
- `dropIntent` no longer fires on unknown errors (was a plan rule 4 violation in
  its own right): on anything but a *classified* refusal both the intent and the
  reservation survive, which is what lets a re-run recover. Only a positive
  refusal releases them.
- `/delpack` keeps the reservation, so a deleted name stays recoverable by its
  owner and unavailable to everyone else — which also matches Telegram's likely
  behaviour for deleted short names (R11).

Four further defects fixed in the same pass:

- **A stale `/delpack` confirmation deleted a live pack's record.** Pending
  actions used a random key per invocation, so two could be live at once, and
  the delete path cleared the record *by owner* without checking it still named
  the set being deleted — precisely the documented delete-then-recreate URL
  change. Now keyed per user (matching `stock/pending_dividend.go`), superseded
  by a newer prompt, and gated on `ownsSet` before clearing.
- **Unbounded storage from a public command.** The same random key meant a user
  who ran `/delpack` and never tapped left a permanent document.
- **The panic barrier missed the detached hook goroutine**, so a panicking
  `CommandHook` (the `stats` module ships one) still killed the process. C9's
  "Phase 1 closes this" was true for handlers only.
- `/delpack`'s result message bypassed `chathelper.Reply`, so in a forum
  supergroup it landed in General instead of the topic.

### 2026-08-25 — Phase 4 review pass

Ten findings applied before implementation began. Six changed what gets written:

- `resolveSource` takes `(ctx, b, ownerID, msg)` from the start, and `photoRef` is gone — the
  photo branch resolves to a `file_id` like every other path, so Phase 5 adds a branch instead
  of rewriting call sites.
- The static-only gate moves onto `resolveSource`, the path that can actually receive a
  non-static sticker, and now also rejects `Type != "regular"` — a mask sticker is static yet
  invalid for a regular set, which `IsAnimated || IsVideo` does not catch.
- `/addsticker` gained its missing no-pack path, and the note on why its refusal is
  deliberately *not* the uniform `resolveOwned` one.
- `/delsticker` takes the same per-user keylock as `/addsticker`; both do the same
  read-modify-write on `Count`.
- `/delsticker`'s "never deletes the record" criterion was scoped to transient and unknown
  errors. Unqualified, it forbade Phase 3's `STICKERSET_INVALID` self-heal — the very
  mechanism that unwedges a user whose set Telegram removed.
- The animated/video check moved ahead of the store read in `resolveOwned`'s numbered gate.

Also: the `Count: 0` aftermath now names its recovery route, `STICKERS_TOO_MUCH` and the
no-pack paths gained success criteria, R12 records the unverified `file_id`-reuse premise, and
Phase 6 must document `<name...>` in the conventions doc rather than shipping an undocumented
parameter form.

#### Consistency sweep — one-pack revision

- Files reread: plan.md and all six phase files.
- Deltas checked: 9 (`<pack>` dropped from `/addsticker`, `/renamepack`, `/delpack`;
  `/packlist` -> `/mypack`; `listPacks` -> `getPack`; key `<ownerID>:<slug>` -> `<ownerID>`;
  `maxPacksPerUser` removed; `matchPack` -> `ownsSet`; `buildSetName` -> `makeSetName`).
- Stale references reconciled: 4 (incl. the frontmatter description, which said
  "multi-pack-per-user" and is surfaced by every `ak plan list`). Two live risk signals in the R2/R7 rows still named
  `/packlist`; Phase 5's `/setpackicon` still said "one of the caller's packs". Phase 5 was
  therefore **not** untouched, contrary to the initial assessment.
- Remaining `/packlist`, `listPacks`, and `maxPacksPerUser` mentions are all deliberate
  comparative or historical text ("this replaced X", the revision log, the red-team table).
- Phases 1 and 5 confirmed free of multi-pack phrasing after the fix.
- Unresolved contradictions: 0

## Red Team Review

### Session — 2026-08-25
**Findings:** 28 raw from 3 reviewers → 16 after dedup (16 accepted, 0 rejected)
**Severity breakdown:** 7 Critical, 6 High, 3 Medium
**Reviewers:** Security Adversary (Fact Checker), Failure Mode Analyst (Flow Tracer),
Assumption Destroyer (Scope Auditor). All findings carried `file:line` evidence, so none
were filtered. Four of the seven Critical findings were independently corroborated by all
three reviewers.

| # | Finding | Severity | Disposition | Applied To |
|---|---------|----------|-------------|------------|
| 1 | Empty `MODULES` loads all modules; "opt-in" false in 3 places | Critical | Accept | C10, Phase 6, Goals |
| 2 | No `recover()` on the update path; a handler panic kills the process | Critical | Accept | C9, Phase 1 |
| 3 | Bot token leaks via `*url.Error` into the dispatcher log | Critical | Accept | Rule 5, R5, Phase 5 |
| 4 | `/packlist` unbounded: 60s client timeout × 10 calls + N+1 | Critical | Accept | C2, Rule 1, Phase 3 |
| 5 | `/delsticker` probe deletes the record on any error | Critical | Accept | Rule 4, R7, Phase 4 |
| 6 | `/delpack` button is a permanent replayable delete capability | Critical | Accept | Phase 3 |
| 7 | `RecordingBot` cannot return structs; 3 phases untestable | Critical | Accept | C11, Phase 1 |
| 8 | Anonymous group admins share one `From.ID` | High | Accept | Rule 3, R6 |
| 9 | Bot rename orphans every pack; `Pack.Name` written never read | High | Accept | R8, Phase 2, Phase 4 |
| 10 | `GetStickerSet` not-found is an undefined branch | High | Accept | Rule 4, Phase 3 |
| 11 | Commit writes on `rootCtx`; deploy is the normal orphan path | High | Accept | Rule 2, R2, Phase 3 |
| 12 | `/help` 4096-rune ceiling; 884 runes headroom measured | High | Accept | Phase 6 |
| 13 | C5's no-adoption conclusion not forced by its premise | High | Accept | C5, R2, Phase 3 |
| 14 | `Put` used where create-only `PutVersioned` exists | Medium | Accept | Phase 3 |
| 15 | `parseSlug` case-preserving value reaches the storage key | Medium | Accept | Folded into #9 |
| 16 | `/newpack` oracle contradicts the plan's own no-disclosure claim | Medium | Accept | Accepted disclosure section |

**User decisions taken during adjudication:** explicit `MODULES` before Phase 6; panic
barrier in `modules.Install` as Phase 1; module-wide deadline plus persisted counts;
write-ahead intent record for orphan recovery.

### Whole-Plan Consistency Sweep
- Files reread: plan.md, phase-01-shared-prerequisites.md, phase-02-store-setname-emoji.md,
  phase-03-pack-lifecycle.md, phase-04-sticker-commands.md, phase-05-photo-pipeline.md,
  phase-06-wiring-docs.md
- Decision deltas checked: 14 (phase renumber 1-5 -> 1-6; `parseSlug` -> `matchPack`;
  `/delsticker` probe removed; `/packlist` API-free via `Pack.Count`; `Pack.Pending`
  write-ahead intent; callback payload slug -> opaque id; C5 rewritten; opt-in claims
  removed; `handlerTimeout` cross-cutting rule; `senderID` rule; `isStickerSetMissing`;
  download sentinel; slug cap decoupled from C3; `Put` -> `PutVersioned` for create)
- Reconciled stale references: 0 remaining. `parseSlug` survives only in phase-02 as the
  named superseded design and in the red-team table as finding #15 — both deliberate
  historical references, not live claims. "opt-in" survives only in C10's negative heading
  and the finding that corrected it.
- Link integrity: all 6 phase links resolve; no orphan phase files.
- Cross-phase references verified consistent under the new numbering.
- Unresolved contradictions: 0

<!-- slug: sticker-pack-module -->
