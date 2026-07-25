# API / DX Review — Integrator and Operator Experience

Date: 2026-07-25
Scope: API contract, error handling, config, scripts, docs. Visual design excluded (covered by other reviewers).
Method: source read + empirical probes via `app.inject`, `loadConfig`, and real CLI runs. All status codes and bodies below are captured output, not inferred.

---

## 1. Verdict

The happy path is solid: validation runs before the semaphore, the bundle is warm-started before `listen`, the semaphore releases in `finally`, temp dirs clean up in `finally`, the token compare is constant-time, and no stack traces leak to callers. All three quality gates pass. The defects are concentrated in the *unhappy* paths an integrator actually hits: malformed JSON, an empty body, and an oversized body all return `500 {"error":"internal_error"}` because the blanket error handler discards Fastify's own `statusCode`, so a bot will retry a permanently-broken request forever. Every `pnpm render:local -- ...` invocation in the README is broken on the installed pnpm — the literal `--` reaches `parseArgs` and the command dies before rendering, and CI never runs it. The wire contract's accepted ranges (`durationMs` 3000-10000, `fps` 12|15|20, `size` 384|480|512, option count/length limits) exist only in the zod schema and are documented in no README, doc, or error-code table, and `docs/deployment.md:47` makes a timeout claim the code does not implement.

---

## 2. Quality gate results

| Gate | Command | Result | Output |
|---|---|---|---|
| Lint | `pnpm lint` | **PASS** | `eslint .` — clean, exit 0 |
| Typecheck | `pnpm typecheck` | **PASS** | `tsc --noEmit` — clean, exit 0 |
| Test | `pnpm test` | **PASS** | `Test Files 9 passed (9) / Tests 58 passed (58)`, 2.94s |
| API smoke | `pnpm api:smoke` | **PASS** | `api-smoke: 441166 bytes`, render 1461ms, 4.1s wall (warm `.tmp/remotion-bundle` on disk) |
| CLI help | `pnpm render:local -- --help` | **FAIL** | `Unable to render GIF: Unexpected argument '--help'. This command does not take positional arguments`, exit 1 |
| CLI help | `node scripts/render-local.js --help` | PASS | usage printed, exit 0 |

Environment: pnpm 11.1.1, Node v24.14.0. Chromium present (`node_modules/.remotion/chrome-headless-shell`), so no gate was skipped for a missing browser.

Note: CI (`.github/workflows/ci.yml`) and the Dockerfile pin pnpm 11.9.0; `package.json` has no `packageManager` field, so local runs float. CI runs `render:smoke` and `api:smoke` but never `render:local`, which is why the broken CLI path ships green.

---

## 3. Findings

### HIGH-1 — Blanket error handler turns 400/413/415 into 500

`src/server.js:33-36`

```js
app.setErrorHandler(async (error, _request, reply) => {
  app.log.error(error);
  return reply.code(500).send({error: 'internal_error'});
});
```

Fastify's content-type-parser errors carry a correct `statusCode`; the handler throws it away. Verified directly — the error object is intercepted and its real code printed:

| Request | Fastify error | Real statusCode | Caller receives |
|---|---|---|---|
| `POST /api/gif` with no body | `FST_ERR_CTP_EMPTY_JSON_BODY` | 400 | `500 {"error":"internal_error"}` |
| Body `{"options":[` | `FST_ERR_CTP_INVALID_JSON_BODY` | 400 | `500 {"error":"internal_error"}` |
| 40 KB body (limit 32 KB) | `FST_ERR_CTP_BODY_TOO_LARGE` | 413 | `500 {"error":"internal_error"}` |
| `content-type: application/xml` | `FST_ERR_CTP_INVALID_MEDIA_TYPE` | 415 | `500 {"error":"internal_error"}` |

Scenario: the bot serializes a payload with a trailing comma or an oversized option list. It gets a 5xx, which by every retry convention means "server fault, retry" — so it retries a request that can never succeed, and each attempt writes a full stack trace at `level:50` in the operator's logs. The client-visible symptom is indistinguishable from a Chromium crash.

Fix (`src/server.js:33`):

```js
app.setErrorHandler(async (error, request, reply) => {
  const status = error.statusCode ?? 500;
  if (status >= 500) {
    request.log.error(error);
    return reply.code(500).send({error: 'internal_error'});
  }
  request.log.warn({err: error}, 'request rejected');
  return reply.code(status).send({error: error.code ?? 'bad_request', message: error.message});
});
```

This also fixes HIGH-4's logging problem (`request.log` carries `reqId`; `app.log` does not — see MEDIUM-4).

---

### HIGH-2 — Every documented `pnpm render:local -- ...` invocation fails

`README.md:70-77` (PowerShell), `README.md:81-88` (bash), `README.md:91` (`--help`), and the usage string itself at `scripts/render-local.js:7`.

pnpm 11 passes the literal `--` through to the script. Captured:

```
$ pnpm render:local -- --output .tmp/probe.gif --option "alpha" --option "beta" --winner 1
$ node scripts/render-local.js "--" "--output" ".tmp/probe.gif" "--option" "alpha" ...
Unable to render GIF: Unexpected argument '--output'. This command does not take positional arguments
```

`node:util.parseArgs` treats `--` as the options terminator, so everything after it becomes a positional, and `allowPositionals: false` (`scripts/render-local-args.js:41`) rejects it. Both the PowerShell and the bash variants fail identically — this is not a shell-quoting difference. The form that works is without `--`:

```
$ pnpm render:local --option alpha --option beta --winner 1
```

Scenario: a new contributor follows the README's primary CLI example verbatim and the tool refuses to run, printing a usage block that repeats the same broken syntax.

Fix: drop `--` from `README.md:71`, `README.md:82`, `README.md:91`, and `scripts/render-local.js:7`. Optionally add `pnpm render:local --help` to CI so the path is exercised.

---

### HIGH-3 — Accepted ranges and defaults are documented nowhere

`README.md:14-24` and `docs/miti99bot-integration.md:18-28` show one example payload with no ranges. The real constraints live only in `src/schemas/wheel-request.js:26-53`:

| Field | Required | Constraint | Default |
|---|---|---|---|
| `options` | yes | array of strings, trimmed, each 1..`MAX_OPTION_CHARS` (40) after trim, 2..`MAX_OPTIONS` (32) items | — |
| `winnerIndex` | no | integer, `0 <= n < options.length` | random (`crypto.randomInt`) |
| `durationMs` | no | integer 3000..10000 | 6500 |
| `holdMs` | no | integer 500..2500 | 1200 |
| `fps` | no | exactly 12, 15, or 20 | 15 |
| `size` | no | exactly 384, 480, or 512 | 512 |
| `theme` | no | `classic` \| `festival` \| `mono` | `classic` |
| unknown keys | — | rejected (`.strict()`) | — |
| body size | — | 32 KB hard cap (`src/server.js:29`) | — |

Two of these are actively surprising and undocumented: `.strict()` means a typo'd key is a hard 400 (`{"colour":"red"}` -> `unrecognized_keys`), and `size`/`fps` are literal unions, so `size: 500` is rejected rather than snapped to the nearest supported value. `MAX_OPTIONS`/`MAX_OPTION_CHARS` are per-deployment, so a hardcoded client limit can disagree with the server's.

Fix: replace the bare example in `README.md:14-24` with the table above; link to it from `docs/miti99bot-integration.md`.

---

### HIGH-4 — No documented error-code contract for the consuming bot

`docs/miti99bot-integration.md:1-39` documents only the 200 response. Step 6 ("If the service fails, bot falls back") gives no way to distinguish retryable from terminal. Captured actual behavior:

| Condition | Status | Body | Retryable? |
|---|---|---|---|
| Missing / wrong / non-`Bearer` token | 401 | `{"error":"unauthorized"}` | no |
| Schema violation | 400 | `{"error":"invalid_request","issues":[...]}` | no |
| Malformed/empty/oversized body | **500** | `{"error":"internal_error"}` | **misleading — see HIGH-1** |
| Concurrency saturated | 429 | `{"error":"too_many_renders"}`, **no `Retry-After`** | yes |
| Render exceeded timeout | 504 | `{"error":"render_timeout"}` | maybe |
| Render crash | 500 | `{"error":"internal_error"}` | maybe |
| `GET /api/gif` | 404 | `{"message":"Route GET:/api/gif not found","error":"Not Found","statusCode":404}` | no |

Two things to note. The 404 body uses Fastify's default `{message, error, statusCode}` shape, inconsistent with the `{error: "snake_case"}` convention everywhere else — a client parsing `body.error` gets `"Not Found"` instead of a machine token. And the 401 is identical for "no header", "wrong token", and "token sent without the `Bearer ` prefix"; the last is the single most common integration mistake and the response gives no hint.

Fix: add this table to `docs/miti99bot-integration.md`. Set `app.setNotFoundHandler` to the `{error: 'not_found'}` shape for consistency.

---

### HIGH-5 — 429 on saturation with no `Retry-After` and no queue

`src/routes/gif.js:71-73`

```js
if (!deps.semaphore.tryAcquire()) {
  return reply.code(429).send({error: 'too_many_renders'});
}
```

With the recommended `MAX_CONCURRENT_RENDERS=1` (`README.md:118-119`, `.env.example:4`, `compose.yml:9`) and a measured render of ~1.5s at 384px/12fps — and `docs/render-benchmarks.md:11` recording 13917ms for the same fixture in Docker — the *second* concurrent chat user gets an instant 429. This is the single most likely production failure, and the response carries no `Retry-After`, no indication of how long a render takes, and no queue. A naive bot retry loop will hot-spin.

Fix: one header plus a doc line.

```js
return reply.code(429).header('Retry-After', '5').send({error: 'too_many_renders'});
```

Document in `docs/miti99bot-integration.md` that the bot must serialize its calls or back off on 429.

---

### MEDIUM-1 — `RENDER_TIMEOUT_MS` is not a total timeout, contrary to the docs

`docs/deployment.md:47` states: "`RENDER_TIMEOUT_MS` is a total render timeout." The implementation does not guarantee that.

In `src/render/render-gif.js:34-61`, the `cancelSignal` from `makeCancelSignal()` is passed **only** to `renderMedia` (line 52). It is not passed to `selectComposition` (lines 44-49) — and cannot be: `selectComposition` does not accept `cancelSignal` in this version (verified against `node_modules/@remotion/renderer/dist/select-composition.d.ts`, which exposes `timeoutInMilliseconds` but no `cancelSignal`). `getRemotionServeUrl()` (line 43) has no timeout at all.

So when the timer fires during browser launch or composition selection, `cancel()` fires into a signal nobody is listening to and the call runs on under Remotion's own per-operation timeout. If browser launch wedges (OOM-killed Chromium, exhausted file descriptors), the promise may never settle: the `finally` never runs, the semaphore slot is never released, and every subsequent request gets 429 forever while `/api/healthz` still returns `{"ok":true}`. Restart is the only recovery, and nothing signals that a restart is needed.

Fix — bound the whole operation, not just `renderMedia`:

```js
const deadline = new Promise((_resolve, reject) => {
  timeoutId = setTimeout(() => {
    timedOut = true;
    cancel();
    reject(new RenderTimeoutError(options.timeoutInMilliseconds));
  }, options.timeoutInMilliseconds);
});
// then: await Promise.race([deadline, renderPipeline()])
```

Either implement the guarantee or correct `docs/deployment.md:47` to say it bounds the render phase only.

---

### MEDIUM-2 — Client disconnect does not cancel the render

`src/routes/gif.js:75-100`. Nothing wires `request.raw` aborts to the existing `cancelSignal`. If the bot's HTTP client times out at 10s and retries, the abandoned render keeps the only slot for its full duration; the retry gets 429. Under `MAX_CONCURRENT_RENDERS=1` a client with a timeout shorter than the render time can deadlock itself indefinitely.

Fix: plumb an abort into `renderWheelGif`, or at minimum document in `docs/miti99bot-integration.md` that the client timeout must exceed `RENDER_TIMEOUT_MS`.

---

### MEDIUM-3 — Config errors are silently swallowed at boot

`src/config.js:20-27`. `parsePositiveInt` returns the fallback for anything unparseable, with no log. Captured from `loadConfig`:

| Env | Result | Problem |
|---|---|---|
| `MAX_CONCURRENT_RENDERS=abc` | `1` | typo silently ignored |
| `MAX_CONCURRENT_RENDERS=0` | `1` | intent (disable) silently inverted |
| `RENDER_TIMEOUT_MS="ten seconds"` | `15000` | silently ignored |
| `MAX_OPTION_CHARS=0` | `40` | silently ignored |
| `PORT=99999` | `99999` | accepted, then `listen` fails with a raw `ERR_SOCKET_BAD_PORT` |
| `MAX_OPTIONS=1` | `1` | **every request 400s forever** |
| `REQUIRE_API_TOKEN=TRUE` | `requiresApiToken: false` | case-sensitive `=== 'true'` (`src/config.js:35`); operator believes auth is enforced when it is not |

`MAX_OPTIONS=1` is the worst: the schema becomes `.min(2).max(1)`, unsatisfiable. Verified — every payload returns `{"code":"too_big","maximum":1,...}`. The service boots healthy and 400s 100% of traffic.

The `REQUIRE_API_TOKEN=TRUE` case is a security-relevant silent failure: the operator sets what they believe is the auth switch, boot succeeds, and the endpoint is wide open.

Fix in `loadConfig`: throw on a set-but-unparseable value instead of falling back, and reject `maxOptions < 2`. Compare `REQUIRE_API_TOKEN` case-insensitively.

---

### MEDIUM-4 — Error logs have no `reqId` and no request context

`src/server.js:34` uses `app.log.error(error)`. Captured output for a simulated Chromium crash:

```
{"level":50,"time":...,"err":{"type":"Error","message":"chromium crashed: page closed unexpectedly","stack":"..."},"msg":"chromium crashed..."}
```

No `reqId` field — compare the surrounding `incoming request` / `request completed` lines, which both carry `"reqId":"req-1"`. With `MAX_CONCURRENT_RENDERS>1` an operator cannot correlate a stack trace with the request that produced it. There is also no render context anywhere: option count, `size`, `fps`, `durationMs` are never logged, so "renders started failing" cannot be narrowed to "only 512px/20fps fails".

Fix: `request.log.error(error)` (included in the HIGH-1 patch), and log a one-line success/failure record in the route with `{optionCount, size, fps, durationMs, byteLength}`.

---

### MEDIUM-5 — `/api/healthz` proves only that the event loop is alive

`src/routes/health.js:4-9` returns a static object. It cannot detect a wedged semaphore (MEDIUM-1), a failed bundle, or a missing Chromium. `compose.yml:16` wires the container healthcheck to it, so Docker will report a permanently-429ing container as healthy.

Mitigating fact worth noting: `src/server.js:24-26` awaits the bundle *before* `Fastify()` and `listen`, so the port does not open until the bundle exists. Readiness is therefore implicitly correct at startup — the gap is steady-state liveness only.

Fix (small): report semaphore state so saturation is observable.

```js
app.get('/api/healthz', async () => ({ok: true, service: 'wheelofnames', activeRenders: semaphore.active, maxConcurrentRenders: config.maxConcurrentRenders}));
```

Requires exposing a read-only `active` from `createRenderSemaphore`.

---

### MEDIUM-6 — Auth silently disabled when `API_TOKEN` is unset outside production

`src/routes/gif.js:47` gates on `deps.config.apiToken` being truthy. If it is unset and `NODE_ENV !== 'production'`, the boot check at `src/config.js:37-39` does not fire and `/api/gif` is fully open — no warning logged, no mention in any doc. A deploy target that does not set `NODE_ENV=production` by default (Cloud Run and Railway among the recommended targets at `docs/deployment.md:8-15`) yields an unauthenticated GIF-rendering endpoint: unmetered CPU for anyone who finds it.

Note `config.requiresApiToken` is computed and returned but never read by the route — the only enforcement is the boot check.

Fix: log a startup warning when `apiToken` is undefined, and state the "no token = no auth" rule in `docs/deployment.md`.

---

### MEDIUM-7 — `compose.yml` defaults the production token to `change-me`

`compose.yml:5,8`: `NODE_ENV` defaults to `production` and `API_TOKEN` defaults to `change-me`. `docker compose up` with no `.env` produces a production deployment whose bearer token is published in this repo. The boot guard at `src/config.js:37` is satisfied and never fires.

Fix: `API_TOKEN: "${API_TOKEN:?API_TOKEN must be set}"` so compose refuses to start instead of defaulting. Same for the `docker run -e API_TOKEN=change-me` example at `README.md:115` — mark it explicitly as a placeholder.

---

### MEDIUM-8 — `.env.example` is orphaned: nothing loads it, nothing documents it

`.env.example` exists with 8 variables. There is no `dotenv` dependency, no `--env-file` flag in any `package.json` script, and `grep` finds zero references to `.env.example` in `README.md`, `AGENTS.md`, or `docs/`. A developer who does the conventional `cp .env.example .env` and runs `pnpm dev` gets no config applied and no error — the defaults silently win.

Also, `.env.example:3` sets `NODE_ENV=production`, which is wrong for the local-dev file it appears to be.

Fix: either add `--env-file=.env` to the `dev` script (Node 24 supports it natively; no dependency needed) and document the copy step in README, or delete `.env.example` and keep `docs/deployment.md`'s shell-variable model as the single story.

---

### MEDIUM-9 — No graceful shutdown

No `SIGTERM`/`SIGINT` handler exists in `src/` (grep confirms the only `app.close()` is in `scripts/api-smoke.js:49`). `Dockerfile:48` uses `CMD ["pnpm", "start"]`, so PID 1 is pnpm and signal forwarding to Node is unreliable. On `docker stop` / a rolling deploy, an in-flight render is killed mid-request and the client sees a connection reset rather than a clean 5xx; the container takes the full 10s kill grace period.

Fix: `CMD ["node", "src/server.js"]`, plus a handler calling `app.close()`.

---

### MEDIUM-10 — A failed bundle is cached as a permanently rejected promise

`src/render/remotion-bundle.js:11-21`. `bundlePromise` is assigned once; if `bundle()` rejects, every later call returns the same rejected promise. On the server path the boot warm-up makes this mostly moot (the process dies). It bites `scripts/render-fixtures.js` and any caller running with `warmRemotionBundle: false`: one transient failure poisons the module for the process lifetime with no retry.

Fix: clear the cache on rejection.

```js
bundlePromise = bundle({...}).catch((error) => { bundlePromise = undefined; throw error; });
```

---

### MEDIUM-11 — CLI ignores the deployment's configured limits

`scripts/render-local-args.js:5-8` hardcodes `{maxOptions: 32, maxOptionChars: 40}` instead of reading `loadConfig()`. An operator who raises `MAX_OPTIONS` cannot reproduce a production payload locally — the CLI rejects it with a limit the server would accept.

Fix: `const limits = loadConfig();` — `AppConfig` already carries both fields.

---

### MEDIUM-12 — Test coverage gaps on exactly the failure modes above

58 tests pass, but the untested paths are the risky ones.

Not covered at all: `src/render/render-gif.js` (timeout classification, temp-dir cleanup on throw), `src/render/remotion-bundle.js`, `src/lib/tmp-files.js`, `src/routes/health.js`.

Covered module, uncovered behavior in `test/gif-route.test.js`:
- malformed / empty / oversized body — would have caught HIGH-1
- `X-Wheel-Winner` percent-encoding: line 39 asserts `'beta'`, which is ASCII, so `encodeURIComponent` is never exercised. The behavior does work (probe: `Chiều nay uống CraneTea` -> `Chi%E1%BB%81u%20nay%20u%E1%BB%91ng%20CraneTea`), but the header contract README advertises is untested.
- 401 discrimination between missing header, wrong token, and missing `Bearer ` prefix
- absence/presence of `Retry-After` on 429
- 404 body shape

Highest-value additions: an oversized-body test and a non-ASCII winner-header test. Both are three lines each in the existing file.

---

### LOW

- **`scripts/render-local.js:37`** — a validation failure prints the raw `ZodError` JSON. Actual output for a missing `--option`: a 12-line pretty-printed array. Format to one line per issue (`path.join('.')` + `message`).
- **`README.md:10`** — `Accept: image/gif` is shown in the request example but the server never reads `Accept`. Decorative; either honor it or drop it.
- **`scripts/render-local-args.js:57-60`** — `--timeout` is validated before the `--help` branch (line 62), so `--help --timeout 100` errors instead of showing help. Move the help check first.
- **`scripts/render-local.js:17`** — `--theme` is the only flag whose usage line omits its default (`classic`).
- **`docs/render-benchmarks.md:13-14`** — two rows still `TBD` under a heading that says "Fill this after running `pnpm render:fixtures`". Either fill or delete the rows.
- **Duplicate options** are accepted (probe: `["dup","dup"]` -> 200). This is intentional — `README.md:73-74` deliberately repeats an option — but it means `X-Wheel-Winner` is ambiguous and only `X-Wheel-Winner-Index` identifies the slot. Worth one sentence in `docs/miti99bot-integration.md`.
- **`Dockerfile`** — no `HEALTHCHECK` (only `compose.yml` has one, so plain `docker run` gets none) and no `USER node`, so the service runs as root. `package.json` has no `packageManager` field despite CI and the Dockerfile both pinning pnpm 11.9.0.
- **`src/server.js:29`** — `bodyLimit` is hardcoded at 32 KB while `MAX_OPTIONS`/`MAX_OPTION_CHARS` are configurable. `MAX_OPTIONS` above ~700 makes a legitimate maximum payload exceed the body limit, surfacing as HIGH-1's bogus 500.

---

## 4. Doc-accuracy table

| Claim | Location | Actual behavior | Verdict |
|---|---|---|---|
| "`RENDER_TIMEOUT_MS` is a total render timeout." | `docs/deployment.md:47` | Bounds `renderMedia` only; `selectComposition` cannot take the cancel signal, `getRemotionServeUrl()` is unbounded | **Wrong** |
| `pnpm render:local -- --option ...` (PowerShell) | `README.md:71` | Fails: `Unexpected argument '--output'` | **Wrong** |
| `pnpm render:local -- --option ...` (bash) | `README.md:82` | Fails identically | **Wrong** |
| "Run `pnpm render:local -- --help`" | `README.md:91` | Fails: `Unexpected argument '--help'` | **Wrong** |
| Usage line `pnpm render:local -- --option <text>` | `scripts/render-local.js:7` | Same broken form echoed in the tool's own help | **Wrong** |
| "Required env vars: PORT, HOST, MAX_CONCURRENT_RENDERS, RENDER_TIMEOUT_MS, MAX_OPTIONS, MAX_OPTION_CHARS, API_TOKEN" | `docs/deployment.md:24-34` | All have defaults; only `API_TOKEN` is required, and only in production | **Misleading** |
| "`API_TOKEN` is required when `NODE_ENV=production`" | `README.md:121` | True, but omits the `REQUIRE_API_TOKEN=true` path (`src/config.js:35`), which is documented nowhere | **Incomplete** |
| Example payload as the API contract | `README.md:14-24`, `docs/miti99bot-integration.md:18-28` | No ranges, no defaults, no `.strict()` warning, no error codes | **Incomplete** |
| `.env.example` implies a loadable env file | `.env.example` (whole file) | No loader, no dependency, zero doc references | **Misleading** |
| "`RENDER_TIMEOUT_MS` defaults to `15000` and is raised to Remotion's `7000ms` floor when lower" | `README.md:119-120`, `.env.example:5-6`, `docs/deployment.md:47-48` | Matches `src/config.js:45-48`; verified `RENDER_TIMEOUT_MS=3000` -> `7000` | **Accurate** |
| "`X-Wheel-Winner` is URL-encoded so non-ASCII labels are safe" | `README.md:32`, `docs/miti99bot-integration.md:38` | Verified by probe | **Accurate** |
| "compose.yml forwards these values with `${VAR:-default}` fallbacks" | `docs/deployment.md:36-37` | Matches `compose.yml:5-12` — but omits that `API_TOKEN` falls back to `change-me` | **Accurate but incomplete** |
| "Docker image runs `pnpm browser:ensure` during build so production requests do not download Chrome on first render" | `docs/deployment.md:44-45` | Matches `Dockerfile:43`. Does not mention that the Remotion *bundle* is still built at boot (`src/server.js:24-26`), which delays first listen | **Accurate but incomplete** |
| Fixture set: `smoke.gif`, `vietnamese.gif`, `sixteen-options.gif` | `README.md:61-62` | Matches `scripts/render-fixtures.js:9-46` | **Accurate** |
| Root `wheel.gif` and `fixtures/*.gif` are git-ignored | `README.md:92-94` | Matches `.gitignore`; `git ls-files` confirms neither is tracked | **Accurate** |
| Themes `classic`, `festival`, `mono` | `README.md:22`, `scripts/render-local.js:17` | Matches `src/remotion/themes.js:2,14,26` | **Accurate** |
| "API validation must reject oversized requests before render work starts" | `AGENTS.md:24` | `src/routes/gif.js:54-73` validates before `tryAcquire` | **Accurate** |
| "Rendering must not rebundle Remotion per request" | `AGENTS.md:25` | Memoized in `src/render/remotion-bundle.js`, warmed at boot | **Accurate** |

---

## 5. Prioritized actions

**Quick wins (each under ~15 minutes, no design decisions)**

1. HIGH-2 — remove `--` from the four documented CLI invocations. Pure doc/string edit; unblocks the primary local workflow.
2. HIGH-1 — honor `error.statusCode` in the error handler and switch to `request.log`. ~6 lines; also resolves MEDIUM-4's missing `reqId`.
3. HIGH-5 — add `Retry-After: 5` to the 429. One line.
4. MEDIUM-7 — `${API_TOKEN:?...}` in `compose.yml`. One line; closes the `change-me` default.
5. MEDIUM-9 — `CMD ["node", "src/server.js"]` in the Dockerfile. One line (the `app.close()` handler can follow separately).
6. MEDIUM-10 — reset `bundlePromise` on rejection. One line.
7. MEDIUM-11 — `loadConfig()` in `scripts/render-local-args.js` instead of hardcoded limits. Two lines.
8. HIGH-3 / HIGH-4 — add the field-constraint table and the error-code table to `README.md` and `docs/miti99bot-integration.md`. Doc-only, highest integrator value per minute spent.
9. MEDIUM-12 (partial) — add the oversized-body and non-ASCII-winner-header tests to `test/gif-route.test.js`. Six lines, locks in HIGH-1 and the advertised header contract.
10. Fix `docs/deployment.md:24` ("Required env vars" -> "Environment variables and defaults") and either fill or drop the `TBD` rows in `docs/render-benchmarks.md:13-14`.

**Larger work (needs a decision, not just an edit)**

1. MEDIUM-1 — enforce a real wall-clock bound with `Promise.race`, then either keep or correct the `docs/deployment.md:47` claim. This is the finding with the worst tail: a wedged slot is unrecoverable without a restart and invisible to the healthcheck.
2. MEDIUM-3 — decide the config-validation posture: fail fast on any set-but-invalid value, or keep silent fallbacks and log them. Either way, `MAX_OPTIONS < 2` and case-insensitive `REQUIRE_API_TOKEN` should be handled.
3. MEDIUM-6 — decide whether an unauthenticated `/api/gif` is ever acceptable. Minimum: a loud startup warning.
4. MEDIUM-2 — client-disconnect cancellation, or a documented client-timeout contract. Interacts with MEDIUM-1; do them together.
5. MEDIUM-5 — decide how much state `/api/healthz` should expose (`activeRenders` is the cheap 80%).
6. MEDIUM-8 — keep `.env.example` and wire `--env-file`, or delete it. Do not leave it orphaned.

---

## 6. Unresolved questions

1. **Is `MAX_CONCURRENT_RENDERS=1` plus immediate-429 the intended contract, or is a short queue expected?** With a single chat bot and multi-second renders, the second concurrent user always fails. Whether that is acceptable depends on the bot's fallback behavior, which lives outside this repo (`docs/miti99bot-integration.md:12` mentions a local fallback renderer). This changes whether HIGH-5 is "add a header" or "add a queue".
2. **Which deploy target is actually in use?** MEDIUM-6's severity depends on whether the target sets `NODE_ENV=production` by default. `docs/deployment.md:8-15` lists six options with different defaults.
3. **Is `POST /api/gif` internet-exposed or on a private network?** Determines whether the bearer token is the only control and how hard MEDIUM-7 bites.
4. **Should `size`/`fps` reject unsupported values or snap to the nearest supported one?** Current strictness is defensible but undocumented, and `size: 500` is an easy client mistake.
5. **Cold-start budget?** The measured 4.1s `api:smoke` used an on-disk `.tmp/remotion-bundle`. A fresh container builds the bundle before `listen`, and that duration was not measured here (would require a clean Docker build). Relevant to platforms with startup-probe deadlines.
