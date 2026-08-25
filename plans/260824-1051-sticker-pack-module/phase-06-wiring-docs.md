---
phase: 6
title: "Phase 6: Wiring, menu, docs"
status: partial
priority: P2
effort: "4h"
dependencies: [1, 2, 3, 4, 5]
---

# Phase 6: Wiring, menu, docs

## Overview

Register the module, make enabling it an explicit operator decision, and bring the user-facing
surfaces named in `AGENTS.md` § "Command Changes" into line. Nothing from Phases 2–5 is
reachable by a user until this phase lands.

## Requirements

- Functional: the module is enabled only by an explicit `MODULES` entry.
- Functional: all nine commands appear in `/help` and the native menu with correct metadata.
- Non-functional: `/help` stays under the 4096-rune ceiling the test suite enforces.
- Non-functional: no stats migration — nothing is renamed or deleted.

## Architecture

### Enablement must come before registration

`internal/modules/registry.go:107-116` expands an empty `MODULES` to **every** registered
factory, and its own comment calls that the documented contract. The repo ships
`.env.example:16` as `MODULES=` (empty) and `compose.yml:14` documents "empty = all modules".

So adding `"sticker": sticker.New` to `factories()` **is** the enablement: a public,
write-capable module would go live on the next deploy with no operator decision. An earlier
draft claimed the opposite in three places — goal, requirement, and a success criterion a
reviewer would have ticked without testing.

Ordered fix, per the user's decision:

1. **First**, set `MODULES` explicitly in the deployed environment to the current eleven
   modules, and verify the bot restarts with an unchanged command set:
   `util,misc,amlich,monkeyd,wordle,loldle,lol,stock,gold,coin,stats`
2. **Then** add the `factories()` entry and merge.
3. Add `sticker` to `MODULES` when the operator chooses to turn it on.

Only after step 1 is "remove `sticker` from `MODULES`" a genuine zero-deploy rollback. Before
it, that rollback means enumerating eleven module names into an empty variable under pressure.

Update `.env.example` with the explicit list and a comment stating why, so a fresh clone does
not reintroduce the empty-means-everything trap.

### Registration

One line in `factories()` (`cmd/server/main.go:83`). Plain string key, matching `util`, `misc`,
and `gold`; the `CollectionName` constant form in `lol`/`coin`/`stock` exists because those
packages reuse the name elsewhere, which this one does not.

`Build` validates names against `^[a-z0-9_]{1,32}$` (`validate.go:10`) and rejects duplicates
(`registry.go:173`). All nine were verified free against the current registry; re-run registry
tests to catch later additions. Note `mypack` replaced `packlist` in the one-pack revision —
re-verify that name specifically, since it was not part of the original conflict check.

### The `/help` rune budget

`cmd/server/command_menu_test.go:110-113` renders the full `/help` body and `t.Fatalf`s above
`telegramMessageMaxRunesForTest = 4096` (`:116-119`).

Measured at HEAD: **3212 runes, 45 public commands, 11 modules — 884 runes of headroom.**

The one-pack revision helps here. Each help line is
`InvocationSentence() + " " + SummarySentence()` (`internal/modules/command_presentation.go:16-24`),
so `Parameters` counts against the budget — and four commands lost their `<pack>` token:

| Command | Was | Now |
|---|---|---|
| `/addsticker` | `<pack> [emoji...]` | `[emoji...]` |
| `/renamepack` | `<pack> <title...>` | `<title...>` |
| `/delpack` | `<pack>` | — |
| `/packlist` → `/mypack` | — | — |

That is roughly 25 runes recovered across the module, leaving about **98 runes per command
line** including the module header rather than ~85. Still not generous: `/ordersticker <position>.`
alone is 24 runes, leaving ~74 for its description.

Write the nine descriptions against that budget *before* wiring, then re-measure — the figure
above is derived, not measured post-change. If they do not fit, decide then whether `/help`
needs pagination; that is a separate change, and it must not be "solved" by trimming other
modules' descriptions.

### Test impact — which tests, and which only look related

| Test | Uses real `factories()`? | Action |
|---|---|---|
| `main_test.go:111` `TestFactoriesIncludesExpectedModules` | No — builds only `{"gold","coin"}` | **No change needed** |
| `command_menu_test.go:19` `TestBotCommandMenu_...ModuleOrder` | No — synthetic `alpha`/`beta` | **No change needed** |
| `command_menu_test.go:54` `TestCommandDiscovery_AllPublicCommandsHaveSafeMetadata` | **Yes** — `modules.Build(nil, factories(), …)` | **Add all nine to `expectedParameters`**; also enforces ≤256-rune descriptions, no `Eg:`, no newlines, and the 4096-rune `/help` ceiling |
| `command_menu_test.go:121` `TestBotCommandMenu_StockDividendContracts` | Stock-specific | No change |

`expectedParameters` entries for the nine: `newpack` → `<pack> <title...>`, `addsticker` →
`[emoji...]`, `delsticker` → ``, `editsticker` → `<emoji...>`, `ordersticker` → `<position>`,
`setpackicon` → ``, `renamepack` → `<title...>`, `delpack` → ``, `mypack` → ``.

**`<name...>` is not yet a documented form.** `docs/command-parameter-conventions.md:15-20`
defines `<name>`, `<name,...>`, `[name]`, and `[name...]` — required *remaining text* appears
nowhere, and no existing command uses it (`rg "Parameters:"` across `internal/` confirms).
Three of the nine (`<title...>` twice, `<emoji...>`) need it. Add the row
`| Required remaining text | \`<name...>\` | \`<title...>\` |` to that table and an example
line, in the same change that registers the commands — the conventions doc is the authority
these registrations are validated against, so shipping an undocumented form silently
demotes it.

### README and docs

Module table row listing the nine commands, then a `### Sticker packs` section: **one pack per
user**; the pack is created on behalf of the calling user; the bot manages only packs it
created; the slug is chosen once at `/newpack` and fixes a permanent share link that
`/renamepack` cannot change; static stickers and photos only; the 120-stickers-per-pack cap;
anonymous group admins are not supported and why.

Note the relationship to `/stickerid` in `util` — it stays put. It is a private debug helper for
reading a `file_id`, not pack management.

`docs/sticker-packs.md` carries: the full command reference with `Parameters` matching handler
usage text exactly (`docs/command-parameter-conventions.md` § Change Checklist); slug rules and
the permanence of the resulting URL; the image contract (512px long edge, PNG; thumbnails
100×100); what is deliberately absent and why (usage-statistics commands — no Bot API support;
animated/video/emoji packs — out of scope; multiple packs per user — see plan R10;
conversational flow — no message hook at `internal/modules/dispatcher.go:66`); the accepted
slug-occupancy disclosure; and the `Count` drift note.

### Stats compatibility

No command is renamed or deleted **in the shipped bot** — `/packlist` never existed outside this
plan, so its replacement by `/mypack` needs no migration. `AGENTS.md` § "Stats Compatibility"
governs renames of live commands and does not apply. New commands accrue stats through the
dispatcher hook (`dispatcher.go:80-84`).

## Related Code Files

- Modify: deployed environment `MODULES` (**before** the code change), `.env.example`
- Modify: `cmd/server/main.go` (factory entry + import)
- Modify: `cmd/server/command_menu_test.go` (`expectedParameters`, +9)
- Modify: `README.md`
- Create: `docs/sticker-packs.md`

## Implementation Steps

1. Set `MODULES` explicitly in the deployed environment; verify an unchanged command set.
2. Draft the nine descriptions against the ~98-rune budget; measure `RenderHelp` locally.
3. Add the factory entry and import.
4. Update `expectedParameters`; run `go test ./cmd/server/...`.
5. README row + section; `docs/sticker-packs.md`.
6. Full gate, then the manual smoke sequence.

## Todo

- [ ] Set `MODULES` explicitly in the deployed environment and verify
- [x] Update `.env.example` with the explicit list and a why-comment
- [x] Confirm `mypack` is free in the registry alongside the other eight
- [x] Draft nine descriptions within the measured budget and re-measure `RenderHelp`
- [x] Add `"sticker": sticker.New` to `factories()`
- [x] Add nine entries to `expectedParameters`
- [x] Add the `<name...>` row + example to `docs/command-parameter-conventions.md`
- [x] README module-table row + `### Sticker packs` section
- [x] `docs/sticker-packs.md`
- [x] Full validation gate
- [ ] Manual smoke sequence against a real token

## Success Criteria

- [x] `gofmt -l .` empty; `go vet ./...` clean; `go test ./...` passes; `golangci-lint run` clean
- [x] `RenderHelp` stays under 4096 runes with all nine commands registered
- [ ] With `MODULES` unset in a scratch environment, the module still loads — confirming C10 is understood rather than assumed away
- [ ] With the explicit `MODULES` list and no `sticker` entry, none of the nine commands register
- [ ] Smoke: reply to a sticker with `/newpack smoke_pack Smoke Pack`; the returned link opens
- [ ] Smoke: a second `/newpack` is refused and names the existing pack
- [ ] Smoke: reply to a photo with `/addsticker 😂` — no pack named — and the sticker renders undistorted
- [ ] Smoke: `/mypack`, `/renamepack New Title`, `/setpackicon`, `/ordersticker 0` all succeed
- [ ] Smoke: `/renamepack` leaves the share link working and unchanged
- [ ] Smoke: a **second** account replying to the first account's sticker with `/delsticker` is refused
- [ ] Smoke: an anonymous group admin is refused with the explanatory message
- [ ] Smoke: `/delpack` confirm prompt shows the title, sticker count, and link before confirming
- [ ] Smoke: `/delpack` → confirm → link 404s; a second press reports already-used; `/newpack` then works again
- [ ] Smoke: **after deleting, attempt `/newpack` with the same slug** — this settles plan R11. Record the outcome in `docs/sticker-packs.md` either way, and if the slug is reserved, add that to `/delpack`'s confirm text
- [ ] Smoke: `/renamepack` reply names the delete-and-recreate route
- [ ] Smoke: observed error strings for an occupied slug and a full pack match `replyAPIError`, or the table is corrected

## Risk Assessment

**The error-code table is unverified until this phase.** Only three MTProto codes are rewritten
into prose by the Bot API server; the rest were inferred from the open-source server's rewrite
table, not a live reproduction (plan R3). The smoke sequence deliberately includes an
occupied-slug and a full-pack case. A mismatch is a polish gap, not a blocker — the generic
fallback means users see a sane message either way.

**Manual smoke is the only Telegram-side coverage.** Every automated test uses `RecordingBot`,
which never contacts Telegram. CI cannot catch a wrong parameter name or a rejected image
format. The smoke sequence is mandatory before announcing the feature.

**Enablement ordering is a process risk, not a code one.** If step 1 is skipped and the factory
entry merges first, the module goes live unannounced. Signal: the deployed bot answers
`/mypack` before anyone enabled it. Response: set `MODULES` immediately; the module is otherwise
harmless until someone runs a command.
