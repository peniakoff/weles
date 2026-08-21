---
name: weles-security
description: Weles v1 security defaults for the feedback API. Use when changing endpoints, validation, CORS, Turnstile, SES, auth, allowlists, or logging.
---

# Weles security (v1)

## Browser abuse controls

- Cloudflare Turnstile verified server-side with secret from SSM SecureString (production) + optional `remoteip` from `RemoteAddr`.
- CORS: reflect `Origin` only when allowlisted; methods `POST`/`OPTIONS`; header `Content-Type`.
- API Gateway throttle limits flood cost.
- **Never** ship API keys or Turnstile secrets to the SPA.
- Do not trust client `X-Forwarded-For` unless `WELES_TRUST_PROXY_XFF=true`.

## Validation

- Enums, length limits, HTTPS page URLs (HTTP only for localhost), no URL userinfo.
- Email optional; reject CR/LF; validate format.
- Unknown JSON fields ignored — never emailed or logged.
- Do not fetch `pageUrl` (SSRF).

## Notifications

- Production notifier: Amazon SES v2 plaintext `SendEmail` (`WELES_NOTIFIER=ses`); local/Docker default: `stdout`.
- Plaintext email bodies only (no HTML).
- Operator From/To come from the apps registry (`fromEmail` / `notificationEmail`; required per app).
- Optional submitter email is Reply-To only (never From).
- Subject/body must not introduce injection vectors from user input beyond what validation already strips.
- IAM: `ses:SendEmail` on each configured SES identity ARN (`SesIdentityArns` / GitHub `SES_IDENTITIES` — one or more domains or emails) and on `configuration-set/${SesConfigurationSet}` (default `default-configuration`; account default sets are authorized even when the API omits `ConfigurationSetName`).
- Per-app `fromEmail` may use different domains; every covering SES identity must be listed at deploy time. Deploy CI runs `deploycheck` so uncovered From addresses fail before SSM/SAM.
- IAM does not reload with the apps YAML; adding a domain still requires updating `SES_IDENTITIES` and redeploying.

## Logging

- Log `reportId`, `appId`, `category`, errors — **not** message, email, or tokens.

## Public examples

- Only `example-app` / `*.example.com` in committed allowlist samples and docs.
