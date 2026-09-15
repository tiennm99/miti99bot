---
phase: 3
title: "Phase 3: /blacklist_rules and /blacklist_check"
status: done
priority: P1
effort: "2h"
dependencies: [2]
---

# Phase 3: `/blacklist_rules` and `/blacklist_check`

## Overview

The two read commands, replacing the Phase 2 stubs. `/blacklist_check` is where the Phase 1
matcher finally meets stored data; `/blacklist_rules` is where arbitrary user text meets
Telegram's message limit.

## Requirements

- Functional: `/blacklist_check` reports a verdict naming the blacklist entry that decided it
  and, when the text was rescued, the whitelist entry responsible.
- Functional: `/blacklist_rules` shows both lists in one message, each under its own heading,
  with empty lists visibly empty rather than absent.
- Functional: a listing too long for one Telegram message is trimmed with a count of what was
  omitted.
- Non-functional: `/blacklist_check` costs one `List` per list and no per-entry reads.
- Non-functional: both commands are bounded by `handlerTimeout` and escape all user text.

## Architecture

### Loading a thread's entries

```go
// entriesFor returns one list's normalized entries, sorted. The key holds the
// normalized text, so this needs no per-entry document read — the difference
// between one round trip and one per rule.
func (s *state) entriesFor(ctx context.Context, chatID int64, threadID int, list string) ([]string, error)
```

`DocStore.List` returns full keys, as `alias` relies on at `handlers.go:260`. Strip
`scopePrefix(...)` with `strings.TrimPrefix`, run `decodeKeyText`, sort.

### `/blacklist_check`

Normalize the argument through `Normalize`, load both lists, call `Check`, and render the
`Verdict`'s three shapes:

- `Blocked` — say so and name the entry.
- Not blocked but `Entry != ""` — say it is allowed, name the entry that matched, and name
  the whitelist entry that rescued it. This case is the reason the whitelist is worth having
  a display for at all: without it, a user who added an exception has no way to confirm it
  is doing anything.
- Zero `Verdict` — nothing matched.

Each reply also states the scope, so a user who runs the command in the wrong topic can see
why the answer surprised them.

### `/blacklist_rules`

```go
const maxListBytes = 3800
```

The same budget `alias` uses (`internal/modules/alias/handlers.go:29-36`) and for the same
reason: Telegram's 4096-character `sendMessage` cap measures the message actually sent, so
the `<code>` tags count too, and at 13 bytes a pair they outweigh a short entry.

One message, two headed sections, blacklist first. Entries come from the stored `Entry.Text`
rather than the key, so the list shows what people typed — which costs one read per *listed*
entry, bounded by what fits in the message rather than by how many entries exist, exactly as
`alias.renderNames` is. Each entry is wrapped in `<code>` so tapping it copies the text ready
to paste into a `_del` command.

Both sections share the single byte budget; the trim notice names how many entries were
omitted. An empty list prints its heading followed by a short "none yet" line, because a
missing heading reads as a bug rather than as an empty list.

## Files

| File | Change |
|---|---|
| `internal/modules/blacklist/handlers.go` | stubs replaced; `entriesFor`, `renderRules` added |
| `internal/modules/blacklist/handlers_test.go` | extended |

## Steps

1. Write `entriesFor`.
2. Replace the `/blacklist_check` stub; render the three `Verdict` shapes.
3. Replace the `/blacklist_rules` stub; write `renderRules` with the shared byte budget.
4. Extend `handlers_test.go`.
5. `gofmt`, run the package tests.

## Validation

`go test ./internal/modules/blacklist/...`:

- The plan's headline case end to end: blacklist `ass`, whitelist `assassin`, then
  `/blacklist_check assassin` is allowed and names both entries, `/blacklist_check dumbass`
  is blocked, and `/blacklist_check I met an assassin, dumbass` is **blocked**.
- `/blacklist_check` against empty lists reports nothing matched.
- `/blacklist_check` in thread B does not see entries added in thread A.
- `/blacklist_rules` on empty lists prints both headings and no entries.
- `/blacklist_rules` shows `Entry.Text` as typed, not the normalized form.
- Enough entries to exceed `maxListBytes` produce a message under 4096 characters carrying an
  accurate omitted count.
- An entry containing `<b>` appears escaped.
- `entriesFor` returns sorted output given keys listed in any order.

## Risk

`renderRules` splits one byte budget across two sections; a naive implementation gives the
first section the whole budget and leaves the whitelist permanently invisible on a busy
thread. Reserve the trim notice before committing a line, as `alias.renderNames` does, and
test with a blacklist alone large enough to exhaust the budget.

## Rollback

Restore the Phase 2 stubs. The mutation commands stay usable.
