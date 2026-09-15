---
phase: 2
title: "Phase 2: Store, factory, mutation commands"
status: done
priority: P1
effort: "3h"
dependencies: [1]
---

# Phase 2: Store, factory, mutation commands

## Overview

The module becomes a real `modules.Module`: a persisted record, a factory registering six
commands, and the four that mutate a list. The two read commands land in Phase 3, but all
six are registered here so the command surface is reviewable in one place — the unimplemented
pair point at stub handlers that Phase 3 fills in.

Blacklist and whitelist share one handler pair parameterised by list tag. They differ only
in which prefix they write to, and duplicating them would be two bugs to fix instead of one.

## Requirements

- Functional: `/blacklist_add`, `/whitelist_add` store an entry in the calling thread's list,
  taking the text from the argument or, when there is none, from the replied-to message.
- Functional: `/blacklist_del`, `/whitelist_del` remove an entry, distinguishing "removed"
  from "was not there".
- Functional: re-adding an existing entry is reported as such rather than silently
  overwriting in silence.
- Functional: text that normalizes to nothing, or exceeds `maxEntryBytes`, is refused with a
  message saying which.
- Non-functional: every handler is bounded by a timeout; none can stall the single update
  worker.
- Non-functional: user text is HTML-escaped at every render site.

## Architecture

### `internal/modules/blacklist/blacklist.go`

Package doc records the two decisions a reader will otherwise re-litigate: the module is
passive by design, and lists are per-thread and world-writable.

```go
// Entry is one stored rule. The storage key holds the normalized form, so Text
// carries what the adder actually typed — the only place the original casing and
// spacing survive, and what /blacklist_rules shows.
type Entry struct {
    Text      string `bson:"text"`
    OwnerID   int64  `bson:"ownerId"`
    CreatedAt int64  `bson:"createdAt"`
}

type Store = storage.DocStore[Entry]

type state struct{ store Store }

func New(deps modules.Deps) modules.Module
```

No `Registry` in `state`: nothing here needs to introspect commands the way `alias` does.

Both lists live in the one `Deps.Store` collection, separated by the key prefixes Phase 1
built. One `storage.Typed[Entry]` view serves both.

### Thread resolution

```go
// threadOf returns the scope of the message. A DM is (user's chat ID, 0) and a
// non-forum group is (chat ID, 0); neither needs a special case.
func threadOf(msg *models.Message) (chatID int64, threadID int)
```

Replies are sent with `chathelper.Reply`/`ReplyHTML`, which already forward
`MessageThreadID` so a reply in a forum topic stays in that topic rather than being routed
to General.

### `internal/modules/blacklist/handlers.go`

```go
const handlerTimeout = 10 * time.Second
const genericFailure = "Something went wrong. Try again in a moment."
```

Ten seconds matches `alias`. These handlers only touch storage, so the budget is generous.

```go
// resolveText picks the entry text out of the update: the command argument when
// there is one, otherwise the replied-to message's text. Returns the raw text
// for echoing and the normalized form for keying.
func resolveText(msg *models.Message) (raw, norm string, err error)
```

Errors are distinguishable, because "say something useful" is the whole job here:
no text found at all; normalizes to empty; longer than `maxEntryBytes`.

The reply path matters more than it looks: the natural gesture is to see a message, reply to
it, and ban it. It also makes the length cap load-bearing, since a replied-to message can
carry 4096 characters, and the refusal must say so rather than failing opaquely.

```go
func (s *state) handleAdd(list string) modules.CommandHandler
func (s *state) handleDel(list string) modules.CommandHandler
```

Closures over the list tag, so the factory registers `s.handleAdd(listBlack)` and
`s.handleAdd(listWhite)`.

`handleAdd` reads before writing to word the reply — "Added" against "Already in the
blacklist" — accepting that a concurrent add makes the noun wrong without making the stored
state wrong, the same trade `alias` documents at `handlers.go:120-123`.

`handleDel` reads before deleting for a sharper reason: `DocStore.Delete` does not
distinguish a missing key from a removed one, so without the read "Removed" would be
reported for something that never existed.

Anyone may remove anyone's entry. The list is shared by the thread, so the permission model
is too; a per-owner rule would strand entries whose adder has left the group.

### Command registration

All `VisibilityPublic`.

| Name | Parameters | Description |
|---|---|---|
| `blacklist_add` | `[text...]` | `Add text to this thread's blacklist, or reply to a message` |
| `blacklist_del` | `<text...>` | `Remove text from this thread's blacklist` |
| `whitelist_add` | `[text...]` | `Add a whitelist exception, or reply to a message` |
| `whitelist_del` | `<text...>` | `Remove a whitelist exception` |
| `blacklist_rules` | — | `List this thread's blacklist and whitelist entries` |
| `blacklist_check` | `<text...>` | `Check whether a text is blacklisted in this thread` |

`[text...]` and `<text...>` follow `docs/command-parameter-conventions.md`: square brackets
for the optional-because-a-reply-works case, the `...` suffix for remaining text.

## Files

| File | Change |
|---|---|
| `internal/modules/blacklist/blacklist.go` | new |
| `internal/modules/blacklist/handlers.go` | new |
| `internal/modules/blacklist/handlers_test.go` | new |

## Steps

1. Write `blacklist.go`: `Entry`, `Store`, `state`, `New` with all six registrations (the
   two read commands pointing at stubs that reply with nothing yet).
2. Write `threadOf` and `resolveText`.
3. Write `handleAdd` and `handleDel` as list-parameterised closures.
4. Write `handlers_test.go` against `storage.NewMemoryProvider()` and the test bot harness
   used by `internal/modules/alias/handlers_test.go`.
5. `gofmt`, then run the package tests.

## Validation

`go test ./internal/modules/blacklist/...`:

- Adding then reading back the key confirms the entry landed under the right prefix with
  `Text` as typed, not as normalized.
- The same text added in two different threads of one chat produces two entries, and
  removing one leaves the other.
- `/blacklist_add` with no argument and no reply returns usage text, not a stored entry.
- `/blacklist_add` replying to a message stores that message's text.
- A 4096-character replied-to message is refused with the length message.
- Text that is only whitespace or only punctuation stripped by normalization is refused.
- Adding an existing entry replies "already", and the stored record is unchanged.
- `/blacklist_del` for an absent entry replies "not there" and is not reported as removed.
- An entry containing `<b>` renders escaped in every reply that echoes it.
- `/whitelist_add` writes under the `w` prefix and is invisible to a `b`-prefix list.

## Risk

`resolveText` is where a caller's 4096-character message meets a 200-byte key. Getting the
order wrong — keying before checking the length — turns a chatty refusal into an opaque
storage error. Normalize, then measure, then key.

## Rollback

Still no importers outside the package. Revert the two files; Phase 1 stands alone.
