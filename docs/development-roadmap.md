# Development Roadmap

## Current State: v0.1.0

Core framework complete. One fully implemented module (trading), three stubs (wordle, loldle, misc). 110 passing tests. Deployed to Cloudflare Workers.

## Completed

### Phase 1: Bot Framework (v0.1.0)
- [x] Cloudflare Workers entry point with webhook validation
- [x] grammY bot factory with memoized cold-start handling
- [x] Plug-n-play module system with static import map
- [x] Three-tier command visibility (public/protected/private)
- [x] Unified conflict detection across all modules
- [x] KVStore interface with auto-prefixed per-module namespacing
- [x] Post-deploy register script (setWebhook + setMyCommands)
- [x] `/info` and `/help` commands
- [x] 56 unit tests covering all framework seams

### Phase 2: Trading Module (v0.1.0)
- [x] Paper trading with 5 commands (topup/buy/sell/convert/stats)
- [x] Real-time prices: CoinGecko (crypto+gold), TCBS (VN stocks), ER-API (forex)
- [x] 60-second price caching in KV with stale fallback
- [x] Per-user portfolio storage with VND as base currency
- [x] P&L tracking (total invested vs current portfolio value)
- [x] 54 unit tests for symbols, formatters, portfolio CRUD, handlers

## Planned

### Phase 3: Game Modules
- [ ] Wordle implementation (currently stub)
- [ ] Loldle implementation (currently stub)
- [ ] Game state persistence in KV

### Phase 4: Infrastructure
- [ ] CI pipeline (GitHub Actions: lint + test on PR)
- [ ] KV namespace IDs in wrangler.toml (currently REPLACE_ME placeholders)
- [ ] Per-module rate limiting consideration

### Future Considerations
- Internationalization (per-module if needed)
- More trading assets (expand symbol registry)
- Trading history / transaction log
- Group chat features
