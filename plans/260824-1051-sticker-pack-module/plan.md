---
title: "Sticker packs module"
description: "internal/modules/sticker — public, multi-pack-per-user Telegram sticker set management via single-shot reply+args commands using @Stickers command names"
status: pending
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

New module `internal/modules/sticker` letting **any** user create and manage their own
Telegram sticker packs through the bot. Packs are created on behalf of the calling user
(`user_id`), named `<slug>_by_<bot_username>`, and stay bot-manageable because the bot
created them.

Command names mirror @Stickers (`/newpack`, `/addsticker`, …) but each command is
**single-shot**: one message carrying its arguments, optionally replying to a sticker or
photo. Commands are **unprefixed**, like the `misc` module (`/ff`, `/random`).

Phase 1 fixes two shared-code gaps this module would otherwise expose. They are
prerequisites, not incidental work.

## Goals

| # | Goal | Priority |
|---|------|----------|
| 1 | Any user can create and fill a personal sticker pack without leaving the chat | P1 |
| 2 | Ownership is enforced structurally — a user can never mutate another's pack | P1 |
| 3 | No sticker command can stall the bot for other users beyond a bounded deadline | P1 |
| 4 | A partial failure never permanently strands a user's pack | P1 |
| 5 | Command names and semantics recognisable to @Stickers users | P2 |
| 6 | Enabling the module is an explicit operator decision | P2 |

## Accepted scope

| Decision | Value |
|---|---|
| Visibility | `VisibilityPublic` — every user manages their own packs |
| Pack model | Named packs, multiple per user, addressed by slug argument |
| Inputs | Existing static stickers (reply) + photos/image documents (reply) |
| Interaction | Single-shot reply + args; no conversation state, no `/cancel` |
| Naming | @Stickers command names, no module prefix |

## Command surface

All `VisibilityPublic`. `Parameters` follows `docs/command-parameter-conventions.md`.

| Command | Parameters | Reply required | API calls |
|---|---|---|---|
| `/newpack` | `<pack> <title...>` | yes (sticker/photo) | `GetStickerSet`, `UploadStickerFile`*, `CreateNewStickerSet` |
| `/addsticker` | `<pack> [emoji...]` | yes (sticker/photo) | `UploadStickerFile`*, `AddStickerToSet` |
| `/delsticker` | — | yes (sticker in own pack) | `DeleteStickerFromSet` |
| `/editsticker` | `<emoji...>` | yes (sticker in own pack) | `SetStickerEmojiList` |
| `/ordersticker` | `<position>` | yes (sticker in own pack) | `SetStickerPositionInSet` |
| `/setpackicon` | — | yes (sticker in own pack) | `GetFile`, `SetStickerSetThumbnail` |
| `/renamepack` | `<pack> <title...>` | no | `SetStickerSetTitle` |
| `/delpack` | `<pack>` | no (inline confirm) | `DeleteStickerSet` |
| `/packlist` | — | no | **none** — counts come from the store |

`*` only on the photo path.

### Deviation from the approved command preview

The preview showed `/editsticker my_memes 😂🔥` with an explicit pack argument. This plan
drops it: the replied-to sticker carries `set_name`, which is matched against the caller's
stored packs. `/delsticker`, `/editsticker`, `/ordersticker`, `/setpackicon` all resolve the
pack that way. Open question O1.

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
| 1 | [Phase 1: Shared prerequisites](./phase-01-shared-prerequisites.md) | Pending |
| 2 | [Phase 2: Store, set names, emoji parsing](./phase-02-store-setname-emoji.md) | Pending |
| 3 | [Phase 3: Pack lifecycle commands](./phase-03-pack-lifecycle.md) | Pending |
| 4 | [Phase 4: Sticker commands (reply path)](./phase-04-sticker-commands.md) | Pending |
| 5 | [Phase 5: Photo pipeline and pack icon](./phase-05-photo-pipeline.md) | Pending |
| 6 | [Phase 6: Wiring, menu, docs](./phase-06-wiring-docs.md) | Pending |

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

- `maxPacksPerUser = 10`, enforced before `CreateNewStickerSet`.
- `handlerTimeout = 10s` on every handler — the primary bound (C2).
- `/packlist` makes **zero** API calls; counts live on the `Pack` record.
- Photo source rejected above 2 MB; decoded dimensions capped at 4096×4096.
- Slug alphabet `^[a-z][a-z0-9_]{2,39}$`, no `__`, no trailing `_`.
- `internal/keylock` per-user serialization — genuinely load-bearing, not decorative,
  because crons and the detached stats hook run concurrently with handlers (C1 caveat).
- Anonymous/bot senders refused outright (cross-cutting rule 3).

## Risks

| # | Risk | Signal it broke | Response |
|---|---|---|---|
| R1 | A handler stalls the bot for all users (C1) | Reply latency spikes for unrelated commands | `handlerTimeout` caps it at 10s; if still felt, lower it, then consider offloading the photo path (Phase 5 R1) |
| R2 | Partial failure strands a pack | User reports a slug reported taken that `/packlist` does not show | Write-ahead intent + `WithoutCancel` commits (Phase 3). Reverse gaps for `/delpack` and `/renamepack` documented in Phase 3 |
| R3 | Error-code matching drifts | Users see the generic reply where a specific one was expected | Match MTProto **codes**, never human text; confirm empirically in Phase 6 smoke |
| R4 | The 512 KB static-sticker ceiling may not be real | Uploads succeed above it, or fail below it | Client-side ceiling only; never stated as spec in user-facing text |
| R5 | Bot token leaks through a transport error | Any log line containing `api.telegram.org/file/bot` | Sentinel conversion at the download boundary + a test asserting the error text is clean (Phase 5) |
| R6 | Ownership collapses for anonymous senders | Two users see each other's packs | Cross-cutting rule 3 refuses them before any store access |
| R7 | A transient error deletes a live pack's record | `/packlist` loses an entry the user can still open via link | Rule 4 — destructive store deletes require positive `STICKERSET_INVALID` |
| R8 | Bot username changes in BotFather | Every pack refuses as "not yours" after restart | Ownership matches stored `Pack.Name` against `Sticker.SetName`; username only builds *new* names (Phase 2) |
| R9 | `golang.org/x/image` resampling disappoints | Visibly soft or aliased stickers in smoke | Swap `CatmullRom` for `ApproxBiLinear`; one line, one file |

## Accepted disclosure

`/newpack` answers "is this slug taken?" for any slug, which reveals that *some* user of this
bot owns it. This is accepted, not solved: `t.me/addstickers/<slug>_by_<bot>` is publicly
probeable without the bot, so the command adds no information an attacker lacks. The earlier
draft claimed no disclosure while shipping this probe — the claim was wrong and is removed.
Pack *management* refusals remain deliberately uniform (Phase 4), because those would
otherwise disclose which of the caller's own guesses correspond to real packs.

## Success Criteria

- [ ] A non-admin user can reply to a sticker with `/newpack mypack My Pack` and receive a working `t.me/addstickers/mypack_by_<bot>` link
- [ ] `/addsticker mypack 😂` on a photo produces a valid static sticker with a 512px long edge and correct aspect ratio
- [ ] Managing a pack the caller does not own fails with an ownership error and makes zero API calls
- [ ] The not-owned reply text is byte-identical whether the set is another user's or another bot's
- [ ] `/packlist` shows slug, title, count, and link for the caller's packs only, making no API calls
- [ ] `/delpack` requires inline confirmation bound to invoker, chat, message, and a TTL
- [ ] Every handler is bounded by an explicit deadline; no handler can exceed `handlerTimeout`
- [ ] A panic in any module handler is contained and logged, and does not terminate the process
- [ ] An interrupted `/newpack` can be completed by re-running the same command
- [ ] Anonymous group admins and bot senders are refused before any store or API access
- [ ] No log line or error string contains the file-download URL
- [ ] Module is enabled only by an explicit `MODULES` entry, verified in a deployed environment
- [ ] `go test ./...`, `go vet ./...`, `gofmt -l .`, and `golangci-lint run` all clean

## Open Questions

- **O1** — Should `/editsticker` keep the explicit `<pack>` argument from the approved
  command preview, or resolve the pack from the replied sticker's `set_name` as planned?
- **O2** — `maxPacksPerUser = 10` no longer drives `/packlist` cost now that counts are
  stored, so it is a pure product choice. Keep 10?

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
