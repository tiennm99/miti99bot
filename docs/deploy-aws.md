# Deploy: AWS (Lambda + DynamoDB + EventBridge)

> **RETIRED.** `miti99bot` is now self-hosted on Coolify + MongoDB Atlas — see
> [`deploy-coolify-selfhosted.md`](./deploy-coolify-selfhosted.md). The AWS stack
> is decommissioned ([`aws-decommission-runbook.md`](./aws-decommission-runbook.md)).
> Kept for historical reference / if AWS is ever revisited.

This is the production deploy path for `miti99bot`. Strict free-tier targets, region `ap-southeast-1`.

> **First-time setup:** see `aws/README.md`. This doc is for steady-state operations.

## Architecture (one diagram)

```
Telegram ──HTTPS──► Lambda Function URL (AuthType: NONE)
                    └─► AWS Lambda Web Adapter ──► localhost:8080
                                                    └─► Go http.Handler (cmd/server)
                                                          ├─► DynamoDB (KV)
                                                          ├─► Gemini API (AI modules)
                                                          └─► Telegram Bot API (replies)

EventBridge Scheduler ──cron──► HTTPS POST <FunctionURL>/cron/{name}
                                + Header X-Cron-Token (from SSM)
```

## Deploy

### Via GitHub Actions (canonical)

```
git push origin main
```

Triggers `.github/workflows/deploy.yml`:
1. OIDC assume `github-deploy-miti99bot` role
2. `make build-lambda` (Go ARM64 ZIP-ready binary)
3. `sam deploy --template-file template.yaml`
4. Smoke `curl <function-url>/`

### Manual (emergency / staging)

```sh
make build-lambda
make sam-deploy            # uses samconfig.toml defaults
ALERT_EMAIL=you@example.com make sam-deploy   # with budget alert wired
```

## Verify

```sh
make logs SINCE=10m

aws cloudformation describe-stacks --stack-name miti99bot \
  --query "Stacks[0].Outputs[?OutputKey=='FunctionUrl'].OutputValue" --output text

curl -fsSL "$(...)/" | jq .                       # health JSON
```

## Set the Telegram webhook

> `.github/workflows/deploy.yml` auto-runs `setWebhook` + `setMyCommands` after every push to `main`. The snippet below is the break-glass equivalent for manual / out-of-band fixes (e.g. rerun from a workstation when CI is unavailable).

```sh
URL=$(aws cloudformation describe-stacks --stack-name miti99bot \
        --query "Stacks[0].Outputs[?OutputKey=='FunctionUrl'].OutputValue" --output text)
SECRET=$(aws ssm get-parameter --name /miti99bot/prod/telegram-webhook-secret \
        --with-decryption --query 'Parameter.Value' --output text)
TOKEN=$(aws ssm get-parameter --name /miti99bot/prod/telegram-bot-token \
        --with-decryption --query 'Parameter.Value' --output text)

curl -X POST "https://api.telegram.org/bot$TOKEN/setWebhook" \
  -d "url=${URL}webhook" \
  -d "secret_token=$SECRET" \
  -d "drop_pending_updates=false" \
  -d "allowed_updates=[\"message\",\"callback_query\"]"
```

Verify:
```sh
curl "https://api.telegram.org/bot$TOKEN/getWebhookInfo" | jq .
```
Expect: `url` matches Function URL, `pending_update_count` ≈ 0, `last_error_date` empty.

## Adding a module or command (registration checklist)

A module only runs in production if its name is in **both** `ModulesCSV` sources — the
`template.yaml` default is ignored once an override is passed, so editing one place is
not enough. A command only appears in the Telegram menu if it is in
`aws/telegram-commands.json`. Missing either is silent: no error, the command just
never dispatches (this is how `coin_*` shipped dark until `coin` was added to the CSVs).

When **adding a new module**, register it in all of:

1. `cmd/server/main.go` — add the factory to the catalog (`"name": pkg.New`).
2. `.github/workflows/deploy.yml` — append the name to `ModulesCSV=…` (CI override).
3. `samconfig.toml` — append the name to `ModulesCSV=…` (manual-deploy override; keep in sync with the workflow).
4. `template.yaml` — append to the `ModulesCSV` `Default` (documents the full set).
5. `aws/telegram-commands.json` — add each new command + description for the Telegram menu.

When **adding a command to an existing, already-enabled module**, only step 5 applies.

**On push to `main`:** CI redeploys and re-runs `setMyCommands` from
`aws/telegram-commands.json` automatically. The Telegram client caches the command
menu, so a changed menu may not show until the chat is reopened — confirm with
`make telegram-commands-info` (calls `getMyCommands`) rather than trusting the app UI.
Only when a push introduces **new public commands** (`VisibilityPublic`) does the menu
need attention — re-confirm registration for those pushes; routine pushes (refactors,
fixes, non-public commands) need no menu action.

## Stock income events API

`/stock_income_events` uses a FireAnt REST API, configured at Lambda runtime:

- `STOCK_INCOME_EVENTS_API_URL`: FireAnt base URL; defaults to `https://restv2.fireant.vn`. The bot calls `/symbols/{symbol}/timescale-marks` with `startDate` and `endDate`.
- `STOCK_INCOME_EVENTS_API_TOKEN`: bearer token for FireAnt. Store it directly only for local dev; in AWS prefer `STOCK_INCOME_EVENTS_API_TOKEN_PARAMETER_NAME`.

FireAnt response is an array of timescale marks with `id`, `label`, `date`, `title`, and `color`. The bot keeps marks whose label/title indicate dividends, ex-right dates, final registration dates, rights issues, or bonus/share dividends.

## Stock price providers

`/stock_buy`, `/stock_sell`, and `/stock_stats` use unofficial public quote endpoints. Zero-value provider order is:

1. KBS current price board (`/stock/iss`).
2. VCI current quote board (`/price/symbols/getList`).
3. SSI iBoard direct quote.

This order is intentional for current-price commands. KBS and VCI both support batch current quotes, while SSI can return a Cloudflare security page. Treat all three as unofficial app-internal endpoints and keep provider/source errors visible in Lambda logs.

## Gold module

`gold` is opt-in for first deploy. Enable it by adding `gold` to the `ModulesCSV` parameter / `MODULES` env, for example `util,misc,wordle,loldle,lolschedule,twentyq,stock,stats,gold`.

Commands:

- `/gold_price` shows current gold price. When VNAppMob SJC is available it prints buy/sell/mid VND/lượng; otherwise it falls back to world spot XAU in USD/oz plus USD/VND rate.
- `/gold_topup <amount>` credits VND. No currency argument is accepted.
- `/gold_buy <luong>` buys gold in Vietnamese `luong`. No symbol or unit argument is accepted.
- `/gold_sell <luong>` sells gold in Vietnamese `luong`.
- `/gold_stats` shows VND balance, gold holding, current price, total value, invested amount, and P&L.

Price source: primary is VNAppMob Vietnam SJC price feed (`api.vnappmob.com/api/v2/gold/sjc`), which returns VND/lượng directly. The client auto-refreshes a free JWT API key and caches it in KV. If VNAppMob fails, the bot falls back to world spot XAU from GoldPrice.org converted through USD/VND from ExchangeRate-API open endpoint. The defaults require no secrets. Optional overrides:

- `GOLD_VNAPP_API_URL`: VNAppMob API base URL override. Remote URLs must be HTTPS; localhost HTTP is allowed for local tests.
- `GOLD_VNAPP_API_KEY`: pre-issued VNAppMob JWT key. When set, auto-refresh is skipped. Useful for local dev or SSM injection.
- `GOLD_VNAPP_API_KEY_PARAMETER_NAME`: SSM SecureString parameter name containing the pre-issued key. Fetched at Lambda cold start.
- `GOLD_PRICE_API_URL`: fallback gold spot JSON endpoint override. Remote URLs must be HTTPS; localhost HTTP is allowed for local tests.
- `GOLD_FX_API_URL`: fallback USD/VND FX JSON endpoint override. Remote URLs must be HTTPS; localhost HTTP is allowed for local tests.

ExchangeRate-API open endpoint requires attribution if surfaced publicly and updates once per day; the bot caches FX responses until the provider `time_next_update_unix` when available.

## Rotate secrets

```sh
aws ssm put-parameter --name /miti99bot/prod/telegram-webhook-secret \
  --value "$(openssl rand -hex 32)" --type SecureString --overwrite
# template.yaml uses ":1" version pin; redeploy to pick up the new value:
make sam-deploy
# Then re-run setWebhook (above) with the new secret_token.
```

> The `:1` in `{{resolve:ssm-secure:…:1}}` is the parameter **version** — it pins to the latest version at deploy time, not version 1 forever. To force a refresh after rotation, redeploy.

## Rollback

CloudFormation handles failed deploys: a failing `sam deploy` triggers automatic rollback to the prior version. To roll back a successful-but-bad deploy:

```sh
aws cloudformation update-stack \
  --stack-name miti99bot \
  --use-previous-template \
  --capabilities CAPABILITY_IAM
```

Or redeploy from a known-good commit:
```sh
git checkout <good-sha>
make sam-deploy
```

## Operational checks (daily during 7-day soak)

```sh
# Errors / warnings in last 24h
aws logs filter-log-events --log-group-name /aws/lambda/miti99bot \
  --start-time $(($(date +%s%3N) - 86400000)) \
  --filter-pattern '{ $.level = "ERROR" }' --max-items 20

# Cold start P95
aws logs start-query --log-group-name /aws/lambda/miti99bot \
  --start-time $(($(date +%s) - 86400)) --end-time $(date +%s) \
  --query-string 'filter @type = "REPORT" | stats avg(@initDuration), pct(@initDuration, 95)'

# DynamoDB throttle
aws cloudwatch get-metric-statistics --namespace AWS/DynamoDB \
  --metric-name ThrottledRequests --dimensions Name=TableName,Value=miti99bot-data \
  --statistics Sum --start-time $(date -u -d '24 hours ago' +%FT%TZ) \
  --end-time $(date -u +%FT%TZ) --period 3600

# Current month spend
aws ce get-cost-and-usage --granularity MONTHLY \
  --time-period Start=$(date -u +%Y-%m-01),End=$(date -u +%F) \
  --metrics UnblendedCost
```

## Free-tier guardrails

| Resource | Free | Watch when |
|---|---|---|
| Lambda req / GB-s | 1M / 400k | Past 50% mid-month |
| DynamoDB req | 200M | Past 5% (sign of runaway loop) |
| DynamoDB storage | 25 GiB | Past 100 MiB (suspect leaks) |
| EventBridge invocations | 14M | Past 1k/mo (suspect mis-config) |
| CloudWatch Logs ingest | 5 GB | Past 50% mid-month |
| Egress | 100 GB | Past 1 GB (wildly high) |

A `$1` budget alert at 80%/100% catches all of these via cost-side fallout.
