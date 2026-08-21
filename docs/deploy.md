# Deploying Weles on AWS

Target region for reference deployments: **`eu-central-1`**.

Stack: **AWS SAM** → API Gateway **HTTP API** → Lambda **`provided.al2023` arm64** → **Amazon SES** plaintext email.

## Prerequisites

- AWS account and credentials (prefer GitHub Actions OIDC — see below)
- [AWS SAM CLI](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/install-sam-cli.html)
- Go 1.24+ (for `sam build` Makefile target)
- Cloudflare Turnstile site + secret
- Operator email inbox you control
- A **verified SES identity** (prefer a domain) covering every app `fromEmail` in the deploy region

## Parameters

| Parameter | Description |
|-----------|-------------|
| `SesIdentityArns` | **Required.** One or more full SES identity ARNs for Lambda IAM (`ses:SendEmail`). Deploy CI builds this from GitHub var `SES_IDENTITIES` (comma-separated domains or emails). Example: two domains → two ARNs |
| `SesConfigurationSet` | SES configuration set name allowed for `ses:SendEmail` (default `default-configuration`). Set to `*` to allow any set. Account default sets apply even when the API omits `ConfigurationSetName` |
| `AppsParameterName` | SSM parameter for apps YAML (default `/weles/prod/apps`, **must start with `/`**) |
| `TurnstileParameterName` | SSM SecureString for Turnstile secret (default `/weles/prod/turnstile-secret`, **must start with `/`**) |
| `StageName` | Default `prod`. Public URLs include this segment (`…/prod/healthz`). Lambda strips it before the Go mux via `WELES_API_STAGE`. Use `$default` only if you want URLs without a stage prefix. |
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

`config/apps.yaml` is gitignored. Use `config/apps.example.yaml` as the public template. **Required** per-app fields:

- `notificationEmail` — SES **To** (operator inbox) for that app
- `fromEmail` — SES **From** for that app (must be covered by one of the identities in `SesIdentityArns` / `SES_IDENTITIES`)

Apps may use **different domains** for `fromEmail`. List every verified domain (or email identity) in `SES_IDENTITIES`. Adding a new domain requires verifying it in SES and redeploying with an updated `SES_IDENTITIES` list; the apps YAML reload alone does not widen IAM.

```bash
# Example: allow From on two verified domains (deploy CI expands names to ARNs).
ACCOUNT=$(aws sts get-caller-identity --query Account --output text)
REGION=eu-central-1
sam build
sam deploy \
  --guided \
  --parameter-overrides \
    "SesIdentityArns=arn:aws:ses:${REGION}:${ACCOUNT}:identity/example.com,arn:aws:ses:${REGION}:${ACCOUNT}:identity/other.example.com" \
    "SesConfigurationSet=default-configuration" \
    "AppsParameterName=/weles/prod/apps" \
    "TurnstileParameterName=/weles/prod/turnstile-secret"
```

The Turnstile secret is **not** injected as a Lambda environment variable. The function reads it from SSM at startup (`WELES_TURNSTILE_SSM`).

The stack does **not** create an `AWS::SES::EmailIdentity`. Verify each app `fromEmail` (or its domain listed in `SES_IDENTITIES`) in the Amazon SES console or CLI before the first successful send. While the account is in the SES sandbox, also verify each `notificationEmail` (To).

**Domain vs email identity:** the Lambda IAM policy allows `ses:SendEmail` on each ARN in `SesIdentityArns` **and** on `arn:…:configuration-set/${SesConfigurationSet}` (default `default-configuration`). For a domain identity, set `SES_IDENTITIES=example.com` (or include that ARN) and use `fromEmail` addresses on that domain. For a single email identity, list that address. Multiple apps on different domains → comma-separated list, e.g. `example.com,other.example.com`.

If the SES account has a **default configuration set**, SES v2 `SendEmail` authorizes that set even when the API call does not pass `ConfigurationSetName`. Without matching configuration-set IAM, delivery fails with `AccessDeniedException`. Set `SesConfigurationSet` to that set's name, or `*` to allow any set.

### Allowlist reload

When `WELES_APPS_SSM` is set, Weles reloads the apps registry about every **5 minutes** by default (`WELES_APPS_RELOAD_SECONDS=300` in the SAM template). Edits to the SSM apps parameter apply without a full redeploy once the TTL elapses (or the Lambda instance is replaced). Set `WELES_APPS_RELOAD_SECONDS=0` to load once per cold start only.

### Migrating from stack-level NotificationEmail / FromEmail

**Do this before merging/deploying this revision**, or Lambda will fail to start / SES will return AccessDenied:

1. Update GitHub secret `APPS_CONFIG`: every app must include `notificationEmail` and `fromEmail` (see `config/apps.example.yaml`).
2. Set GitHub var `SES_IDENTITIES` to a comma-separated list of every verified SES domain or email identity that covers those `fromEmail` values (example: `example.com,other.example.com`). `SES_IDENTITY` still works as a single-value alias.
3. Remove unused `NOTIFICATION_EMAIL` secret and `FROM_EMAIL` var (and any local `samconfig` overrides for `NotificationEmail` / `FromEmail` / `SesIdentity`).
4. Deploy CI runs `go run ./cmd/deploycheck` **before** writing SSM: invalid apps YAML, missing emails, or `fromEmail` outside `SES_IDENTITIES` fails the job (no partial outage from a bad allowlist push).
5. After deploy: smoke-test `GET /healthz` and one real feedback submit per integrating app.

Older stacks passed `NotificationEmail` and optional `FromEmail` as SAM parameters. Those parameters are removed in favor of the apps registry + `SesIdentityArns` (built from `SES_IDENTITIES`).

Local check (same logic as CI):

```bash
go run ./cmd/deploycheck \
  -apps config/apps.yaml \
  -identities "example.com,other.example.com" \
  -region eu-central-1 \
  -account 123456789012 \
  -print-arns
```


### Migrating from an SNS-based stack

The first deploy of the SES revision **deletes** the former SNS topic and email subscription from the CloudFormation stack. Confirm the SES From identity before relying on mail delivery.

## After first deploy

1. Verify the SES identity for every app `fromEmail` (and To if still in sandbox).
2. Note stack outputs `FeedbackURL` and `ApiEndpoint`.
3. Point integrating apps at `FeedbackURL`.
4. Smoke-test liveness: `GET {ApiEndpoint}/healthz` should return JSON `{"status":"ok"}`.
5. Smoke-test from an allowlisted origin with a real Turnstile token (once per app / From domain if you use multiple identities).

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
- `SES_IDENTITIES` (**required** unless `SES_IDENTITY` is set; comma-separated SES domains or emails for IAM — may list multiple domains)
- `SES_IDENTITY` (optional alias for a single identity when `SES_IDENTITIES` is unset)
- `SES_CONFIGURATION_SET` (optional; SES configuration set name for IAM; when unset, `default-configuration`)

### GitHub secrets / environment secrets

- `TURNSTILE_SECRET`
- `APPS_CONFIG` (full apps YAML string, including required `notificationEmail` / `fromEmail` per app)

### IAM role

Trust the repository (`repo:OWNER/weles:ref:refs/heads/main` and/or `environment:production`). Attach permissions for CloudFormation, SAM, Lambda, API Gateway, SES (`ses:SendEmail` on each identity in `SesIdentityArns`), IAM pass-role, CloudWatch Logs (including `logs:PutResourcePolicy` / `logs:DeleteResourcePolicy` if needed for access-log policies), S3 for SAM artifacts, and `ssm:PutParameter` / `ssm:GetParameter` on `/weles/*`.

## Custom domain

Not included in v1. Later: ACM certificate + API Gateway custom domain (see [ROADMAP.md](../ROADMAP.md)).

## Cost note

At very low volume (a handful of submissions per week), HTTP API + Lambda + SES typically stay within free-tier / sub-cent territory. Avoid attaching AWS WAF until abuse requires it (Web ACL has a fixed monthly cost).
