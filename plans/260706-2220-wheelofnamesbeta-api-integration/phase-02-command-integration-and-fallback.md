---
phase: 2
title: Command Integration And Fallback
status: completed
priority: P2
dependencies:
  - 1
---

# Phase 2: Command Integration And Fallback

## Overview

Wire `/wheelofnamesbeta` to try the remote API first when configured, while
preserving the existing local GIF renderer as fallback and preserving Telegram
upload behavior.

## Requirements

- Functional: Keep `splitWheelOptions` and `pickWheelOption` as the only winner
  selection source in the bot.
- Functional: Use remote GIF only when API call succeeds with valid GIF bytes.
- Functional: Fall back to `renderWheelOfNamesBetaGIF(options, winner)` on any
  remote error.
- Functional: Preserve `sendAnimation`, `MessageThreadID`, filename, and
  non-spoiler caption.
- Non-functional: Remote failure should log enough to diagnose status/error but
  should not reply with stack traces or token-bearing details.
- Non-functional: Existing command behavior remains unchanged when env is unset.

## Architecture

Introduce a small orchestration helper in package `misc`:

```go
func renderWheelOfNamesBetaAnimation(ctx context.Context, options []string, winner int) ([]byte, int, int, int, error)
```

Return bytes plus Telegram metadata (`duration`, `width`, `height`). Suggested
metadata:
- Remote success: duration `7`, width/height `512`.
- Local fallback: existing `wheelBetaDuration`, `wheelBetaSize`,
  `wheelBetaSize`.

Alternative: return a tiny struct:

```go
type wheelBetaAnimation struct {
    Data     []byte
    Duration int
    Width    int
    Height   int
}
```

The command handler should stay simple:
1. parse options
2. pick winner
3. call `renderWheelOfNamesBetaAnimation`
4. send animation
5. on final render/upload failure, text fallback remains `options[winner]`

## Related Code Files

- Modify: `/config/workspace/tiennm99/miti99bot/internal/modules/misc/wheelofnames_beta_command.go`
- Modify: `/config/workspace/tiennm99/miti99bot/internal/modules/misc/wheelofnames_beta.go` only if helper placement requires it
- Modify: `/config/workspace/tiennm99/miti99bot/internal/modules/misc/handlers_test.go`

## Implementation Steps

1. Add the orchestration helper that creates the env client and attempts remote
   render.
2. If client is unconfigured, skip remote without warning noise.
3. If client is configured but fails, log warning with safe fields:
   - command: `wheelofnamesbeta`
   - status/error category
   - never token or request body
4. On remote success, return remote bytes and remote metadata.
5. On remote failure, call current local `renderWheelOfNamesBetaGIF`.
6. Update `wheelOfNamesBetaCommand` to use returned metadata in
   `SendAnimationParams`.
7. Keep caption exactly `"Spinning..."`.

## Success Criteria

- [ ] URL unset path still sends local GIF and existing tests pass.
- [ ] URL configured path calls remote service and sends remote GIF.
- [ ] Remote success path uses Telegram metadata `duration=7`,
  `width=512`, `height=512`.
- [ ] Remote failure path sends local GIF, not text, when local renderer works.
- [ ] Telegram upload failure still replies with the selected winner text.
- [ ] Caption does not contain the winner.
- [ ] Message thread forwarding remains covered.

## Risk Assessment

- Risk: Remote and local metadata diverge.
  Mitigation: use explicit metadata struct; tests assert remote width/height and
  local compatibility.
- Risk: Remote API chooses a different winner.
  Mitigation: always pass `winnerIndex`; ignore remote winner headers for bot
  caption/source of truth.
- Risk: A configured but down service makes the command slower.
  Mitigation: timeout and fallback; consider lowering timeout later only after
  production timing data.
