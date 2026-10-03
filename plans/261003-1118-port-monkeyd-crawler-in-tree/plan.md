---
status: completed
mode: port
---

# Port monkeyd-crawler into the monkeyd module

## Outcome

The bot builds without the `third_party/monkeyd-crawler` submodule. The crawler,
PDF renderer, and export flow live in-tree under `internal/modules/monkeyd/`,
trimmed to what the bot uses. Coolify deploys succeed again.

Why now: `tiennm99/monkeyd-crawler` was merged into `tiennm99/mttools` and the
old repo is gone, so every recursive clone fails (Coolify deployment
`6klach0etmr5ymnfrvbkq77a`, "Repository not found").

## Source manifest

- Source: `tiennm99/mttools`, path `monkeyd-crawler/`, commit `03b1400`
- It matches the pinned submodule commit `d88f2a4` except for import paths
- License: Apache-2.0, same author as this repo (also Apache-2.0); bundled
  DejaVu Sans keeps its `NOTICE.md`

## Source anatomy

| Package | Role | Bot uses |
|---|---|---|
| `monkeyd` | HTTP client (rate limit, retries, size cap), disk cache, novel and chapter parsing, CSS word-class decoding, worker pool | `Crawler.NovelInfo` and `NewClient` for `/monkeyd_tags`, plus the export path |
| `pdfout` | fpdf layout, page presets, font discovery plus bundled DejaVu | the export path, plus `Presets` in a test |
| `export` | orchestration: defaults, validation, crawl, render | `Export`, `Request`, `Result`, `DefaultFontSize`, `DefaultDelay` |
| `cmd/monkeyd-crawler` | CLI | nothing |

## Dependency matrix

| Source | Local | Status |
|---|---|---|
| `monkeyd` package | `internal/modules/monkeyd/crawler` | NEW (port as-is) |
| `pdfout` package | `internal/modules/monkeyd/pdf` | NEW (drop system font discovery) |
| `export` package | `internal/modules/monkeyd/export` | NEW (trim the request surface) |
| CLI | none | DROPPED |
| `go-pdf/fpdf`, `x/net`, `x/sync`, `x/image` | `go.mod` | EXISTS (indirect deps become direct) |
| submodule, `replace`, Dockerfile COPY, CI `submodules: true` | none | REMOVED |

## Decision matrix

| Decision | Source's way | Our way | Recommendation |
|---|---|---|---|
| Distribution | Go module in mttools | in-tree packages | In-tree, as requested. No cross-repo pin to drift, and Coolify needs no recursive clone |
| Font | system font discovery, then bundled | bundled only | Bundled only. Production has no system fonts, so its output is unchanged, and dev machines render the same PDF |
| Page presets | phone, a5, a4 | phone only | Fixed phone page; the bot exposes no page option |
| Export knobs | page, font file, spacing, margin, workers, retries, delay, limit, out path, no-cache, no-delay | URL, out dir, font size, cache dir, log | Keep only what the bot sets. The rest become constants with the source's defaults |
| CLI | `cmd/monkeyd-crawler` | none | Drop; mttools keeps the CLI |

Risk score: 3 of 10. The logic is Go to Go and identical to what runs today.
The main risk is a trim that drops behavior, and the ported tests cover that.

## Phases

1. Port the packages. Copy `monkeyd`, `pdfout`, and `export` with their tests
   into `internal/modules/monkeyd/{crawler,pdf,export}`. Rewrite imports and
   apply the trims above. Remove tests that only cover dropped features.
2. Rewire the module. Point `export_job.go`, `monkeyd.go`, `tags_command.go`,
   and `handlers_test.go` at the new packages.
3. Remove the submodule. Delete `third_party/monkeyd-crawler`, `.gitmodules`,
   and the `replace`/`require` lines; run `go mod tidy`. Drop the Dockerfile
   COPY and CI `submodules: true`.
4. Update docs: README (layout, run locally, clone instructions), AGENTS.md,
   and `docs/deploy-coolify-selfhosted.md` (recursive clone note).

## Acceptance criteria

- `go build ./...`, `go vet ./...`, `go test ./...`, and `golangci-lint run`
  pass with no submodule present
- `docker compose build` passes
- A live export of a real novel URL produces a PDF with Vietnamese diacritics
- The Coolify deployment of the pushed commit finishes

## Rollback

Revert the commits. The old submodule URL no longer resolves, so a rollback
needs `.gitmodules` pointed at `tiennm99/mttools` with the path adjusted, or a
module dependency on `github.com/tiennm99/mttools/monkeyd-crawler`.
