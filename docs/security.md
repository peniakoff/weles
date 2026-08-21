# Security

## Threat model (v1)

Weles accepts unauthenticated browser traffic on a public HTTPS endpoint. Attackers can inspect frontend code and DevTools. Therefore:

- **No API keys in the SPA.** Anything shipped as `VITE_*` / `NEXT_PUBLIC_*` is public.
- **Cloudflare Turnstile** proves a human-ish client before notification.
- **Origin allowlisting** (CORS) and **page host allowlisting** bind submissions to registered apps.
- **API Gateway throttling** limits flood cost.
- **Strict validation** of category, lengths, email (no CR/LF), and URL shape.
- **Plaintext email only** (no HTML) to reduce markup injection into inboxes.
- **SES From** is operator-configured and verified; optional submitter email is used only as **Reply-To** after validation (never as From).
- **No outbound fetch of `pageUrl`** — the URL is never retrieved by the server (no SSRF).
- **No database in v1** — injection surface is limited to notification content.

## Guarantees

| Guarantee | Detail |
|-----------|--------|
| Transport | HTTPS via API Gateway (production) |
| PII in logs | Message, email, and Turnstile token are not logged |
| Secrets | Turnstile secret is read from SSM SecureString in production (not a Lambda env var) |
| Client IP | `RemoteAddr` preferred for Turnstile `remoteip`; `X-Forwarded-For` only if `WELES_TRUST_PROXY_XFF=true` |
| Error responses | Generic `400` / `403` / `429` / `503` without echoing input |

## Non-guarantees

- Turnstile and throttling reduce abuse; they do not stop a determined attacker with CAPTCHA farms.
- SES email delivery is plaintext only; From must be a verified SES identity (or covered by a verified domain identity via `SesIdentity`).
- Without WAF/CloudFront (intentionally deferred), volumetric attacks rely on Gateway limits.
- Clients must not put secrets or sensitive calculator/payment data into `message` or `metadata`.

## Operator responsibilities

1. Keep production allowlists accurate and out of public git.
2. Verify the SES From identity (and To while in SES sandbox). If only a domain identity is verified, set `SesIdentity` to that domain. If the account uses a default SES configuration set, set `SesConfigurationSet` to that set's name (stack default `default-configuration`; `*` allows any).
3. Rotate the Turnstile secret if leaked.
4. Publish retention/privacy expectations in integrating apps before enabling submit.
5. Review CloudWatch logs for volume anomalies (without expecting message bodies there).

## Reporting vulnerabilities

Open a private security advisory on the GitHub repository, or contact the maintainer listed in the repository profile. Do not file public issues that include working exploit payloads against live deployments.
