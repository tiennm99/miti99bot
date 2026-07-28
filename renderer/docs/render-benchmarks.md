# Render Benchmarks

Fill this after running:

```sh
pnpm render:fixtures
```

| Fixture | Size | FPS | Duration | Bytes | Render Time | Notes |
|---|---:|---:|---:|---:|---:|---|
| smoke | 384 | 12 | 3.5s | 315661 | 13917ms | Docker `pnpm render:smoke` on 2026-07-06 |
| api-smoke | 384 | 12 | 3.5s | 298374 | 4100ms | Docker `pnpm api:smoke`, after startup bundle warm-up |
| vietnamese | 512 | 15 | 7.7s | TBD | TBD | label readability |
| sixteen-options | 512 | 15 | 7.7s | TBD | TBD | dense wheel |
