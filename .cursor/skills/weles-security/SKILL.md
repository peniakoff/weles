---
name: weles-security
description: Weles v1 security defaults for the feedback API. Use when changing endpoints, validation, CORS, Turnstile, SES, auth, allowlists, or logging.
---

# Weles security (v1)

## Browser abuse controls

- Cloudflare Turnstile verified server-side with secret from SSM SecureString (production) + optional `remoteip` from `RemoteAddr`.
- CORS: reflect `Origin` only when allowlisted; methods `POST`/`OPTIONS`; header `Content-Type`.
- API Gateway throttle + Lambda reserved concurrency limit flood cost.
- **Never** ship API keys or Turnstile secrets to the SPA.
- Do not trust client `X-Forwarded-For` unless `WELLS_TRUST_PROXY_XFF=true`.

## Validation

- Enums, length limits, HTTPS page URLs (HTTP only for localhost), no URL userinfo.
- Email optional; reject CR/LF; validate format.
- Unknown JSON fields ignored — never emailed or logged.
- Do not fetch `pageUrl` (SSRF).

## Notifications

- Production notifier: Amazon SES v2 plaintext `SendEmail` (`WELLS_NOTIFIER=ses`); local/Docker default: `stdout`.
- Plaintext email bodies only (no HTML).
- Operator From/To from config; optional submitter email is Reply-To only (never From).
- Subject/body must not introduce injection vectors from user input beyond what validation already strips.
- IAM: `ses:SendEmail` on the configured SES identity (email address or verified domain via `SesIdentity`).

## Logging

- Log `reportId`, `appId`, `category`, errors — **not** message, email, or tokens.

## Public examples

- Only `example-app` / `*.example.com` in committed allowlist samples and docs.
