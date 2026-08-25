# Sticker module — round 7 scoped verification

- Branch `feature/sticker-pack-module`, HEAD `5e4fb0f`, Go 1.27, golangci-lint v2.13.1.
- Scope: only H1, H2/H3, M1 from `verify-260825-1700-sticker-round6.md`, plus a scan of `5e4fb0f` for anything new. The 16-call Telegram enumeration was NOT redone.
- All mutations were applied to working-tree copies, then restored from backup. Final state: `git status --short` empty, 27/27 md5sums match, `git diff HEAD --stat` empty.

## Verdict

**SAFE_TO_MERGE.** H1, H2/H3 and M1 are closed. The `!found` equivalent-mutant claim is correct and is itself test-pinned. Nothing unintended was found in the commit.

## Mutation results

Guard under test, `internal/modules/sticker/delpack_callback.go:197`:

```go
if !found || current.Pending || !ownsSet(current, action.SetName) {
```

| # | Mutation | Result | Killing test |
|---|---|---|---|
| M1 | drop `!found` (`_ = found` added to compile) | **SURVIVED** | none — equivalent mutant, see below |
| M2 | drop `current.Pending` | **KILLED** | `TestDelPackCallback_StaleAuthorityNeverReachesTelegram/record_is_unconfirmed` |
| M3 | drop `!ownsSet(...)` | **KILLED** | `.../record_moved_on` **and** `TestDelPackCallback_StalePressLeavesTheCurrentPackAlone` |
| M4 | full revert to the round-5 blocklist (`if found && current.Pending && ownsSet(...)`) | **KILLED** | `.../record_gone`, `.../record_moved_on`, `TestDelPackCallback_StalePressLeavesTheCurrentPackAlone` |
| M5 | re-add `resumed.Name = intent.Name` (`pack_handlers.go`) | **KILLED** | `TestNewPack_ResumeKeepsTheStoredSetName` (`set name = "mypack_by_testbot", want the stored "mypack_by_oldbot"`) |
| M6 | drop `resumed.Title = intent.Title` (control, prior round's fix) | **KILLED** | `TestNewPack_ResumeUsesTheTitleJustTyped` (3 assertions) |
| M7 | drop `s.dropPendingDelete(ctx, key)` from the guard body | **SURVIVED** | none — cleanup, not authority; see Informational |

### H1 — closed

The replacement test is not vacuous. M4 (full guard revert) fails two of the three table
cases; every case therefore executes past `pending.Get` and reaches the under-lock
allowlist, which is exactly what the round-6 test did not do. Non-vacuity is further
proven by the *specificity* of M2 and M3: `record_is_unconfirmed` fails only when
`current.Pending` is removed, which means `ownsSet` returned **true** there — so the
record was really loaded and really matched, and the case is testing the Pending disjunct
and nothing else. Same argument for `record_moved_on` and `!ownsSet`.

### H2/H3 — closed to the extent it can be

`current.Pending` is now individually killed (M2), and `!ownsSet` by two tests (M3).
`!found` survives (M1) — correctly, as an equivalent mutant.

### M1 (`resumed.Name`) — closed

M5 is killed with a specific message. The control M6 confirms the sibling title
assertion was not weakened while the file was edited.

## The `!found` equivalent-mutant claim — CONFIRMED

Independently verified three ways, not just by the surviving mutation:

1. **Source.** `getPack` (`internal/modules/sticker/pack.go:89-98`) returns a literal
   `Pack{}` on *both* non-found paths (`ErrNotFound` and error), and the error path
   returns early at the call site. So at the guard, `found == false` implies
   `current == Pack{}` implies `current.Name == ""`.
2. **`ownsSet`.** `internal/modules/sticker/setname.go:80-82` returns false when
   `pack.Name == ""`. Hence `!ownsSet(current, _)` is already true whenever `!found`,
   and `current.Pending` is false, so the mutated expression is bit-for-bit identical
   on every reachable input.
3. **The equivalence is itself pinned.** `internal/modules/sticker/setname_test.go:79`
   asserts `ownsSet(Pack{}, "anything") == false`. This is the property the redundancy
   depends on, so a future edit to `ownsSet` that breaks the equivalence — making
   `!found` load-bearing and silently untested — fails a test rather than passing
   quietly. This is the one thing that made the claim safe to accept rather than merely
   plausible.

Searched for a counterexample and found none:

- **Can a missing record yield a non-empty `current.Name`?** No. Both miss paths in
  `getPack` discard the decoded value and return the zero `Pack`.
- **Can `action.SetName` be empty?** It is written once, at
  `delpack_callback.go:65` (`SetName: pack.Name`), from a record that is already proven
  `found && !pack.Pending`. Even if a corrupt store record carried `Name == ""`, `ownsSet`
  returns false for an empty `setName` too, so the guard refuses. Fail-closed either way,
  and `DeleteStickerSet` is never reached with an empty name.

Keeping `!found` with the comment is the right call: it costs nothing and the alternative
is a guard whose correctness silently depends on a helper's empty-string branch.

## Correctness of the reverted `resumed.Name` (not just coverage)

The divergence only exists after a BotFather rename: `makeSetName` is deterministic in
`(slug, username)` and the branch requires `existing.Slug == slug`, so `intent.Name !=
existing.Name` is *only* possible when the bot's username changed between the interrupted
attempt and the retry. Both sub-cases were driven with a probe test:

**(a) The interrupted attempt did create the set.** `createPack` probes
`GetStickerSet(existing.Name)`, finds it, drops the intent, releases the slug and answers
"name taken". The old set is stranded but the user is told. Refreshing the name instead
would have probed the *new* name, found it free, and created a second set — orphaning the
first one silently. The revert is the better behaviour here.

**(b) The interrupted attempt never created the set.** Probe output — the resume path
really does send the stale name to Telegram:

```
PROBE getStickerSet name="mypack_by_oldbot"
PROBE createNewStickerSet name="mypack_by_oldbot"
```

Real Telegram refuses a create whose short name does not end in `_by_<current username>`.
This is the one place the revert costs something, so I drove it rather than assuming.
Injecting Telegram's actual refusal:

```
PROBE after refusal: found=false pack={}          # intent dropped
PROBE reservation still held: false               # slug released
PROBE reply="Telegram rejected that pack name. Use lowercase letters, digits and single underscores."
PROBE attempt2 createNewStickerSet name="mypack_by_testbot"
PROBE attempt2 stored = {Slug:mypack Name:mypack_by_testbot ... Pending:false}
```

`createRefused` (`errors.go:112-123`) already matches `PACK_SHORT_NAME_INVALID` /
`"invalid sticker set name"`, so the refusal is classified as proof-nothing-was-created,
the stale intent and reservation are torn down, and the very next `/newpack` succeeds under
the current username. **There is no permanent wedge** — the cost is one misleading error
message in a rename-plus-interrupted-attempt window. That is strictly cheaper than the
silent orphan in (a), so the revert is correct, not merely test-pinned.

Answering the two specific questions asked:

- *Stored `Name` never validated?* Every write of `Pack.Name` in production goes through
  `makeSetName`, which errors on an empty username and enforces `maxSetNameLen`. A record
  with an unvalidated or empty `Name` cannot be produced by this module, and if one
  existed, both `ownsSet` and the create probe fail closed.
- *Stored `Name` belonging to a different slug?* Unreachable in this branch, which is
  gated on `existing.Slug == slug`; a differing slug routes to `resolveStaleIntent`, which
  re-reads the reservation before touching anything under the old name.

## Scan of `5e4fb0f` for anything new or unintended

Five files: two production (comment-only + one deleted line), two test, one report.

- `delpack_callback.go`: **comment only**. No behaviour change.
- `pack_handlers.go`: one line deleted, replaced by a comment. Verified by M5/M6 that the
  surviving `resumed.Title` assignment is unchanged and still pinned.
- No new production code, no new helper, no new abstraction, no `any` widening, no lint
  suppression, no error swallowed.
- The replaced test dropped its seeding of the victim's *slug reservation*. That seeding
  was never asserted on in the old test either, so no assertion was weakened — the victim's
  pack record check is retained in every table case.
- No phantom tests: every new test is mutation-killed (M4, M5) except by design.
- No scope drift; nothing outside `internal/modules/sticker/` and `plans/reports/`.

## Gates

| Check | Result |
|---|---|
| `go vet ./...` | clean |
| `go test ./...` | all pass |
| `go test -race ./...` | all pass |
| `go test -race -count=20 ./internal/modules/sticker/` | ok, 87.3s, no races, no flakes |
| `golangci-lint run ./...` | `0 issues.` |
| `gofmt -l .` | empty |

## Informational (non-blocking)

1. **M7 survivor.** Nothing pins that the guard clears the stale `PendingDelete` before
   refusing. Removing `s.dropPendingDelete(ctx, key)` from the guard body leaves the whole
   suite green. This is *not* an authority hole — the guard still refuses on every
   subsequent press, and the confirmation expires on its own — so it is cleanup hygiene, not
   safety. Worth one assertion in the table (`pending.Get` returns `ErrNotFound` after the
   refusal) if a future round touches this file; not worth blocking on.
2. **Misleading refusal text after a bot rename.** In case (b) above the user is told their
   *pack name* is invalid when the real cause is that the bot was renamed. Cosmetic, rare,
   self-healing on retry.
3. The `break_` field name in the table trips no linter under the project's config
   (`0 issues.`), so it is left alone.

## Unresolved questions

None.
