# Sticker module — round 5 adversarial verification

Commit `e81b1b7` ("fix(sticker): never adopt an existing pack"), branch
`feature/sticker-pack-module`, Go 1.27, golangci-lint v2.13.1.

**Verdict: DO_NOT_MERGE.** The adoption class is genuinely closed — every route
to a committed `Pack` record naming a foreign set was enumerated and executed,
and all of them refuse. But the round-5 pattern repeated in the *other*
direction: the commit correctly identified that `DeleteStickerSet` is a
cross-user primitive keyed by set name, added a guard for one way of reaching
it, and left a second way open. A stale `/delpack` confirmation destroys
whichever user holds that name at press time.

---

## C1 (Critical) — a stale `/delpack` confirmation deletes a re-issued name's pack

`internal/modules/sticker/delpack_callback.go:177-185`

```go
if current, found, err := getPack(ctx, s.store, action.OwnerID); err != nil {
    ...
} else if found && current.Pending && ownsSet(current, action.SetName) {
    // refuse
}
// falls through to DeleteStickerSet(action.SetName)
```

The re-check is **negative** — it blocks exactly one bad state (`Pending`) — where
it needed to be **positive**: only proceed when the record still authorises this
delete. `!found` and "record now names a different set" both fall through to the
destructive call. `dropPackRecordIfSet` performs precisely the right check
(`found && ownsSet`), but it runs *after* `DeleteStickerSet`, so it protects the
local record and not the set.

### Reproduction (executed; all steps are ordinary public commands)

Test `TestProbe_StaleConfirmationDeletesReissuedName`, run against
`internal/modules/sticker`:

| # | Actor | Command | Effect |
|---|-------|---------|--------|
| 1 | U | `/newpack mypack Mine` | committed record + reservation `mypack` |
| 2 | U | `/delpack`, **do not press** | `PendingDelete{SetName: mypack_by_testbot}` stored, TTL 10 min |
| 3 | U | `/delsticker` down to zero, then any command | set gone at Telegram → `STICKERSET_INVALID` → `dropPackRecord` drops the record **and releases the reservation**. The unpressed confirmation is untouched — no `dropPackRecord` path clears `s.pending`. |
| 4 | V | `/newpack mypack Victim Pack` | reservation free, `GetStickerSet` missing → V legitimately creates and owns `mypack_by_testbot` |
| 5 | U | presses the button from step 2 | binding OK, not expired, re-check sees `found=false` → **`deleteStickerSet name=mypack_by_testbot`** |

Observed output:

```
step3 self-heal: U record found=false, reservation held=false
step3 U's unpressed confirmation SURVIVED the self-heal
step5 V pack found=true {Slug:mypack Name:mypack_by_testbot ... OwnerID:4242 Pending:false}
step6 methods = [deleteStickerSet editMessageReplyMarkup sendMessage answerCallbackQuery]
HOLE CONFIRMED: U deleted "mypack_by_testbot" — V's pack, created after U's record was gone
step6 V record still found=true (local record survives, set does not)
step6 answer = "Pack deleted."
```

V is left with a committed record pointing at a destroyed set, and U is told the
delete succeeded.

**Reachability: production, not hypothetical.** Dispatch is serial
(`internal/telegram/client.go:28` `WithNotAsyncHandlers`, single worker) — this is
a plain sequential command sequence, no race. Steps 1-3 are fully under the
attacker's control and take seconds; the only constraint is that step 4 lands
inside the 10-minute `pendingDeleteTTL`. The same sequence also occurs with no
attacker at all: run `/delpack`, get distracted, empty the pack, run one more
command, and whoever takes the freed name loses it when you finally press.

Second, milder variant, also executed
(`TestProbe_StaleConfirmationAfterRecordMovedOn`): with the record moved to a
different pack, the press still issues
`deleteStickerSet name="mypack_by_testbot"` while the record names
`other_by_testbot`. `dropPackRecordIfSet` correctly leaves the record alone —
after the set is already gone.

### Fix shape

Invert the guard to a positive authorisation, matching `dropPackRecordIfSet`:

```go
current, found, err := getPack(ctx, s.store, action.OwnerID)
if err != nil { ...transient answer... }
if !found || current.Pending || !ownsSet(current, action.SetName) {
    s.dropPendingDelete(ctx, key)
    clearButton(ctx, b, action.ChatID, action.MessageID)
    return answerCallback(ctx, b, query.ID, "That pack is no longer yours to delete; nothing was deleted at Telegram.")
}
```

Additionally, every `dropPackRecord` / `dropPackRecordIfSet` should clear
`pendingDeleteKey(ownerID)`. A record that no longer exists must not leave a live
capability behind it; the TTL is the only thing bounding it today.

---

## H1 (High) — the round-5 re-check is not pinned by any test

Mutation M2b: delete the entire `getPack` re-check block from
`handleDelPackCallback` (delpack_callback.go:177-185).

**SURVIVED.** Full suite `ok`. Item 4 of the commit description — "a defensive
re-check was also added in `handleDelPackCallback` under the lock" — has zero
test coverage. This is the same defect class the memory file records: a claim
asserted in the commit text that no test exercises. It is also the exact guard
whose incompleteness produces C1, so the gap and the bug are the same omission.

---

## Mutation results (full)

Backed up to scratchpad, restored, `md5sum -c` all match, `git status --short`
empty (verified after every mutation).

| # | Mutation | Result | Killing test |
|---|----------|--------|--------------|
| M1 | `createPack` `err == nil` → `finishNewPack` (adoption restored) | **killed** | `TestNewPack_InterruptedAttemptWithLiveSetIsRefused`, `TestNewPack_WipedStoreCannotAdoptSurvivingPack`, `TestNewPack_InconclusiveProbeThenLiveSetCannotTakeOver` |
| M2 | remove `/delpack` pending short-circuit | **killed** | `TestDelPack_PendingRecordDeletesNothingAtTelegram` |
| **M2b** | **remove `handleDelPackCallback`'s under-lock re-check** | **SURVIVED** | — |
| M3 | neutralise `releaseSlug` ownership check | **killed** | `TestReleaseSlug_RefusesANameHeldBySomeoneElse` |
| M4 | re-attach `releaseSlug`'s ownership read to the request ctx | **killed** | `TestReleaseSlug_CompletesOnACancelledContext` |
| M5b | `resolveStaleIntent` `err == nil` branch adopts the old set (R4's second path) | **killed** | `TestNewPack_DifferentSlugDoesNotAdoptExistingSet` |
| M6 | release the slug unconditionally on claim bail (drop the `created` guard) | **killed** | `TestNewPack_ResumedReservationNotReleasedWhenClaimBails` |
| **M7** | **`resolveStaleIntent` missing-branch no longer releases the dead name** | **SURVIVED** | — |
| M8 | `createPack` unknown-lookup branch drops intent + reservation | **killed** | `TestNewPack_UnknownLookupErrorAborts` |
| M9 | `createPack` refusal keeps the intent | **killed** | `TestNewPack_InterruptedAttemptWithLiveSetIsRefused` |

M7 detail: `TestNewPack_DifferentSlugReplacesDeadIntent` (pack_handlers_test.go:149)
is named for replacing a dead intent but asserts only the *new* pack. Nothing
checks that `oldslug`'s reservation was freed, so the R1 name-burn class is
unpinned. The code is correct today; only the regression barrier is missing.

The four tests added by this commit are otherwise non-vacuous: M1 kills
`TestNewPack_InconclusiveProbeThenLiveSetCannotTakeOver`, M2 kills
`TestDelPack_PendingRecordDeletesNothingAtTelegram`, M3/M4 kill the two
`TestReleaseSlug_*` tests. The `ctxHonouringSlugs` wrapper is load-bearing — its
comment ("this assertion passed whether or not the code detached until the store
was made to honour cancellation") is accurate.

---

## What the commit did close (verified by execution, not inspection)

### No path to a committed record naming a foreign set

`Pending = false` is written in exactly one place, `finishNewPack`
(pack_handlers.go:352), reachable only after `CreateNewStickerSet` returns nil.
`adjustCount` and `handleRenamePack` preserve/require the flag. Enumerated and
executed in `TestProbe_PostWipeAttackSurface`, `TestProbe_PendingToCommittedSweep`,
`TestProbe_CommittedRecordOnlyAfterCreate`:

- post-wipe `/newpack victimslug` with the set live → `slugTaken`, record dropped,
  reservation released, no `createNewStickerSet`
- inconclusive probe, then a second `/newpack` with the set live → refused, cleaned
- resume with a pending record naming the victim's set → refused, cleaned
- `createNewStickerSet` refused (`PACK_SHORT_NAME_OCCUPIED`) → intent and
  reservation both released
- with a pending record naming the victim's set, all six other commands refuse:
  `/addsticker` and `/renamepack` → `noPackYet`; `/delsticker`, `/editsticker`,
  `/ordersticker`, `/setpackicon` → `notOwnedRefusal`. **Zero** Telegram
  mutations in every case.

### `/delpack` pending short-circuit cannot be aimed at another user

`getPack(ctx, s.store, senderID(msg))` and `dropPackRecord(ctx, ownerID)` are
owner-keyed throughout, and `releaseSlug` re-verifies the holder itself
(M3 confirms). Executed: a caller can only free a reservation they hold
(`sticker_release_slug_refused` logged otherwise). Freeing a name that still has
a live set behind it is possible but not exploitable — the next claimant is
refused by `createPack`'s occupancy probe (verified: third party gets
`slugTaken`).

### No wedge

`TestProbe_WedgeAudit`, 4 leftover states × 3 escape routes = 12 runs. Every
state escapes via `/newpack <other-slug>`; 11 of 12 also escape via the same
slug or `/delpack`. The one refusal (`pending record + foreign reservation`,
same slug) is correct and has two working escapes.

---

## M (Medium) — resuming an interrupted `/newpack` silently uses the old title

`claimSlug`'s resume branch (pack_handlers.go:246) returns `existing`, discarding
the freshly parsed `title`. Executed (`TestProbe_ResumeIgnoresNewTitle`): after a
pending `mypack`/"Old" record, `/newpack mypack Brand New Title` calls
`createNewStickerSet title="Old"` and answers `"Created Old."` — confirming a
title the user did not type, with `/renamepack` the only fix. The same branch
also reuses `existing.Name`, so after a BotFather rename every resume builds a
set name with the stale `_by_<old>` suffix that Telegram will reject; the
freshly computed `setName` is discarded.

## L1 — `resolveStaleIntent` leaks the old reservation permanently

The `err == nil` branch keeps the old reservation with no record pointing at it
(`TestProbe_StaleIntentReservationLeak`: `reservation[oldslug] owner=42`, no pack
record). Correct in intent — a set really occupies the name — but the entry is
unreachable by any code path while the user holds another committed pack. Not
weaponisable: reaching the branch requires a set to genuinely exist under the
name, so it is not a cheap name-burn primitive.

## L2 — confirmed delete leaks the reservation when the record moved on

`TestProbe_ConfirmedDeleteLeaksReservationWhenRecordMoved`: `DeleteStickerSet`
succeeds, `dropPackRecordIfSet` correctly declines to touch the moved record, and
nobody releases `mypack` — a name whose set is now definitely gone stays reserved
forever. Same code path as C1's second variant.

## L3 — `releaseSlug` read-then-delete is not atomic

`getSlugReservation` then `Delete` on the same key with no CAS. Under concurrent
dispatch, a reservation re-claimed between the two calls would be deleted by the
previous holder. **Not reachable today** (serial dispatch), but `state.go:83`
documents the lock as "load-bearing rather than decorative" because of the cron
scheduler and stats hook — worth a `PutVersioned`-style compare-and-delete or an
explicit note that neither touches this store.

---

## Gates

| Gate | Result |
|------|--------|
| `go build ./...` | pass |
| `go vet ./...` | pass |
| `gofmt -l .` | clean |
| `golangci-lint run ./...` | 0 issues |
| `go test ./...` | pass |
| `go test -race ./...` | pass |
| `go test -race -count=20 ./internal/modules/sticker/...` | pass, 86.5s, no flakes |
| workspace restored | `md5sum -c` all match, `git status --short` empty |

## Recommended actions

1. **Blocking** — fix C1: invert the `handleDelPackCallback` re-check to positive
   authorisation (`!found || Pending || !ownsSet` → refuse).
2. **Blocking** — clear `pendingDeleteKey(ownerID)` in `dropPackRecord` and
   `dropPackRecordIfSet`, so a dropped record cannot leave a live delete
   capability behind.
3. **Blocking** — add a test for the re-check (kill M2b) covering all three bad
   states: `!found`, `Pending`, and record-names-another-set. The end-to-end
   sequence in C1 is the right shape.
4. High — extend `TestNewPack_DifferentSlugReplacesDeadIntent` to assert the old
   reservation was released (kill M7).
5. Medium — carry the new title (and freshly computed set name) through the
   resume branch, or state in the reply that the original title was kept.
6. Low — release the reservation in L2's branch; document or close L3.

## Unresolved questions

- Does Telegram in fact delete a sticker set when its last sticker is removed?
  The module documents this as undocumented behaviour. C1's step 3 uses it as the
  cheapest self-service way to make the set vanish, but the hole does not depend
  on it — any external deletion, or any `STICKERSET_INVALID` self-heal, reaches
  the same state.
- Is `pendingDeleteTTL` (10 min) intended as a security bound? It is currently
  the only thing limiting C1's exploitation window, and the comment justifies it
  on irreversibility grounds rather than as an authorisation control.
