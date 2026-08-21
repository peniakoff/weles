# Roadmap

Product capabilities planned for Weles. Status values: `planned` · `in progress` · `done`.

Public docs never list which real-world products integrate with Weles. Consumers are described generically (`example-app`, “integrating application”).

Agents and contributors must **not** implement items from future versions unless explicitly asked. When starting work, move the item into `CHANGELOG.md` under `[Unreleased]`.

---

## v1.0 — Intake + email

**Status:** `done` (2026-08-21)

- `POST /v1/feedback` with Turnstile, CORS/host allowlists, SES plaintext email
- Dual runtime: Lambda + Docker
- OpenAPI + integration docs
- CI (vet, race tests, builds, SAM validate) + OIDC deploy
- Per-app From and notification (To) addresses in the apps registry (required per app; verified **SES domain or email identities** via `SES_IDENTITIES` — this is mail From, not an API hostname)
- SSM Turnstile secret, apps YAML TTL reload, pre-deploy `deploycheck`, stage-prefix strip, SES configuration-set IAM

**Non-goals:** database, dashboard, WAF, API Gateway custom hostname (ACM / Route53), HMAC, HTML email

---

## v1.1 — Client hardening

**Status:** `done` (2026-08-21)

- Integration docs: Turnstile widget, env-based API URL (execute-api), enable-submit checklist (generic)
- Privacy/retention copy guidance for integrating apps
- **Dropped:** API Gateway custom hostname (`feedback.<domain>`) + ACM / Route53 — fixed DNS and custom-domain cost; the execute-api URL is the supported public endpoint (SES From on verified domains remains v1.0)

---

## v1.2 — Better delivery

**Status:** `planned`

- Optional notifier retry / `503` with `Retry-After`
- Optional HTML (multipart) email alongside plaintext

---

## v1.3 — Server-to-server auth

**Status:** `planned`

- Optional HMAC (app secret) for BFF / non-SPA callers
- Turnstile still required for browser-direct calls
- Docs describing when to use which mode

---

## v2.0 — Persistence

**Status:** `planned`

- SQS between intake and processing (fast `202` + DLQ)
- DynamoDB (on-demand); reconsider Postgres if a rich query UI appears
- Report status (`received` / `notified` / `failed`) and lookup by `reportId`
- Retention and PII deletion process for optional email

---

## v2.1 — Operator UI

**Status:** `planned`

- Simple dashboard (list, filter by category/app, detail view)
- Operator authentication (not a public app)

---

## v2.2 — Abuse hardening

**Status:** `planned` (only when spam becomes material)

- CloudFront + WAF (accept fixed Web ACL cost)
- Per-app / per-IP limits beyond API Gateway

---

## v3.0 — Lightweight tracker

**Status:** `planned`

- Tickets: assignment, comments, states (`open` / `triaged` / `closed`)
- Optional status notifications back to the submitter email
- Export and webhooks (e.g. GitHub Issues)
