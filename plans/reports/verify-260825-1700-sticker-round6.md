# Adversarial verification — sticker module, round 6

- Commit under review: `b7803ce` ("fix(sticker): prove authority before a confirmed pack delete")
- Branch: `feature/sticker-pack-module`, Go 1.27, golangci-lint v2.13.1
- Method: static enumeration + driven end-to-end probes + mutation testing.
  All source mutations were backed up, restored, and verified byte-identical
  (`git status --short` empty, md5sums match baseline).

## Verdict

**The security fix is correct.** I could not reach any of the seven
owner-unscoped Telegram mutations with authority the caller does not hold,
under serial dispatch or under forced concurrency. The R5 hole
(`DeleteStickerSet` via a stale confirmation) is closed twice over, and I
confirmed by driven probe that the allowlist alone still holds in the one
production state where change 2 fails.

**The recurring pattern did repeat, one level down.** The flagship new test is
vacuous with respect to the guard it is named for, and the two disjuncts of the
new allowlist that the commit message itself identifies as the R5 bug are
completely unpinned. Nothing in the suite stops this fix from regressing back
into exactly the blocklist it replaced.

That is a test-integrity defect, not a live exploit. See "Merge position".

---

## 1. Enumeration of every owner-unscoped Telegram mutation

`grep` over `internal/modules/sticker/*.go` (non-test) yields exactly these
mutating calls. For each: what proves ownership at the moment of the call, and
whether that proof can go stale or be manufactured.

| Call | Site | Proof of authority at call time | Can it go stale / be forged? |
|---|---|---|---|
| `DeleteStickerSet` | `delpack_callback.go:210` | Under `lockUser`, immediately before the call: `found && !current.Pending && ownsSet(current, action.SetName)` re-read from the store | **No.** Non-`Pending` records are written only by `finishNewPack` (after a successful `CreateNewStickerSet`) and by `commitPack` from `adjustCount`/`handleRenamePack`, both of which copy an existing record's `Name`. So a non-`Pending` record proves this owner created that set. Gap between check and call is one store `Delete` on the pending key, no dispatch point. |
| `CreateNewStickerSet` | `pack_handlers.go:370` | `GetStickerSet(pack.Name)` must positively return `STICKERSET_INVALID` in the same handler | No adoption path remains; `err == nil` (occupied) drops the intent and releases the reservation. Verified by probe C below. |
| `SetStickerSetTitle` | `pack_handlers.go:591` | `getPack(ownerID)` found and `!Pending`; `Name` taken from that record | Owner-keyed read, same handler. Read happens *before* `lockUser` — hypothetical-concurrency only (see L2). |
| `SetStickerSetThumbnail` | `setpackicon.go:44` | `resolveOwned` → `ownsSet(pack, replied.Sticker.SetName)`; `Name` from the caller's own record | Slow media leg sits between check and call, but no dispatch point under serial dispatch. |
| `AddStickerToSet` | `sticker_handlers.go:69` | `getPack(ownerID)` found and `!Pending`; `UserID` is always the caller, `Name` from the caller's record | Same shape. |
| `DeleteStickerFromSet` | `sticker_handlers.go:122` | `resolveOwned` → `ownsSet(pack, st.SetName)` on the replied sticker | `st.SetName` and `st.FileID` come from the same Telegram-rendered `Sticker`; not client-forgeable. Old scrollback stickers from a deleted-then-reclaimed pack are stopped because the record is dropped alongside the set. |
| `SetStickerEmojiList` | `sticker_handlers.go:175` | `resolveOwned` | Same. |
| `SetStickerPositionInSet` | `sticker_handlers.go:214` | `resolveOwned` | Same. |

**No deferred capability exists anywhere except `PendingDelete`.** Every other
command resolves authority and consumes it inside the same handler invocation,
so the stale-authority shape found at `/delpack` has no sibling at
`/addsticker`, `/delsticker`, `/editsticker`, `/ordersticker`, `/setpackicon`
or `/renamepack`. I drove `/addsticker` and `/delsticker` end to end against a
record that had moved on; both refuse at `resolveOwned`/`getPack`.

## 2. Attacks driven end to end (probe results)

Probes were written as a temporary test file, run, and removed.

| Probe | Setup | Result |
|---|---|---|
| **A** — record gone, confirmation alive | non-`Pending` pack + live `PendingDelete`, record deleted out from under it, victim seeded holding `mypack_by_testbot` | `methods = [editMessageReplyMarkup answerCallbackQuery]`. **0 `deleteStickerSet`.** Victim record intact. Allowlist holds. |
| **A2** — record moved to `Pending`, confirmation alive | same, record replaced with a `Pending` intent naming the same set | **0 `deleteStickerSet`.** |
| **C** — `/delpack` on a `Pending` record frees a name with a live set behind it, next claimant attacks | Bob's interrupted attempt created the set; Bob `/delpack` (frees `mypack`); Alice `/newpack mypack` | Alice: `[getMe getStickerSet sendMessage]`, reply `"That pack name is taken."` No record, no adoption. Alice's follow-up `/delpack`: `"You don't have a pack yet."`, 0 API calls. **`createPack`'s occupancy probe is the wall and it holds.** |
| **E** — `dropPackRecord` with a failing pending store | `pending.Delete` returns an error | Record deleted, reservation released, **confirmation survives**. This is the state that makes the allowlist's `!found` disjunct load-bearing in production. |
| **G** — can a `Pending` record coexist with a live confirmation via handlers alone? | `/delpack` (prompt live) then `/newpack second Two` | Refused at the precheck: `"You already have a pack (mypack)."` Not reachable through handlers — but *is* reachable after an E-style failed clear. |
| **Concurrency** — 3 goroutines (`handleDelPackCallback` + `handleDelPack` + `handleAddSticker`) × 50 iterations × 3 runs, `-race` | | No races, never more than one `deleteStickerSet`. |

Dispatch model re-confirmed serial: `internal/telegram/client.go:27-28`
(`WithSkipGetMe`, `WithNotAsyncHandlers`), no `WithWorkers` anywhere, so the
library default of one worker applies. All concurrency observations below are
labelled hypothetical.

## 3. Mutation testing

Backup → mutate → `go test ./internal/modules/sticker/` → restore.

| # | Mutation | Outcome | Killing test |
|---|---|---|---|
| 1 | Allowlist reverted to the R5 blocklist (`found && current.Pending && ownsSet(...)`) | **KILLED** | `TestDelPackCallback_StalePressLeavesTheCurrentPackAlone` (`delpack_callback_test.go:286`) — *only* this one |
| 2 | `dropPendingDelete` removed from `dropPackRecord` | **KILLED** | `TestDropPackRecord_ClearsAnOutstandingConfirmation` (`pack_handlers_test.go:874`) — *only* this one |
| 3 | Resume returns `existing` verbatim (both carry-overs removed) | **KILLED** | `TestNewPack_ResumeUsesTheTitleJustTyped` |
| 3b | Only `resumed.Name = intent.Name` removed | **SURVIVED** | — |
| 4 | `releaseSlug` removed from `resolveStaleIntent`'s `isStickerSetMissing` branch | **KILLED** | `TestNewPack_DifferentSlugReplacesDeadIntent` (`pack_handlers_test.go:171`) |
| 5 | Allowlist disjunct `!found` dropped (`_ = found`) | **SURVIVED** | — |
| 6 | Allowlist disjunct `current.Pending` dropped | **SURVIVED** | — |
| 7 | Allowlist disjunct `!ownsSet(current, action.SetName)` dropped | **KILLED** | `TestDelPackCallback_StalePressLeavesTheCurrentPackAlone` |
| 8 | Mutations 1 **and** 2 together | **KILLED** | all three of `StalePressLeavesTheCurrentPackAlone`, `StalePressCannotDeleteTheNextHolder`, `DropPackRecord_ClearsAnOutstandingConfirmation` |

Gates: `go build ./...` OK · `go test ./... -race -count=20` OK ·
`golangci-lint run ./...` → `0 issues.` · `gofmt -l .` → clean.

---

## Findings

### H1 — The flagship round-6 test does not exercise the round-6 guard (High, test integrity)

`TestDelPackCallback_StalePressCannotDeleteTheNextHolder`
(`internal/modules/sticker/delpack_callback_test.go:411-453`) is documented as
the regression test for the allowlist. It is not.

Reproduction (mutation 1, in isolation):

```
$ # revert the allowlist to the R5 blocklist, nothing else
$ go test ./internal/modules/sticker/ -run TestDelPackCallback_StalePressCannotDeleteTheNextHolder -v
--- PASS: TestDelPackCallback_StalePressCannotDeleteTheNextHolder (0.00s)
```

Cause: at line 434 the test calls

```go
	// U's set vanishes at Telegram; a self-heal clears the record and the name.
	s.dropPackRecord(ctx, testUser)
```

`dropPackRecord` now (change 2) deletes the `PendingDelete` as well, so the
callback returns at `delpack_callback.go:124`
(`pending.Get` → `storage.ErrNotFound` → `"This confirmation expired or was
already used."`) roughly fifty lines before the allowlist at line 193. The test
proves change 2, and only change 2 — which is already proven by
`TestDropPackRecord_ClearsAnOutstandingConfirmation`.

Mutation 2 in isolation also leaves this test **passing** (the allowlist then
catches it). Only the double revert (mutation 8) fails it. A test that requires
both defences to be removed before it fires cannot detect either one
regressing.

This is the fourth consecutive round in which a test was named for a behaviour
a structurally earlier guard prevents it from reaching.

**Fix:** seed the state directly instead of routing through `dropPackRecord` —
`s.store.Delete(ctx, packKey(testUser))` and leave the confirmation in place —
so the press actually arrives at the under-lock re-check. Probe A above is a
working version of that test; it passes on `HEAD` and fails under mutation 1.

### H2 — The `!found` disjunct is unpinned, and the state it guards is production-reachable (High)

Mutation 5 (`_ = found; if current.Pending || !ownsSet(current, action.SetName)`)
survives the entire suite. That disjunct is the exact half of the R5 bug the
commit message calls out first ("fell through on the two that mattered: no
record at all").

It is not dead code. `dropPackRecord` logs and continues when the pending
delete cannot be removed:

```go
func (s *state) dropPendingDelete(ctx context.Context, key string) {
	commitCtx, cancel := commitContext(ctx)
	defer cancel()
	if err := s.pending.Delete(commitCtx, key); err != nil && !errors.Is(err, storage.ErrNotFound) {
		log.Error("sticker_drop_pending_delete", "err", err)
	}
}
```

Probe E confirms the resulting state on `HEAD`: pack record gone, reservation
released, confirmation still live and pressable. Probe A confirms `!found` is
what refuses the press in that state, and that without it the press lands on
whoever holds the name now. A single Mongo write failure is enough to enter it.

**Fix:** add the probe-A test (record deleted directly, confirmation left
alive, victim seeded under the same name, assert zero `deleteStickerSet`).

### H3 — The `current.Pending` disjunct is unpinned (Medium-High)

Mutation 6 survives. Reachable in production by composing H2 with a normal
`/newpack`: once a failed `dropPendingDelete` has left a confirmation alive
with no record, `/newpack` passes the precheck and writes a fresh `Pending`
intent. A `Pending` record is bookkeeping written *before* Telegram is called —
`handleDelPack` and `TestDelPack_PendingRecordDeletesNothingAtTelegram` both
say so explicitly — so it must never authorise a delete. Probe A2 shows the
guard works today; nothing pins it.

**Fix:** add probe A2 as a test.

### M1 — `resumed.Name = intent.Name` is unpinned and repoints the record at a different set under a bot rename (Medium)

`pack_handlers.go`, `claimSlug` resume branch:

```go
		resumed := existing
		resumed.Title = intent.Title
		resumed.Name = intent.Name
		return resumed, false, nil
```

Mutation 3b (removing only the `Name` line) survives the whole suite — the
title carry-over is the only half the new test covers, despite the `Name` line
being the only one of the two that changes *which set* is touched.

Probe B, driven end to end: seed an interrupted attempt with
`Name = "mypack_by_oldbot"` (bot renamed in BotFather since), stub `getMe` →
`testbot`, re-run `/newpack mypack Title`:

```
probed name  = "mypack_by_testbot"
created name = "mypack_by_testbot"
stored pack  = {Slug:mypack Name:mypack_by_testbot ... Pending:false}
```

Before `b7803ce` the probe targeted `mypack_by_oldbot`. If the interrupted
attempt did create that set, the old behaviour answered `slugTaken` and cleaned
up; the new behaviour creates a second set and orphans the first with no local
record pointing at it and no route to reach it through the bot. The `mypack`
reservation stays held (same slug), so no cross-user damage — this is a
resource leak and a behaviour regression, not a security defect.

It also directly contradicts the invariant `ownsSet`'s own doc comment states
(`setname.go:73-79`): "It deliberately does not re-derive the name from the
live bot username. Renaming the bot in BotFather is supported and leaves
existing set names untouched." The resume branch now re-derives it.

**Fix:** either drop the `Name` carry-over (the title fix is what the commit
message describes; the `Name` line is unexplained scope), or keep it and add a
test that pins the intent under a changed username. As written it is an
unexplained, untested line inside a security-sensitive commit.

### L1 — `ownsSet` uses Unicode case folding on a security comparison (Low, informational)

`strings.EqualFold` applies simple Unicode folding, so
`ownsSet(Pack{Name: "mypack_by_testbot"}, "mypacKk_by_testbot")` (U+212A
KELVIN SIGN) returns **true** — verified by probe F. Not exploitable: Telegram
constrains sticker-set short names to `[A-Za-z0-9_]`, `validateSlug` forces
`^[a-z][a-z0-9_]{2,39}$`, and the only two inputs are a stored record name and
a Telegram-rendered `Sticker.SetName`. Recording it because the comment
justifies `EqualFold` on casing grounds alone and does not note the folding
surface it brings along. `strings.ToLower` comparison would be equally correct
and narrower.

### L2 — Read-modify-write outside the lock in four handlers (Low, hypothetical concurrency)

`state.go`'s corrected `lockUser` comment says the lock "stays because every
mutation here is a read-modify-write, which is wrong the moment dispatch stops
being serial." Four handlers do not honour that:

- `handleDelPack` — reads the pack, then writes `s.pending`, with **no lock at
  all** on the prompt path (the lock is taken only inside the `pack.Pending`
  branch).
- `handleAddSticker`, `handleDelSticker`, `handleRenamePack` — `getPack` runs
  *before* `defer s.lockUser(ownerID)()`, so the record they act on was read
  outside the critical section.

Only `handleNewPack` takes the lock first. Moot under
`WithNotAsyncHandlers` + one worker; flagged because the comment asserts a
property the code does not have, which is precisely the class of defect change
6 was written to fix.

### L3 — `internal/keylock` package doc contradicts the dispatch model (Low, out of scope)

`internal/keylock/keylock.go:6-8`: "The bot dispatcher runs each Telegram update
in its own goroutine". `internal/telegram/client.go:18-22` and
`internal/modules/dispatcher.go:124-126` both say the opposite, and change 6
corrected `state.go` to match. Same wrong-reason-for-a-right-guard shape, one
package over. Not this commit's responsibility; worth a follow-up.

---

## Previously closed classes — re-confirmed still closed

| Class | Evidence |
|---|---|
| Post-wipe adoption | No adoption branch remains (`createPack` has only `occupied → refuse` / `missing → create` / `unknown → abort`). `TestNewPack_WipedStoreCannotAdoptSurvivingPack` passes; mutation of the occupancy branch is out of scope but the branch is asserted on directly. |
| Inconclusive probe then live set | `TestNewPack_InconclusiveProbeThenLiveSetCannotTakeOver` passes; the guard it defeated no longer exists (refusal is unconditional). |
| Pending record as delete authority | `handleDelPack` refuses to prompt for a `Pending` record and drops it locally; `TestDelPack_PendingRecordDeletesNothingAtTelegram` passes; the callback's `current.Pending` disjunct is a second wall (probe A2). |
| Name-burning DoS | Precheck ordering (record read before `reserveSlug`) intact; `TestNewPack_RefusedRunsClaimNoNames` and `TestNewPack_FreshReservationReleasedWhenClaimBails` cover it. |
| Cross-user `releaseSlug` | Ownership verified inside the operation, not the caller; `TestReleaseSlug_RefusesANameHeldBySomeoneElse` passes. |

## Change 2 audit (`dropPackRecord` now writes `s.pending`)

Every call site passes an owner the caller already owns — no cross-user aim is
possible:

| Call site | `ownerID` source |
|---|---|
| `handleDelPack` (pending branch) | `senderID(msg)` |
| `handleRenamePack` (`isStickerSetMissing`) | `senderID(msg)` |
| `handleAddSticker` / `handleDelSticker` / `handleEditSticker` / `handleOrderSticker` / `handleSetPackIcon` (`isStickerSetMissing`) | `senderID(msg)` |
| `dropPackRecordIfSet` ← delpack callback success | `action.OwnerID`, and the callback already proved `query.From.ID == action.OwnerID` and loaded the action under the presser's own key |

`senderID` additionally rejects bots, anonymous group admins
(`SenderChat != nil`) and `From.ID == 0`, so `pendingDeleteKey` can never be
built from the shared `GroupAnonymousBot` identity. Failure of the added write
is logged and non-fatal, leaving the record deleted and the confirmation alive
— the H2 state, which the allowlist covers.

## Merge position

`b7803ce` is a genuine, correct security fix and I would not block it on
correctness. What I do block on is the test claim: the commit ships a test
named for the guard it introduces, that guard can be fully reverted with the
test still green, and two of the guard's three load-bearing disjuncts have zero
coverage. Given five prior rounds where a false-clean was produced by exactly
this — a same-named test that never reaches the branch — the fix should not
land with its own regression detector inoperative.

Blocking work is small and mechanical: replace `s.dropPackRecord(ctx, testUser)`
in `StalePressCannotDeleteTheNextHolder` with a direct `s.store.Delete`, and add
the probe-A2 variant. Both are ten-line changes and both fail on `HEAD` under
the corresponding mutation.

M1 (`resumed.Name`) should be resolved before merge too — decided either way,
but not left as an untested, undescribed line in a commit about proving
authority.

## Unresolved questions

1. Is `resumed.Name = intent.Name` intentional, and if so what should happen to
   a set stranded under the pre-rename name? The commit message describes only
   the title fix.
2. Does Telegram reserve a deleted sticker set's short name? Plan note R11
   still marks this unverified, and `dropPackRecord`'s release-the-name
   behaviour is documented as a no-op if it does. Unchanged by this commit.
