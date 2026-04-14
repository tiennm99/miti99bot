# Code Standards

## Language & Runtime

- **JavaScript** (ES modules, `"type": "module"` in package.json)
- **No TypeScript** — JSDoc typedefs for type contracts (see `kv-store-interface.js`, `registry.js`)
- **Cloudflare Workers runtime** — Web APIs only, no Node.js built-ins, no `nodejs_compat`
- **grammY** for Telegram bot framework

## Formatting (Biome)

Enforced by `npm run lint` / `npm run format`:

- 2-space indent
- Double quotes
- Semicolons: always
- Trailing commas: all
- Line width: 100 characters
- Imports: auto-sorted by Biome

Run `npm run format` before committing.

## File Organization

- **Max 200 lines per code file.** Split into focused submodules when approaching the limit.
- Module code lives in `src/modules/<name>/` — one folder per module.
- Shared utilities in `src/util/`.
- DB layer in `src/db/`.
- Tests mirror source structure: `tests/modules/<name>/`, `tests/db/`, `tests/util/`.

## Naming Conventions

- **Files:** lowercase, hyphens for multi-word (`stats-handler.js`, `fake-kv-namespace.js`)
- **Directories:** lowercase, single word preferred (`trading/`, `util/`)
- **Functions/variables:** camelCase
- **Constants:** UPPER_SNAKE_CASE for frozen config objects (e.g. `CURRENCIES`)
- **Command names:** lowercase + digits + underscore, 1-32 chars, no leading slash

## Module Conventions

Every module default export must have:

```js
export default {
  name: "modname",     // === folder name === import map key
  commands: [...],     // validated at load time
  init: async ({ db, env }) => { ... },  // optional
};
```

- Store module-level `db` reference in a closure variable, set during `init`
- Never access `env.KV` directly — always use the prefixed `db` from `init`
- Handlers receive grammY `ctx` — use `ctx.match` for command arguments, `ctx.from.id` for user identity
- Reply with `ctx.reply(text)` — plain text or Telegram HTML

## Error Handling

- **Load-time failures** (bad module, command conflicts, missing env): throw immediately — fail loud at deploy, not at runtime.
- **Handler-level errors** (API failures, bad user input): catch and reply with user-friendly message. Never crash the handler — grammY logs unhandled rejections but the user sees nothing.
- **KV failures**: best-effort writes (wrap in try/catch), guard reads with `?.` and null coalescing.
- `getJSON` swallows corrupt JSON and returns null — modules must handle null gracefully.

## Testing

- **Framework:** Vitest
- **Style:** Pure-logic unit tests. No workerd, no Telegram integration, no network calls.
- **Fakes:** `tests/fakes/` provides `fake-kv-namespace.js`, `fake-bot.js`, `fake-modules.js`. Inject via parameters, not `vi.mock`.
- **External APIs:** Stub `global.fetch` with `vi.fn()` returning canned responses.
- **Coverage:** `npx vitest run --coverage` (v8 provider, text + HTML output).

## Commit Messages

Conventional commits:

```
feat: add paper trading module
fix: handle null price in sell handler
docs: update architecture for trading module
refactor: extract stats handler to separate file
test: add portfolio edge case tests
```

## Security

- Secrets live in Cloudflare Workers secrets (runtime) and `.env.deploy` (local, gitignored). Never commit secrets.
- `.dev.vars` is gitignored — local dev only.
- grammY validates webhook secret on every update. No manual header parsing.
- Module KV prefixing is a code-review boundary, not a cryptographic one.
- Private commands are discoverability control, not access control.
- HTML output in `/help` uses `escapeHtml` to prevent injection.
