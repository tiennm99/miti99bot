# Command Parameter Conventions Journal

## Context

Command parameter labels had grown organically, especially for comma-separated
arguments. Researched common CLI notation to establish a minimal display syntax
for Telegram's native menu, `/help`, and handler usage text.

## What Changed

- Added the evergreen `docs/command-parameter-conventions.md` reference with
  concise rules for required, optional, variadic, alternative, and structured
  parameters.
- Recorded the user-selected `<option,...>` notation for required
  comma-separated values.
- Updated `/random` and `/wheelofnames` metadata, usage text, and contract tests
  from `<options(comma-separated)>` to `<option,...>`.
- Linked project guidance and README command-discovery documentation to the
  shared convention reference.

## Reflection

The compact notation communicates both one-or-more values and the literal comma
separator without turning display metadata into a schema language. Keeping the
rules in an evergreen document avoids repeating policy in project instructions
and feature docs.

## Decisions

- Parameter strings remain presentation-only; handlers own validation.
- `/random` and `/wheelofnames` parsing and runtime behavior remain unchanged.
- Metadata, usage errors, and tests must use the same exact notation.

## Verification

- Passed: focused command-presentation and misc tests.
- Passed: `go test ./...`
- Passed: `go vet ./...`
- Passed: `go build ./...`
- Passed: `golangci-lint run`

## Next Steps

- Apply the evergreen conventions whenever command parameters change.
