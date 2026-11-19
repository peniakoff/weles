# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

@AGENTS.md

Project rules live in `AGENTS.md`, `.cursor/rules/weles.mdc` and the two skills under
`.cursor/skills/` (release docs, security). Read the relevant skill before touching
CHANGELOG/ROADMAP or any validation, CORS, Turnstile, SES or logging code.

## Commands

- `make run` — local server on `:8080` (defaults: `WELES_TURNSTILE_MODE=skip`, `WELES_NOTIFIER=stdout`)
- `make test` / `make test-race` — CI runs `go vet ./...` plus the race variant
- Single test: `go test ./internal/feedback -run 'TestValidate_Table/short_message' -count=1`
- `make lint` — golangci-lint (not run in CI)
- `make build` / `make lambda-build` — server binary / Lambda `bootstrap` (linux/arm64, `lambda.norpc`)
- `make sam-validate` — `sam validate --lint`, run in CI
- `make deploycheck APPS=config/apps.yaml IDENTITIES=example.com` — checks that every app's
  `fromEmail` is covered by an SES identity; deploy CI fails on it before touching SSM/SAM
- `docker compose up --build` — local container (Turnstile skipped, stdout notifier)

## Architecture

Go module `github.com/peniakoff/weles`. One shared core, three entrypoints:

- `cmd/server` — `net/http` server (Docker/local)
- `cmd/lambda` — the same handler behind `httpadapter.NewV2` (API Gateway HTTP API), wrapped in
  `httpapi.StripStagePrefix` because API Gateway passes `/{stage}` in the path
- `cmd/deploycheck` — deploy-time validator built on `internal/sesiam`

`internal/app.Build` is the only composition root and picks implementations from env:

- registry: `WELES_APPS_YAML` > `WELES_APPS_SSM` > `WELES_APPS_CONFIG` file; when it comes from
  SSM it is wrapped in `config.ReloadingRegistry` (default TTL 5 min) so allowlist edits apply
  without redeploy
- `turnstile.Verifier`: `Skip` or `Cloudflare` (secret from env or SSM SecureString)
- `notify.Publisher`: `Stdout` or `SES` (From/To resolved per app from the registry)

Request flow in `httpapi.handleFeedback`: origin allowlist + CORS → JSON content type and
`MaxBytesReader` → `feedback.Validate` (pure: enums, lengths, page URL/host allowlist,
sanitizing; returns `ValidationError` → 400 / `ForbiddenError` → 403) → Turnstile → Publish
(failure → 503) → 202 `{reportId}`. Error bodies are deliberately generic.

Boundaries are interfaces (`config.AppRegistry`, `turnstile.Verifier`, `notify.Publisher`,
`notify.SESAPI`); tests fake them and never hit AWS or Cloudflare.

## Gotchas

- Env vars are `WELES_*`; deprecated `WELLS_*` are still read as fallback. Go through
  `config.prefixedEnv` when adding variables.
- IAM for SES is fixed at deploy time (`SES_IDENTITIES` → `SesIdentityArns`). A new `fromEmail`
  domain in the apps YAML needs a redeploy, not just an SSM update.
- `config/apps.yaml` is gitignored; only `config/apps.example.yaml` with fictional values is committed.
- A push to `main` touching `cmd/`, `internal/`, `config/`, `template.yaml`, `Makefile` or `go.*`
  triggers a production deploy.
- Public contract changes must update `api/openapi.yaml` and `docs/integration.md`.
