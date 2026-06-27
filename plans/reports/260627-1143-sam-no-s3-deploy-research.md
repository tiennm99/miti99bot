# Research Report: Skip S3 For miti99bot Deploys

Timestamp: 2026-06-27 11:43 UTC

## Executive Summary

Official AWS docs do not show a supported SAM/CloudFormation ZIP deploy path that avoids S3 for this Go Lambda. SAM deploy/package uploads local ZIP artifacts to S3; CloudFormation ZIP functions expect S3 object location, except inline Node.js/Python source.

Official no-S3 path exists only outside SAM/CloudFormation stack code management: `aws lambda update-function-code --zip-file fileb://...`. This repo's current ZIP is ~7.8 MB, below Lambda direct upload limit. Use it only for code-only deploys/hotfixes, or accept drift from CloudFormation.

Best zero-cost-risk answer: keep SAM for infra, but use direct Lambda code upload for routine code deploys if S3 must be eliminated. Run SAM only when infra/config changes, or replace SAM with imperative AWS CLI provisioning. No perfect official "SAM but no S3" option found.

## Codebase Context

- Project: Go 1.25 Lambda app, `provided.al2023`, ARM64.
- Deploy: `.github/workflows/deploy.yml` runs `make build-lambda` then `sam deploy --template-file template.yaml`.
- SAM config: `samconfig.toml` has `resolve_s3 = true` and `s3_prefix = "miti99bot"`.
- CloudFormation code: `template.yaml` uses `CodeUri: build/lambda/`.
- Current artifact: `build/lambda/function.zip` about 7.8 MB, direct upload compatible.

## Official Findings

### 1. SAM ZIP deploy requires S3 for local artifacts

AWS SAM package docs:
- `sam deploy` implicitly performs package.
- `--resolve-s3` creates an S3 bucket for packaging.
- If artifact > 51,200 bytes, `--s3-bucket` or `--resolve-s3` required.

Source: https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/sam-cli-command-reference-sam-package.html

AWS SAM tutorial:
- SAM deploy uploads application files to S3.
- SAM creates an S3 bucket and uploads `.aws-sam` directory.

Source: https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-getting-started-hello-world.html

### 2. CloudFormation ZIP Lambda uses S3, except inline Node/Python

CloudFormation `AWS::Lambda::Function Code`:
- ZIP package can specify S3 object.
- Inline `ZipFile` only for Node.js and Python, max 4 MB.
- Container images use ECR.

Sources:
- https://docs.aws.amazon.com/AWSCloudFormation/latest/TemplateReference/aws-properties-lambda-function-code.html
- https://docs.aws.amazon.com/AWSCloudFormation/latest/TemplateReference/aws-resource-lambda-function.html

This repo is compiled Go (`provided.al2023`), so inline code is not viable.

### 3. Lambda direct ZIP upload skips S3

Lambda Go ZIP docs:
- For ZIP smaller than 50 MB, AWS CLI can upload from local file.
- Larger files must use S3.

AWS CLI docs:
- `aws lambda update-function-code --zip-file fileb://my-function.zip` updates function code directly.

Sources:
- https://docs.aws.amazon.com/lambda/latest/dg/golang-package.html
- https://docs.aws.amazon.com/cli/latest/reference/lambda/update-function-code.html
- https://docs.aws.amazon.com/lambda/latest/dg/troubleshooting-deployment.html

### 4. ECR avoids S3 but is not better for zero-cost private deploys

SAM supports image package deployments via ECR. ECR private repository free tier is limited: 500 MB/month for first year for new ECR customers. Public repositories have larger always-free storage, but public bot images may not be acceptable.

Source: https://aws.amazon.com/ecr/pricing/

## Brainstormed Options

### Option A: Keep SAM + S3, add lifecycle cleanup

Pros:
- Official canonical SAM path.
- CloudFormation remains source of truth.
- Lowest operational risk.

Cons:
- Does not satisfy "skip S3".
- Still has S3 bucket, requests, storage, and possible artifact accumulation.

Use when:
- Accept S3 free tier / credits and control storage lifecycle.

### Option B: Direct Lambda code upload for code-only deploys

Pros:
- Official AWS Lambda path.
- No S3 artifact bucket needed for code updates.
- Current repo artifact fits 50 MB direct upload limit.
- Simple for this single-function app.

Cons:
- Bypasses CloudFormation code state.
- Next SAM deploy can overwrite direct code.
- Need separate logic for infra/config changes.
- Must keep env, role, Function URL, EventBridge, DynamoDB managed elsewhere.

Use when:
- Non-negotiable: no S3 deploy artifact bucket.
- App remains single Lambda and ZIP stays < 50 MB.

### Option C: Replace ZIP Lambda with container image in ECR

Pros:
- Official SAM/CloudFormation path without S3 ZIP artifacts.
- CloudFormation can track `ImageUri`.

Cons:
- Uses ECR, another billable storage service.
- Private ECR free tier is only first-year/limited; image layers can grow fast.
- More build complexity than current Go binary ZIP.

Use when:
- Container image needed for runtime/dependency reasons. Not true now.

### Option D: Fully imperative AWS CLI provisioning, no SAM

Pros:
- Can create/update Lambda with direct local ZIP upload.
- Avoids SAM-managed S3.

Cons:
- You reimplement infra orchestration: IAM roles, Function URL, DynamoDB, EventBridge, logs, budgets, permissions.
- Higher drift risk and maintenance burden.
- Less rollback safety.

Use when:
- Absolute no-S3 policy outranks IaC simplicity.

## Recommendation

No official recommended "SAM deploy ZIP without S3" path exists for this Go Lambda. If S3 must be zero, use:

1. SAM/CloudFormation only for first infra provisioning and infra changes.
2. Direct `aws lambda update-function-code --zip-file` for code-only deploys.
3. Never run SAM for routine code-only changes unless willing to use S3 again.
4. Add a guard/check that fails direct deploy if ZIP >= 50 MB.

This is a compromise, not pure IaC.

## Candidate Command

```sh
make build-lambda
zip -j build/lambda/function.zip build/lambda/bootstrap
aws lambda update-function-code \
  --function-name miti99bot \
  --zip-file fileb://build/lambda/function.zip
aws lambda wait function-updated --function-name miti99bot
```

## Kill Criteria

- ZIP reaches 50 MB.
- Need code + env/config changes in one atomic deploy.
- Need CloudFormation to be sole source of truth.
- Need Lambda versions/aliases/code signing workflow through IaC.

## Unresolved Questions

- Do you want zero S3 resources, or just zero S3 cost risk?
- Are you okay with CloudFormation drift for code-only deploys?
- Should direct upload become canonical CI deploy, or only emergency/manual path?
