---
phase: 2
title: "In-Process Cron Scheduler"
status: pending
priority: P1
dependencies: []
effort: "S"
---

# Phase 2: In-Process Cron Scheduler

## Overview

Off AWS there is no EventBridge Scheduler to hit `/cron/{name}`. Add an opt-in in-process scheduler that reads each registered cron's existing `Cron.Schedule` field and fires `Cron.Handler` on time. Gated by `CRON_MODE=internal` so the Lambda/EventBridge path is unchanged by default.

## Requirements

- Functional: when `CRON_MODE=internal`, parse every `reg.Crons()[i].Schedule` and invoke its handler on schedule, in UTC (match EventBridge's `ScheduleExpressionTimezone: UTC`).
- Functional: default (`CRON_MODE` unset/`external`) starts NO scheduler — preserves current Lambda behavior where EventBridge owns timing.
- Functional: the existing `/cron/{name}` HTTP route stays as-is (still usable for manual/curl triggers and as the sidecar fallback).
- Non-functional: each tick runs under the same 60s budget as the HTTP path (`defaultCronTimeout`); a panicking handler is recovered and logged, scheduler keeps running.
- Non-functional: scheduler stops cleanly on `rootCtx` cancellation (graceful shutdown).

## Architecture

The `Cron` struct already carries `Schedule string` (currently "documentation only", e.g. lolschedule `"0 1 * * *"`). Promote it to the real trigger source for self-host.

Use `github.com/robfig/cron/v3` (standard, well-maintained 5-field cron parser) with `cron.New(cron.WithLocation(time.UTC))`. For each registered cron, `c.AddFunc(schedule, fn)` where `fn` dispatches the handler through the existing cron dispatcher path so logging/metrics/timeout/panic-recovery are identical to the HTTP route.

Dispatch: the existing exported helper is **`modules.DispatchScheduled(ctx, name, reg)`** (`cron_dispatcher.go:19` — note arg order: name BEFORE reg). It does ONLY a registry lookup + `cron.Handler(ctx, deps)` — it has **no timeout, no panic-recovery, no structured logging**. That wrapping lives in the server-package `cronHandler` (`internal/server/router.go`), NOT in the dispatcher. So the scheduler must add its own:
- wrap each fire in `context.WithTimeout(ctx, 60s)` (the `defaultCronTimeout` constant is in `internal/server/timeouts.go`; do not import `internal/server` into the scheduler — define a local `cronTimeout = 60*time.Second` or lift the constant to a shared package to avoid a layering inversion),
- `recover()` around the handler call, logging the cron name on panic so one bad cron doesn't kill the scheduler,
- structured `log.Info("cron triggered", …)` / `log.Error("cron failed", …)` mirroring the HTTP path.
Call `modules.DispatchScheduled(ctx, name, reg)` inside that wrapper.

New file `internal/cron/scheduler.go` (new small package, mirrors `internal/metrics` lifecycle style):
```
func Run(ctx context.Context, reg *modules.Registry) (stop func(), err error)
```
- Skips crons with empty `Schedule` (logs a warning — a self-hosted cron with no schedule never fires).
- Validates schedule strings at startup; a bad expression is fatal (fail fast, like other config errors).

Wire in `cmd/server/main.go` after `modules.Install`, gated:
```
if strings.EqualFold(cfg.CronMode, "internal") {
    stop, err := cron.Run(rootCtx, reg)
    if err != nil { log.Fatal("cron scheduler init failed", "err", err) }
    defer stop()
    log.Info("internal cron scheduler started", "crons", len(reg.Crons()))
}
```

Add `CronMode` to `config` from env `CRON_MODE`.

## Related Code Files

- Create: `internal/cron/scheduler.go` — scheduler lifecycle.
- Create: `internal/cron/scheduler_test.go` — fake registry with a fast schedule (`@every 1s` or injected clock) asserts handler fires; bad-schedule errors; ctx-cancel stops.
- Modify: `cmd/server/main.go` — `CronMode` config + gated `cron.Run`.
- Modify: `internal/modules/module.go` doc comments — update `CronHandler`/`Cron.Schedule` text that currently says "real schedule lives in EventBridge" to note the `CRON_MODE=internal` path. (The scheduler calls the existing `modules.DispatchScheduled`; if the 60s-timeout/panic-recover wrapper is worth sharing with the HTTP `cronHandler`, lift it to a shared helper — but that is optional, not required.)
- Modify: `go.mod` / `go.sum` — add `github.com/robfig/cron/v3`.
- Modify: `README.md` — document `CRON_MODE` (external default vs internal self-host).
- Modify: `internal/modules/lolschedule/cron.go` — add an idempotency guard: the daily-push handler reads/writes a KV "last push UTC date" key and no-ops if already pushed today (defends against all double-fire windows). This also makes the existing EventBridge path safe during cutover overlap.

## Implementation Steps

1. `go get github.com/robfig/cron/v3`.
2. Call `modules.DispatchScheduled(ctx, name, reg)` (`cron_dispatcher.go:19`) from inside a scheduler-local wrapper that adds the 60s timeout + `recover()` + logging (the dispatcher provides none of these).
3. Write `internal/cron/scheduler.go`: build `cron.New(cron.WithLocation(time.UTC))`, register each non-empty schedule, `c.Start()`, return a `stop` that calls `c.Stop()` and waits for the context done.
4. Wire gated startup in `main.go`; add `CronMode` config field + env read.
5. Update the now-stale "documentation only / EventBridge owns timing" comments on `Cron.Schedule` and `CronHandler`.
6. Tests + `make vet && make test`.

## Success Criteria

- [ ] With `CRON_MODE=internal`, lolschedule daily push fires at 01:00 UTC; observable in logs (`cron triggered`).
- [ ] With `CRON_MODE` unset, no scheduler starts (Lambda/EventBridge path byte-for-byte unchanged).
- [ ] Bad schedule string fails startup with a clear error.
- [ ] Handler panic is recovered (scheduler-local `recover()`); scheduler survives and fires next tick.
- [ ] Each fire runs under a 60s timeout (scheduler-local, not imported from `internal/server`).
- [ ] Daily push is idempotent per UTC date: invoking the handler twice on the same date sends subscribers exactly one digest.
- [ ] Scheduler stops within shutdown grace period on SIGTERM.

## Risk Assessment

- **Double-fire (Critical — the daily push is NOT idempotent)**: `lolschedule` `runDailyPush` (`cron.go:128-191`) fans out to every subscriber unconditionally — no "already sent today" marker. So ANY double-fire = every subscriber DM'd twice. Three concrete double-fire windows the opt-in flag does NOT cover:
  1. **Cutover overlap**: EventBridge `AWS::Scheduler::Schedule` (`template.yaml:271-289`) invokes the Lambda DIRECTLY, independent of the webhook URL — re-pointing the webhook does NOT stop it. It must be **disabled/deleted before** the Coolify container runs `CRON_MODE=internal` (ordered prerequisite in Phase 4, not "N days later" cleanup).
  2. **Rolling deploy**: Coolify/compose may run old+new containers briefly; both run the in-process scheduler. A redeploy near 01:00 UTC double-fires.
  3. Operator misconfig (`internal` set on a second instance).
  **Primary mitigation (covers all three cheaply)**: add a KV "last push date" guard in `lolschedule` — the handler records the date it pushed and no-ops if already pushed for that UTC date. This makes the push idempotent regardless of trigger count. Secondary: prefer stop-first redeploy in Coolify; keep `CRON_MODE` opt-in and never set `internal` on Lambda.
- **5-field vs 6-field cron**: `"0 1 * * *"` is 5-field standard. Mitigation: use robfig `cron/v3` default 5-field parser (not the seconds-enabled one).
- **Single-instance assumption**: if Coolify scales the service to >1 replica, crons fire per replica. Mitigation: document "run exactly 1 replica" (the bot is a single-instance webhook consumer anyway); revisit with a DB lock only if scaling is ever needed (YAGNI now).
- **Missed fire while container restarts**: a deploy at 01:00 UTC could skip that day's push. Mitigation: accept (same risk class as Lambda cold-start miss); not data-loss.
