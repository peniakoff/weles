# Deploying Weles on AWS

Target region for reference deployments: **`eu-central-1`**.

Stack: **AWS SAM** → API Gateway **HTTP API** → Lambda **`provided.al2023` arm64** → **Amazon SES** plaintext email.

## Prerequisites

- AWS account and credentials (prefer GitHub Actions OIDC — see below)
- [AWS SAM CLI](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/install-sam-cli.html)
- Go 1.24+ (for `sam build` Makefile target)
- Cloudflare Turnstile site + secret
- Operator email inbox you control
- A **verified SES identity** for the From address (email click verification or domain DKIM) in the deploy region

## Parameters

| Parameter | Description |
|-----------|-------------|
| `NotificationEmail` | Operator inbox (SES **To** address) |
| `FromEmail` | Optional verified SES **From** address; empty means use `NotificationEmail` as From |
| `SesIdentity` | Optional SES identity name for the Lambda IAM policy (email **or** verified domain). Empty means use the From address. When only a **domain** identity is verified, set this to the domain (e.g. `example.com`) while `FromEmail` is an address on that domain |
| `AppsParameterName` | SSM parameter for apps YAML (default `/weles/prod/apps`, **must start with `/`**) |
| `TurnstileParameterName` | SSM SecureString for Turnstile secret (default `/weles/prod/turnstile-secret`, **must start with `/`**) |
| `StageName` | Default `prod` |
| `ThrottleRate` / `ThrottleBurst` | Defaults `1` / `5` |

**Do not commit** production allowlists or secrets. Publish them to SSM before (or via) deploy:

```bash
aws ssm put-parameter \
  --name /weles/prod/apps \
  --type String \
  --value "$(cat config/apps.yaml)" \
  --overwrite

aws ssm put-parameter \
  --name /weles/prod/turnstile-secret \
  --type SecureString \
  --value '0x...' \
  --overwrite
```

`config/apps.yaml` is gitignored. Use `config/apps.example.yaml` as the public template.

```bash
sam build
sam deploy \
  --guided \
  --parameter-overrides \
    "NotificationEmail=ops@example.com" \
    "FromEmail=noreply@example.com" \
    "SesIdentity=example.com" \
    "AppsParameterName=/weles/prod/apps" \
    "TurnstileParameterName=/weles/prod/turnstile-secret"
```

The Turnstile secret is **not** injected as a Lambda environment variable. The function reads it from SSM at startup (`WELLS_TURNSTILE_SSM`).

The stack does **not** create an `AWS::SES::EmailIdentity`. Verify From (and, while the account is in the SES sandbox, also To) in the Amazon SES console or CLI before the first successful send. Leaving `FromEmail` empty uses `NotificationEmail` for both From and To — the easy sandbox path when that single address is verified.

**Domain vs email identity:** the Lambda IAM policy allows `ses:SendEmail` only on `arn:…:identity/${SesIdentity}` (or the From address when `SesIdentity` is empty). If you verified a **domain** identity and send as `noreply@example.com`, set `SesIdentity=example.com`. If you verified the **email address** itself, leave `SesIdentity` empty.

### Allowlist reload

When `WELLS_APPS_SSM` is set, Weles reloads the apps registry about every **5 minutes** by default (`WELLS_APPS_RELOAD_SECONDS=300` in the SAM template). Edits to the SSM apps parameter apply without a full redeploy once the TTL elapses (or the Lambda instance is replaced). Set `WELLS_APPS_RELOAD_SECONDS=0` to load once per cold start only.

### Migrating from an SNS-based stack

The first deploy of this SES revision **deletes** the former SNS topic and email subscription from the CloudFormation stack. Confirm the SES From identity before relying on mail delivery.

## After first deploy

1. Verify the SES From identity (and To if still in sandbox).
2. Note stack outputs `FeedbackURL` and `ApiEndpoint`.
3. Point integrating apps at `FeedbackURL`.
4. Smoke-test from an allowlisted origin with a real Turnstile token.

## Local / Docker

```bash
docker compose up --build
# Defaults: Turnstile mode=skip, notifier=stdout — for local use only.
# Never expose that default image to the public Internet without cloudflare mode + secret.
```

## GitHub Actions (OIDC)

Workflow [`.github/workflows/ci.yml`](../.github/workflows/ci.yml) runs on every push and pull request to `main`: `go vet`, race tests, `make build`, `make lambda-build`, and `sam validate --lint`.

Deploy workflow [`.github/workflows/deploy.yml`](../.github/workflows/deploy.yml) uses OIDC. It writes `APPS_CONFIG` (String) and `TURNSTILE_SECRET` (SecureString) to SSM (normalizing names to start with `/`), then runs `sam deploy`.

### GitHub variables

- `AWS_REGION` (e.g. `eu-central-1`)
- `AWS_ROLE_TO_ASSUME` (IAM role ARN trusting `token.actions.githubusercontent.com`)
- `APPS_PARAMETER_NAME` (optional; default `/weles/prod/apps`)
- `TURNSTILE_PARAMETER_NAME` (optional; default `/weles/prod/turnstile-secret`)
- `FROM_EMAIL` (optional; verified SES From; when unset, `NotificationEmail` is used as From)
- `SES_IDENTITY` (optional; SES identity name for IAM — email or domain; when unset, uses From)

### GitHub secrets / environment secrets

- `NOTIFICATION_EMAIL`
- `TURNSTILE_SECRET`
- `APPS_CONFIG` (full apps YAML string)

### IAM role

Trust the repository (`repo:OWNER/weles:ref:refs/heads/main` and/or `environment:production`). Attach permissions for CloudFormation, SAM, Lambda, API Gateway, SES (`ses:SendEmail` on the `SesIdentity` or From identity), IAM pass-role, CloudWatch Logs (including `logs:PutResourcePolicy` / `logs:DeleteResourcePolicy` if needed for access-log policies), S3 for SAM artifacts, and `ssm:PutParameter` / `ssm:GetParameter` on `/weles/*`.

## Custom domain

Not included in v1. Later: ACM certificate + API Gateway custom domain (see [ROADMAP.md](../ROADMAP.md)).

## Cost note

At very low volume (a handful of submissions per week), HTTP API + Lambda + SES typically stay within free-tier / sub-cent territory. Avoid attaching AWS WAF until abuse requires it (Web ACL has a fixed monthly cost).
