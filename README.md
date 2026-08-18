![Weles emblem: a horned threshold with a leaf in the gate](docs/images/icon.png)

# weles

The bridge between you and your users. A universal feedback intake API that collects bug reports and suggestions from web apps and delivers them to an operator inbox.

Named after **Veles** (Weles), the Slavic god of the underworld, waters, and cattle — a guardian of thresholds. This service sits between product UIs and the people who maintain them.

![Night river ford with wooden posts and a faint serpent in the water](docs/images/social-preview.jpg)

## Features (v1)

- `POST /v1/feedback` — validate, abuse-check, notify
- Cloudflare Turnstile verification
- Per-app origin and page-host allowlists
- Amazon SES plaintext email delivery (or stdout for local/Docker), with optional Reply-To from the submitter email
- Dual runtime: AWS Lambda (`provided.al2023` arm64) and Docker/`net/http`
- CI on push/PR to `main` (vet, race tests, server + Lambda builds, SAM validate)

## Quick start (Docker)

```bash
cp config/apps.example.yaml config/apps.yaml   # optional local override (gitignored)
docker compose up --build
curl -s http://localhost:8080/healthz
```

The Compose image defaults to **`WELLS_TURNSTILE_MODE=skip`** and stdout notifications for local development only. Do **not** expose that configuration on the public Internet; set `cloudflare` mode and provide `WELLS_TURNSTILE_SSM` or `WELLS_TURNSTILE_SECRET` for any shared deployment.

Local defaults skip Turnstile verification and print notifications to stdout. See [docs/integration.md](docs/integration.md) for the request contract.

## Quick start (Go)

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
make run
# WELLS_TURNSTILE_MODE=skip WELLS_NOTIFIER=stdout
```

```bash
make test
```

## Documentation

| Document | Purpose |
|----------|---------|
| [docs/integration.md](docs/integration.md) | API contract, Vue/Next examples |
| [docs/security.md](docs/security.md) | Threat model and guarantees |
| [docs/deploy.md](docs/deploy.md) | SAM deploy, SES verify, CI |
| [api/openapi.yaml](api/openapi.yaml) | OpenAPI 3 description |
| [ROADMAP.md](ROADMAP.md) | Planned versions |
| [CHANGELOG.md](CHANGELOG.md) | Keep a Changelog |

## Configuration

| Variable | Default | Meaning |
|----------|---------|---------|
| `WELLS_LISTEN_ADDR` | `:8080` | HTTP listen address (server) |
| `WELLS_APPS_CONFIG` | `config/apps.example.yaml` | Path to apps allowlist YAML |
| `WELLS_APPS_YAML` | _(empty)_ | Inline apps YAML (overrides file) |
| `WELLS_APPS_SSM` | _(empty)_ | SSM parameter name with apps YAML (used on Lambda; leading `/` normalized) |
| `WELLS_APPS_RELOAD_SECONDS` | `0` (or `300` when SSM is set without override) | How often to reload apps from SSM |
| `WELLS_TURNSTILE_MODE` | `skip` | `skip` or `cloudflare` — **use `cloudflare` in any public deploy** |
| `WELLS_TURNSTILE_SECRET` | — | Turnstile secret (local/dev); prefer SSM in production |
| `WELLS_TURNSTILE_SSM` | — | SSM SecureString parameter name for the Turnstile secret |
| `WELLS_TRUST_PROXY_XFF` | `false` | If `true`, use first `X-Forwarded-For` hop for Turnstile `remoteip` |
| `WELLS_NOTIFIER` | `stdout` | `stdout` or `ses` |
| `WELLS_SES_FROM` | — | Verified SES From address (required when notifier is `ses`; bare email only) |
| `WELLS_SES_TO` | — | Operator inbox To address (required when notifier is `ses`; bare email only) |
| `AWS_REGION` | `eu-central-1` | AWS region |
| `WELLS_MAX_BODY_BYTES` | `8192` | Max JSON body size |
| `WELLS_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `WELLS_API_STAGE` | _(empty)_ | API Gateway HTTP API stage name; Lambda strips `/{stage}` from the request path (SAM sets this from `StageName`) |

Production app allowlists (real origins/hosts) must **not** be committed. Use `config/apps.example.yaml` as the public template and publish real values to SSM (`WELLS_APPS_SSM`) at deploy time.

## License

MIT — see [LICENSE](LICENSE).
