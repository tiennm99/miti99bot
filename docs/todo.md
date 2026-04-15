# TODO

Manual follow-ups after the D1 + Cron infra rollout (plan: `plans/260415-1010-d1-cron-infra/`).

## Pre-deploy (required before next `npm run deploy`)

- [ ] Create the D1 database:
  ```bash
  npx wrangler d1 create miti99bot-db
  ```
  Copy the returned UUID.

- [ ] Replace `REPLACE_ME_D1_UUID` in `wrangler.toml` (`[[d1_databases]]` → `database_id`) with the real UUID.

- [ ] Commit `wrangler.toml` with the real UUID (the ID is not a secret).

## First deploy verification

- [ ] Run `npm run db:migrate -- --dry-run` — confirm it lists `src/modules/trading/migrations/0001_trades.sql` as pending.

- [ ] Run `npm run deploy` — chain is `wrangler deploy` → `npm run db:migrate` → `npm run register`.

- [ ] Verify in Cloudflare dashboard:
  - D1 database `miti99bot-db` shows `trading_trades` + `_migrations` tables
  - Worker shows a cron trigger `0 17 * * *`

## Post-deploy smoke tests

- [ ] Send `/buy VNM 10 80000` (or whatever the real buy syntax is) via Telegram, then `/history` — expect 1 row.

- [ ] Manually fire the cron to verify retention:
  ```bash
  npx wrangler dev --test-scheduled
  # in another terminal:
  curl "http://localhost:8787/__scheduled?cron=0+17+*+*+*"
  ```
  Check logs for `trim-trades` output.

## Nice-to-have (not blocking)

- [ ] End-to-end test of `wrangler dev --test-scheduled` documented with real output snippet in `docs/using-cron.md`.

- [ ] Decide on migration rollback story (currently forward-only). Either document "write a new migration to undo" explicitly, or add a `down/` convention.

- [ ] Tune `trim-trades` schedule if 17:00 UTC conflicts with anything — currently chosen as ~00:00 ICT.

- [ ] Consider per-environment D1 (staging vs prod) if a staging bot is added later.
