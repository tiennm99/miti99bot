---
phase: 1
title: Design persistence and migration
status: completed
priority: P1
effort: medium
dependencies: []
---

# Phase 1: Design persistence and migration

## Overview

Extend both portfolio documents and add retry-safe startup migrations that
initialize legacy holdings from current quotes before command handlers run.

## Requirements

- Functional: persist `costBasis` as ticker/coin → total remaining cost.
- Functional: initialize missing legacy basis as `quantity × current price`.
- Functional: preserve any positive basis already written by a previous retry.
- Functional: scan every boot even after completion; the marker is an audit
  record and never suppresses invariant checks.
- Non-functional: startup fails on a required quote/storage error; migration is
  idempotent and bounded by a migration-wide deadline.

## Architecture

`cmd/server` checks which modules are loaded, then invokes each module's
`InitStore` before `modules.Install`. Every boot lists `user:` documents and
checks `positive holding => finite positive basis`. Migration fetches each
required symbol once, verifies the complete requested set, and writes only
missing basis using bounded CAS reload/retry. The shared `system` marker records
completion but does not skip future scans. Existing populated basis acts as
per-position retry progress.

## Related Code Files

- Modify: `internal/modules/stock/portfolio.go`
- Modify: `internal/modules/coin/portfolio.go`
- Create: `internal/modules/stock/startup.go`
- Create: `internal/modules/coin/startup.go`
- Modify: `cmd/server/main.go`
- Create: stock/coin startup memory and MongoDB tests

## Implementation Steps

1. Add and defensively initialize `CostBasis map[string]float64` with
   `json:"costBasis" bson:"costBasis"` in both portfolio types.
2. Validate before normalization. Reject corrupt basis, noncanonical symbols,
   canonical-key collisions, and non-finite holdings; do not delete or merge
   ambiguous legacy assets.
3. Implement injectable stock and coin migrations with stable `system` keys,
   `user:` listing, unique-symbol quote caching, complete quote-set validation,
   bounded CAS reload/retry, progress logs, and a completion marker written last.
4. Wire migration only for loaded modules after registry construction and
   before handler installation. Return startup-fatal errors with module/symbol
   context but no credentials.
5. Apply an overall startup-migration timeout; timeout is startup-fatal and
   leaves the marker incomplete.
6. Test empty, legacy, mixed, partial-retry, post-marker missing rows, partial
   quote maps, noncanonical/corrupt rows, CAS conflicts, timeout, storage
   failure, and idempotent second-boot cases in memory and MongoDB 8.

## Success Criteria

- [ ] New portfolios always contain initialized basis maps.
- [ ] Existing documents without `costBasis` still decode.
- [ ] Each legacy symbol receives migration-time current-price basis.
- [ ] Retry never overwrites basis initialized by an earlier attempt.
- [ ] Any required failure prevents the completion marker and aborts startup.
- [ ] Disabled modules do not call external quote providers.
- [ ] A completion marker never hides a later holding with missing basis.

## Risk Assessment

- External quote outages can block startup by explicit owner choice; provider
  timeouts bound the delay and logs identify the module/symbol.
- Partial writes are recoverable because populated per-symbol basis is never
  repriced and the global marker is written only after all rows succeed.
- Runtime validation fails closed if an old writer or restored row reintroduces
  a positive holding without valid basis after startup.
- Storage listing remains unpaginated; the migration-wide deadline bounds the
  current small, one-replica deployment without broad storage-API scope.
