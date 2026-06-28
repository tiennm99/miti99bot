# AWS Footprint & Cleanup Audit

Scout of full git history (init `328ce45` → now, 154 commits) + current tree.
Question: what was ever created with AWS, and is everything AWS-related cleaned up?

**Bottom line:** Cloud resources = 100% deleted + verified (2026-06-28). Repo
AWS code/IaC = intentionally retained for history (not cruft), but AWS SDK still
compiles into the server binary (dormant). No orphaned cloud resources found.

## AWS timeline

- 2026-05-10 (`c07d764`): AWS introduced — SAM template, Lambda/DynamoDB/EventBridge, GHA OIDC. Region `ap-southeast-1`, acct `225603493174`.
- 2026-05-18 (`2d1864f`): cron settled on EventBridge **Scheduler** (rejected Rule+ApiDestination for cost).
- 2026-05-25 (`daeaf0c`): CF→AWS migration tooling deleted post-migration.
- 2026-06-27/28: Coolify+MongoDB self-host; `b0b4b4b` disabled AWS deploy workflow.
- 2026-06-28 (`3e8d6bf`): DynamoDB Go backend deleted from codebase.
- 2026-06-28 (`3023272`): AWS cloud resources deleted per runbook (this session). ~49 days operational.

## Everything ever created on AWS

CloudFormation-managed (removed by `sam delete`):
DynamoDB `miti99bot-data` · Lambda `miti99bot` + Function URL + invoke perms ·
Lambda exec role + `SchedulerExecutionRole` · EventBridge Scheduler
`miti99bot-lolschedule-daily-push` · SQS `miti99bot-cron-dlq` · log group
`/aws/lambda/miti99bot` + cold-start metric filter · budget `miti99bot-monthly`.

Manual (operator-deleted this session):
4 SSM SecureStrings (`/miti99bot/prod/{telegram-bot-token,telegram-webhook-secret,gemini-api-key,cron-shared-secret}`) ·
IAM role `github-deploy-miti99bot` (+10 attached managed policies) ·
OIDC provider `token.actions.githubusercontent.com` ·
SAM S3 bucket `aws-sam-cli-managed-default-samclisourcebucket-ctwpsmoxnwvm` + bootstrap stack.

Never created (verified, no gaps): ECR, extra S3, KMS (used AWS-managed key via
SSM), Secrets Manager, Route53, VPC/NAT, CloudWatch alarms, DynamoDB PITR.
Cloudflare verified clean 2026-06-27. GCP never used.

## Cloud cleanup status — COMPLETE

All 8 runbook checks pass (stack gone, SSM empty, IAM role gone, OIDC gone, no
non-deleted stacks, no `app=miti99bot` tagged resources, SAM bucket 404, log
group empty). Self-expiring/non-billable only: custom CloudWatch metric
namespace `miti99bot` (ages out, not deletable via API — no action).

## Repo AWS traces still present (by design vs removable)

RETIRED-BUT-PRESENT (kept for history per README + runbook decision — zero cost):
- `template.yaml`, `samconfig.toml` — SAM IaC, points to deleted stack.
- `aws/` — IAM policy/trust JSON, rollback script, README, telegram-commands.json.
- `.github/workflows/deploy.yml` — trigger neutralized to `workflow_dispatch`-only; depends on deleted OIDC role so a manual run fails.
- `docs/deploy-aws.md`, `docs/deploy-aws-free-tier-guide.md`, `docs/aws-decommission-runbook.md`.
- Makefile `sam-*` + AWS `telegram-webhook*` targets.

LIVE-BUT-DORMANT (AWS code compiled into the running server, never executes on Coolify):
- `cmd/server/main.go` — `resolveSSMSecrets()` runs at startup but short-circuits
  when no `*_PARAMETER_NAME` env vars set (Coolify uses plain env vars → no AWS call).
- `go.mod` — 4 direct AWS SDK deps still required: `aws-sdk-go-v2`, `config`,
  `service/dynamodb`, `service/ssm` (+ transitive sts/sso/credentials/etc).

MIGRATION-ONLY (one-off, job done):
- `cmd/migrate-dynamo-to-mongo/` — standalone migrator binary (not imported by server).
- `internal/storage/dynamodb_client.go` — only used by the migrator; dead in server.

CI (active, AWS-agnostic): `ci.yml` runs `sam validate` **offline** — no AWS auth.

## Optional further cleanup (repo purge, if a full AWS removal is desired)

Removing these would drop the AWS SDK from the build and delete dead code:
1. Delete `cmd/migrate-dynamo-to-mongo/` + `internal/storage/dynamodb_client.go`.
2. Strip `resolveSSMSecrets` + `*_PARAMETER_NAME` config from `cmd/server/main.go`.
3. `go mod tidy` → removes 4 direct + ~13 transitive AWS SDK deps.
4. Optionally delete `template.yaml`, `samconfig.toml`, `aws/`, `deploy.yml`, AWS docs, Makefile `sam-*` targets.
Trade-off: loses the "re-bootstrap AWS" path and migration tool. README + runbook
currently document keeping them intentionally.

## Unresolved questions

- Keep AWS IaC/code for history (current stated policy), or do a full repo purge now that migration is done + verified? (Items above are the scope.)
- Cost Explorer $0 confirmation deferred to next billing period (not actionable now).
- Optional: rotate Telegram bot token + Gemini key (they lived in SSM/CloudWatch).
