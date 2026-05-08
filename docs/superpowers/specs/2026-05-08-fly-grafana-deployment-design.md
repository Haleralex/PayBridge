# Deployment Design: Fly.io + Grafana Cloud

**Date:** 2026-05-08  
**Status:** Approved

## Goal

Deploy PayBridge to production without ngrok. Provide a permanent public HTTPS URL for the Telegram Mini App. All services free of charge using Fly.io free tier + Grafana Cloud free tier.

## Architecture

```
Telegram / Browser
       │ HTTPS (fly.dev)
       ▼
┌─────────────────────────────────────────┐
│                Fly.io                   │
│                                         │
│  paybridge-api.fly.dev  (VM 1, 256MB)   │
│  Go API + Telegram Mini App static      │
│         │                               │
│         │ NATS (Fly private network)    │
│         ▼                               │
│  paybridge-nats.internal  (VM 2, 256MB) │
│         │                               │
│         ▼                               │
│  paybridge-notifier  (VM 3, 256MB)      │
│         │                               │
│  fly-postgres.internal  (managed)       │
└─────────────────────────────────────────┘
       │ OTLP/HTTP
       ▼
Grafana Cloud (distributed tracing, free tier)
```

### Networking

- Services communicate via **Fly private network** using DNS `<app>.internal`
- Only the API is publicly exposed on port 8080 — Fly provides automatic TLS + `*.fly.dev` domain
- NATS (`paybridge-nats.internal:4222`) and Notifier are internal only
- Fly Postgres does not count toward the 3 free VM limit

### Free Tier Budget

| Resource | Provider | Limit |
|---|---|---|
| VM 1 — API | Fly.io | shared-cpu-1x, 256MB RAM |
| VM 2 — NATS | Fly.io | shared-cpu-1x, 256MB RAM |
| VM 3 — Notifier | Fly.io | shared-cpu-1x, 256MB RAM |
| PostgreSQL | Fly Postgres | managed, free for small DBs |
| Distributed tracing | Grafana Cloud | 50GB traces/month free |

## New Files

| File | Purpose |
|---|---|
| `fly.api.toml` | Fly config for Go API |
| `fly.nats.toml` | Fly config for NATS broker (official image) |
| `fly.notifier.toml` | Fly config for Notifier service |
| `.github/workflows/deploy.yml` | GitHub Actions auto-deploy on push to main |

## Changes to Existing Files

### `configs/config.production.yaml`
- `enable_mock_auth: false` — currently enabled, must be disabled in production
- `environment: production`
- `cors.allowed_origins` → `["https://paybridge-api.fly.dev"]`
- `telemetry.otlp_endpoint` → Grafana Cloud OTLP endpoint

### Telemetry (OTLP headers support)
The current telemetry setup sends traces to `jaeger:4318`. Grafana Cloud requires an `Authorization: Basic <base64>` header. Need to add `OTEL_EXPORTER_OTLP_HEADERS` env var support to the OTLP exporter configuration.

### `Dockerfile.notifier`
Verify it reads all configuration from environment variables (not hardcoded config file path).

## Secrets (via `fly secrets set`, never in code)

```
PAYBRIDGE_AUTH_TELEGRAM_BOT_TOKEN=<from BotFather>
PAYBRIDGE_AUTH_JWT_SECRET=<strong random secret>
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Basic <grafana-api-key-base64>
PAYBRIDGE_DATABASE_HOST=<fly-postgres-host>
PAYBRIDGE_DATABASE_PASSWORD=<fly-postgres-password>
PAYBRIDGE_NATS_URL=nats://paybridge-nats.internal:4222
```

## Deployment Flow

### First Deploy (one-time)

1. `flyctl auth login`
2. Create Grafana Cloud account → obtain OTLP endpoint + API key
3. `fly postgres create --name paybridge-db`
4. `fly apps create paybridge-nats` → `fly deploy --config fly.nats.toml`
5. `fly apps create paybridge-api` → set secrets → `fly deploy --config fly.api.toml`
6. `fly apps create paybridge-notifier` → set secrets → `fly deploy --config fly.notifier.toml`
7. Run DB migrations: `fly machine run --app paybridge-api -- /app/migrate -config /app/configs`

### Subsequent Deploys

```bash
fly deploy --config fly.api.toml
fly deploy --config fly.notifier.toml
```

### CI/CD (GitHub Actions)

`.github/workflows/deploy.yml` triggers on push to `main`:
- Builds and deploys API and Notifier
- `FLY_API_TOKEN` stored in GitHub Secrets

## Out of Scope

- Custom domain (can be added later via `fly certs`)
- Jaeger self-hosted — replaced entirely by Grafana Cloud Tempo
- pgAdmin — not deployed to production
- Prometheus metrics scraping — `/metrics` endpoint remains available but no scraper configured
