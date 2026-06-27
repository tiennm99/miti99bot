---
phase: 5
title: "AWS Full Decommission"
status: pending
priority: P1
dependencies: [4]
effort: "S"
---

# Phase 5: AWS Full Decommission

## Overview

After migration + cutover verify (Phase 4), delete **everything** miti99bot ever deployed to AWS, plus verify/clean the legacy pre-AWS services (Cloudflare, GCP — section D). The trap: `sam delete` only removes CloudFormation-managed resources — the GitHub OIDC IAM role, the SSM Parameter Store secrets, and SAM's own S3 deploy bucket were created **manually outside CloudFormation** (`aws/README.md` steps 2-4) and must be deleted separately or they linger (and the secrets keep your bot token/Gemini key sitting in the cloud). GitHub itself is already clean (verified: no stored secrets/keys — AWS deploy used OIDC only).

This phase is informed by a git-history scan of every `template.yaml` version + the `aws/` setup docs, **verified against the live account on 2026-06-27 (admin profile)**. Account `225603493174`, region `ap-southeast-1`.

**Live verification (2026-06-27, read-only):** stack `miti99bot` = `UPDATE_COMPLETE`. The account has exactly TWO CloudFormation stacks (`miti99bot` + `aws-sam-cli-managed-default`) and ONE OIDC provider — so **miti99bot is the sole SAM project AND the sole OIDC consumer**, which makes the shared-resource deletes (OIDC provider, SAM bucket) safe and unconditional. SSM holds exactly the 4 known secrets (no FireAnt/VNAppMob extras). SAM deploy bucket: `aws-sam-cli-managed-default-samclisourcebucket-ctwpsmoxnwvm`.

## Requirements

- Functional: zero miti99bot resources remain in AWS after this phase; Cost Explorer trends to $0.
- Functional: run ONLY after Phase 4 `--verify` passes and the bot is confirmed live on Coolify (DynamoDB holds the only copy of prod data until migrated).
- Non-functional: do NOT delete account-shared resources (OIDC provider, SAM bucket) if other stacks/repos use them — verify first.
- Non-functional: executed by the user with the `admin` profile (this session has no AWS credentials); plan provides the exact commands.

## Architecture — Complete Resource Inventory

### A. CloudFormation-managed → removed by `sam delete --stack-name miti99bot`
From the current `template.yaml`:
- `AWS::DynamoDB::Table` — `miti99bot-data` (**destroyed — must be migrated first**)
- `AWS::Serverless::Function` — `miti99bot` + its Function URL + the LWA layer reference
- `AWS::Logs::LogGroup` — `/aws/lambda/miti99bot` (+ `AWS::Logs::MetricFilter` ColdStartInitDuration)
- `AWS::SQS::Queue` — `miti99bot-cron-dlq`
- `AWS::IAM::Role` — `SchedulerExecutionRole` AND the SAM-auto-generated Lambda execution role `miti99bot-BotFunctionRole-*` (both CFN-managed; `sam delete` removes both — no separate action) + the public Function-URL invoke `AWS::Lambda::Permission`s
- `AWS::Scheduler::Schedule` — `miti99bot-lolschedule-daily-push`
- `AWS::Budgets::Budget` — `miti99bot-monthly` (only if AlertEmail was set)

Historical (git history shows earlier `template.yaml` used these, since replaced by Scheduler): `AWS::Events::Rule`, `AWS::Events::Connection`, `AWS::Events::ApiDestination`. CloudFormation deleted them when the template changed, so they are NOT orphans — but a post-delete tag sweep (step 6) confirms.

### B. Created manually OUTSIDE CloudFormation → `sam delete` does NOT touch these
From `aws/README.md`:
- **SSM Parameter Store SecureStrings** (`aws/README.md:23-39`, verified — exactly these 4, no extras): `/miti99bot/prod/telegram-bot-token`, `/miti99bot/prod/telegram-webhook-secret`, `/miti99bot/prod/gemini-api-key`, `/miti99bot/prod/cron-shared-secret`. **These hold live secrets — deleting them is the security-relevant step.**
- **IAM role** `github-deploy-miti99bot` + inline policy `miti99bot-deploy` (`aws/README.md:57-73`).
- **IAM OIDC identity provider** `token.actions.githubusercontent.com` (`aws/README.md:43-53`) — verified as the account's ONLY OIDC provider and used solely by miti99bot → **safe to delete.**
- **SAM managed S3 deploy bucket** `aws-sam-cli-managed-default-samclisourcebucket-ctwpsmoxnwvm` + its bootstrap stack `aws-sam-cli-managed-default` — verified miti99bot is the sole SAM project → **safe to delete the bucket + bootstrap stack.**
- **Bootstrap `admin` IAM user + access keys** (`aws/README.md:15`) — user's discretion; out of scope unless they want a full account wipe.

### C. Not deletable / self-expiring (no action)
- Custom CloudWatch metric namespace `miti99bot` (metrics age out; not billable, not deletable).

### D. Legacy services from the pre-AWS lineage (verify + clean — not AWS)
The bot's history is **Cloudflare Worker (original) → [Cloud Run port, abandoned] → AWS → Coolify (this plan)**. Two legacy footprints to confirm:
- **Cloudflare (verified live 2026-06-27, account `miti99` / `7774466151858e13a3c482af5f9ccd6b`):**
  - **KV namespaces**: only `claude-status` remains — NO miti99bot KV → the legacy bot KV namespace was already deleted. ✅ no action.
  - **D1 databases**: none → legacy `trading_trades` D1 already deleted. ✅ no action.
  - **Workers**: 4 exist — `claude-status-webhook`, `rplace`, `miti-loki` (Grafana Loki log-shipper), and `miti-telegram`. All confirmed (user, 2026-06-27) as separate active projects, NOT the legacy miti99bot. `miti-telegram` is a different single-chat notifier the user still uses.
  - **Conclusion: Cloudflare is fully clean — NO action.** The legacy miti99bot Worker was already deleted during the May 2026 migration; only its data stores (KV/D1) were ever in scope and both are gone.
- **GCP project `miti99bot-prod`** — almost certainly never created (Cloud Run port superseded before phase-01 ran; no bootstrap artifacts ever existed). Optional certainty check: `gcloud projects list | grep miti99bot` → if a project exists, `gcloud projects delete miti99bot-prod`. Low probability.

## Related Code Files

- Create: `docs/aws-decommission-runbook.md` — the ordered teardown commands below (so it's repeatable + auditable).
- Modify: `.github/workflows/deploy.yml` — **delete or disable** (the AWS deploy path is retired; otherwise a push to `main` re-creates the stack). On the `feature/selfhosted` branch this file is replaced by the Coolify flow.
- Modify: `README.md` / `docs/deploy-aws*.md` — mark the AWS deployment path as retired, point to the Coolify guide.
- Keep (do not delete): `aws/` dir + `template.yaml` in git history — useful if AWS is ever revisited; they cost nothing as files.

## Implementation Steps (runbook — user runs with `admin` profile)

Precondition: Phase 4 done — data migrated, `--verify` green, bot live on Coolify, Telegram webhook pointed at Coolify, EventBridge schedule already disabled at cutover.

```sh
AWS_PROFILE=admin; REGION=ap-southeast-1; ACCT=225603493174

# 1. Final safety check — confirm bot is NOT serving from AWS anymore
aws --profile $AWS_PROFILE cloudformation describe-stacks --stack-name miti99bot \
  --query "Stacks[0].StackStatus"   # exists → about to be deleted

# 2. Delete the CloudFormation stack (DynamoDB, Lambda, FunctionUrl, Logs, SQS, Scheduler, IAM role, Budget)
aws --profile $AWS_PROFILE sam delete --stack-name miti99bot --region $REGION --no-prompts
#   (or: aws cloudformation delete-stack --stack-name miti99bot ; then wait)
aws --profile $AWS_PROFILE cloudformation wait stack-delete-complete --stack-name miti99bot

# 3. Delete SSM secrets (NOT managed by CFN)
aws --profile $AWS_PROFILE ssm get-parameters-by-path --path /miti99bot --recursive \
  --query "Parameters[].Name" --output text                     # list what exists first
for P in telegram-bot-token telegram-webhook-secret gemini-api-key cron-shared-secret; do
  aws --profile $AWS_PROFILE ssm delete-parameter --name /miti99bot/prod/$P
done
#   delete any extra /miti99bot/* the list in step 3 revealed

# 4. Delete the GitHub deploy IAM role (inline policy first, then role)
aws --profile $AWS_PROFILE iam delete-role-policy \
  --role-name github-deploy-miti99bot --policy-name miti99bot-deploy
aws --profile $AWS_PROFILE iam delete-role --role-name github-deploy-miti99bot

# 5. OIDC provider — verified sole consumer = miti99bot, safe to delete.
#    (Re-confirm it's still the only one before deleting, in case the account changed.)
aws --profile $AWS_PROFILE iam list-open-id-connect-providers
aws --profile $AWS_PROFILE iam delete-open-id-connect-provider \
  --open-id-connect-provider-arn arn:aws:iam::$ACCT:oidc-provider/token.actions.githubusercontent.com

# 6. SAM S3 deploy bucket + bootstrap stack — verified miti99bot is the sole SAM project.
#    (Re-confirm only miti99bot + aws-sam-cli-managed-default stacks exist first.)
aws --profile $AWS_PROFILE cloudformation list-stacks \
  --query "StackSummaries[?StackStatus!='DELETE_COMPLETE'].StackName" --output text
aws --profile $AWS_PROFILE s3 rb \
  s3://aws-sam-cli-managed-default-samclisourcebucket-ctwpsmoxnwvm --force
aws --profile $AWS_PROFILE cloudformation delete-stack --stack-name aws-sam-cli-managed-default

# 7. Verify nothing tagged app=miti99bot remains
aws --profile $AWS_PROFILE resourcegroupstaggingapi get-resources \
  --tag-filters Key=app,Values=miti99bot --region $REGION
aws --profile $AWS_PROFILE cloudformation list-stacks \
  --query "StackSummaries[?contains(StackName,'miti99bot')].[StackName,StackStatus]" --output table
```

Then in the repo (on `feature/selfhosted`): remove/disable `.github/workflows/deploy.yml` and mark AWS docs retired.

## Success Criteria

- [ ] `cloudformation describe-stacks --stack-name miti99bot` → does not exist (DELETE_COMPLETE).
- [ ] No `/miti99bot/*` SSM parameters remain (secrets purged from cloud).
- [ ] `github-deploy-miti99bot` role gone; OIDC provider gone OR confirmed still needed by another project.
- [ ] SAM bucket emptied/deleted OR `miti99bot/` prefix cleared (and rationale recorded).
- [ ] `resourcegroupstaggingapi get-resources` for `app=miti99bot` returns empty.
- [ ] `.github/workflows/deploy.yml` removed/disabled so `main` pushes no longer recreate the stack.
- [ ] Cost Explorer shows $0 for the following billing period.
- [x] Cloudflare verified clean (2026-06-27): KV + D1 already gone; all 4 Workers confirmed as separate active projects (no legacy bot remnant). No action.
- [ ] GCP checked: no `miti99bot-prod` project exists (or deleted).
- [ ] GitHub confirmed clean: no AWS secrets/keys stored; `deploy-aws.yml` removed (only OIDC was used, nothing to revoke beyond the role deleted above).

## Risk Assessment

- **Deleting before migration completes (Critical)**: `sam delete` destroys `miti99bot-data` (DynamoDB). Mitigation: hard dependency on Phase 4 `--verify`; runbook step 1 is an explicit precondition check. Never run Phase 5 standalone.
- **Deleting shared account resources**: verified on 2026-06-27 that miti99bot is the sole OIDC consumer and sole SAM project, so steps 5-6 are safe. Mitigation: the runbook re-confirms (list-first) before each delete in case the account changes before you run it; if a new stack/provider appears, leave the shared resource (it costs ~nothing).
- **Re-creation by CI**: a `main` push with `deploy.yml` still active would rebuild the whole stack post-teardown. Mitigation: disable/delete the workflow as part of this phase (and it's already superseded on `feature/selfhosted`).
- **Lost rollback**: per the Phase 4 validated decision, teardown ends the AWS rollback option. Mitigation: that was explicitly accepted; optionally keep the stack a few days before running step 2.
- **Secret hygiene**: rotate the Telegram bot token + Gemini key after teardown if you want defense-in-depth (they lived in SSM and CloudWatch under the accepted trade-offs). Optional.
