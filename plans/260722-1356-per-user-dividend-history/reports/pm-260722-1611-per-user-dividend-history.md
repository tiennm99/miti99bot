---
type: project-status
plan: per-user-dividend-history
generated_at: 2026-07-22T16:11:50+07:00
status: completed
---

# Plan Complete: Per-User Dividend History

## Summary

| Metric | Result |
|---|---|
| Phases | 4/4 completed |
| Acceptance checks | 26/26 completed |
| Full tests | 607 passed, 1 intentional helper skip |
| Race test | Stock package passed |
| Static gates | Vet, build, lint, diff check passed |
| Review | Primary and adversarial PASS |

## Achievements

- Replaced asset dividend cursors with retained per-user SSI event history.
- Added exact 30-day discovery and historical missing-date refresh.
- Added Record-date approval gating and minimal opaque pending actions.
- Repeated unprocessed notices after every portfolio request while keeping
  multiple buttons financially idempotent.
- Added 90-day cleanup and idempotent MongoDB legacy-field migration.
- Updated runtime, deployment, and user-facing documentation.

## Accepted Risk

- Legacy hashed applied IDs are intentionally deleted. A recently applied
  legacy event could be rediscovered and credited again; the owner explicitly
  accepted this before landing.

## Documentation

- `README.md`
- `docs/deploy-coolify-selfhosted.md`
- Approved brainstorm report and all implementation phases

## Unresolved Questions

- None blocking. Optional future hardening: direct concurrent-callback and
  callback-save-failure tests.
