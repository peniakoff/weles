# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Feedback intake API (`POST /v1/feedback`) with Cloudflare Turnstile verification, per-app origin and host allowlisting, and Amazon SES plaintext email delivery (verified From/To; optional Reply-To when the submitter provides an email).
- Dual runtime: AWS Lambda (`provided.al2023` arm64) via SAM, and Docker / local `net/http` server.
- OpenAPI description, integration guide (Vue and Next.js examples on `example-app`), security and deploy docs.
- Project Cursor rules and skills for changelog/roadmap hygiene and security defaults.
- GitHub Actions CI on push/PR to `main` (`go vet`, race tests, server and Lambda builds, SAM validate) and an OIDC-based deploy workflow.
- Project icon and GitHub social preview artwork in the README.
- Required per-app `notificationEmail` (SES To) and `fromEmail` (SES From) in the apps registry.
- Pre-deploy `deploycheck` tool: validate apps YAML and that every `fromEmail` is covered by `SES_IDENTITIES` before SSM publish / SAM deploy.

### Security

- Load the Turnstile secret from an SSM SecureString at startup instead of placing it in the Lambda environment.
- Prefer `RemoteAddr` for Turnstile `remoteip` and ignore client-controlled `X-Forwarded-For` unless `WELES_TRUST_PROXY_XFF=true`.
- Grant API Gateway permission to write HTTP API access logs via a CloudWatch Logs resource policy.
- Align the threat model with the stack: flood cost is limited by API Gateway throttle (Lambda reserved concurrency is not set).
- Validate SES From/To as bare email addresses in the apps registry (reject display names and CR/LF).

### Changed

- Rename process environment variables from `WELLS_*` to `WELES_*`. Deprecated `WELLS_*` names are still read as fallback (a warning is logged; `WELES_*` wins when both are set).
- Reload the apps allowlist from SSM on a TTL (default 5 minutes when using SSM) so allowlist edits apply without redeploying.
- Normalize SSM parameter names to always start with `/` in config loading and deploy CI.
- Support a required `SesIdentityArns` SAM parameter (built from GitHub var `SES_IDENTITIES`, comma-separated domains/emails — one or many) so Lambda IAM can allow `ses:SendEmail` on multiple SES identities.
- Scope Lambda `ses:SendEmail` IAM to the configured SES configuration set (`SesConfigurationSet`, default `default-configuration`) instead of every set in the account.
- Remove stack-level SES From/To (`NotificationEmail` / `FromEmail`, `WELES_SES_FROM` / `WELES_SES_TO`, GitHub `NOTIFICATION_EMAIL` / `FROM_EMAIL`). Operator addresses come only from the apps registry; SES IAM identities come from `SES_IDENTITIES`.

### Fixed

- Strip the API Gateway HTTP API stage prefix (for example `/prod`) before ServeMux matching so `GET /healthz` and `POST /v1/feedback` work on the named-stage execute-api URL.
- Include the publisher error on `publish failed` logs so SES delivery failures are diagnosable without guessing.
- Allow `ses:SendEmail` on the configured SES configuration set so delivery works when the account has a default configuration set.
