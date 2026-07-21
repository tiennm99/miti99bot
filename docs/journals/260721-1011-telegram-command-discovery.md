# Telegram Command Discovery Journal

## Context

Telegram's native command menu and `/help` exposed only short descriptions,
leaving users to discover parameters and examples through failed invocations or
source documentation.

## What Changed

- Extended the shared command registration with `Parameters` and `Example`
  metadata and presentation helpers used by both discovery surfaces.
- Added metadata for all 40 public commands, including the exact stats grammar:
  `[users | user <username> | cmd <command_name>]`.
- Normalized placeholders to lowercase descriptive names, including meaningful
  units or currencies; `[...]` marks optional input, `...` remaining free text,
  and parentheses structured input.
- Normalized `/wheelofnames` to `<options(comma-separated)>` across metadata,
  usage text, and tests; its example remains
  `/wheelofnames pizza, sushi, pho`.
- Finalized dividend placeholders as `<vnd_per_share> <ticker>`,
  `<ratio(owned:new)> <ticker>`, and
  `<vnd_per_share> <ratio(owned:new)> <ticker>` for cash, share, and combined
  commands. Examples and parsing remain unchanged.
- Native menu descriptions show only the summary for no-parameter commands.
  Parameterized commands append an explicit example with the short `Eg:` label
  on the same plain-text line.
- `/help` renders each invocation and summary together. Parameterized commands
  append `Eg: <code>invocation</code>` on the same line, with only the copyable
  invocation inside Telegram's HTML `<code>` formatting. No-parameter commands
  omit the label and example. Dynamic command fields are HTML-escaped.
- Registration validation rejects multiline metadata, examples for another
  command, public commands that provide only one of parameters or example, and
  public descriptions over Telegram's 256-character limit.
- Updated user and deployment documentation for the shared registry behavior.

## Reflection

Keeping syntax and examples beside each handler registration prevents the
native menu, `/help`, and implementation from drifting independently. The
native surface stays compact, while `/help` uses the richer layout Telegram
supports without sacrificing safe HTML rendering.

## Decisions

- Existing command names, handlers, parsers, and persisted data remain
  unchanged; normalization is presentation-only.
- No command was added, renamed, or deleted, so no stats migration is needed.
- Public commands with parameters require an explicit example; commands
  without parameters omit it.
- The complete `/help` output remains within Telegram's 4,096-character limit.

## Verification

- Passed: command presentation, validation, menu, and `/help` tests for all 40
  public commands, including inline `<code>` rendering, no-parameter omission,
  and both parameter/example validation branches.
- Passed: `go test ./...`, including real MongoDB Testcontainers suites.
- Passed: `go vet ./...`
- Passed: `go build ./...`
- Passed: `golangci-lint run`

## Next Steps

- Require parameter and example metadata updates alongside future public
  command contract changes.
