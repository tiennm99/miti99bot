# /stats failure root-cause report — 2026-05-22

**Status:** DONE

**Summary:**
The `stats` module is absent from the live Lambda's `MODULES` env var (`util,misc,wordle,loldle,lolschedule,twentyq,trading` — 7 entries, no `stats`). Consequently `/stats` commands arrive, are logged as dispatched, but no handler is registered so no reply is sent. The cause is commit `5e40ffe`: when it hardcoded non-secret CFN params in `deploy.yml` to fix the `sed` regex breakage, it listed `BotOwnerID` and `AdminUserIDs` but omitted `ModulesCSV`, leaving `sam deploy --parameter-overrides` without it. Because SAM CLI `--parameter-overrides` *replaces* (not merges with) `samconfig.toml`, the stack retained its previously-deployed value of the param — which at that point did not yet include `stats`.

---

## Evidence

### 1. Live Lambda env — `stats` absent from MODULES

```
aws lambda get-function-configuration --function-name miti99bot ...
MODULES = "util,misc,wordle,loldle,lolschedule,twentyq,trading"
```
7 entries. `stats` is not present.

### 2. Deployed CloudFormation stack parameter confirms the same

```json
{ "ParameterKey": "ModulesCSV", "ParameterValue": "util,misc,wordle,loldle,lolschedule,twentyq,trading" }
```
(from `aws cloudformation describe-stacks`)

### 3. Template default has `stats`; samconfig.toml has `stats`

`template.yaml:17`:
```yaml
ModulesCSV:
  Default: util,misc,wordle,loldle,lolschedule,twentyq,trading,stats
```

`samconfig.toml:16` (parameter_overrides):
```
ModulesCSV="util,misc,wordle,loldle,lolschedule,twentyq,trading,stats"
```

Neither is used in CI — `sam deploy --parameter-overrides "$OVERRIDES"` overrides everything.

### 4. deploy.yml OVERRIDES string omits ModulesCSV

`.github/workflows/deploy.yml:62`:
```bash
OVERRIDES="CronSharedSecret=$CRON_SECRET BotOwnerID=1064111334 AdminUserIDs=1064111334"
```
`ModulesCSV` is not included. This was introduced by commit `5e40ffe` ("fix(deploy): hardcode non-secret CFN params instead of sed-parsing samconfig"). That commit fixed the `sed` regex breakage (commit `85579e5`) but did not carry over `ModulesCSV`.

### 5. CloudWatch logs: `/stats` dispatched, zero handler hits

```
2026-05-22T08:26:01Z  {"msg":"dispatch","text":"/stats@miti99bot"}
2026-05-22T08:34:14Z  {"msg":"dispatch","text":"/stats@miti99bot"}
2026-05-22T08:47:02Z  {"msg":"dispatch","text":"/stats@miti99bot"}
2026-05-22T09:06:17Z  {"msg":"dispatch","text":"/stats@miti99bot"}
2026-05-22T09:49:35Z  {"msg":"dispatch","text":"/stats@miti99bot"}
```

Metrics logs never record `stats` command; only `trade_stats`:
```
2026-05-22T08:31:24Z  {"msg":"metrics","commands":{"trade_stats":1}}
2026-05-22T08:40:01Z  {"msg":"metrics","commands":{"trade_stats":1}}
2026-05-22T09:12:00Z  {"msg":"metrics","commands":{"trade_stats":1}}
```

### 6. `modules loaded` confirms 7 modules, not 8

```
2026-05-22T08:25:49Z  {"msg":"modules loaded","modules":7,"commands":28,"crons":1}
```
Every cold start since `5e40ffe` merged shows `modules:7`.

### 7. DynamoDB table has zero `stats`-module rows

Full table scan (`miti99bot-data`, 23 items): no item with `pk = "stats"`. The module has never run in production. (`count:*` and `stats:count:*` prefix scans also return 0 rows — confirming no data written under alternative key schemes.)

### 8. Code path — why `/stats` produces no reply when module absent

- `cmd/server/main.go:114`: `modules.Build(cfg.Modules, factories(), ...)` — only builds modules listed in `cfg.Modules` (= `MODULES` env).
- `internal/modules/registry.go:120-186`: `stats` factory is never called; `/stats` command never registered in `reg.AllCommands`.
- `internal/modules/dispatcher.go:57-84`: `modules.Install` only registers handlers for commands in `reg.AllCommands`. No entry for `stats` → `matchCommand("stats", update)` never fires → bot silently ignores the update.

---

## Root Cause

Commit `5e40ffe` hardcoded `BotOwnerID` and `AdminUserIDs` into `deploy.yml`'s `OVERRIDES` string but omitted `ModulesCSV`. Because `sam deploy --parameter-overrides` replaces `samconfig.toml`'s overrides entirely (SAM CLI behavior), the stack deployed with `ModulesCSV` at the CFN-stored value from the prior deploy — which predated the `stats` addition — and the `stats` module was never enabled in production.

---

## Recommended Fix

**File:** `.github/workflows/deploy.yml`, line 62

Add `ModulesCSV` to the `OVERRIDES` string:

```bash
OVERRIDES="CronSharedSecret=$CRON_SECRET BotOwnerID=1064111334 AdminUserIDs=1064111334 ModulesCSV=util,misc,wordle,loldle,lolschedule,twentyq,trading,stats"
```

This is the only change needed. No Lambda code changes required. After the next deploy the `MODULES` env will include `stats`, the module will load, and `/stats` will reply.

**Alternatively** (more robust long-term): read `ModulesCSV` from `samconfig.toml` at deploy time using `tomlq` or a simple `grep`/`awk` so `samconfig.toml` remains the single source of truth:
```bash
MODULES_CSV=$(grep 'ModulesCSV=' samconfig.toml | sed 's/.*ModulesCSV="\([^"]*\)".*/\1/')
OVERRIDES="... ModulesCSV=$MODULES_CSV"
```
But the direct hardcode is sufficient and consistent with how `BotOwnerID` is handled.

---

## Unresolved Questions

None. Root cause and fix are unambiguous.
