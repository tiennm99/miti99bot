# Round-4 Verification — sticker module (adversarial, mutation-driven)

Date: 2026-08-25 · Branch `feature/sticker-pack-module` @ `30fa3b3` (identical to
`origin/feature/sticker-pack-module`) · Go 1.27.0 linux/arm64 · golangci-lint v2.13.1

Method: every claim tested by mutating source and re-running the package, not by
reading. 34 mutations applied and reverted. All files restored byte-identical
(md5 verified, `git status --short` empty).

## Verdict

**All six round-4 claims verified.** No claim refuted. But three *previously
claimed* guarantees are unpinned and one test name is still misleading (the same
class of defect as round 3's `...SurvivesABail`). None of this is a new production
defect; it is test-coverage overstatement.

## 1. Mutation results

### Round-4 claims under test

| # | Mutation | Result | Killing test |
|---|---|---|---|
| M1a | `handleNewPack`: delete `if created { s.releaseSlug(...) }` | **KILLED** | `TestNewPack_FreshReservationReleasedWhenClaimBails` |
| M1b | `handleNewPack`: make the release unconditional | **KILLED** | `TestNewPack_ResumedReservationNotReleasedWhenClaimBails` |
| M2 | `createOrAdopt`: remove the `if freshReservation` adoption gate | **KILLED** | `TestNewPack_WipedStoreCannotAdoptSurvivingPack` |
| M3 | `handleAddSticker`: early `return nil` (handler is a no-op) | **KILLED** | `TestAddSticker_EmojiPrecedence` (+both subtests), `TestAddSticker_FallsBackToDefaultEmoji`, +6 others |
| M3b | invert precedence: source emoji beats explicit args | **KILLED** | `TestAddSticker_EmojiPrecedence/explicit_wins` |
| M3c | drop source-emoji inheritance | **KILLED** | `TestAddSticker_EmojiPrecedence/inherits_from_replied_sticker` |
| M4 | remove `"sticker": sticker.New` + import from `factories()` | **KILLED** | `TestCommandDiscovery_AllPublicCommandsHaveSafeMetadata` — and by the **reverse check specifically**: 9 lines `command_menu_test.go:133: /<cmd> is expected but not registered` |
| M5 | `recording_bot.handle`: tolerate any multipart parse failure (`if err == nil`) | **KILLED** | `TestRecordingBot_RejectsMalformedMultipart` |
| M6a | `isBinding`: disable tag-block (`E0020..E007F`) binding | **KILLED** | `TestParseEmoji_ClusterEdgeCases/tag_sequence_flag` |
| M6b | `splitClusters`: ZWJ absorbs the next rune unconditionally | **KILLED** | `.../joiner_before_a_flag` |
| M6c | `trimJoiners` → identity | **KILLED** | `.../joiner_before_a_flag`, `.../trailing_joiner` |
| M6d | `isEmojiCluster`: accept a lone regional indicator | **KILLED** | `.../lone_regional_indicator`, `.../odd_regional_indicator_count` |
| M6e | drop each of the 9 new `emojiSingletons` entries, one at a time | **9/9 KILLED** | `.../copyright`, `/registered`, `/arrow_curving_up`, `/arrow_curving_down`, `/circled_m`, `/wavy_dash`, `/part_alternation`, `/japanese_congratulations`, `/japanese_secret` |

Claims 1-6: **verified, all directions.** Claim 1's two-direction pinning is real —
M1a and M1b are killed by *different* tests, and by only one test each.

### Additional adversarial mutations (not claimed, run to find gaps)

| # | Mutation | Result | Killing test |
|---|---|---|---|
| M8 | `createOrAdopt` unknown-error branch releases the slug | KILLED | `TestNewPack_UnknownLookupErrorAborts`, `TestNewPack_ResumedReservationSurvivesABail` |
| M10 | `releaseSlug`: drop its **own** ownership check | **SURVIVED** | — |
| M11b | `resolveStaleIntent`: skip the reservation-owner re-proof | KILLED | `TestNewPack_StaleIntentCannotAdoptForeignName` |
| M12b | `reserveSlug`: treat another user's reservation as resumable | KILLED | `TestNewPack_CannotSeizeAnotherUsersPack`, `TestNewPack_ForeignReservationRefusedBeforeAnyAPICall` |
| M13 | refused adoption leaves intent + reservation behind | KILLED | `TestNewPack_WipedStoreCannotAdoptSurvivingPack` |
| M14 | `releaseSlug`: read reservation on request ctx, not `commitContext` | **SURVIVED** | — |
| M16b | delpack callback: drop `query.From.ID != action.OwnerID` | **SURVIVED** | — |
| M17 | delpack callback: drop the message-binding check | KILLED | `TestDelPackCallback_RejectsWrongBinding` (+subtests), `TestDelPackCallback_BystanderCannotTouchAnotherUsersPrompt` |
| M18 | delpack callback: drop the expiry check | KILLED | `TestDelPackCallback_RejectsExpired` |
| M19b | delpack callback: drop the `action.ID != id` nonce check | **SURVIVED** | — |
| M6e' | drop each pre-existing singleton `203C`, `2049`, `2122`, `2139` | **4× SURVIVED** | — |
| M7 | drop each `emojiRanges` entry, one at a time | 3 KILLED (`1F300`, `2600`, `2B00`) / **5 SURVIVED** (`1F000`, `2190`, `2300`, `25A0`, `1F1E6`) | — |

**Score: 29 killed / 34 mutation slots, 12 survivors across 5 distinct sites.**

## 2. Emoji differential probe (full rune space)

Temp in-package test walked `0x0..0x10FFFF` comparing `isEmojiRune` against a
reconstructed round-3 switch (same 8 blocks + the 4 pre-existing singletons).
`emoji.go` exists in exactly one commit (`d831747`) on this branch and nowhere in
history/reflog/other branches, so the old switch could not be recovered verbatim
— it was reconstructed from the range table plus the round-3 correctness report
(`correctness-review-260825-1515-sticker-module.md:116-131`, which enumerates the
9 refused codepoints and names `2122`/`2139` as already special-cased).

```
deliberate additions observed: 9 of 9 -> [U+00A9 U+00AE U+24C2 U+2934 U+2935 U+3030 U+303D U+3297 U+3299]
UNEXPECTED classification changes: 0
```

**Result: zero unexpected classification changes.** The switch→array+map
restructure is behaviour-preserving modulo exactly the 9 intended additions.

Second, non-circular probe against authoritative Unicode data
(`unicode.org/Public/UCD/latest/ucd/emoji/emoji-data.txt`, 1288 lines):

```
accepted-but-not Emoji/Extended_Pictographic: 2140   (e.g. U+2190..U+21FF arrows, U+25A0.. shapes)
Emoji-property runes rejected: 12 -> [U+0023 U+002A U+0030..U+0039]
```

The 12 rejections are the keycap bases, handled by the `keycapCombining` branch in
`isEmojiCluster` — not a defect. The 2140 false positives are the documented
block-approximation trade-off (`emoji.go` comment: "Deliberately ranges rather
than a property lookup"), pre-existing and low impact: Telegram answers
`STICKER_EMOJI_INVALID` and `apiRefusal` renders a sane message. Not a round-4
regression. Informational only.

## 3. New / still-wrong tests found

### F-1 (MEDIUM) — `TestNewPack_ResumedReservationSurvivesABail` still does not test what its name and comment claim

`pack_handlers_test.go:573`. Its comment says *"A reservation the caller merely
resumed must not be released when a later step bails — it predates this command."*
That is the `created`-flag distinction. Proven false by mutation:

- under **M1b** (release made unconditional — i.e. the resumed/fresh distinction
  deleted outright) this test still passes: `ok ... 0.019s`. Only
  `TestNewPack_ResumedReservationNotReleasedWhenClaimBails` catches M1b.
- what it *actually* pins is M8: `createOrAdopt`'s unknown-error branch must not
  release. So does `TestNewPack_UnknownLookupErrorAborts`, which M8 also killed.

It is a duplicate of `TestNewPack_UnknownLookupErrorAborts` wearing the name of the
round-4 test that replaced it. Round 4 correctly added the two real tests but left
the misnamed one in place. Rename to `TestNewPack_UnknownLookupErrorKeepsTheReservation`
or delete it.

### F-2 (MEDIUM) — `TestDelPackCallback_RejectsOtherUser` does not exercise the owner check

`delpack_callback_test.go`. **M16b survived**: deleting
`if query.From.ID != action.OwnerID` from `delpack_callback.go:118` breaks no test.
Reason: the record is fetched with `key := pendingDeleteKey(query.From.ID)` (`:108`),
so a foreign presser has no record at all and exits three lines earlier via
`ErrNotFound` → *"This confirmation expired or was already used."* The owner check
is structurally unreachable; the test named for it passes through a different
branch. Authorization is genuinely enforced (by the key), so this is a test-naming
and dead-code issue, not a hole. Fix: assert the *reply text*, or drop the
unreachable branch.

### F-3 (LOW) — `releaseSlug`'s own ownership check is unpinned

**M10 survived.** The code comment (`pack_handlers.go:176-181`) states the check
exists precisely because *"this module has already been bitten once by an ownership
check that lived in the caller instead of the operation."* Nothing tests it. Every
current call site happens to pass the correct owner, so the check is presently
redundant — which is exactly why a future call site could silently regress it.

### F-4 (LOW) — round-3's `commitContext` fix in `releaseSlug` is unpinned

**M14 survived.** Moving the ownership read back onto the request context breaks
nothing: there is no cancelled-context test in the package (`grep context.WithCancel
internal/modules/sticker/*_test.go` → 0 hits). The comment at `:183-190` describes a
concrete failure mode (SIGTERM mid-release leaves an unreachable reservation) with
no test behind it.

### F-5 (LOW) — delpack nonce check unpinned

**M19b survived.** Acknowledged in-code as defence-in-depth subsumed by the
message binding (`:141-143`), so lower priority than F-2, but it is untested
dead weight.

### F-6 (LOW) — dead `emojiRanges` entry with a load-bearing-sounding comment

`{0x1F1E6, 0x1F1FF}` is fully contained in `{0x1F000, 0x1F2FF}` — **M7 confirms both
survive individual deletion**. Its comment ("regional indicators; isEmojiCluster
requires a pair") reads as though it is required. It is not. Same for the untested
`2190`, `2300`, `25A0`, `1F000` entries and the four pre-existing singletons
(`203C`, `2049`, `2122`, `2139`) — all silently deletable.

### No vacuous-loop assertions remain

Swept every `for _, call := range rb.Sent()` in the changed packages. The four hits
are either negative assertions (correct as loops), counting helpers, or already
gated by a preceding `countMethod(...) != 1` fatal. Round 4's `addedStickerPayload`
helper is genuine — M3 proves it. Also scanned all 89 sticker tests + the new
dispatcher/testutil/cmd tests for zero-assertion bodies: 4 heuristic hits, all
false positives (the assertion is a `Fatalf` on `err == nil`).

## 4. Regression sweep — `internal/testutil/recording_bot.go`

26 test files across 18 packages construct a `RecordingBot`. All pass.

The behaviour change (non-empty body that fails multipart parse → 400 instead of
200-with-empty-form) is scoped correctly:

- `go-telegram/bot@v1.20.0 raw_request.go:28-71` always sends multipart; when
  `params == nil` (parameterless methods) it skips both `buildRequestForm` and
  `form.Close()`, so the body is **zero bytes** — the `len(body) > 0` gate is the
  right discriminator. `TestRecordingBot_ServesParameterlessCall` confirms getMe.
- The exact hazard the comment warns about exists in the repo:
  `internal/modules/stock/dividend_flow_test.go:115` and `:139` assert
  `Form["reply_markup"] != ""` is false. Both still pass — the change *protects*
  them rather than breaking them.
- No caller asserts on a field only present under a file part.

`go test -race -count=1 ./...` — clean, no races, exit 0.
`go test -count=20 ./internal/modules/sticker/` — `ok ... 7.964s`, no flakes.

Nit (non-blocking): `recording_bot.go handle()` carries two overlapping comment
paragraphs saying the same thing — a stale round-3 paragraph left above the
round-4 one.

## 5. Gates

```
golangci-lint run ./...   → 0 issues.
gofmt -l . (minus third_party) → (empty)
go test -race -count=1 ./... → clean
```

## 6. Final state (verbatim)

```
$ git status --short
$ 
```
(empty)

md5 of every tracked file diffed against the pre-review baseline: **MD5 IDENTICAL**.

```
$ go test ./...
ok  	github.com/tiennm99/miti99bot/cmd/server	(cached)
ok  	github.com/tiennm99/miti99bot/internal/cron	(cached)
ok  	github.com/tiennm99/miti99bot/internal/deploynotify	(cached)
ok  	github.com/tiennm99/miti99bot/internal/keylock	(cached)
ok  	github.com/tiennm99/miti99bot/internal/log	(cached)
ok  	github.com/tiennm99/miti99bot/internal/metrics	(cached)
ok  	github.com/tiennm99/miti99bot/internal/modules	(cached)
ok  	github.com/tiennm99/miti99bot/internal/modules/amlich	(cached)
ok  	github.com/tiennm99/miti99bot/internal/modules/coin	(cached)
ok  	github.com/tiennm99/miti99bot/internal/modules/gold	(cached)
ok  	github.com/tiennm99/miti99bot/internal/modules/lol	(cached)
ok  	github.com/tiennm99/miti99bot/internal/modules/loldle	(cached)
ok  	github.com/tiennm99/miti99bot/internal/modules/misc	(cached)
ok  	github.com/tiennm99/miti99bot/internal/modules/monkeyd	(cached)
ok  	github.com/tiennm99/miti99bot/internal/modules/stats	(cached)
ok  	github.com/tiennm99/miti99bot/internal/modules/sticker	(cached)
ok  	github.com/tiennm99/miti99bot/internal/modules/stock	(cached)
ok  	github.com/tiennm99/miti99bot/internal/modules/util	(cached)
ok  	github.com/tiennm99/miti99bot/internal/modules/util/chathelper	(cached)
ok  	github.com/tiennm99/miti99bot/internal/modules/wordle	(cached)
ok  	github.com/tiennm99/miti99bot/internal/server	(cached)
ok  	github.com/tiennm99/miti99bot/internal/storage	(cached)
?   	github.com/tiennm99/miti99bot/internal/systemstate	[no test files]
ok  	github.com/tiennm99/miti99bot/internal/telegram	(cached)
ok  	github.com/tiennm99/miti99bot/internal/testutil	(cached)
ok  	github.com/tiennm99/miti99bot/internal/testutil/mongotest	(cached)
```

## 7. Recommended actions

1. **F-1** — rename or delete `TestNewPack_ResumedReservationSurvivesABail`. Third
   round running that a test in this file claims more than it proves; the name is
   what misled round 3.
2. **F-2** — assert the reply text in `TestDelPackCallback_RejectsOtherUser`, or
   remove the unreachable `query.From.ID != action.OwnerID` branch.
3. **F-3 / F-4** — add two small tests: a cross-owner `releaseSlug` call, and a
   `releaseSlug` under a cancelled parent context. Both are ~10 lines and pin
   fixes whose comments describe real past incidents.
4. **F-6** — delete `{0x1F1E6, 0x1F1FF}` from `emojiRanges` or fix its comment.
5. Drop the duplicated comment paragraph in `recording_bot.go handle()`.

None of 1-5 blocks merge. All are test/comment hygiene against real production
behaviour that is correct today.

## Unresolved questions

- The pre-round-4 `isEmojiRune` switch is unrecoverable from git (single squashed
  commit). The differential probe is therefore partly circular *for the singleton
  set* — it is fully independent for the eight ranges. If the round-3 source is
  available elsewhere, re-running the probe against the verbatim original would
  close the last gap.
- Are the four pre-existing singletons (`203C ‼`, `2049 ⁉`, `2122 ™`, `2139 ℹ`)
  intentionally unlisted in `TestParseEmoji_ClusterEdgeCases`, or an oversight when
  round 4 added the nine?
