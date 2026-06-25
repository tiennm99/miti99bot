# Fix: stats handlers starve Telegram reply of context budget

Status: DONE (2026-06-25) — implemented, `make vet` + `make test -race` green, code-review DONE (no critical/high).

## Problem

Update handler runs under one 10s ctx (`telegram/webhook.go:81`). Stats handlers reuse
that same ctx for both upstream price fetches and the final `sendMessage`. Per-upstream
HTTP timeout is also 10s (== handler budget), so one slow upstream drains the deadline and
the reply fails: `context deadline exceeded`. Confirmed in CloudWatch for `/stock_stats`
(dispatch→error exactly 10.0s; coin/gold succeeded same window — KBS was slow).

## Remedies (all approved)

1. **Reserve reply budget** — fetches run under a derived sub-ctx that leaves a reserve for
   the reply; the reply itself uses the original handler ctx.
2. **Lower per-fetch HTTP timeout** 10s → 3s, so one hung upstream can't eat the budget.
3. **Parallelize** the per-symbol fetch loops (stock, coin) so total ≈ slowest single fetch.

## Acceptance criteria

- `/stock_stats`, `/coin_stats` with N holdings: total fetch time bounded by slowest single
  fetch (~3s), not N×. Reply always delivered if any upstream responds within budget.
- A single unresponsive upstream → that line shows "(no price)" / "(price unavailable)";
  the summary still sends. No more whole-reply `context deadline exceeded`.
- `make vet` + `make test` green. No public-contract changes. Output text/order unchanged.

## Scope

IN: stock/coin/gold stats handlers; gold price handler; the 4 HTTP-timeout consts.
OUT: buy/sell single-fetch handlers (the 3s timeout alone leaves ~7s reply headroom);
`incomeEventsHTTPTimeout` (separate command, not a stats loop); webhook 10s budget itself.

## Changes

### New shared helper — `internal/modules/util/chathelper/chathelper.go`
- `const replyReserve = 3 * time.Second`
- `func FetchContext(ctx) (context.Context, context.CancelFunc)` — derive child ctx leaving
  `replyReserve` before the parent deadline (floor 1s); pass-through if parent has no deadline.

### `internal/modules/stock/prices.go`
- `kbsHTTPTimeout` 10s → 3s.

### `internal/modules/stock/handlers.go` (`handleStats`)
- Build `heldList`, fetch prices concurrently via `errgroup` (SetLimit 8) into an indexed
  results slice (preserves order), using `chathelper.FetchContext(ctx)`. Reply on original ctx.
- Fix the stale "in parallel" comment (now actually parallel).

### `internal/modules/coin/prices.go` + `internal/modules/coin/views.go` (`handleStats`)
- `coinHTTPTimeout` 10s → 3s.
- Parallelize the `sortedAssetSymbols` loop the same way; reply on original ctx.

### `internal/modules/gold/prices.go`, `vnappmob_client.go`, `handlers.go`
- `goldHTTPTimeout` + `vnappmobHTTPTimeout` 10s → 3s.
- `handleStats` + `handlePrice`: fetch under `chathelper.FetchContext(ctx)`, reply on original ctx.
  (Single fetch but composite tries providers sequentially → reserve guarantees reply headroom.)

### `go.mod`
- `errgroup` (golang.org/x/sync) moves indirect → direct via `go mod tidy`.

## Risks / rollback

- Concurrency: `PriceClient`/coin client HTTP clients are safe for concurrent reuse (shared
  pool via sync.Once). Result slice written by index → no shared-write race.
- 3s may be tight on Lambda cold-start TLS to a slow upstream; mitigated by reserve+parallel
  and graceful "(no price)" fallback. Revert = restore constants to 10s.
- Tests inject HTTP client + URL; lowering real-client timeout doesn't affect injected ones.

## Validation

`make vet`, `make test`. Add focused test: stats handler with a hanging upstream returns a
reply (not a deadline error) within budget.
