---
name: weles-release-docs
description: Maintains ROADMAP.md and CHANGELOG.md (Keep a Changelog, SemVer) for Weles. Use when releasing, versioning, shipping, updating Unreleased notes, planning features against the roadmap, or editing changelog/roadmap files.
---

# Weles release documentation

## Before planning or coding a feature

1. Read [ROADMAP.md](../../../ROADMAP.md).
2. If the work is not in the current version, stop unless the user explicitly expanded scope.
3. Do not name real consumer products in ROADMAP or CHANGELOG.

## Changelog rules

File: [CHANGELOG.md](../../../CHANGELOG.md)

### Mandatory Unreleased entry

Every change visible to an operator or integrating app (feature, fix, security hardening, breaking config/API, material deploy behaviour) **must** get an entry under `## [Unreleased]` **in the same change** — never “we’ll add the changelog later”.

Skip only pure internal refactors with no behaviour change, private-notes typo fixes, or ROADMAP/skill-only edits that do not affect users.

Before treating the task as done: open `CHANGELOG.md` and confirm `[Unreleased]` reflects the work.

### Format

- Keep `## [Unreleased]` at the top.
- Categories only: `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, `Security`.
- English full sentences; explain user-facing impact, not a file dump.
- On tag `vX.Y.Z`: move Unreleased bullets into `## [X.Y.Z] - YYYY-MM-DD` and add a compare link when previous tags exist.
- Breaking API changes are MAJOR and called out under `Changed` or `Removed`.
- Security entries: mitigation only — no exploit PoC, no secrets.
- Forbidden: empty “misc updates”; listing which apps “will use” the API.

## Roadmap rules

- Status: `planned` | `in progress` | `done`.
- Mark items `done` with a date note; do not delete history.
- Consumers stay generic (`example-app`, “integrating application”).

## SemVer heuristic

| Change | Version bump |
|--------|----------------|
| New endpoint/field (compatible) | MINOR |
| Bug fix / hardening | PATCH |
| Remove/rename required API field | MAJOR |

See [examples.md](examples.md) for sample entries.
