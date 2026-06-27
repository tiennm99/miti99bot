# Research Report: Whole-Project Free-Tier Audit + S3 Elimination

Timestamp: 2026-06-27 18:49 ICT

Supersedes/extends: `260627-1143-sam-no-s3-deploy-research.md` (S3-only research). This adds a full per-service free-tier audit and a verified 2026 free-tier picture.

## Executive Summary

S3 is the **only** AWS resource in this stack with no permanent free tier. Everything else (Lambda, DynamoDB on-demand, EventBridge Scheduler, SQS, CloudWatch Logs, X-Ray, SSM Standard, Budgets) is **always-free** within monthly caps the bot will never approach. The user's instinct is correct: keep SAM-on-every-push and you keep an S3 line item forever (microscopic, but nonzero).

For a compiled Go Lambda there is no IaC path that avoids both S3 and ECR — SAM/CloudFormation must stage the ZIP somewhere. The only zero-S3 option is to deliver code with `aws lambda update-function-code --zip-file fileb://...` and run SAM only when infra changes. Recommended below.

Magnitude check (honesty): the SAM bucket for this app costs ~$0.002/month storage + sub-cent request charges per deploy after the free window. Real, but trivial. The recommendation assumes the stated hard rule "zero cost line items" — if the rule is actually "stay within free tier / no meaningful cost," keeping S3 with a lifecycle rule is also fine.

## Free-Tier Audit (every resource in `template.yaml`)

| Resource | Service | Free-tier status | Verdict |
|---|---|---|---|
| `BotFunction` | Lambda (`provided.al2023`, arm64, 256MB) | Always-free: 1M req/mo + 400k GB-s/mo | ✅ Free forever |
| `BotTable` | DynamoDB `PAY_PER_REQUEST` | Always-free: 25GB + 2.5M RRU + 1M WRU/mo | ✅ Free forever |
| `LolscheduleDailyPushSchedule` | EventBridge Scheduler | Always-free: 14M invocations/mo | ✅ Free forever |
| `CronDLQ` | SQS | Always-free: 1M requests/mo | ✅ Free forever |
| `BotFunctionLogGroup` (7-day retention) | CloudWatch Logs | Always-free: 5GB ingest + 5GB store/mo | ✅ Free (retention caps growth) |
| `ColdStartMetricFilter` → `ColdStartInitDuration` | CloudWatch custom metric | Free: 10 custom metrics. This is 1. | ✅ Free (watch the count) |
| `Tracing: Active` | X-Ray | Always-free: 100k traces recorded/mo | ✅ Free (bot volume ≪ cap) |
| SSM `GetParameter`/`GetParameters` (cold-start secret load) | SSM Parameter Store | Standard tier + standard throughput = free | ✅ Free (do NOT switch to Advanced/High-throughput) |
| `MonthlyBudget` | AWS Budgets | First 2 budgets free/account. This is 1. | ✅ Free |
| **SAM deploy bucket** (`aws-sam-cli-managed-default-*`) | **S3** | **No always-free tier.** 12-mo (legacy) / 6-mo credits (new acct) only | ⚠️ **Only paid line item** |
| Lambda Function URL (`AuthType: NONE`) | Lambda | No extra charge (counts as Lambda req) | ✅ Free |
| Lambda Web Adapter layer | Lambda layer | No charge | ✅ Free |

External (off-AWS, no AWS bill): Telegram Bot API, FireAnt, GoldPrice.org, ExchangeRate-API, VNAppMob, Binance/Coinbase/CoinGecko, Gemini API. Gemini has its own free quota — out of scope for AWS cost.

### Watch-items (free now, could tip over if scaled)

- **X-Ray `Tracing: Active`** — free to 100k traces/mo. A personal bot is nowhere near. But it is *not* always-free above the cap. If you never plan to inspect traces, you can set `Tracing: PassThrough` (or remove) to drop the dependency entirely. Low priority.
- **Custom metrics** — only 1 of 10 free slots used. Adding ~10 more metric filters would start billing $0.30/metric/mo.
- **CloudWatch Logs** — 7-day retention is set, which is what keeps ingest/storage under 5GB. Don't remove it.
- **SSM throughput** — secrets load once per cold start at Standard throughput (free). Don't enable "higher throughput" (paid) or convert params to Advanced tier ($0.05/param/mo).

## S3: the one thing that isn't free

Verified 2026: AWS overhauled the Free Tier on 2025-07-15. S3 never had an always-free tier; the 5GB/12-month trial (legacy accounts) and the new $100–$200 6-month credit plan (accounts created on/after 2025-07-15) are both **time-boxed**. After the window, S3 Standard = $0.023/GB-mo + per-request charges. There is no S3 storage allotment that survives the trial.

Why SAM forces S3 here:
- `sam deploy` packages the local ZIP and uploads it to S3 (`resolve_s3=true`, `s3_prefix="miti99bot"` in `samconfig.toml`). Artifacts > 51,200 bytes *must* go to S3; this binary is ~7.8MB.
- CloudFormation `AWS::Lambda::Function` ZIP code requires an S3 object. Inline code is Node/Python-only — not viable for a Go binary.
- ECR avoids S3 but is itself billable storage with only a time-boxed free tier — strictly worse for this single small ZIP.

So: with pure SAM/CFN, a compiled Go Lambda cannot avoid both S3 and ECR. To reach zero S3 you must move *code delivery* out of CloudFormation.

## Brainstormed Options (zero-S3)

### Option A — Keep SAM + S3, add 1-day lifecycle expiry
Add an S3 lifecycle rule so artifacts expire after 1 day; storage trends to ~0.
- Pros: zero code change to deploy flow; CFN stays sole source of truth; cost ≈ a few cents/year.
- Cons: still a nonzero S3 line item (PUT/GET per deploy). **Fails the strict "zero line items" rule.**

### Option B — Direct `aws lambda update-function-code` for code; SAM only for infra (RECOMMENDED)
Routine pushes update Lambda code directly from a local ZIP (no S3). Run SAM only when `template.yaml` changes.
- Pros: official Lambda path; zero S3 for the 99% code-only case; trivially simple for one function; ZIP (~7.8MB) is far under the 50MB direct-upload limit.
- Cons: code-only deploys bypass CloudFormation's view of code (benign drift for a single function); first-ever create and infra changes still touch S3 once.
- Fit: best match for this repo. Single Lambda, small ZIP, free-tier-hard requirement.

### Option C — Container image in ECR
- Rejected: trades S3 for ECR, another time-boxed-free storage service. More build complexity, no benefit here.

### Option D — Fully imperative provisioning, no SAM at all
- Rejected: you'd reimplement IAM, Function URL, DynamoDB, Scheduler, SQS, Logs, Budgets and lose rollback safety to delete one sub-penny S3 dependency. Not worth it.

## Recommendation

Adopt **Option B**. Concretely:

1. Keep `template.yaml` as the infra source of truth.
2. Change `.github/workflows/deploy.yml`: on push to `main`, deploy code only via direct upload (no S3). Run `sam deploy` only when `template.yaml` (or params) changed — detect via `git diff`.
3. First-time creation and infra changes use SAM (one-time S3 touch — accept it, or delete the managed bucket afterward).
4. Guard: fail the direct deploy if the ZIP ≥ 50MB (forces a rethink before silently breaking).

This yields zero S3 for every code-only push, which is the overwhelming majority of deploys.

### Candidate CI deploy step (code-only path)

```sh
make build-lambda
zip -j build/lambda/function.zip build/lambda/bootstrap
SIZE=$(stat -c%s build/lambda/function.zip)
[ "$SIZE" -lt 52428800 ] || { echo "ZIP >= 50MB; use S3/SAM"; exit 1; }
aws lambda update-function-code \
  --function-name miti99bot \
  --zip-file fileb://build/lambda/function.zip \
  --publish
aws lambda wait function-updated --function-name miti99bot
```

Env vars / Function URL / Scheduler / DynamoDB / role stay managed by the SAM stack — direct code upload doesn't touch them.

### CI routing sketch (only run SAM when infra changed)

```sh
if git diff --quiet HEAD~1 -- template.yaml samconfig.toml; then
  # code-only: direct upload (no S3)
else
  # infra change: sam deploy (touches S3 once)
fi
```

Note: the OIDC deploy role (`github-deploy-miti99bot`) needs `lambda:UpdateFunctionCode` + `lambda:GetFunction` for the direct path; it likely already has broad Lambda perms via SAM, but verify before switching CI.

## Bottom line

- The project is **already free** on every service except S3, and S3's real cost is fractions of a cent — but it is a permanent nonzero line item once the free window ends.
- The clean way to hit truly $0 is Option B: direct Lambda code upload for routine deploys, SAM reserved for infra.
- No other resource needs changing to stay free; just don't add custom metrics past 10, keep log retention, and keep SSM at Standard tier.

## Unresolved Questions

1. Strict rule check: do you want **zero S3 resources** (→ Option B), or just **zero meaningful S3 cost** (→ Option A lifecycle rule is simpler)? Memory says hard "no line items," so Option B assumed.
2. Is the AWS account legacy (pre-2025-07-15, 12-mo trial) or new (6-mo credits)? Determines whether S3 is already billing today or still inside a free window.
3. OK with benign CloudFormation drift on Lambda code for code-only deploys?
4. Should direct-upload become the canonical CI path, or only a manual/emergency path with SAM staying primary?

## Sources

- [AWS Free Tier in 2026 — what changed / always-free services](https://infratally.com/articles/aws-free-tier-2026/)
- [What's New in AWS Free Tier (2025)](https://dev.to/aws-builders/whats-new-in-aws-free-tier-2025-2ba5)
- [AWS Lambda pricing / free tier](https://aws.amazon.com/pm/lambda/)
- [Amazon CloudWatch pricing (custom metrics, free tier)](https://aws.amazon.com/cloudwatch/pricing/)
- [AWS X-Ray pricing (100k traces/mo free)](https://aws.amazon.com/xray/pricing/)
- [Amazon S3 pricing (no always-free storage tier)](https://aws.amazon.com/s3/pricing/)
- [SAM package — S3 required for artifacts > 51,200 bytes](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/sam-cli-command-reference-sam-package.html)
- [CloudFormation Lambda Function Code — ZIP requires S3, inline is Node/Python only](https://docs.aws.amazon.com/AWSCloudFormation/latest/TemplateReference/aws-properties-lambda-function-code.html)
- [lambda update-function-code (direct ZIP upload)](https://docs.aws.amazon.com/cli/latest/reference/lambda/update-function-code.html)
