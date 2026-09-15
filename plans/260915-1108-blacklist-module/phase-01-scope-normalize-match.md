---
phase: 1
title: "Phase 1: Scope keys, normalization, matcher"
status: done
priority: P1
effort: "3h"
dependencies: []
---

# Phase 1: Scope keys, normalization, matcher

## Overview

The whole decision core of the module, with no Telegram and no storage in it. Three pure
files that a test can drive directly: how a thread becomes a key prefix, how arbitrary user
text becomes a comparable and storable form, and how a text is judged against two lists.

Every correctness risk in the module lives here. Isolating it means the matcher's behaviour
is settled and tested before a single handler exists.

## Requirements

- Functional: a `(chatID, threadID, list)` triple maps to a unique, collision-free key
  prefix, and an entry's normalized text maps to a key under it that survives
  `internal/storage/keys.go` validation.
- Functional: normalization composes Unicode, folds case and collapses whitespace, and
  leaves combining diacritics intact.
- Functional: the matcher reports whether a text is blocked, which blacklist entry decided
  it, and which whitelist entry rescued it when one did.
- Non-functional: no dependency on `go-telegram/bot` or `internal/storage` in any of the
  three files, so the package's core is unit-testable in isolation.
- Non-functional: deterministic output for the same inputs regardless of slice order.

## Architecture

### `internal/modules/blacklist/scope.go`

A thread is `(Chat.ID, MessageThreadID)`, the same pair `lol` already uses for subscriptions
(`internal/modules/lol/subscribers.go:14-24`). `MessageThreadID == 0` means a non-forum
group, a forum's General topic, or a DM — a real scope, not a missing value.

```go
const (
    listBlack = "b"
    listWhite = "w"
)

// scopePrefix is the key prefix for one list of one thread. Both numbers are
// decimal and the list tag is a single letter, so the third ':' always ends the
// prefix even though entry text may contain ':' of its own.
func scopePrefix(chatID int64, threadID int, list string) string

// entryKey names one entry. normText must already be normalized.
func entryKey(chatID int64, threadID int, list, normText string) string
```

Key shape: `<chatID>:<threadID>:<b|w>:<encoded text>`. Group chat IDs are negative, which
costs a leading `-` and nothing else.

`internal/storage/keys.go:24-39` forbids `/`, the exact strings `.` and `..`, the
`__namespace__` pattern, and keys over 1500 bytes. The numeric prefix makes the middle three
unreachable by construction. `/` and the length cap are handled explicitly:

```go
// encodeKeyText escapes the two characters that cannot appear literally in a
// key. '%' is escaped first so the escape marker itself stays unambiguous —
// reversing the order would make an entry containing "%2F" decode as '/'.
func encodeKeyText(s string) string  // "%" -> "%25", then "/" -> "%2F"
func decodeKeyText(s string) string  // "%2F" -> "/", then "%25" -> "%"
```

`maxEntryBytes = 200`, measured on the normalized text before encoding. Worst-case encoding
triples it to 600 bytes, which with a prefix under 30 bytes stays far inside 1500. Two
hundred bytes is roughly 65 Vietnamese characters — ample for a phrase, and a deliberate
refusal to let someone paste an essay into a key.

### `internal/modules/blacklist/normalize.go`

```go
// Normalize converts user text to the single form used for both storage keys
// and matching. Returns ok=false when nothing comparable is left.
func Normalize(s string) (norm string, ok bool)
```

Three steps, in this order:

1. **NFKC** via `golang.org/x/text/unicode/norm`. Canonical composition is what makes
   Vietnamese portable: iOS clients may send `má` as `m` + `a` + combining acute (NFD) while
   Android sends the precomposed character (NFC), and without this they are different
   strings. The compatibility half additionally folds full-width and other presentation
   variants onto their plain forms, which closes an evasion route for free. It does **not**
   strip combining marks, so `má` stays `má` — the project owner's choice.
2. **`strings.ToLower`** after normalization, not before, because folding can otherwise
   interact with composition.
3. **Whitespace collapse** — `strings.Fields` joined by a single space, which also trims.
   `cat  dog` and `cat dog` are the same rule; a newline is just whitespace.

Empty input, or input that is only whitespace, returns `ok=false`.

`golang.org/x/text` is already at v0.41.0 in `go.sum` as an indirect dependency, so this
promotes an existing line rather than adding a dependency.

### `internal/modules/blacklist/match.go`

```go
// Verdict is the outcome of checking one text against one thread's rules.
type Verdict struct {
    Blocked   bool
    Entry     string // the blacklist entry that decided the verdict; "" when none matched
    RescuedBy string // the whitelist entry covering Entry; set only when !Blocked && Entry != ""
}

// Check judges normText. Both slices hold normalized entries and are sorted by
// the caller.
func Check(normText string, blacklist, whitelist []string) Verdict
```

The algorithm is span containment, not mere presence:

1. Collect every occurrence of every whitelist entry in `normText` as a `[start, end)` span,
   by looping `strings.Index` over the remainder.
2. For each blacklist entry, in sorted order, walk its occurrences. A span is **rescued**
   only if some whitelist span `[a, b)` satisfies `a <= start && end <= b`.
3. The first unrescued span wins: return `Blocked: true` naming its entry.
4. If every occurrence was rescued, return `Blocked: false` naming the first rescued entry
   and the whitelist entry that covered it. If nothing matched at all, return the zero
   `Verdict`.

The containment rule is the point of the phase. The tempting shortcut — *allow the text if
any whitelist entry appears anywhere in it* — passes the `assassin` case and then lets
`I met an assassin, dumbass` through, because the whitelist span never covers the second
match. A whitelist entry that merely overlaps a blacklist match without containing it does
not rescue it, which is the same rule stated from the other side.

Sorting matters for more than tidiness: `DocStore.List` gives no ordering guarantee, so
without it the same text could be reported against a different entry on each invocation.

## Files

| File | Change |
|---|---|
| `internal/modules/blacklist/scope.go` | new |
| `internal/modules/blacklist/normalize.go` | new |
| `internal/modules/blacklist/match.go` | new |
| `internal/modules/blacklist/scope_test.go` | new |
| `internal/modules/blacklist/normalize_test.go` | new |
| `internal/modules/blacklist/match_test.go` | new |
| `go.mod` | `golang.org/x/text` moves from indirect to direct |

## Steps

1. Write `scope.go` with the two key builders and the encode/decode pair.
2. Write `normalize.go`.
3. Write `match.go`.
4. Write the three test files (see Validation).
5. Run `go mod tidy`, confirming the only diff is `golang.org/x/text` changing section.
6. `gofmt` every new file.

## Validation

`go test ./internal/modules/blacklist/...` covering at minimum:

**scope_test.go**
- A negative chat ID and a zero thread ID produce a prefix ending in exactly three `:`.
- Two different threads of the same chat produce prefixes where neither is a prefix of the
  other — the property that keeps `List` from leaking across threads.
- `encodeKeyText` then `decodeKeyText` round-trips text containing `/`, `%`, `%2F`, `:` and
  a newline.
- A key built from a 200-byte entry passes `storage`'s key rules.

**normalize_test.go**
- The same Vietnamese word in NFC and NFD normalizes to one string.
- `Má` and `má` normalize alike; `ma` and `má` do **not**.
- A full-width variant folds onto its plain form.
- `"  cat   dog  "` normalizes to `"cat dog"`.
- `""` and `"   "` return `ok=false`.

**match_test.go** — blacklist `ass`, whitelist `assassin` unless stated:
- `assassin` → not blocked, `Entry: "ass"`, `RescuedBy: "assassin"`.
- `dumbass` → blocked.
- `I met an assassin, dumbass` → **blocked**. The regression test for the whole phase.
- `hello` → zero `Verdict`.
- Empty whitelist → `assassin` is blocked.
- An entry equal to the whole text is rescued by an identical whitelist entry.
- Shuffling the input slices does not change which entry a verdict names.

## Risk

The containment rule is easy to implement subtly wrong in a way that passes the obvious two
test cases. The mitigation is that the three-clause `assassin`/`dumbass` case is written
first and named for the behaviour it protects.

## Rollback

The package has no importers until Phase 4 wires it into `cmd/server`. Delete the directory
and revert the `go.mod` line.
