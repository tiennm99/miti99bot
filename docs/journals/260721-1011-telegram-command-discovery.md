# Telegram Command Discovery Journal

> Historical note: the later command-parameter convention simplifies
> `<options(comma-separated)>` to `<option,...>`, keeping the literal delimiter
> visible without prose inside the placeholder. See
> `docs/command-parameter-conventions.md`.

## Context

Telegram's native command menu and `/help` exposed only short descriptions,
leaving users to discover parameters through failed invocations or source
documentation.

## What Changed

- Extended the shared command registration with `Parameters` metadata and
  presentation helpers used by both discovery surfaces.
- Added metadata for all 40 public commands, including the exact stats grammar:
  `[users | user <username> | cmd <command_name>]`.
- Normalized placeholders to lowercase descriptive names, including meaningful
  units or currencies; `[...]` marks optional input, `...` remaining free text,
  and parentheses structured input.
- Normalized `/wheelofnames` to `<options(comma-separated)>` across metadata,
  usage text, and tests.
- Finalized dividend placeholders as `<vnd_per_share> <ticker>`,
  `<ratio(owned:new)> <ticker>`, and
  `<vnd_per_share> <ratio(owned:new)> <ticker>` for cash, share, and combined
  commands. Parsing remains unchanged.
- Native menu descriptions show parameters followed by the summary, while
  `/help` renders the complete invocation followed by the summary. Neither
  discovery surface includes example invocations. Dynamic fields are
  HTML-escaped in `/help`.
- Registration validation rejects multiline metadata and public descriptions
  over Telegram's 256-character limit.
- Updated user and deployment documentation for the shared registry behavior.

## Reflection

Keeping syntax beside each handler registration prevents the native menu,
`/help`, and implementation from drifting independently. Both discovery
surfaces stay compact without sacrificing safe HTML rendering in `/help`.

## Decisions

- Existing command names, handlers, parsers, and persisted data remain
  unchanged; normalization is presentation-only.
- No command was added, renamed, or deleted, so no stats migration is needed.
- Discovery surfaces intentionally omit example invocations; handler usage
  errors may still include focused examples.
- The complete `/help` output remains within Telegram's 4,096-character limit.

## Verification

- Passed: command presentation, validation, menu, and `/help` tests for all 40
  public commands.
- Passed: `go test ./...`, including real MongoDB Testcontainers suites.
- Passed: `go vet ./...`
- Passed: `go build ./...`
- Passed: `golangci-lint run`

## Next Steps

- Require parameter metadata updates alongside future public command contract
  changes.
