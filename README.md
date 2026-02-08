# PayBridge

**Production-ready payment gateway** built with Clean Architecture, Domain-Driven Design, and Hexagonal Architecture principles.

[![Tests](https://github.com/Haleralex/PayBridge/actions/workflows/tests.yml/badge.svg)](https://github.com/Haleralex/PayBridge/actions/workflows/tests.yml)
[![CI](https://github.com/Haleralex/PayBridge/actions/workflows/ci.yml/badge.svg)](https://github.com/Haleralex/PayBridge/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/badge/coverage-56.0%25-brightgreen.svg)](https://github.com/Haleralex/PayBridge)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

PayBridge is a financial-grade payment processing system with support for multi-currency wallets, transactions, KYC verification, and real-time payment processing. The system emphasizes security, reliability, and scalability while maintaining clean, testable code.

---

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [API Endpoints](#api-endpoints)
- [Testing](#testing)
- [Docker](#docker)
- [Development](#development)
- [Security](#security)
- [Production Readiness](#production-readiness)
- [License](#license)

---

## Features

## Features

### Architecture & Design
- **Clean Architecture** - Domain, Application, Infrastructure, and Adapters layers with clear separation of concerns
- **Domain-Driven Design** - Aggregates, Value Objects, Domain Events, and Repository patterns
- **Hexagonal Architecture** - Ports and Adapters for framework-agnostic business logic
- **CQRS Pattern** - Separate command and query responsibilities

### Financial Operations
- **Multi-Currency Wallets** - Support for fiat (USD, EUR, GBP) and crypto (BTC, ETH, USDT, USDC)
- **Transaction Management** - Deposits, withdrawals, transfers, refunds with full lifecycle tracking
- **Optimistic Locking** - Prevents concurrent modification conflicts with version-based locking
- **Idempotency** - Guarantees exactly-once execution using idempotency keys
- **Double-Entry Bookkeeping** - Every transaction affects two accounts for audit consistency
- **Transactional Outbox** - Reliable event delivery with at-least-once semantics

### Security & Compliance
- **JWT Authentication** - Secure token-based authentication with refresh tokens
- **KYC Verification** - User verification workflow with document validation
- **Rate Limiting** - Per-user and per-endpoint rate limits with Redis backend
- **IDOR Protection** - Resource-level authorization checks
- **Audit Logging** - Complete audit trail of all financial operations
- **Input Validation** - Comprehensive validation at all layers
- **Password Security** - bcrypt hashing with configurable cost

### Developer Experience
- **High Test Coverage** - 178 tests across unit, integration, and E2E levels
- **Docker Support** - Multi-stage builds, Docker Compose for local development
- **Structured Logging** - Context-aware logging with slog
- **Comprehensive Documentation** - 25+ docs covering architecture, patterns, and problem-solving
- **CI/CD Ready** - GitHub Actions workflows for testing and deployment
- **Database Migrations** - Version-controlled schema evolution with golang-migrate

---

## Architecture

## 🏗️ Архитектура

### Clean Architecture Layers

```
┌─────────────────────────────────────────────────┐
│            Adapters (HTTP/WS/Webhooks)         │
│  ┌─────────────────────────────────────────┐   │
│  │     Application (Use Cases/Ports)       │   │
│  │  ┌──────────────────────────────────┐   │   │
│  │  │   Domain (Entities/Value Objects)│   │   │
│  │  │        Business Logic            │   │   │
│  │  └──────────────────────────────────┘   │   │
│  └─────────────────────────────────────────┘   │
│            Infrastructure (Postgres/Redis)      │
└─────────────────────────────────────────────────┘
```

Each layer has strict dependencies:
- **Domain** - No external dependencies, pure business logic
- **Application** - Depends on domain, defines use cases and workflows
- **Infrastructure** - Implements application ports, database/external services
- **Adapters** - HTTP/WebSocket/Webhook handlers, depends on application layer

### Tech Stack

| Component | Technology | Purpose |
|-----------|-----------|---------|
| **Language** | Go 1.21+ | Core application language |
| **Web Framework** | Gin v1.9+ | HTTP routing and middleware |
| **Database** | PostgreSQL 16 | Primary data store with ACID guarantees |
| **DB Driver** | pgx v5 | High-performance PostgreSQL driver with connection pooling |
| **Configuration** | Viper | Multi-source config (files, env vars, defaults) |
| **Environment** | godotenv | Automatic .env file loading |
| **Testing** | testify + testcontainers | Unit and isolated integration tests |
| **Containerization** | Docker + Docker Compose | Local development and deployment |
| **Migrations** | SQL files | Version-controlled schema management |
| **Security** | JWT + Rate Limiting | Authentication and DDoS protection |

### Project Structure

```
PayBridge/
├── cmd/
│   ├── api/                        # Application entry point
│   └── migrate/                    # Migration runner
├── configs/
│   ├── config.example.yaml         # Template (committed to Git)
│   └── config.yaml                 # Your secrets (in .gitignore)
├── internal/
│   ├── domain/                     # Core business logic (no external deps)
│   │   ├── entities/               # User, Wallet, Transaction
│   │   ├── valueobjects/           # Money, Currency, TransactionType
│   │   ├── events/                 # WalletCreated, TransactionProcessed
│   │   └── errors/                 # Domain-specific errors
│   ├── application/                # Use cases and workflows
│   │   ├── usecases/               # CreateTransaction, ProcessTransfer, etc
│   │   ├── dtos/                   # Data transfer objects
│   │   └── ports/                  # Repository/service interfaces
│   ├── infrastructure/             # Technical implementations
│   │   └── persistence/            # PostgreSQL repositories, pooling
│   └── adapters/                   # Entry points and external interfaces
│       ├── http/                   # REST API handlers
│       ├── websocket/              # Real-time communication
│       └── webhooks/               # External webhook handling
├── migrations/                     # SQL schema migrations
├── docs/                           # Comprehensive documentation (25+ files)
│   ├── DEEP_DIVE_RU.md            # Architecture deep dive
│   ├── PHASE_*_SUMMARY.md         # Development phase summaries
│   ├── SECURITY_GUIDELINES.md     # Security standards
│   └── adr/                       # Architecture Decision Records
├── scripts/                        # Automation scripts
│   ├── coverage.ps1               # Test coverage analysis
│   ├── security_audit.ps1         # Security scanning
│   └── pre-commit-security.ps1    # Git hooks
└── .env                           # Local secrets (in .gitignore)
```

---

## Quick Start

### Prerequisites

- **Go 1.21+** - [Download](https://go.dev/dl/)
- **PostgreSQL 15+** - Database (or use Docker)
- **Docker & Docker Compose** - Container orchestration (optional but recommended)
- **Make** - Build automation (optional, can use Go commands directly)

### Installation Steps

```bash
# 1. Clone the repository
git clone https://github.com/Haleralex/PayBridge.git
cd PayBridge

# 2. Copy configuration template
cp configs/config.example.yaml configs/config.yaml

# 3. Create .env file with your secrets
cat > .env << 'EOF'
# Telegram Bot Token (get from @BotFather)
PAYBRIDGE_AUTH_TELEGRAM_BOT_TOKEN=your_bot_token_here

# JWT Secret (min 32 characters for production)
PAYBRIDGE_AUTH_JWT_SECRET=your-secure-random-secret-min-32-chars

# Database (override defaults if needed)
PAYBRIDGE_DATABASE_HOST=localhost
PAYBRIDGE_DATABASE_PORT=5432
PAYBRIDGE_DATABASE_PASSWORD=postgres
EOF

# 4. Start PostgreSQL using Docker Compose
docker-compose up -d postgres

# Wait for PostgreSQL to be ready
sleep 3

# 5. Run database migrations
make migrate-up
# Or manually: psql -h localhost -U postgres -d paybridge -f migrations/000001_create_users.up.sql

# 6. Install Go dependencies
go mod download

# 7. Run the application
go run cmd/api/main.go
```

The API server will start on `http://localhost:8080`

### Verify Installation

```bash
# Check health endpoint
curl http://localhost:8080/health

# Expected response:
# {"status":"ok","timestamp":"2024-01-15T10:30:00Z"}

# Check readiness
curl http://localhost:8080/ready
```

### Docker Quick Start

If you prefer Docker for everything:

```bash
# Start all services (PostgreSQL + API + WebApp)
docker-compose up -d

# View logs
docker-compose logs -f app

# Stop all services
docker-compose down
```

---

## Configuration

PayBridge supports multiple configuration methods with clear precedence rules.

### Configuration Priority

**Priority Order (highest to lowest):**
1. Environment variables (`PAYBRIDGE_*`)
2. `.env` file (loaded automatically at startup)
3. `configs/config.yaml` file
4. Built-in default values

### Method 1: .env File (Recommended)

Create a `.env` file in the project root:

```bash
# Authentication
PAYBRIDGE_AUTH_TELEGRAM_BOT_TOKEN=YOUR_TG_TOKEN
PAYBRIDGE_AUTH_JWT_SECRET=my-super-secret-jwt-key-minimum-32-characters

# Database
PAYBRIDGE_DATABASE_HOST=localhost
PAYBRIDGE_DATABASE_PORT=5432
PAYBRIDGE_DATABASE_USER=postgres
PAYBRIDGE_DATABASE_PASSWORD=postgres
PAYBRIDGE_DATABASE_NAME=paybridge
PAYBRIDGE_DATABASE_SSLMODE=disable

# Server
PAYBRIDGE_SERVER_HOST=0.0.0.0
PAYBRIDGE_SERVER_PORT=8080
PAYBRIDGE_SERVER_READ_TIMEOUT=15s
PAYBRIDGE_SERVER_WRITE_TIMEOUT=15s

# Application
PAYBRIDGE_APP_ENVIRONMENT=development
PAYBRIDGE_APP_LOG_LEVEL=info
```

**Advantages:**
- Automatically loaded on application startup (via godotenv)
- Safe for local secrets (`.env` is in `.gitignore`)
- Docker Compose reads `.env` automatically
- No code changes needed

### Method 2: config.yaml File

Edit `configs/config.yaml`:

```yaml
auth:
  telegram_bot_token: "YOUR_TG_TOKEN"
  jwt_secret: "my-super-secret-jwt-key-minimum-32-characters"

database:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "postgres"
  name: "paybridge"
  sslmode: "disable"

server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: "15s"
  write_timeout: "15s"

app:
  environment: "development"
  log_level: "info"
```

**Note:** This file is in `.gitignore` - it's safe to store secrets here locally.

### Method 3: Environment Variables

Set variables directly in your shell:

**PowerShell (Windows):**
```powershell
$env:PAYBRIDGE_AUTH_TELEGRAM_BOT_TOKEN="your_token"
$env:PAYBRIDGE_AUTH_JWT_SECRET="your-secret"
go run cmd/api/main.go
```

**Bash (Linux/macOS):**
```bash
export PAYBRIDGE_AUTH_TELEGRAM_BOT_TOKEN="your_token"
export PAYBRIDGE_AUTH_JWT_SECRET="your-secret"
go run cmd/api/main.go
```

### Docker Compose Configuration

Docker Compose automatically reads `.env` files. The `docker-compose.yml` uses variable interpolation:

```yaml
services:
  app:
    environment:
      - PAYBRIDGE_AUTH_TELEGRAM_BOT_TOKEN=${PAYBRIDGE_AUTH_TELEGRAM_BOT_TOKEN}
      - PAYBRIDGE_AUTH_JWT_SECRET=${PAYBRIDGE_AUTH_JWT_SECRET}
```

Just ensure your `.env` file exists with the required variables.

---

## API Endpoints

### Health & Monitoring
- `GET /health` - Service health check
- `GET /ready` - Readiness probe (checks database connectivity)

### Wallet Management
- `POST /api/v1/wallets` - Create new wallet
- `GET /api/v1/wallets/:id` - Get wallet details and balance
- `GET /api/v1/wallets` - List all wallets
- `GET /api/v1/wallets/:wallet_id/transactions` - Get wallet transaction history

### Transactions
- `POST /api/v1/wallets/:id/credit` - Credit money to wallet
- `POST /api/v1/wallets/:id/debit` - Debit money from wallet
- `POST /api/v1/wallets/:id/transfer` - Transfer money between wallets
- `GET /api/v1/transactions/:id` - Get transaction details

### Authentication
- `POST /api/v1/auth/telegram` - Authenticate via Telegram
- `POST /api/v1/auth/refresh` - Refresh JWT token

**API Specification:** OpenAPI format available in repository

---

## Testing

### Running Tests Locally

**Using PowerShell Scripts (Windows):**
```powershell
# All tests with coverage report
.\test.ps1 test-all

# Unit tests only (no database required)
.\test.ps1 test-unit

# Integration tests only (requires PostgreSQL)
.\test.ps1 test-integration

# Generate coverage report
.\scripts\coverage.ps1
```

**Using Make (Linux/macOS):**
```bash
# All tests with coverage
make test-all

# Unit tests only
make test-unit

# Integration tests only
make test-integration

# Coverage report
make coverage
```

**Using Go Commands Directly:**
```bash
# Unit tests (fast, no external dependencies)
go test ./...

# Integration tests (requires PostgreSQL)
go test -tags=integration ./...

# Coverage report
go test -tags=integration -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Race detector (concurrent safety check)
go test -race -tags=integration ./...
```

### Test Infrastructure

- **Framework:** Go's built-in `testing` package + `testify` assertions
- **Isolation:** `testcontainers-go` for automatic PostgreSQL provisioning
- **Retry Logic:** Exponential backoff for transient failures (deadlocks, timeouts)
- **Parallelization:** Tests run in parallel where safe

**Integration Test Setup:**
```go
// Automatically creates and manages PostgreSQL container
container, db := setupTestDB(t)
defer container.Terminate(ctx)
```

### Test Coverage Statistics

**Overall Coverage:** 56.0%

**By Module:**
- Transaction Creation: 65.6%
- Transaction Transfer: 63.6%
- Transaction Processing: 71.2%
- Transaction Cancellation: 52.4%
- Wallet Operations: 58.3%
- Domain Entities: 72.1%

**Test Count:** 178 tests
- Unit tests: 89 tests
- Integration tests: 89 tests

**Execution Time:**
- Unit tests: ~3.6s
- Integration tests: ~10s
- Total CI time: ~12s (parallel execution)

### CI/CD Testing

GitHub Actions workflow runs automatically on every push and pull request:

1. **Unit Tests** - Fast tests without database
2. **Integration Tests** - With PostgreSQL service container
3. **Race Detector** - Concurrent safety validation
4. **Security Scan** - Gosec vulnerability detection
5. **Linting** - golangci-lint with custom rules
6. **Coverage Report** - Uploaded to coverage service

---

## Docker

### Development with Docker Compose

```bash
# Start all services (PostgreSQL + API + WebApp)
docker-compose up -d

# Start only PostgreSQL
docker-compose up -d postgres

# View logs
docker-compose logs -f app

# View PostgreSQL logs
docker-compose logs -f postgres

# Stop all services
docker-compose down

# Clean up volumes (WARNING: deletes database data)
docker-compose down -v
```

### Building Docker Image

```bash
# Build image manually
docker build -t paybridge:latest .

# Or using Make
make docker-build

# Run container
docker run -p 8080:8080 --env-file .env paybridge:latest
```

### Docker Compose Services

| Service | Port | Description |
|---------|------|-------------|
| **postgres** | 5432 | PostgreSQL 16 database |
| **app** | 8080 | PayBridge API server |
| **webapp** | 8081 | Web interface (if enabled) |

### Database Connection from Host

When running PostgreSQL in Docker Compose:

```bash
# Connect with psql
psql -h localhost -p 5432 -U postgres -d paybridge

# Or using Make
make db-shell

# Or using Docker Compose
docker-compose exec postgres psql -U postgres -d paybridge
```

---

## Development

### Available Make Commands

```bash
make help               # Show all available commands

# Building
make build              # Build Go binary
make docker-build       # Build Docker image

# Running
make run                # Run application locally
make docker-up          # Start all Docker services
make db-up              # Start PostgreSQL only

# Testing
make test-all           # Run all tests with coverage
make test-unit          # Run unit tests only
make test-integration   # Run integration tests
make coverage           # Generate coverage report

# Database
make migrate-up         # Apply all migrations
make migrate-down       # Rollback last migration
make db-shell           # Connect to PostgreSQL shell

# Quality
make lint               # Run golangci-lint
make fmt                # Format code with gofmt
make vet                # Run go vet

# CI/CD
make ci                 # Run full CI pipeline locally
```

### Database Migrations

Migrations are SQL files in the `migrations/` directory:

```
migrations/
├── 000001_create_users.up.sql
├── 000001_create_users.down.sql
├── 000002_create_wallets.up.sql
├── 000002_create_wallets.down.sql
└── ...
```

**Apply Migrations:**
```bash
# Using Make
make migrate-up

# Using migrate tool
migrate -path migrations -database "postgresql://postgres:postgres@localhost:5432/paybridge?sslmode=disable" up

# Manually with psql
psql -h localhost -U postgres -d paybridge -f migrations/000001_create_users.up.sql
```

### Code Quality Tools

**Linting:**
```bash
# Run all linters
golangci-lint run

# Auto-fix issues
golangci-lint run --fix
```

**Security Scanning:**
```bash
# Vulnerability check
govulncheck ./...

# Static analysis
gosec ./...

# Run security audit
.\scripts\security_audit.ps1
```

---

## Security

**Security Rating: 6.5/10** - Production-ready with continuous improvements

### Implemented Security Controls

**Authentication & Authorization:**
- JWT authentication with HS256 signing
- Telegram bot integration for user verification
- Wallet ownership validation (IDOR protection)
- Token refresh mechanism

**Input Validation:**
- Comprehensive request validation
- SQL injection prevention (parameterized queries)
- XSS protection
- Amount validation (positive values, precision limits)

**Data Protection:**
- Optimistic locking for concurrent updates
- Transaction idempotency keys
- Audit logging for all operations
- Structured logging with sensitive data filtering

**Infrastructure Security:**
- Rate limiting (100 req/min general, 30 req/min financial ops)
- Non-root Docker containers
- Environment variable isolation
- Connection pooling with limits

**Transaction Safety:**
- ACID compliance via PostgreSQL transactions
- Deadlock detection and retry mechanism
- State machine validation
- Balance consistency checks

### Security Audit

**Run Full Security Audit:**
```powershell
# Complete security scan (recommended before commits)
.\scripts\security_audit.ps1

# Quick vulnerability check
govulncheck ./...

# Static security analysis
gosec ./...
```

---

## Production Readiness

### Testing & Quality Assurance
- 178 comprehensive tests (unit + integration)
- 56.0% code coverage with detailed module reports
- Zero race conditions detected in stress testing
- Automatic retry mechanism for transient failures
- testcontainers-go for isolated integration testing

### CI/CD Pipeline
- Automated testing on every push/PR
- Parallel test execution (unit + integration)
- golangci-lint with custom security rules
- Gosec vulnerability scanning
- PostgreSQL provisioning in CI environment

### Reliability Features
- Optimistic locking for concurrent wallet updates
- Transaction state machine with validation
- Idempotency keys for duplicate prevention
- Exponential backoff retry mechanism
- Comprehensive error handling with recovery

### Performance Characteristics
- Connection pooling with pgxpool
- Efficient database queries with indexes
- Sub-second response times for most operations
- Horizontal scalability ready (stateless API)

---

## License

This project is licensed under the MIT License. See LICENSE file for details.

---

## Links

- **GitHub Repository:** https://github.com/Haleralex/paybridge
- **Issue Tracker:** https://github.com/Haleralex/paybridge/issues

---

**Built with Go, PostgreSQL, and clean architecture principles.**
