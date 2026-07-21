# Cross-Platform Go Workflow Journal

## Context

Removed Unix-oriented Make targets so Windows, macOS, and Linux contributors
can use the same standard Go workflow with platform-specific environment syntax.

## What Changed

- Deleted `Makefile` and replaced its development shortcuts with documented
  `go test`, `go vet`, `go build`, `go run`, and direct Docker commands.
- Documented Telegram webhook inspection and cleanup with PowerShell
  `Invoke-RestMethod` and POSIX `curl` examples.
- Deleted `telegram-commands.json`; the runtime module registry already builds
  and registers the command menu on every startup, so the JSON duplicated the
  authoritative Go definitions.
- Replaced Makefile linker flags with Go's embedded VCS build metadata and kept
  the existing seven-character commit SHA behavior for deploy notifications.
- Updated CI and MongoDB test guidance to point contributors to the portable
  README workflow.

## Reflection

Removing the task wrapper makes the repository less convenient for habitual
`make` users, but avoids maintaining shell-specific orchestration and duplicate
command-menu data. Standard Go, Docker, and HTTP tools keep each operation
explicit and work across supported development platforms.

## Decisions

- The Go module registry is the single source of truth for Telegram commands.
- Local binaries obtain their short revision from Go build information; the
  deployment-provided `SOURCE_COMMIT` remains preferred at runtime.
- Platform differences are documented only where shell syntax or HTTP tooling
  differs.

## Verification

- Passed: `go test ./...`
- Passed: `go vet ./...`
- Passed: `go build ./...`
- Unavailable: `golangci-lint run` because the binary is not installed.

## Next Steps

- Run the lint gate when `golangci-lint` is available.
- Keep command registration tests aligned with future public command changes.
