# Integrating with Weles

This guide describes how a browser app (Vue, Next.js, or similar) submits feedback to Weles.

Public examples use the fictional app `example-app` on `https://app.example.com`.

## Prerequisites

1. Your application is registered in the Weles apps allowlist (`appId`, allowed CORS `origins`, allowed `pageUrl` hosts).
2. Cloudflare Turnstile is configured for your site hostnames; you have the **site key** (public) and Weles has the **secret** (server-only).
3. You know the deployed feedback URL, for example:

   `https://{api-id}.execute-api.{region}.amazonaws.com/prod/v1/feedback`

   App paths are `/v1/feedback` and `/healthz`. The `/prod` segment is the API Gateway stage (SAM `StageName`), not part of the mux routes.

## Request

`POST /v1/feedback`  
`Content-Type: application/json`  
Max body size: **8 KiB**.

```json
{
  "appId": "example-app",
  "category": "bug",
  "message": "The calculated value is wrong after switching units.",
  "email": "user@example.com",
  "context": {
    "locale": "en",
    "pageUrl": "https://app.example.com/en/tools/roof",
    "metadata": {
      "toolId": "roof",
      "toolName": "Roof calculator"
    }
  },
  "turnstileToken": "<token-from-widget>",
  "submittedAt": "2026-08-17T18:00:00.000Z"
}
```

Field name for the page URL is **`pageUrl`** (camelCase), not `pageURL`.

### Field rules

| Field | Required | Rules |
|-------|----------|-------|
| `appId` | yes | Known ID from the allowlist |
| `category` | yes | `bug` \| `suggestion` \| `other` |
| `message` | yes | Trimmed, 10–4000 characters |
| `email` | no | Max 254 chars, valid address, no CR/LF |
| `context.locale` | yes | Short safe token (e.g. `en`, `pl`) |
| `context.pageUrl` | yes | `https` (or `http` only for `localhost`), host allowlisted, no userinfo |
| `context.metadata` | no | Max 10 keys; opaque labels for your UI |
| `turnstileToken` | yes | Verified server-side (or any non-empty string when Weles runs in `skip` mode) |
| `submittedAt` | no | RFC3339; informational only |

Unknown JSON fields are **ignored** (not stored or emailed).

When the optional `email` field is present and valid, Weles sets it as the notification **Reply-To**. The operator **From** and **To** addresses come from the apps registry (`fromEmail` / `notificationEmail` for the request `appId`).

### Success

```http
HTTP/1.1 202 Accepted
Content-Type: application/json

{"reportId":"01J2FEEDBACK8M7QK3R5C4"}
```

### Errors

Generic bodies only — never echo the submission:

| Status | Meaning |
|--------|---------|
| `400` | Validation / malformed JSON / body too large |
| `403` | Origin/host/app/Turnstile rejected |
| `429` | Throttled (API Gateway) |
| `503` | Notification backend unavailable — safe to retry |

## CORS

Weles reflects `Access-Control-Allow-Origin` only when the request `Origin` is on the allowlist for some registered app. Allowed methods: `POST`, `OPTIONS`. Allowed header: `Content-Type`.

## Turnstile (browser)

1. Load the Turnstile script and render a widget with your **site key**.
2. On submit, read the token and send it as `turnstileToken`.
3. Never put the Turnstile **secret** or any AWS credentials in frontend env vars (`VITE_*`, `NEXT_PUBLIC_*`, etc.).

Example (explicit render; replace `YOUR_SITE_KEY` and mount the widget where the form lives):

```html
<script src="https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit" async defer></script>
<div id="turnstile-widget"></div>
<script>
  let turnstileToken = ''
  window.onloadTurnstileCallback = () => {
    turnstile.render('#turnstile-widget', {
      sitekey: 'YOUR_SITE_KEY',
      callback: (token) => { turnstileToken = token },
      'expired-callback': () => { turnstileToken = '' },
    })
  }
</script>
```

Load the script with `onload=onloadTurnstileCallback` (or call `turnstile.render` after the API is ready). On form submit, send the current `turnstileToken` in the JSON body. Prefer resetting the widget after a failed submit so the user gets a fresh token.

## Privacy and retention (for integrating apps)

This is not legal advice. Publish clear copy in your UI before enabling submit. Align it with how Weles works in v1:

- **No database.** Weles does not store submissions; after validation it sends a plaintext email via Amazon SES and returns `reportId`.
- **Who receives the mail.** The operator inbox is the app’s `notificationEmail` in the apps registry. The optional submitter `email` is used only as **Reply-To** (never as From).
- **Logs.** Application logs record `reportId`, `appId`, and category — not `message`, email, or Turnstile tokens. API Gateway access logs (when enabled) are request metadata; the stack retains those log groups for **14 days**.
- **Inbox retention.** How long the notification stays in the operator mailbox is controlled by that mail provider, not by Weles.
- **What not to collect.** Do not put passwords, payment data, or unnecessary PII into `message` or `metadata`.

## Client checklist

1. Deploy Weles and verify the SES From identity (see [deploy.md](deploy.md)).
2. Register your `appId`, origins, and hosts with the operator.
3. Set a public env var to the full `POST /v1/feedback` URL (e.g. `VITE_FEEDBACK_API_URL` or `NEXT_PUBLIC_FEEDBACK_API_URL`) — typically the stack `FeedbackURL` execute-api URL.
4. Add Turnstile; mirror server validation on the client for UX only.
5. Wire pending / success / error / retry UI states.
6. Enable the submit button only after CORS, Turnstile, and [privacy copy](#privacy-and-retention-for-integrating-apps) are ready.
7. Do not collect passwords, payment data, or calculator inputs unless you have a documented need.

## Vue example

```ts
export type FeedbackCategory = 'bug' | 'suggestion' | 'other'

export interface FeedbackSubmission {
  appId: string
  category: FeedbackCategory
  message: string
  email?: string
  context: {
    locale: string
    pageUrl: string
    metadata?: Record<string, string>
  }
  turnstileToken: string
  submittedAt?: string
}

export async function submitFeedback(
  submission: FeedbackSubmission,
  signal?: AbortSignal,
): Promise<{ ok: true; reportId: string } | { ok: false }> {
  const endpoint = import.meta.env.VITE_FEEDBACK_API_URL?.trim()
  if (!endpoint) return { ok: false }

  const response = await fetch(endpoint, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(submission),
    signal,
  })
  if (!response.ok) return { ok: false }

  const result = (await response.json()) as { reportId?: string }
  if (typeof result.reportId !== 'string') return { ok: false }
  return { ok: true, reportId: result.reportId }
}
```

Call with `appId: 'example-app'`, current `pageUrl` (`window.location.href`), locale, optional metadata, and the Turnstile token.

## Next.js (App Router) example

Prefer calling Weles **from the browser** with Turnstile (same as Vue). A Route Handler is optional and does not replace Turnstile for public pages.

```ts
// app/actions are not required — client fetch is enough:
const res = await fetch(process.env.NEXT_PUBLIC_FEEDBACK_API_URL!, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    appId: 'example-app',
    category: 'suggestion',
    message: 'Please add dark mode to the settings page.',
    context: {
      locale: 'en',
      pageUrl: window.location.href,
      metadata: { route: '/settings' },
    },
    turnstileToken,
    submittedAt: new Date().toISOString(),
  }),
})
```

Server-to-server HMAC auth is planned later (see [ROADMAP.md](../ROADMAP.md)); do not invent API keys in the SPA.

## OpenAPI

See [api/openapi.yaml](../api/openapi.yaml).
