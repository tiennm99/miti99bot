# Direct Local GIF Rendering

## Context

Custom wheel GIFs previously required starting the API server, while the smoke renderer only supported predefined fixtures. The goal was a direct CLI that accepts user options and renders through the existing Remotion pipeline.

## What Happened

- Added `pnpm render:local` for direct GIF rendering with no hosted or local HTTP server.
- Reused the shared wheel request schema so CLI and API validation stay aligned.
- Supported repeated `--option` flags, including duplicate Unicode text such as Vietnamese labels.
- Verified the produced file by its `GIF89a` signature, not only by command success.
- Passed all 45 tests plus lint, typecheck, and render smoke checks.
- Updated local usage docs and clarified that `wheel.gif` and `fixtures/*.gif` are ignored, while custom output paths may require their own ignore rule.

## Reflection

Routing the CLI through the production renderer kept behavior consistent and avoided a second rendering path. Repeated flags made Unicode and duplicate values easier to pass than inline JSON. The main documentation nuance was avoiding a broad GIF ignore rule that could conceal intentionally tracked assets.

## Decisions

| Decision | Rationale | Impact |
|---|---|---|
| Render directly through `renderWheelGif` | Reuse the tested production pipeline without server startup | Local generation is simpler and behavior matches API output |
| Validate with the shared schema | Prevent CLI/API rules from drifting | Limits, defaults, and errors remain consistent |
| Accept repeated `--option` flags | Preserve duplicates and shell-safe Unicode input | Custom multilingual wheels are straightforward to invoke |
| Keep output ignores narrow | Avoid accidentally ignoring legitimate GIF assets | Documented paths are safe; custom paths need deliberate ignore rules |

## Next

- Keep CLI options synchronized with future request-schema changes.
- Add new documented output locations to `.gitignore` only when they become standard project artifacts.
