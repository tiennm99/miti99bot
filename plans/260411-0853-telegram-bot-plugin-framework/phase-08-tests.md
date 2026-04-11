# Phase 08 — Tests

## Context Links
- Plan: [plan.md](plan.md)
- Phases 03, 04, 05

## Overview
- **Priority:** P1
- **Status:** pending
- **Description:** vitest unit tests for pure-logic modules — registry, DB prefixing wrapper (incl. `getJSON` / `putJSON`), dispatcher routing, help renderer, command validators, HTML escaper. NO tests that spin up workerd or hit Telegram.

## Key Insights
- Pure-logic testing > integration testing at this stage. Cloudflare's `@cloudflare/vitest-pool-workers` adds complexity and slow starts; skip for v1.
- Mock `env.KV` with an in-memory `Map`-backed fake that implements `get/put/delete/list` per the real shape (including `list_complete` and `keys: [{name}]`).
- For dispatcher tests, build a fake `bot` that records `command()` + `on()` registrations; assert the right handlers were wired.
- For help renderer tests, construct a synthetic registry directly — no need to load real modules.

## Requirements
### Functional
- Tests run via `npm test` (vitest, node env).
- No network. No `fetch`. No Telegram.
- Each test file covers ONE source file.
- Coverage target: registry, db wrapper, dispatcher, help renderer, validators, escaper — these are the logic seams.

### Non-functional
- Test suite runs in < 5s.
- No shared mutable state between tests; each test builds its fixtures.

## Architecture
```
tests/
├── fakes/
│   ├── fake-kv-namespace.js    # Map-backed KVNamespace impl
│   ├── fake-bot.js             # records command() calls
│   └── fake-modules.js         # fixture modules for registry tests
├── db/
│   ├── cf-kv-store.test.js
│   └── create-store.test.js
├── modules/
│   ├── registry.test.js
│   ├── dispatcher.test.js
│   └── validate-command.test.js
├── util/
│   └── escape-html.test.js
└── modules/util/
    └── help-command.test.js
```

## Related Code Files
### Create
- `tests/fakes/fake-kv-namespace.js`
- `tests/fakes/fake-bot.js`
- `tests/fakes/fake-modules.js`
- `tests/db/cf-kv-store.test.js`
- `tests/db/create-store.test.js`
- `tests/modules/registry.test.js`
- `tests/modules/dispatcher.test.js`
- `tests/modules/validate-command.test.js`
- `tests/util/escape-html.test.js`
- `tests/modules/util/help-command.test.js`

### Modify
- `vitest.config.js` — confirm `environment: "node"`, `include: ["tests/**/*.test.js"]`.

### Delete
- none

## Test cases (per file)

### `fake-kv-namespace.js`
- In-memory `Map`. `get(key, {type: "text"})` returns value or null. `put(key, value, opts?)` stores; records `opts` for assertions. `delete` removes. `list({prefix, limit, cursor})` filters by prefix, paginates, returns `{keys:[{name}], list_complete, cursor}`.

### `cf-kv-store.test.js`
- `get/put/delete` round-trip with fake KV.
- `list()` strips to normalized shape `{keys: string[], cursor?, done}`.
- `put` with `expirationTtl` passes through to underlying binding.
- `putJSON` serializes then calls `put`; recoverable via `getJSON`.
- `getJSON` on missing key returns `null`.
- `getJSON` on corrupt JSON (manually insert `"{not json"`) returns `null`, does NOT throw, emits a warning.
- `putJSON(key, undefined)` throws.

### `create-store.test.js`
- `createStore("wordle", env).put("k","v")` results in raw KV key `"wordle:k"`.
- `createStore("wordle", env).list({prefix:"games:"})` calls underlying `list` with `"wordle:games:"` prefix.
- Returned keys have the `"wordle:"` prefix STRIPPED.
- Two stores for different modules cannot read each other's keys.
- `getJSON` / `putJSON` through a prefixed store also land at `<module>:<key>` raw key.
- Invalid module name (contains `:`) throws.

### `validate-command.test.js`
- Valid command passes for each visibility (public / protected / private).
- Command with leading `/` rejected (any visibility).
- Command name > 32 chars rejected.
- Command name with uppercase rejected (`COMMAND_NAME_RE` = `/^[a-z0-9_]{1,32}$/`).
- Missing description rejected (all visibilities — private also requires description for internal debugging).
- Description > 256 chars rejected.
- Invalid visibility rejected.

### `registry.test.js`
- Fixture: fake modules passed via an injected `moduleRegistry` map (avoid `vi.mock` path-resolution flakiness on Windows — phase-04 exposes a loader injection point).
- `MODULES="a,b"` loads both; `buildRegistry` flattens commands into 3 visibility maps + 1 `allCommands` map.
- **Unified namespace conflict:** module A registers `/foo` as public, module B registers `/foo` as private → `buildRegistry` throws with a message naming both modules AND the command.
- Same-visibility conflict (two modules, both public `/foo`) → throws.
- Unknown module name → throws with message.
- Empty `MODULES` → throws.
- `init` called once per module with `{db, env}`; db is a namespaced store (assert key prefix by doing a `put` through the injected db and checking raw KV).
- After `buildRegistry`, `getCurrentRegistry()` returns the same instance; `resetRegistry()` clears it and subsequent `getCurrentRegistry()` throws.

### `dispatcher.test.js`
- Build registry from fake modules (one each: public, protected, private), install on fake bot.
- Assert `bot.command()` called for EVERY entry — including private ones (the whole point: unified routing).
- Assert no other bot wiring (no `bot.on`, no `bot.hears`). Dispatcher is minimal.
- Call count = total commands across all visibilities.

### `help-command.test.js`
- Build a synthetic registry with three modules: A (1 public), B (1 public + 1 protected), C (1 private only).
- Invoke help handler with a fake `ctx` that captures `reply(text, opts)`.
- Assert output:
  - Contains `<b>A</b>` and `<b>B</b>`.
  - Does NOT contain `<b>C</b>` (no visible commands — private is hidden from help).
  - Does NOT contain C's private command name anywhere in output.
  - Protected command has ` (protected)` suffix.
  - `opts.parse_mode === "HTML"`.
  - HTML-escapes a module description containing `<script>`.

### `escape-html.test.js`
- Escapes `&`, `<`, `>`, `"`.
- Leaves safe chars alone.

## Implementation Steps
1. Build fakes first — `fake-kv-namespace.js`, `fake-bot.js`, `fake-modules.js`.
2. Write tests file-by-file, running `npm test` after each.
3. If a test reveals a bug in the source, fix the source (not the test).
4. Final full run, assert all green.

## Todo List
- [ ] Fakes: kv namespace, bot, modules
- [ ] cf-kv-store tests (incl. `getJSON` / `putJSON` happy path + corrupt-JSON swallow)
- [ ] create-store tests (prefix round-trip, isolation, JSON helpers through prefixing)
- [ ] validate-command tests (uniform regex, leading-slash rejection)
- [ ] registry tests (load, unified-namespace conflicts, init injection, reset)
- [ ] dispatcher tests (every visibility registered via `bot.command()`)
- [ ] help renderer tests (grouping, escaping, private hidden, parse_mode)
- [ ] escape-html tests
- [ ] All green via `npm test`

## Success Criteria
- `npm test` passes with ≥ 95% line coverage on the logic seams (registry, db wrapper, dispatcher, help renderer, validators).
- No flaky tests on 5 consecutive runs.
- Unified-namespace conflict detection has dedicated tests covering same-visibility AND cross-visibility collisions.
- Prefix isolation (module A cannot see module B's keys) has a dedicated test.
- `getJSON` corrupt-JSON swallowing has a dedicated test.

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Fake KV diverges from real behavior | Med | Med | Keep fake minimal, aligned to real shape from [KV basics report](../reports/researcher-260411-0853-cloudflare-kv-basics.md) |
| `vi.mock` path resolution differs on Windows | Med | Low | Use injected-dependency pattern instead — pass `moduleRegistry` map as a fn param in tests |
| Tests couple to grammY internals | Low | Med | Use fake bot; never import grammY in tests |
| Hidden state in registry module-scope leaks between tests | Med | Med | Export a `resetRegistry()` test helper; call in `beforeEach` |

## Security Considerations
- Tests must never read real `.dev.vars` or hit real KV. Keep everything in-memory.

## Next Steps
- Phase 09 documents running the test suite as part of the deploy preflight.
