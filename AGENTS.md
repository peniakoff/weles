# Agent guide — Weles

Read these before changing behaviour or docs:

1. [ROADMAP.md](ROADMAP.md) — what is in scope for the current version
2. [CHANGELOG.md](CHANGELOG.md) — record user-facing changes under `[Unreleased]`
3. [docs/security.md](docs/security.md) — threat model
4. [docs/integration.md](docs/integration.md) — public API contract

## Cursor project skills

- [weles-release-docs](.cursor/skills/weles-release-docs/SKILL.md) — changelog / roadmap / SemVer
- [weles-security](.cursor/skills/weles-security/SKILL.md) — validation, CORS, Turnstile, SES, logging

## Always-apply rule

- [.cursor/rules/weles.mdc](.cursor/rules/weles.mdc)

## Hard constraints

- English in the public repository
- No real consumer product names or production domains in committed docs/examples/config
- Do not implement future ROADMAP items unless the user asks
- Every user-facing change must update `CHANGELOG.md` under `[Unreleased]` in the same change (see weles-release-docs)
