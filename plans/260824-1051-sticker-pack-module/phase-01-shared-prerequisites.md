---
phase: 1
title: "Phase 1: Shared prerequisites"
status: done
priority: P1
effort: "4h"
dependencies: []
---

# Phase 1: Shared prerequisites

## Overview

Two gaps in shared code that the sticker module would otherwise expose. Both live outside
`internal/modules/sticker` and both block every later phase, so they land first and merge
independently of the feature.

1. **No panic barrier on the update path** (plan C9) — a handler panic terminates the
   process, and Phase 5 proposes decoding attacker-supplied images in a handler.
2. **`RecordingBot` cannot return structured API results** (plan C11) — three later phases
   have success criteria that are unimplementable against the current harness.

## Requirements

- Functional: a panic in any command or callback handler is recovered, logged with the same
  context as a handler error, and does not terminate the process.
- Functional: tests can make any Bot API method return a chosen JSON result, and can make a
  method fail with a Telegram-shaped error carrying an `error_code`.
- Non-functional: no behaviour change for existing modules; every existing test passes
  unmodified.
- Non-functional: the recovery path must not swallow the failure silently — it increments an
  error metric and logs at ERROR.

## Architecture

### Panic barrier in `modules.Install`

`internal/modules/dispatcher.go:65-111` registers two closures per module — one for commands,
one for callback data. Neither recovers. With `bot.WithNotAsyncHandlers()` (plan C1) the
handler runs inline on the single polling goroutine, so an unrecovered panic ends the
process for every user.

The repo already has the exact pattern to copy at `internal/cron/scheduler.go:66-74`, which
recovers around cron handlers. Mirror it:

```go
defer func() {
    if rec := recover(); rec != nil {
        metrics.IncError("handler-panic")
        log.Error("command panic", "command", cmdCopy.Name, "recovered", rec,
            "stack", string(debug.Stack()))
    }
}()
```

Apply to both closures. The callback variant should also attempt `AnswerCallbackQuery` so
the user's client stops showing a spinner, guarded so a failure there cannot panic again.

**Also fix the stale comment at `internal/modules/dispatcher.go:167`,** which claims
"would panic the goroutine before our `recover()` in webhook.go".
`internal/telegram/webhook.go` contains only `DeleteWebhook` — there has been no
webhook-served handler since the move to long polling. The comment currently tells a reader
a protection exists that does not.

### `RecordingBot` structured responses

`internal/testutil/recording_bot.go:178-195` (`okResponseFor`) returns
`{"ok":true,"result":true}` for every method not in `isMessageProducingMethod`
(`:156-165`). Methods decoding into a struct therefore always error:

| Method | Decodes into | Current test behaviour |
|---|---|---|
| `getStickerSet` | `*models.StickerSet` | `json: cannot unmarshal bool` |
| `getFile` | `*models.File` | same |
| `uploadStickerFile` | `*models.File` | same |
| `getMe` | `*models.User` | same |

Add two capabilities, both additive:

```go
// StubMethod makes method return the given JSON as its "result" field.
func (rb *RecordingBot) StubMethod(method string, resultJSON string)

// FailMethodCode makes method fail with a Telegram-shaped error carrying an
// error_code, so library errors take the same ErrorBadRequest / ErrorForbidden
// shape production emits.
func (rb *RecordingBot) FailMethodCode(method string, errorCode int, description string)
```

`FailMethod` (`:99-112`) stays as-is for existing callers, but its doc comment must state
that it produces a **codeless** failure that does **not** take the `ErrorBadRequest` shape —
that distinction is what plan rule 4 depends on, and a future reader must not confuse the two.

Precedence when both a stub and a failure are registered for one method: the failure wins,
so a test can override a stubbed happy path without unregistering it.

### Why this is a separate phase

Both changes touch files every other module's tests depend on
(`internal/modules/dispatcher.go`, `internal/testutil/recording_bot.go`). Landing them
alone, with the full suite green, keeps the blast radius reviewable and means a problem here
is not entangled with sticker logic.

## Related Code Files

- Modify: `internal/modules/dispatcher.go` (recover in both closures; fix the stale comment)
- Modify: `internal/testutil/recording_bot.go` (`StubMethod`, `FailMethodCode`, doc fix)
- Create: `internal/modules/dispatcher_panic_test.go`
- Modify: `internal/testutil/recording_bot_test.go`
- Reference: `internal/cron/scheduler.go:66-74` (the pattern to mirror)
- Reference: `internal/metrics` (`IncError`), `internal/log` (`Error`)

## Implementation Steps

1. Add the recover to the command closure in `Install`, with metric + structured log.
2. Add the recover to the callback closure, including a guarded `AnswerCallbackQuery`.
3. Correct the stale comment at `dispatcher.go:167`.
4. Add `StubMethod` and `FailMethodCode` to `RecordingBot`; document `FailMethod`'s codeless
   shape.
5. Tests per the Todo list.
6. Run the full suite — every existing test must pass untouched.

## Todo

- [x] `recover()` in the command closure with `metrics.IncError("handler-panic")`
- [x] `recover()` in the callback closure with guarded `AnswerCallbackQuery`
- [x] Fix the stale `recover()` comment at `dispatcher.go:167`
- [x] `RecordingBot.StubMethod(method, resultJSON)`
- [x] `RecordingBot.FailMethodCode(method, errorCode, description)`
- [x] Document that `FailMethod` produces a codeless failure
- [x] `dispatcher_panic_test.go`: panicking command handler
- [x] `dispatcher_panic_test.go`: panicking callback handler
- [x] `recording_bot_test.go`: stubbed `getStickerSet` decodes into `models.StickerSet`
- [x] `recording_bot_test.go`: `FailMethodCode` yields `bot.ErrorBadRequest`

## Success Criteria

- [x] A command handler that panics is recovered; the test process survives and the error metric increments
- [x] A callback handler that panics is recovered and the callback query is still answered
- [x] `rg "recover\(\)" internal/modules/dispatcher.go` returns two hits
- [x] No comment in the repo claims a `recover()` exists in `webhook.go`
- [x] `rb.StubMethod("getStickerSet", ...)` lets `b.GetStickerSet` return a populated `*models.StickerSet` with a nil error
- [x] `rb.FailMethodCode("getStickerSet", 400, "Bad Request: STICKERSET_INVALID")` produces an error satisfying `errors.Is(err, bot.ErrorBadRequest)`
- [x] `go test ./...` passes with no changes to any existing test file other than additions

## Risk Assessment

**Recovering a panic can mask a real bug.** A handler that panics on every invocation would
now fail quietly per-request instead of crashing loudly. Mitigation: log at ERROR with the
full stack and increment a distinct `handler-panic` metric, so the condition is visible
rather than silent. This is the same trade the cron scheduler already made
(`cron/scheduler.go:66-74`); consistency with it is worth more than a second opinion here.

**Changing shared test infrastructure can break other modules' tests.** Both additions are
new methods; no existing signature or default behaviour changes. The success criterion
"no changes to any existing test file other than additions" is what proves it.

**Scope note.** The panic barrier is a pre-existing repo-wide gap, not one this module
introduces — the module only makes it far easier to reach. It is included here on an
explicit user decision rather than as silent scope expansion.
