# Changelog entry examples

## Feature

```markdown
### Added
- Optional `metadata` map on feedback context so integrating apps can attach opaque UI labels without schema changes.
```

## Fix

```markdown
### Fixed
- Reject page URLs that include userinfo to prevent credential leakage into operator email.
```

## Security

```markdown
### Security
- Strip CR/LF from optional contact emails before notification to block header injection into SES subjects/bodies.
```

## Bad (do not write)

```markdown
### Added
- Wired MyCoolApp and OtherProduct to Weles.
- misc updates
```
