# AGENTS.md

## Project Context

`wheelofnames` is a Node.js 24 JavaScript + JSDoc API service that renders
animated wheel-of-names GIFs with Remotion.

## Development Rules

- Use JavaScript and JSDoc. Do not add TypeScript source files.
- Keep public API routes under `/api`.
- Run `pnpm lint`, `pnpm typecheck`, and `pnpm test` before committing code
  changes.
- Run `pnpm render:smoke` when renderer, Remotion composition, Docker runtime,
  or GIF output behavior changes.
- Run `pnpm api:smoke` when API render behavior, bundle warm-up, or production
  render flow changes.
- Do not commit `.env`, generated GIFs, tokens, secrets, or temporary render
  artifacts.
- Prefer Remotion libraries and CSS transforms over custom SVG path generation.

## Quality Gates

- API validation must reject oversized requests before render work starts.
- Rendering must not rebundle Remotion per request.
- Winner metadata must be returned with GIF responses.
- Keep Docker and README deployment docs in sync with runtime requirements.
