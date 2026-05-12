# PayBridge

**Production-grade payment gateway** built with Go, Clean Architecture, and an enterprise observability stack.

[![CI/CD](https://github.com/Haleralex/PayBridge/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/Haleralex/PayBridge/actions/workflows/ci-cd.yml)
[![Coverage](https://img.shields.io/badge/coverage-56%25-brightgreen.svg)](https://github.com/Haleralex/PayBridge)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

---

## What it does

PayBridge handles multi-currency wallets (fiat + crypto), transfers, deposits, withdrawals, and transaction lifecycle — all with strong consistency guarantees, real-time fraud detection, and full distributed observability.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.25 |
| HTTP | Gin, JWT, Telegram Mini App auth |
| Messaging | NATS (pub/sub events) |
| RPC | gRPC (fraud detection service) |
| Database | PostgreSQL 16, pgx/v5, connection pooling |
| Cache / Rate Limiting | Redis |
| Observability | OpenTelemetry → Grafana Cloud (Loki + Tempo + Mimir) |
| Metrics | Prometheus + OTel MeterProvider bridge |
| Log routing | Grafana Alloy (OTLP pipeline) |
| Containerization | Docker multi-stage, Docker Compose |
| Deployment | Fly.io |
| CI/CD | GitHub Actions (lint, test, race detector, gosec) |
| Testing | testify + testcontainers-go (real PostgreSQL in CI) |

---

## Architecture

Clean/Hexagonal Architecture with strict layer boundaries:

```
┌──────────────────────────────────────────────────────┐
│  Adapters  ─  HTTP · gRPC · NATS · WebApp (Telegram) │
│  ┌────────────────────────────────────────────────┐  │
│  │  Application  ─  Use Cases · CQRS · Ports      │  │
│  │  ┌──────────────────────────────────────────┐  │  │
│  │  │  Domain  ─  Entities · Value Objects     │  │  │
│  │  │           · Events · Business Rules      │  │  │
│  │  └──────────────────────────────────────────┘  │  │
│  └────────────────────────────────────────────────┘  │
│  Infrastructure  ─  PostgreSQL · Redis · NATS client  │
└──────────────────────────────────────────────────────┘
```

The domain layer has zero external dependencies. All infrastructure implements application-defined ports (interfaces).

---

## Key Features

**Financial**
- Multi-currency wallets — fiat (USD, EUR, GBP) and crypto (BTC, ETH, USDT, USDC)
- Deposit, withdrawal, and peer-to-peer transfers
- Optimistic locking — version-based concurrency control, no dirty reads
- Idempotency keys — exactly-once semantics for all mutation endpoints
- Transaction state machine with validation guards
- Exponential backoff retry for deadlock recovery

**Security**
- JWT (HS256) authentication with refresh tokens
- Telegram Mini App signature verification
- Per-user and per-endpoint rate limiting via Redis
- IDOR protection on all wallet endpoints
- Parameterized queries throughout — no ORM

**Observability (LGTM stack)**
- Distributed traces via OpenTelemetry SDK → Grafana Tempo
- Structured logs forwarded via Fly.io log drain + Grafana Alloy → Grafana Loki
- Application metrics via Prometheus + OTel bridge → Grafana Mimir
- gRPC and Gin middleware both instrumented with OTel spans

**Fraud Detection**
- Dedicated gRPC fraud service interface
- No-op implementation included; pluggable backend

**Testing**
- 178 tests: 89 unit + 89 integration
- Integration tests spin up a real PostgreSQL container via testcontainers-go
- Race detector runs in CI on every push
- gosec static analysis in CI pipeline

---

## API Overview

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/auth/telegram` | Authenticate via Telegram |
| `POST` | `/api/v1/wallets` | Create wallet |
| `GET` | `/api/v1/wallets/:id` | Get wallet + balance |
| `POST` | `/api/v1/wallets/:id/credit` | Deposit |
| `POST` | `/api/v1/wallets/:id/debit` | Withdraw |
| `POST` | `/api/v1/wallets/:id/transfer` | Transfer to another wallet |
| `GET` | `/api/v1/wallets/:id/transactions` | Transaction history |
| `GET` | `/health` | Health check |
| `GET` | `/ready` | Readiness probe (checks DB) |

---

## Project Structure

```
cmd/api/              — entry point
internal/
  domain/             — entities, value objects, domain events
  application/        — use cases, CQRS, ports
  infrastructure/     — PostgreSQL, Redis repositories
  adapters/
    http/             — Gin handlers, JWT middleware
    grpc/             — fraud detection service
    nats/             — event publisher / subscriber
migrations/           — versioned SQL migrations
docs/                 — architecture & observability guides
```

---

## License

MIT
