# Test / harness / wiring review — sticker module

Reviewer lens: test quality, harness correctness, integration & wiring.
Method: read all sources, then **mutation-tested 22 production mutations** against the
suite. Security exploitation and deep state-machine correctness owned by other reviewers.

Verdict: test quality is **high** — 19 of 22 mutations killed, and the ownership gates,
callback binding, self-heal, panic barrier and count bookkeeping are all genuinely pinned.
Two rounds previously claimed "all new tests non-vacuous"; that is **refuted in three
places**: the `created`/reservation-release machinery, two `/addsticker` emoji tests, and
the module-registration wiring. None is a bug in shipped behaviour today; all are real
holes that would let a future regression land green.

---

## 1. Mutation-test results

Every mutation applied to production source only, run, then reverted. Restoration verified
by md5 + `diff -r` (see §7).

| # | Mutation applied | File | Guarding test(s) | Result |
|---|---|---|---|---|
| M1 | Delete the `ownsSet` gate from `resolveOwned` | resolve.go:116 | `TestResolveOwned_RefusalsAreIdentical` | **KILLED** |
| M2 | `!found \|\| pack.Pending` → `!found` | resolve.go:113 | `TestResolveOwned_PendingRefusesIdentically` | **KILLED** |
| M3 | Disable foreign-holder refusal in `reserveSlug` | pack_handlers.go:159 | `TestNewPack_CannotSeizeAnotherUsersPack`, `..._ForeignReservationRefusedBeforeAnyAPICall` | **KILLED** |
| M4 | Disable reservation-owner proof in `resolveStaleIntent` | pack_handlers.go:263 | `TestNewPack_StaleIntentCannotAdoptForeignName` | **KILLED** |
| **M5** | **`if created { releaseSlug }` → never release** | **pack_handlers.go:111** | *(none)* | **SURVIVED** |
| **M6** | **Delete the pre-reservation quota check entirely** | **pack_handlers.go:92-99** | *(none)* | **SURVIVED** |
| **M7** | **`reserveSlug` resumed path returns `created=true`** | **pack_handlers.go:165** | *(none)* | **SURVIVED** |
| M8 | `dropPackRecordIfSet` → `dropPackRecord` (blind by-owner delete) | delpack_callback.go:178 | `TestDelPackCallback_StalePressLeavesTheCurrentPackAlone` | **KILLED** |
| M9 | Disable chat/message binding check | delpack_callback.go:137 | `..._RejectsWrongBinding` (2 subtests), `..._BystanderCannotTouchAnotherUsersPrompt` | **KILLED** |
| M10 | Remove consume-before-destructive-call | delpack_callback.go:159 | `TestDelPackCallback_SecondPressIsInert` | **KILLED** |
| M11 | Disable expiry check | delpack_callback.go:149 | `TestDelPackCallback_RejectsExpired` | **KILLED** |
| M12b | `dropPackRecord` never releases the slug | pack_handlers.go:478 | `TestSelfHeal_ReleasesTheName`, `TestDelPackCallback_ReleasesTheName` | **KILLED** |
| M13 | `isStickerSetMissing` → `err != nil` (transient read as "gone") | errors.go:163 | 4 tests incl. both `_TransientErrorKeepsRecord` | **KILLED** |
| M14 | Remove the negative-count floor | pack_handlers.go:407 | `TestDelSticker_CountFlooredAtZero` | **KILLED** |
| M15 | Invert emoji precedence (replied beats explicit) | sticker_handlers.go:283 | `TestAddSticker_HappyPath`, `..._EmojiPrecedence/explicit_wins` | **KILLED** |
| **M16** | **Early-return before `AddStickerToSet` (no API call at all)** | **sticker_handlers.go:292** | *(none — see F2)* | **SURVIVED** |
| M17 | Drop the inherit-from-replied-sticker fallback | sticker_handlers.go:283 | `..._EmojiPrecedence/inherits_from_replied_sticker` | **KILLED** |
| M18 | Remove the command panic barrier | dispatcher.go:75 | `TestInstall_CommandPanicIsContained` (binary would die) | **KILLED** |
| M19 | Remove the command-hook panic barrier | dispatcher.go:83 | `TestInstall_CommandHookPanicIsContained` | **KILLED** |
| M20 | Callback barrier `onPanic` → nil (stop answering the query) | dispatcher.go:104 | `TestInstall_CallbackPanicIsContainedAndAnswered` | **KILLED** |
| M21 | `claimSlug` `PutVersioned(…,0,…)` → `Put` | pack_handlers.go:216 | `TestNewPack_DifferentSlugAdoptsExistingSet` | **KILLED** |
| M22 | `reserveSlug` `PutVersioned(…,0,…)` → `Put` | pack_handlers.go:139 | `TestNewPack_CannotSeize…`, `..._ForeignReservation…` | **KILLED** |
| **W1** | **Delete `"mypack": ""` from `expectedParameters`** | **cmd/server/command_menu_test.go:76** | *(none — see F4)* | **SURVIVED** |
| **W2** | **Unregister `sticker` from `factories()` (import removed too)** | **cmd/server/main.go:95** | *(none — see F5)* | **SURVIVED** |
| W3 | Corrupt a non-empty expectation (`"newpack": "WRONG"`) — control | command_menu_test.go:75 | `TestCommandDiscovery_AllPublicCommandsHaveSafeMetadata` | **KILLED** |

Survivors M5/M6/M7 and W2 were each re-run against the **full** sticker suite / **full repo
suite** (`./...`), not just a `-run` subset. All still survived.

---

## 2. Findings

### F1 — HIGH — the `created` reservation-release machinery has zero test coverage

`internal/modules/sticker/pack_handlers.go:106-115` and `:138-165`.

Both directions of the `created` flag survive mutation:

- **M5** (never release): survived the full suite.
- **M7** (always release, even a resumed reservation): survived the full suite.

Both branches are reachable. I proved it with two temporary probe tests (since removed):

- *Release-on-bail is real*: `/newpack newslug` while a pending record for `oldslug`
  exists and the old set resolves → `reserveSlug` writes `newslug` (`created=true`),
  `claimSlug`→`resolveStaleIntent` adopts `oldslug` and returns `done=true`, so
  `releaseSlug(newslug)` must run. Under M5 the probe failed with
  `newslug still reserved with no pack behind it - name burned`. Reservations are global
  and permanent, so this is a per-invocation namespace leak.
  **`TestNewPack_DifferentSlugAdoptsExistingSet` (pack_handlers_test.go:111) walks exactly
  this path and asserts nothing about `newslug`.** One added line closes it.
- *Not-releasing-a-resumed-reservation is real*: reserve `oldslug` for the caller, pending
  record under a different slug, `getStickerSet` fails unclassifiably → `resolveStaleIntent`
  bails `done=true`. Under M7 that probe failed: the caller's pre-existing reservation was
  destroyed, handing the name to the next asker while the set may still exist.

**`TestNewPack_ResumedReservationSurvivesABail` (pack_handlers_test.go:573) does not test
what its name says.** In that test `claimSlug` returns `done=false` (same-slug resume), so
the `if created` branch is never reached; the bail happens later in `createOrAdopt`, which
never consults `created`. M7 survives it. It is a duplicate of
`TestNewPack_UnknownLookupErrorAborts` wearing a different name.

Fix: assert `newslug` is unreserved in `TestNewPack_DifferentSlugAdoptsExistingSet`, and
re-point `TestNewPack_ResumedReservationSurvivesABail` at a bail inside
`claimSlug`/`resolveStaleIntent` (a pending record under a *different* slug + a 500 from
`getStickerSet` reaches it).

### F2 — MEDIUM — two `/addsticker` emoji tests pass when no sticker is added at all

`handlers_test.go:134-159` (`TestAddSticker_EmojiPrecedence`) and `:163-178`
(`TestAddSticker_FallsBackToDefaultEmoji`).

Both use the pattern:

```go
for _, call := range rb.Sent() {
    if call.Method == "addStickerToSet" && !strings.Contains(call.Form["sticker"], tc.want) {
        t.Errorf(...)
    }
}
```

Zero matching calls ⇒ zero assertions ⇒ pass. **M16** confirms it: an early `return` placed
before `b.AddStickerToSet` leaves both tests green. They are half-live (M15/M17 kill
individual subtests via the emoji value) but they do not guard "a sticker was added".

`TestAddSticker_HappyPath:108` has the right guard (`countMethod(...) != 1` + `Fatalf`).
Add the same two lines to both tests. Same latent shape at `resolve_test.go` — no, those
use explicit `countMethod` comparisons and are fine.

### F3 — MEDIUM — `docs/sticker-packs.md` contradicts the reservation lifecycle it describes

`docs/sticker-packs.md:99-101`:

> "A name is claimed only when a pack is actually created, and it is released when that
> pack is deleted … A `/newpack` that is refused claims nothing."

The first clause is false and inverts the module's central safety property. `reserveSlug`
writes the reservation **before Telegram is touched** — that write-ahead claim is precisely
what makes adoption safe, and `pack.go:236-239` plus `pack_handlers.go:120-137` say so at
length. A name is therefore held while a `/newpack` is merely *pending*, and
`TestNewPack_UnknownLookupErrorAborts:181` asserts the reservation **must survive** when no
pack was created. An interrupted `/newpack` holds its name indefinitely with no pack behind
it — the doc tells a reader the opposite.

The second clause is also too strong: a *classified* refusal releases, but an unknown
`getStickerSet`/`createNewStickerSet` failure deliberately keeps both intent and
reservation (`pack_handlers.go:325-331, 347-356`).

Everything else in the doc checks out against source: 512px long edge / 100×100 thumbnail
(`image.go:23,25`), 4096px cap (`maxDecodeDimension = 4096`), 2 MB (`maxSourceBytes`),
120 stickers, 1–20 emoji, 10-minute confirm TTL, 10-second handler deadline, slug rules,
`MODULES` semantics, uniform ownership refusals.

### F4 — LOW — `command_menu_test` does not verify that a command has an expectation

`cmd/server/command_menu_test.go:104`: `got != expectedParameters[command.Name]`. A missing
map key yields `""`, so any public command with empty `Parameters` that nobody added to the
map passes silently. **W1** confirms: deleting `"mypack": ""` changes nothing. Four of the
nine new entries (`mypack`, `delsticker`, `setpackicon`, `delpack`) are therefore
decorative. The test does not verify what its name ("AllPublicCommandsHaveSafeMetadata")
promises for parameterless commands.

Fix: `want, ok := expectedParameters[command.Name]; if !ok { t.Errorf("no expectation for /%s") }`.
That also turns the test into the missing registration guard for F5.

### F5 — MEDIUM — nothing pins that the sticker module is registered at all

**W2**: with both the import and `"sticker": sticker.New` removed from `cmd/server/main.go`,
`go test ./...` is **fully green** — all 25 packages pass, including
`internal/modules/sticker` (its tests construct `state` directly and never go through
`factories()`). A bad merge or rebase that drops the factory line ships a bot with none of
the nine commands and a green CI.

`command_menu_test.go` only iterates whatever `reg.PublicCommands()` returns, so an absent
module is invisible to it. Cheapest fix is F4's `ok` check, which makes the map an
inventory rather than a lookup.

### F6 — LOW — `/newpack` documented but unregistered `MODULES` default changed silently

`.env.example` flips `MODULES=` (empty ⇒ load everything) to an explicit 12-module list.
The list matches `factories()` exactly (verified key by key), so no module is dropped today.
But it is now a hand-maintained duplicate of `factories()` with no test tying the two
together — the next module added will be silently excluded for anyone starting from the
template. Worth a comment pointing at `factories()`, or a test.

---

## 3. Harness review — `internal/testutil/recording_bot.go`

All four questions checked empirically. **The harness changes are correct and
backwards-compatible.**

- **`FailMethodCode` produces the real sentinels — confirmed.** The library switches on
  `r.ErrorCode` decoded from the *body* and ignores the HTTP status
  (`go-telegram/bot@v1.20.0/raw_request.go:102-131`). `FailMethodCode` marshals
  `{"ok":false,"error_code":…,"description":…}`, so `errors.Is(err, bot.ErrorBadRequest)`
  holds. `recording_bot_test.go:113` pins this, and `:130` pins the negative contrast for
  bare `FailMethod`. The doc comment's `raw_request.go:103-125` citation is accurate.
- **`StubMethod` / `FailMethod` precedence is correct and tested.** `handle()` checks
  `shouldFail` before `hasStub` (`:200-209`); `TestRecordingBot_FailureWinsOverStub` covers it.
- **`Reset()` is coherent and unchanged for existing callers.** It clears `calls` only —
  which is exactly what it did before this changeset; the diff only adds a doc comment
  explaining it. `nextMessageID` is deliberately not reset (IDs stay unique across a
  Reset), which no caller depends on. All 13 other packages that use the harness call only
  `Reset()`; **no module outside `sticker` uses `FailMethod`, `FailMethodCode` or
  `StubMethod`**, so the new precedence rule cannot affect them.
- **Message IDs start at 1, not 0.** `handle()` increments *before* assigning
  (`:192-195`), so the first `sendMessage` returns `message_id: 1`. This matters because
  production rejects a binding with `action.MessageID == 0` (`delpack_callback.go:137`) —
  I verified the full round trip (`/delpack` → press the button the handler itself
  produced → `deleteStickerSet` fires exactly once). No collision. *(Observation only:
  no test in the suite actually performs that round trip; the pieces are covered
  separately.)*

**One real harness concern (MEDIUM):**

The multipart-parse tolerance (`:180`) is justified — I confirmed `getMe` genuinely fails
with `multipart: NextPart: EOF` and `ContentLength=-1`, so the old code made every
parameterless method untestable. But the tolerance is **wider than the justification**: I
posted a deliberately malformed multipart body (`Content-Type: multipart/form-data;
boundary=zzz` with non-multipart content) to `/sendMessage` and the harness answered
**HTTP 200** and recorded `{Method:sendMessage Form:map[]}`. A real Telegram would 400 it.

Impact is bounded — the bot library always builds well-formed multipart, so production
cannot realistically emit garbage. The live risk is **masking**: any test asserting a form
field is *absent* would falsely pass if the whole form silently failed to parse. Such
assertions already exist outside this module, e.g.
`internal/modules/stock/dividend_flow_test.go:115,139` (`calls[1].Form["reply_markup"] != ""`).

Suggested narrowing: tolerate only the empty-body case (`r.ContentLength <= 0`, or the
`NextPart: EOF` shape) and keep the 400 for genuinely malformed bodies; or record a
`ParseFailed bool` on `SentCall` so a masked parse is visible in `dumpCalls()`.

---

## 4. Untested error branches

Prioritised by blast radius if a bug landed there. `✗` = no test reaches the branch.

**`pack_handlers.go` — state-corrupting or namespace-leaking:**

- `:111` `if created { releaseSlug }` — ✗ **both directions** (F1). Name-burn / name-theft.
- `:178-196` `releaseSlug`'s own cross-user ownership guard (`held.OwnerID != ownerID`) — ✗.
  Defence-in-depth against a bad call site, with zero coverage; a caller passing the wrong
  owner would be caught only here.
- `:325-331` `createOrAdopt` default branch is covered, but **`:347-356` create failing with
  an *unclassifiable* error** (intent + reservation must both survive) — ✗. This is the exact
  mirror of `TestNewPack_UnknownLookupErrorAborts` and is the higher-risk half, since a
  wrong answer here strands a slug whose set may exist.
- `:396-413` `adjustCount`'s `!found` → `storage.ErrNotFound` path, and both handler
  fallbacks that consume it (`sticker_handlers.go:308-314`, `:352-359`) — ✗. These
  synthesise a count for the reply; a bug shows the user a wrong number.
- `:147-158` `reserveSlug` conflict-but-unreadable (`getErr != nil || !found` → treat as
  taken) — ✗. Comment calls out that guessing the other way *is* the takeover.
- `:220-229` `claimSlug` non-conflict store error / re-read failure — ✗.
- `:258-272` `resolveStaleIntent` reservation-read error, and both `Put` failures
  (`:267`, `:296`) — ✗.
- `:283-286` adopt-commit failure — ✗.
- `:73-77` `resolver.resolve` (GetMe) failure, `:78-81` `makeSetName` failure through the
  handler — ✗ (`makeSetName` is unit-tested, the handler branch is not).
- `:92-94` pre-check store error, `:495-499` `/mypack` store error, `:531-535` /`renamepack`
  store error, `:542-547` `/renamepack`'s `isStickerSetMissing` self-heal — ✗.
- `:366-369`, `:550-554` commit failures — ✗ (both are best-effort by design).

**`delpack_callback.go`:**

- `:144-147` `action.ID != id` → clear button + "replaced by a newer /delpack" — ✗.
  `TestDelPack_SecondPromptSupersedesTheFirst` is rejected earlier, at the *binding* check
  (`:137`), so this branch and its `clearButton` side effect never execute in tests.
- `:97-100` malformed callback data through the handler — ✗ (`parseDeleteCallback` is
  unit-tested at `delpack_callback_test.go:84` for the happy case only; no test feeds
  over-length, non-hex or wrong-prefix data to `handleDelPackCallback`).
- `:105-107` `query.From.ID == 0`, `:118-120` `From.ID != action.OwnerID` — ✗ (both
  unreachable given the owner-keyed lookup; defence in depth).
- `:113-116` pending-store read error, `:159-165` both consume-delete failure branches — ✗.
- `:32-36` `/delpack` store error and `:37-39` `!found` ("you don't have a pack") — ✗.
- `:70-73` `SendMessage` failure — ✗. Note this is the one path in the module that returns
  a **raw** API error to the dispatcher rather than a `userError`/generic reply.
- `:80-83` pending `Put` failure — ✗. Leaves a live button with no server-side action.

**`resolve.go`:**

Well covered. Only `:108-111` `getPack` store error is ✗. `resolveSource`'s `replied == nil`
refusal (`:51-53`) is ✗ directly, though `resolveOwned`'s equivalent is tested.

**Whole-feature gap — Phase 5 photo pipeline has no integrated coverage.** Every test
message is built by `stickerReply()`, which always sets `Sticker`. Grep confirms no test
constructs a `Photo:` / `Document:` reply and feeds it to a handler. Consequently
`resolvePhotoSource` (`photo.go:342`) is never executed, and **`handleSetPackIcon`
(`setpackicon.go:174`) is reached only by `TestHandlers_RefuseAnonymousSenders`, which
returns at the sender check before doing anything** — its download → resize →
`SetStickerSetThumbnail` → self-heal body is entirely untested. The pieces (`photoFileID`,
`downloadFile`, `toStickerPNG`, `toThumbnailPNG`) are individually well tested; the wiring
between them is not. `StubMethod("uploadStickerFile", …)` now makes this testable — that is
what the harness change was for.

---

## 5. Test isolation, races, lint

- `go test -race ./internal/modules/... ./internal/testutil/... ./cmd/server/...` — **clean,
  exit 0, 0 `DATA RACE`**, all 17 packages ok.
- `golangci-lint run` on the four changed packages — **0 issues**.
- No shared mutable fixtures: every test builds its own `newTestState()` over a fresh
  `storage.NewMemoryProvider()` and its own `RecordingBot`. No ordering dependence found.
- `syncBuffer` (dispatcher_panic_test.go:33) correctly mutex-guards the log sink for the
  detached-goroutine test, and `waitForLog` polls rather than sleeping. Good.
- Two globals are mutated without isolation in `dispatcher_panic_test.go`: `log.SetDefault`
  (restored via defer) and `metrics` counters (`metrics.Flush()` at :116, never reset).
  Harmless today — nothing runs in parallel and no other test in `modules_test` asserts on
  error counters — but the metrics assertion at :130 would become order-dependent if one
  ever did. Worth a note, not a change.
- `seedPack` correctly seeds the slug reservation alongside the pack record
  (handlers_test.go:45-50), and `seedInterrupted` does the same
  (pack_handlers_test.go:387-398). Both carry a comment explaining that seeding the record
  alone builds a state production cannot reach. This is the right instinct and it is why
  M12b/M22 kill cleanly.

## 6. Wiring & plan accuracy

**Registered correctly:** all 9 commands appear in `sticker.go:22-82` with
`VisibilityPublic`, descriptions, and `Parameters` matching
`docs/command-parameter-conventions.md` (the new `<name...>` "required remaining text" row
is a genuine addition, used by `<title...>`). All 9 are menu-described and
parameter-documented. README table and `docs/sticker-packs.md` list the same 9.

**Callback prefix is unique.** `callbackPrefix = "sticker_pack:"` vs the only other
callback in the repo, `stock`'s `"stock_div:"`. `registry.go:218-220` enforces this
**bidirectionally** (`HasPrefix` both ways), so the check is real, not nominal.

**Plan accuracy** — `plan.md` status is honestly `partial`, and all 16 unchecked boxes are
live-Telegram smoke tests plus the unresolved R11 (does Telegram reserve deleted short
names). No inflated completion. Two checked boxes are contradicted by shipped code:

- `phase-03:245` — "`/newpack` where `GetStickerSet` fails with a non-missing error aborts,
  **deletes the pending record**, and never calls `CreateNewStickerSet`" is `[x]`, but the
  shipped code deliberately **keeps** both intent and reservation
  (`pack_handlers.go:325-331`), and `TestNewPack_UnknownLookupErrorAborts:174-183` asserts
  that. The file's own superseding note at `phase-03:52-55` says this step is wrong — the
  checkbox was ticked against the superseded text.
- `phase-03:225` — "`sticker.go` factory registering **4 commands** + callback prefix" is
  `[x]`; the shipped factory registers 9. Stale phase-scoped wording, harmless.

**`internal/modules/wordle/lookup_test.go`** — confirmed a pure `gofmt` alignment change.
The 8 map keys and all 8 values are byte-identical; only leading whitespace inside the
composite literal moved (the longest key `"  crane  "` no longer forces extra padding).
No behaviour change.

## 7. Final state

Every mutated file restored and verified byte-identical against a pre-review backup:

```
md5sum -c backup/md5.txt          → all 31 files OK (no mismatches)
diff -r backup/cmdserver cmd/server        → cmd/server IDENTICAL
diff -r backup/testutil internal/testutil  → testutil IDENTICAL
diff -r backup/sticker  internal/modules/sticker → sticker IDENTICAL
diff    backup/dispatcher.go internal/modules/dispatcher.go → dispatcher IDENTICAL
```

Three temporary probe test files were created and removed (`zz_probe_test.go`,
`zz_probe2_test.go` in `sticker`; `zz_probe_test.go` in `testutil`); none remain.

`git status --short`:

```
 M .env.example
 M README.md
 M cmd/server/command_menu_test.go
 M cmd/server/main.go
 M docs/command-parameter-conventions.md
 M go.mod
 M go.sum
 M internal/modules/dispatcher.go
 M internal/modules/wordle/lookup_test.go
 M internal/testutil/recording_bot.go
 M internal/testutil/recording_bot_test.go
 M plans/260824-1051-sticker-pack-module/phase-01-shared-prerequisites.md
 M plans/260824-1051-sticker-pack-module/phase-02-store-setname-emoji.md
 M plans/260824-1051-sticker-pack-module/phase-03-pack-lifecycle.md
 M plans/260824-1051-sticker-pack-module/phase-04-sticker-commands.md
 M plans/260824-1051-sticker-pack-module/phase-05-photo-pipeline.md
 M plans/260824-1051-sticker-pack-module/phase-06-wiring-docs.md
 M plans/260824-1051-sticker-pack-module/plan.md
?? docs/sticker-packs.md
?? internal/modules/dispatcher_panic_test.go
?? internal/modules/sticker/
?? plans/reports/correctness-review-260825-1515-sticker-module.md
?? plans/reports/security-review-260825-1515-sticker-module.md
```

Identical to the state at review start, plus the two peer reviewers' reports and this file.

`go test ./...` — **all 25 packages ok**, zero failures.
`go test -race` on all module + testutil + cmd/server packages — **ok, 0 data races**.
`golangci-lint run` on changed packages — **0 issues**.

## 8. Recommended actions

1. **(F1, high)** Assert `newslug` is released in `TestNewPack_DifferentSlugAdoptsExistingSet`;
   re-point `TestNewPack_ResumedReservationSurvivesABail` at a bail inside
   `claimSlug`/`resolveStaleIntent` so it kills M7. Two tests, ~6 lines.
2. **(F5 + F4, medium)** Add the `want, ok := expectedParameters[name]` presence check in
   `command_menu_test.go`. Closes both the decorative-entry hole and the missing
   registration guard in one edit.
3. **(F2, medium)** Add the `countMethod(rb, "addStickerToSet") != 1` + `Fatalf` guard to
   `TestAddSticker_EmojiPrecedence` and `TestAddSticker_FallsBackToDefaultEmoji`.
4. **(F3, medium)** Correct `docs/sticker-packs.md:99-101` to describe the write-ahead
   reservation: the name is claimed *before* Telegram is called and is held while an
   attempt is pending; only a positively-classified refusal or a confirmed delete releases it.
5. **(§3, medium)** Narrow the multipart tolerance to the empty-body case, or surface a
   `ParseFailed` flag on `SentCall`.
6. **(§4, medium)** Add one integrated photo test (`StubMethod("uploadStickerFile", …)`)
   and one `handleSetPackIcon` happy path — the largest untested surface in the module.
7. **(§4, low)** Cover `createOrAdopt`'s unclassifiable-create-error branch and
   `delpack_callback.go:144` (`action.ID != id`).
8. **(§6, low)** Untick or correct `phase-03:245`; fix the "4 commands" wording at `:225`.

## 9. Unresolved questions

- `phase-06` leaves R11 (does Telegram permanently reserve a deleted short name?) open, and
  `dropPackRecord`'s comment reasons about it both ways. Nothing here can settle it without
  a live token; the code's behaviour is safe under either answer, so it is correctly
  deferred to the manual smoke list.
- Is the `.env.example` switch from empty-`MODULES` to an explicit list intended to change
  deployed behaviour, or only to document intent? If deployments copy the template, adding
  a future module will require an `.env` edit that nothing warns about.
