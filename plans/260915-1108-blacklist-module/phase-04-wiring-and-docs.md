---
phase: 4
title: "Phase 4: Wiring and documentation"
status: done
priority: P2
effort: "1h"
dependencies: [3]
---

# Phase 4: Wiring and documentation

## Overview

Register the module in the composition root and document it. This is the phase that makes
the six commands reachable by a real user.

## Requirements

- Functional: the module is in `factories()`, so it loads by default and can be selected or
  omitted through `MODULES`.
- Functional: `/help` and Telegram's native command menu list all six commands, which follows
  automatically from registration and needs only to be asserted.
- Non-functional: README and a feature doc describe the behaviour a user can observe,
  including the two things most likely to surprise them.

## Architecture

### `cmd/server/main.go`

One line in `factories()` (`cmd/server/main.go:80-99`):

```go
"blacklist": blacklist.New,
```

Plain string rather than a `CollectionName` constant, matching `alias`, `gold` and `misc`;
only modules whose name is referenced elsewhere export one.

An unset or empty `MODULES` loads every registered module (`internal/modules/registry.go:178`),
so this line alone enables it on the existing deployment. No `init*Store` call is needed —
the module has no startup migration and no cross-module state.

### `docs/blacklist.md`

Following `docs/aliases.md` in shape. It has to answer the three questions a user will
actually arrive with:

- **The bot does not police chat.** The lists are inert until `/blacklist_check` asks. Worth
  stating first and plainly, because the module's name promises enforcement it deliberately
  does not perform.
- **Lists are per topic.** Entries added in one forum topic are invisible in the next, and a
  DM has its own private list. This is the most likely support question.
- **Diacritics are significant.** `ma`, `má` and `mà` are three entries; catching all three
  means adding all three. Case and spacing are not significant, and the same word typed on
  an iPhone and on Android match.

Plus a worked whitelist example, since span containment is not guessable: blacklist `ass`,
whitelist `assassin`, and the three outcomes including `I met an assassin, dumbass`.

### `README.md`

One row in the module table:

```text
| `blacklist` | Per-topic text deny-list with whitelist exceptions: `/blacklist_add`, `/blacklist_del`, `/whitelist_add`, `/whitelist_del`, `/blacklist_rules` lists both, `/blacklist_check` judges a text. Passive — the bot never scans chat. See [docs/blacklist.md](docs/blacklist.md) |
```

## Files

| File | Change |
|---|---|
| `cmd/server/main.go` | import + one `factories()` entry |
| `cmd/server/main_test.go` | extended |
| `docs/blacklist.md` | new |
| `README.md` | one table row |

## Steps

1. Add the import and the `factories()` entry.
2. Extend `TestFactoriesIncludesExpectedModules` (`cmd/server/main_test.go:111`) to build a
   registry containing `blacklist` and assert all six command names resolve.
3. Write `docs/blacklist.md`.
4. Add the README row.
5. `gofmt`, then the full gate below.

## Validation

- `go test ./...`
- `go vet ./...`
- `golangci-lint run`
- `MODULES=blacklist` builds a registry with exactly the six commands and no conflict.
- Unset `MODULES` builds a registry including `blacklist` alongside every existing module —
  the check that no command name collides with an existing one.
- Every link in `README.md` and `docs/blacklist.md` resolves.

## Risk

A command-name collision surfaces only when the full catalog is built, because
`Registry.addCommands` rejects duplicates across modules at startup. The unset-`MODULES`
test is what turns that from a deploy-time crash into a test failure. `blacklist_*` and
`whitelist_*` are unused by any existing module today, so the expected result is green.

## Rollback

Revert the `factories()` entry; the package becomes dead code without affecting a running
deployment. Revert the docs separately.
