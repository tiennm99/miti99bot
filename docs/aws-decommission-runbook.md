# AWS Decommission Runbook

Delete **everything** `miti99bot` ever deployed to AWS, after the migration +
cutover to Coolify is verified. Run by the operator with the `admin` profile.

> **Precondition (hard):** run ONLY after the Phase 4 cutover —
> `make migrate-verify` green, bot confirmed live on Coolify via long polling,
> and the EventBridge schedule already disabled at cutover. `sam delete`
> destroys the DynamoDB table, which is the sole copy of prod data until
> migrated. Never run this standalone.

Account `225603493174`, region `ap-southeast-1` (verified live 2026-06-27).

## What `sam delete` removes (CloudFormation-managed)

DynamoDB table `miti99bot-data`, the Lambda + Function URL + invoke
permissions, the Lambda execution role and `SchedulerExecutionRole`, the
`/aws/lambda/miti99bot` log group + metric filter, the `miti99bot-cron-dlq`
SQS queue, the `miti99bot-lolschedule-daily-push` schedule, and the
`miti99bot-monthly` budget (if `AlertEmail` was set).

## What it does NOT remove (created manually, outside CloudFormation)

These linger — and the SSM secrets keep your bot token / Gemini key in the
cloud — unless deleted separately:

- **SSM SecureStrings** (exactly 4): `/miti99bot/prod/telegram-bot-token`,
  `/miti99bot/prod/telegram-webhook-secret`, `/miti99bot/prod/gemini-api-key`,
  `/miti99bot/prod/cron-shared-secret`. **Deleting these is the security step.**
- **IAM role** `github-deploy-miti99bot` + inline policy `miti99bot-deploy`.
- **IAM OIDC provider** `token.actions.githubusercontent.com` — verified the
  account's only OIDC provider and used solely by miti99bot → safe to delete.
- **SAM deploy bucket** `aws-sam-cli-managed-default-samclisourcebucket-ctwpsmoxnwvm`
  + bootstrap stack `aws-sam-cli-managed-default` — verified miti99bot is the
  sole SAM project → safe to delete.

## Runbook

```sh
AWS_PROFILE=admin; REGION=ap-southeast-1; ACCT=225603493174

# 1. Safety check — stack still exists (about to be deleted).
aws --profile $AWS_PROFILE cloudformation describe-stacks --stack-name miti99bot \
  --query "Stacks[0].StackStatus"

# 2. Delete the CloudFormation stack.
aws --profile $AWS_PROFILE sam delete --stack-name miti99bot --region $REGION --no-prompts
aws --profile $AWS_PROFILE cloudformation wait stack-delete-complete --stack-name miti99bot

# 3. Delete SSM secrets (NOT CFN-managed). List first, then delete.
aws --profile $AWS_PROFILE ssm get-parameters-by-path --path /miti99bot --recursive \
  --query "Parameters[].Name" --output text
for P in telegram-bot-token telegram-webhook-secret gemini-api-key cron-shared-secret; do
  aws --profile $AWS_PROFILE ssm delete-parameter --name /miti99bot/prod/$P
done
#   delete any extra /miti99bot/* the list revealed

# 4. Delete the GitHub deploy IAM role (inline policy first).
aws --profile $AWS_PROFILE iam delete-role-policy \
  --role-name github-deploy-miti99bot --policy-name miti99bot-deploy
aws --profile $AWS_PROFILE iam delete-role --role-name github-deploy-miti99bot

# 5. OIDC provider — re-confirm it's the only one, then delete.
aws --profile $AWS_PROFILE iam list-open-id-connect-providers
aws --profile $AWS_PROFILE iam delete-open-id-connect-provider \
  --open-id-connect-provider-arn arn:aws:iam::$ACCT:oidc-provider/token.actions.githubusercontent.com

# 6. SAM deploy bucket + bootstrap stack — re-confirm only miti99bot +
#    aws-sam-cli-managed-default stacks exist first.
aws --profile $AWS_PROFILE cloudformation list-stacks \
  --query "StackSummaries[?StackStatus!='DELETE_COMPLETE'].StackName" --output text
aws --profile $AWS_PROFILE s3 rb \
  s3://aws-sam-cli-managed-default-samclisourcebucket-ctwpsmoxnwvm --force
aws --profile $AWS_PROFILE cloudformation delete-stack --stack-name aws-sam-cli-managed-default

# 7. Confirm nothing tagged app=miti99bot remains.
aws --profile $AWS_PROFILE resourcegroupstaggingapi get-resources \
  --tag-filters Key=app,Values=miti99bot --region $REGION
aws --profile $AWS_PROFILE cloudformation list-stacks \
  --query "StackSummaries[?contains(StackName,'miti99bot')].[StackName,StackStatus]" --output table
```

Then in the repo: the `.github/workflows/deploy.yml` AWS deploy is disabled on
the `feature/selfhosted` branch (the trigger is removed so a `main` push can't
recreate the stack).

## Verification checklist

- [ ] `describe-stacks --stack-name miti99bot` → does not exist.
- [ ] No `/miti99bot/*` SSM parameters remain (secrets purged).
- [ ] `github-deploy-miti99bot` role gone; OIDC provider gone.
- [ ] SAM bucket + bootstrap stack deleted.
- [ ] `resourcegroupstaggingapi` for `app=miti99bot` returns empty.
- [ ] `deploy.yml` no longer recreates the stack on `main`.
- [ ] Cost Explorer shows $0 the following billing period.
- [x] Cloudflare verified clean (2026-06-27): legacy KV/D1 already gone; the 4
      remaining Workers are separate active projects. No action.
- [ ] GCP: no `miti99bot-prod` project (`gcloud projects list | grep miti99bot`).

## Notes

- **Secret hygiene (optional):** rotate the Telegram bot token + Gemini key
  after teardown — they lived in SSM/CloudWatch under accepted trade-offs.
- **Keep in git:** the `aws/` dir + `template.yaml` history cost nothing and are
  useful if AWS is ever revisited.
