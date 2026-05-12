# PayBridge

**Production-grade payment platform** built with Go — multi-currency wallets, transfers, fraud detection, and full LGTM observability. Deployed on Fly.io.

[![CI/CD](https://github.com/Haleralex/PayBridge/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/Haleralex/PayBridge/actions/workflows/ci-cd.yml)
[![Coverage](https://img.shields.io/badge/coverage-56%25-brightgreen.svg)](https://github.com/Haleralex/PayBridge)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

---

## What it does

A financial backend that handles user onboarding (with KYC), multi-currency wallets (fiat + crypto), deposits, withdrawals, peer-to-peer transfers and currency exchange — with strong consistency, idempotency, and asynchronous fraud screening. Comes with a Telegram Mini App as the reference client.

---

## Architecture

Three independent Go services deployed to Fly.io, communicating over NATS and gRPC:

```
┌────────────────────────┐      gRPC       ┌──────────────────┐
│   paybridge-api        │ ──────────────► │  fraud-detector  │
│   (HTTP · WebSocket)   │                 │  (risk scoring)  │
└─────────┬──────────────┘                 └──────────────────┘
          │ NATS pub/sub
          ▼
┌────────────────────────┐
│  paybridge-notifier    │  ──►  Telegram Bot API
│  (event consumer)      │
└────────────────────────┘
```

Each service is built with **Clean / Hexagonal Architecture**:

```
Adapters     ─  HTTP · gRPC · NATS · WebApp
Application  ─  Use Cases · CQRS (Command/Query buses) · Ports
Domain       ─  Entities · Value Objects · Domain Events
Infrastructure ─ PostgreSQL · Redis · NATS · OTel
```

The domain layer has zero external dependencies. Every infrastructure component implements an application-defined port.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.25 |
| HTTP | Gin · JWT (HS256) · Telegram Mini App signature auth |
| RPC | gRPC + protobuf (fraud detection service) |
| Messaging | NATS JetStream (domain events) |
| Database | PostgreSQL 16 · pgx/v5 · golang-migrate |
| Cache & Locks | Redis (rate limiting · token blacklist · distributed locks) |
| Tracing | OpenTelemetry SDK → Grafana Tempo |
| Logs | Fly.io log drain → Grafana Alloy → Loki |
| Metrics | Prometheus + OTel MeterProvider bridge → Mimir |
| Containers | Docker multi-stage (33 MB final image, non-root) |
| Deployment | Fly.io (3 separate apps) + Neon (managed Postgres) |
| CI/CD | GitHub Actions — lint · race detector · gosec · integration tests |
| Testing | testify · testcontainers-go (real Postgres in CI) |

---

## Engineering Highlights

**Consistency & correctness**
- **Optimistic locking** on wallet balances (version column) — concurrent transfers cannot lose updates
- **Idempotency keys** on every mutating endpoint — exactly-once semantics, safe retries
- **Transactional Outbox pattern** — domain events committed atomically with state changes, then dispatched to NATS by a background poller
- **Transaction state machine** with explicit guard rails (PENDING → PROCESSING → COMPLETED/FAILED/CANCELLED)
- **Distributed locks via Redis** for cross-instance critical sections

**Security**
- JWT auth with refresh tokens and Redis-backed token blacklist (logout)
- Telegram Mini App `initData` signature verification with freshness check
- IDOR-protected resource access (wallet ownership checked on every operation)
- Layered rate limiting — 100 req/min general, 30 req/min for financial ops
- All SQL through parameterized queries; no ORM, no string concatenation
- gosec runs on every CI build

**Observability (LGTM stack on Grafana Cloud)**
- Distributed traces across HTTP → application → DB via OpenTelemetry
- gRPC and Gin handlers instrumented with otel middleware
- Structured logs forwarded from Fly.io via log drain → Grafana Alloy
- Custom Prometheus metrics bridged into OTel and pushed to Mimir

**Patterns**
- **CQRS** — separate command and query buses with middleware pipeline (logging, validation, metrics)
- **Domain Events** — WalletCreated, TransactionCompleted, KycApproved
- **Money as value object** with currency-aware arithmetic (no floats — minor units stored as `BIGINT`)

---

## Domain Model

**Currencies supported:** USD, EUR, GBP (fiat) · BTC, ETH, USDT, USDC (crypto)

**Transaction types:** Deposit · Withdraw · Payout · Transfer · Fee · Refund · Adjustment

**Wallet features:** Available + pending balance · Daily and monthly limits · ACTIVE / SUSPENDED / LOCKED / CLOSED states

---

## API

All endpoints under `/api/v1`. Auth via `Authorization: Bearer <JWT>`.

| Method | Path | Description |
|---|---|---|
| `POST` | `/users` | Register user |
| `POST` | `/auth/telegram` | Authenticate via Telegram Mini App |
| `POST` | `/auth/logout` | Revoke JWT (added to Redis blacklist) |
| `GET` | `/users/:id` · `/users` | Fetch user · list users |
| `POST` | `/users/:id/kyc/start` · `/users/:id/kyc` | Start · approve KYC |
| `POST` | `/wallets` | Create wallet |
| `GET` | `/wallets` · `/wallets/me` · `/wallets/:id` | List · own · single |
| `POST` | `/wallets/:id/credit` | Deposit |
| `POST` | `/wallets/:id/debit` | Withdraw |
| `POST` | `/wallets/:id/transfer` | P2P transfer |
| `POST` | `/wallets/:id/exchange` | Cross-currency exchange |
| `GET` | `/wallets/:id/transactions` | Transaction history |
| `GET` | `/transactions/:id` · `/transactions/by-key/:key` | Lookup by id · idempotency key |
| `POST` | `/transactions/:id/retry` · `/transactions/:id/cancel` | Retry · cancel |
| `GET` | `/health` · `/ready` · `/metrics` | Health · readiness · Prometheus |

---

## Project Structure

```
cmd/
  api/              — REST API entry point
  fraud-detector/   — gRPC fraud scoring service
  notifier/         — NATS consumer → Telegram notifications
  migrate/          — Migration runner

internal/
  domain/           — Entities, value objects, domain events (no deps)
  application/
    usecases/       — Transaction, Wallet, User use cases
    cqrs/           — Command/Query buses, middleware
    ports/          — Repository & service interfaces
  infrastructure/
    persistence/    — PostgreSQL repositories (pgx/v5)
    cache/          — Redis: rate limiter, distributed lock, blacklist
    messaging/      — NATS publisher/subscriber
    poller/         — Outbox poller
    telemetry/      — OTel tracer + meter provider
    exchange/       — FX rate provider
  adapters/
    http/           — Gin handlers, JWT/Telegram middleware
    grpc/           — Fraud detection client + server
    nats/           — Event publisher/subscriber

migrations/         — Versioned SQL migrations
webapp/             — Telegram Mini App frontend
docs/               — Architecture & observability guides
fly.api.toml · fly.nats.toml · fly.notifier.toml  — Per-service deployment configs
```

---

## License

MIT
