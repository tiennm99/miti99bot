---
title: "Blacklist module: from brainstorm to deploy"
date: 2026-09-15
summary: "Shipped a passive per-topic blacklist/whitelist module; review caught a /help rune ceiling, four unpinned escaping sites, and a reply-chain scoping trap"
---

# Blacklist module: from brainstorm to deploy

## What happened

Delivered `internal/modules/blacklist` end to end in one session: brainstorm, plan
(`plans/260915-1108-blacklist-module/`), four phases, review, and push to `main` as `e4412d5`.

Six public commands over two per-topic lists. The module is passive by explicit decision —
it never scans chat traffic, because enforcement would need a dispatcher message hook,
BotFather privacy mode off, and per-group admin delete rights.

Four owner decisions shaped it, each settled before any code: passive registry over
auto-moderation; whitelist as an exception layer rather than an independent list; per-thread
and world-writable scope; and diacritic-sensitive matching (`ma`, `má`, `mà` are three
entries).

## What the review caught

Four things a passing test suite did not:

1. **`/help` was 9 runes from its 4096-rune pin.** Measured directly: the module cost 480 of
   the 489 runes that were left. `RenderHelp` HTML-escapes both halves of every line, so each
   `'` costs 5 runes and each `<text...>` costs 14. Shortening six command descriptions
   restored headroom to 76. The ceiling is structural — `/help` sends one un-chunked message —
   and the next module anywhere in the repo will hit it.
2. **Four surviving mutants.** Three HTML-escaping sites and the post-normalization byte cap
   were correct in code but pinned by no test. Fixed, and each new test was verified to fail
   with its guard removed rather than assumed to be load-bearing.
3. **Usage strings contradicted their own `Parameters` metadata** (`<text>` vs `<text...>`),
   against `docs/command-parameter-conventions.md` item 3.
4. **Reply-chain thread ids.** `threadOf` trusted `msg.MessageThreadID` unconditionally.
   Telegram associates a thread id with any reply chain in a supergroup, not only with a
   forum topic, so a reply-form `/blacklist_add` in a plain supergroup would have written to a
   scope no standalone `/blacklist_rules` could read back. Now gated on `IsTopicMessage`.

## DocStore.Scan landed mid-work

The push was rejected: four commits had landed on `main` meanwhile, one adding
`DocStore.Scan` specifically to kill the List-then-Get-per-key pattern. The phase-3 design
had copied exactly that pattern from `alias.renderNames`, and `/blacklist_rules` paid it
twice per call. Rebased and adopted `Scan` before pushing. `/blacklist_check` still uses
`List`, needing only entry names.

Worth noting for next time: a plan written against a contract can be overtaken by that
contract while the plan is being executed. The rebase was the moment to re-read what changed,
not just to resolve conflicts.

## Decisions

- Per-thread entry count stays unbounded, matching `alias`. Entries remain capped at 200
  bytes each.
- `internal/modules/module.go` shows a one-space gofmt drift under go1.27.1 but is untouched
  by this work and accepted by `golangci-lint`. Left alone rather than adding unrelated churn.

## Next steps

- Watch `/help` — 76 runes of headroom is one command away from red. Chunking it is a shared
  surface change nobody has scoped yet.
- Confirm on the live bot that reply-form `/blacklist_add` and a later `/blacklist_rules`
  agree in a non-forum supergroup.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
