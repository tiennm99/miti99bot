---
title: "Blacklist module"
description: "internal/modules/blacklist — per-thread deny-list and exception-list of text, curated by anyone, queried on demand. Passive: the bot never acts on chat traffic."
status: done
priority: P2
effort: ""
tags: ["blacklist", "telegram-bot", "module"]
created: 2026-09-15
branch: main
blockedBy: []
blocks: []
---

# Blacklist module

## Overview

New module `internal/modules/blacklist`. Each chat thread curates two lists of text: a
**blacklist** of forbidden entries and a **whitelist** of exceptions that rescue false
positives. `/blacklist_check <text>` scans a text against that thread's rules and reports a
verdict.

The module is **passive**. It never reads ordinary chat messages and never deletes, warns,
or restricts anyone. It only answers when a command is invoked.

## Accepted scope

| Decision | Value |
|---|---|
| Behaviour | Passive registry. No message scanning, no enforcement |
| Scope | Per thread — `(Chat.ID, MessageThreadID)`; a DM is `(userID, 0)` |
| Visibility | All `VisibilityPublic`; anyone in a thread may modify that thread's lists |
| Matching | Substring, in normalized space |
| Normalization | NFKC + lowercase + whitespace collapse. **Diacritics preserved** |
| Whitelist | Exception layer — rescues a blacklist match only by span containment |

### Why passive

Enforcement would need a `Module.MessageHook` primitive the dispatcher does not have
(`internal/modules/dispatcher.go` registers only commands, callback prefixes, one fallback
and one inline query), plus privacy mode disabled in BotFather and group-admin delete
rights. All of that is out of scope. Nothing in this plan forecloses adding it later: the
matcher is a pure function a future hook can call unchanged.

### Why diacritics are preserved

Chosen by the project owner. `ma`, `má` and `mà` are three distinct entries. The cost is
that dropping a diacritic evades an entry, so each form worth catching must be added
separately — a curation choice, not a defect. NFKC still runs, so full-width and
compatibility variants of the same characters do collapse, and Vietnamese text composed as
NFD by iOS clients matches the same text composed as NFC by Android clients.

## Command surface

All `VisibilityPublic`. `Parameters` follows `docs/command-parameter-conventions.md`.

| Command | Parameters | Behaviour |
|---|---|---|
| `/blacklist_add` | `[text...]` | Add to this thread's blacklist. No argument → the replied-to message's text |
| `/blacklist_del` | `<text...>` | Remove from this thread's blacklist |
| `/whitelist_add` | `[text...]` | Add an exception. No argument → the replied-to message's text |
| `/whitelist_del` | `<text...>` | Remove an exception |
| `/blacklist_rules` | — | This thread's blacklist and whitelist entries, in one message |
| `/blacklist_check` | `<text...>` | Verdict for the text, naming the entry that matched and any exception that rescued it |

`/blacklist_rules` carries the `blacklist_` prefix so Telegram's native menu sorts it beside
the other `blacklist_*` commands, and its description states that it shows both lists.

## Goals

| # | Goal | Priority |
|---|------|----------|
| 1 | Two threads of one forum keep entirely separate lists | P1 |
| 2 | A whitelist entry rescues only the blacklist matches it actually spans | P1 |
| 3 | Vietnamese text matches regardless of the client's Unicode composition | P1 |
| 4 | No command can stall the bot beyond a bounded deadline | P1 |
| 5 | Arbitrary user text is safe as a storage key and as Telegram HTML | P1 |
| 6 | A list too long for one Telegram message is trimmed, not dropped | P2 |

## Phases

| # | Phase | Depends on | Effort |
|---|---|---|---|
| 1 | [Scope keys, normalization, matcher](phase-01-scope-normalize-match.md) | — | done |
| 2 | [Store, factory, mutation commands](phase-02-store-and-mutations.md) | 1 | done |
| 3 | [`/blacklist_rules` and `/blacklist_check`](phase-03-rules-and-check.md) | 2 | done |
| 4 | [Wiring and documentation](phase-04-wiring-and-docs.md) | 3 | done |

Phases 2 and 3 landed in one edit: both write `internal/modules/blacklist/handlers.go`,
so the planned stub-then-replace step was skipped.

Phase 1 is pure Go with no Telegram and no storage, so it carries the whole matcher
test suite and can be reviewed on its own.

## Acceptance criteria

1. Adding, listing, checking and deleting in two threads of the same forum supergroup do not
   leak across threads; the same commands work in a DM.
2. Given blacklist `ass` and whitelist `assassin`: `assassin` is ALLOWED, `dumbass` is
   BLACKLISTED, and `I met an assassin, dumbass` is BLACKLISTED.
3. `má` does not match an entry `ma`; `Má` does match an entry `má`; the same Vietnamese
   word in NFC and NFD form match each other.
4. An entry containing `/`, `%`, `:` or a newline round-trips through storage and renders
   escaped in `/blacklist_rules`.
5. `/blacklist_rules` with more entries than fit in 4096 characters returns a trimmed
   message with a count of what was omitted.
6. `go test ./...`, `go vet ./...` and `golangci-lint run` all pass.

## Stats compatibility

Six new command names, no rename and no deletion, so `AGENTS.md`'s stats migration rules
impose no work. Command names must not change after release without the migration those
rules require.

## Risks

| Risk | Mitigation |
|---|---|
| User text used as a storage key hits the `/`-forbidden and 1500-byte rules in `internal/storage/keys.go` | Percent-encode `%` then `/`; cap normalized entries at 200 bytes (Phase 1) |
| `DocStore.List` has no ordering guarantee, so verdicts could name different entries across runs | Sort entries before scanning (Phase 1) |
| Unescaped user text in an HTML reply | Every entry passes through `html.EscapeString` at every render site (Phases 2-3) |
| `golang.org/x/text` promoted from indirect to direct dependency | Already present at v0.41.0 in `go.sum`; no new download, `go mod tidy` only moves the line |

## Outcome

All four phases delivered; every acceptance criterion above verified. Full gate green:
`go test ./...`, `go vet ./...`, `golangci-lint run` (0 issues).

Two files the plan did not anticipate had to change, both required by existing repo
contracts rather than by the feature:

- `cmd/server/command_menu_test.go` pins every public command's `Parameters` string, and
  its reverse loop makes even an empty entry load-bearing. The six new commands are
  registered there.
- The six command `Description` strings were shortened after review. `/help` renders as one
  un-chunked message pinned at 4096 runes, and the first draft left 9 runes of headroom;
  the shorter wording restores it to 76. See the open questions below — that ceiling is a
  shared limit this module did not create and cannot fix alone.

`/blacklist_rules` reads each list with `DocStore.Scan`, which landed on `main` while this
work was in progress. The phase-3 design called for `List` followed by a `Get` per displayed
entry, copying `alias.renderNames`; `Scan` exists precisely to remove that N+1, and this
command paid it twice per invocation. `/blacklist_check` still uses `List`, because it needs
only the entry names and never their stored text.

Test integrity was checked by mutation: fourteen mutants were injected against the matcher,
key encoding, normalization, trimming and escaping. The four that initially survived —
three HTML-escaping sites and the post-normalization byte cap — are now covered by tests
verified to fail without the guard they pin.

## Post-review decisions

1. **Reply-thread scoping.** `threadOf` now gates on `msg.IsTopicMessage` rather than
   trusting `msg.MessageThreadID` alone. Telegram associates a thread id with any reply chain
   in a supergroup, not only with a forum topic, so the original code would have given a
   reply-form `/blacklist_add` in a plain supergroup a scope that a later standalone
   `/blacklist_rules` could never read back. A forum topic keeps its own lists; a plain
   group, a DM, and a forum's General topic all resolve to thread 0. Pinned by
   `TestReplyChainThreadIsNotATopic`, verified to fail without the gate.
2. **Per-thread entry count stays unbounded**, matching `alias`, which is equally
   world-writable and equally uncapped. Entries remain capped at 200 bytes each. Revisit only
   if a thread's list grows large enough to make `/blacklist_check` slow.
