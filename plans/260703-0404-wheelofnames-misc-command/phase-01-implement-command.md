---
phase: 1
title: Implement command
status: completed
priority: P2
dependencies: []
---

# Phase 1: Implement command

## Overview

Add the `/wheelofnames` command inside `internal/modules/misc/misc.go`, using the current misc module pattern: small command factory returning `modules.Command`, `chathelper.ArgAfterCommand` for parsing, and `chathelper.Reply` for plain text response.

## Requirements

- Functional: parse text after `/wheelofnames` as comma-separated options.
- Functional: trim whitespace and drop empty options.
- Functional: reply usage when no valid options exist.
- Functional: randomly pick one option from valid entries; preserve duplicates as weighting.
- Non-functional: keep implementation local to misc; no storage dependency; no new module.

## Architecture

Flow:

```text
Telegram update
  -> dispatcher matches wheelofnames
  -> wheelOfNamesCommand handler
  -> chathelper.ArgAfterCommand
  -> splitWheelOptions
  -> math/rand/v2 rand.N(len(options))
  -> chathelper.Reply
```

Use a tiny pure helper for parsing:

```go
func splitWheelOptions(arg string) []string {
    parts := strings.Split(arg, ",")
    out := make([]string, 0, len(parts))
    for _, part := range parts {
        if s := strings.TrimSpace(part); s != "" {
            out = append(out, s)
        }
    }
    return out
}
```

No crypto-grade randomness required; this is a casual selection command, not auth, payment, or security behavior.

## Related Code Files

- Modify: `/config/workspace/tiennm99/miti99bot/internal/modules/misc/misc.go`
- Create: none
- Delete: none

## Implementation Steps

1. Add `math/rand/v2` import, unless existing Go version/toolchain rejects it; fallback to standard `math/rand` only if needed.
2. Update package comment to include `/wheelofnames`.
3. Add `wheelOfNamesUsage` constant, e.g. `Usage: /wheelofnames <option1>, <option2>, ...`.
4. Add `wheelOfNamesCommand()` returning public `modules.Command`.
5. Add command to `New()` after other public misc commands or near `/ping`.
6. Add `splitWheelOptions(arg string) []string` helper near the command.
7. Handler:
   - return nil for nil message
   - parse args with `chathelper.ArgAfterCommand(update.Message.Text)`
   - reply usage if parsed options length is zero
   - pick `options[rand.N(len(options))]`
   - reply selected option as plain text

## Success Criteria

- [x] `misc.New` registers `wheelofnames` as public command.
- [x] Handler replies usage for missing/empty option list.
- [x] Handler never returns an empty option.
- [x] Handler uses plain text reply, preserving forum topic behavior through `chathelper.Reply`.

## Risk Assessment

Main risk: flaky tests if randomness is asserted too strictly. Mitigate by testing deterministic one-option input and set membership for multi-option input.

Security: no HTML parse mode; user-supplied option text is plain text, so no HTML escaping needed.
