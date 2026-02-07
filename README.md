# PayBridge - Digital Payment Platform

[![Tests](https://github.com/Haleralex/PayBridge/actions/workflows/tests.yml/badge.svg)](https://github.com/Haleralex/PayBridge/actions/workflows/tests.yml)
[![CI](https://github.com/Haleralex/PayBridge/actions/workflows/ci.yml/badge.svg)](https://github.com/Haleralex/PayBridge/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Haleralex/PayBridge)](https://goreportcard.com/report/github.com/Haleralex/PayBridge)
[![Coverage](https://img.shields.io/badge/coverage-61.3%25-brightgreen.svg)](https://github.com/Haleralex/PayBridge)

A production-ready payment gateway backend built with Go, featuring wallet management, transactions, and real-time processing.

## 🏗️ Architecture

Built on **Clean Architecture** principles with clear separation of concerns:

```
internal/
├── domain/              # Business logic & rules
├── application/         # Use cases & workflows
├── infrastructure/      # Database, messaging, cache
└── adapters/           # HTTP, WebSocket, webhooks
```

## 🚀 Tech Stack

- **Go 1.21+** - Core language
- **PostgreSQL** - Primary database with pgx driver
- **Gin** - HTTP framework
- **Viper** - Configuration management
- **Docker** - Containerization
- **testcontainers-go** - Integration testing

## 🧪 Testing

### Local Testing

```bash
# Unit tests only
go test ./internal/application/usecases/transaction/...

# Integration tests (requires PostgreSQL)
go test -tags=integration ./internal/application/usecases/transaction/...

# All tests with coverage
go test -tags=integration -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Race detector
go test -race -tags=integration ./...
```

### CI/CD Testing

The project uses GitHub Actions for automated testing:

- **Unit Tests**: Run on every push/PR (no database required)
- **Integration Tests**: Run with PostgreSQL service container
- **Race Detector**: Validates concurrent operations
- **Coverage Analysis**: Tracks code coverage metrics
- **Gosec Security Scan**: Checks for security vulnerabilities

**Test Statistics:**
- 178 tests passing (19 unit + 29 integration in transaction package)
- 61.3% overall coverage
- 0 race conditions detected
- ~12s CI execution time (parallel jobs)

## 🏃 Quick Start

### Prerequisites
- Go 1.21+
- Docker & Docker Compose
- PostgreSQL 15+

### Setup

```bash
# Clone repository
git clone https://github.com/Haleralex/paybridge.git
cd paybridge

# Start PostgreSQL with Docker Compose
make db-up

# Or use full Docker Compose
docker-compose up -d

# Run application
make run

# Or manually
go run cmd/api/main.go -config ./configs
```

### Configuration

Configuration via YAML (`configs/config.yaml`) or environment variables:

```bash
# Environment variables
PAYBRIDGE_APP_ENVIRONMENT=development
PAYBRIDGE_SERVER_PORT=8080
PAYBRIDGE_DATABASE_HOST=localhost
PAYBRIDGE_DATABASE_PASSWORD=postgres
PAYBRIDGE_AUTH_JWT_SECRET=your-secret-key

# Or legacy format
DATABASE_URL=postgresql://postgres:postgres@localhost:5432/paybridge?sslmode=disable
```

See `.env.example` for all available options.

## 📡 API Endpoints

### Health
- `GET /health` - Service health check
- `GET /ready` - Readiness probe

### Wallets
- `POST /api/v1/wallets` - Create wallet
- `GET /api/v1/wallets/:id` - Get wallet details
- `GET /api/v1/wallets` - List wallets

### Transactions
- `POST /api/v1/wallets/:id/credit` - Credit wallet
- `POST /api/v1/wallets/:id/debit` - Debit wallet
- `POST /api/v1/wallets/:id/transfer` - Transfer between wallets
- `GET /api/v1/transactions/:id` - Get transaction

## 🔧 Development

### Running Tests

```bash
# PowerShell (Windows)
.\test.ps1 test-all              # All tests with coverage
.\test.ps1 test-unit             # Unit tests only
.\test.ps1 test-integration      # Integration tests only

# Make (Linux/Mac)
make test-all                     # All tests with coverage
make test-unit                    # Unit tests only
make test-integration            # Integration tests only

# Go commands directly
go test ./...                                          # Unit tests
go test -tags=integration ./...                       # All tests
go test -race -tags=integration ./...                 # With race detector
```

### Test Infrastructure

Integration tests use **testcontainers-go** for automatic PostgreSQL provisioning:

```go
// Automatically creates and manages PostgreSQL container
container, db := setupTestDB(t)
defer container.Terminate(ctx)
```

**Retry Mechanism** - Handles transient failures:
- 10 retry attempts
- Exponential backoff (10ms-1000ms)
- Automatic recovery from deadlocks

### Database Migrations

Migrations are located in `internal/infrastructure/persistence/migrations/`

Apply manually:
```bash
# PostgreSQL
for f in migrations/*_up.sql; do
  psql -h localhost -U postgres -d paybridge -f "$f"
done

# Or individually
psql -h localhost -U postgres -d paybridge -f migrations/001_create_users_up.sql
```

## 🐳 Docker

```bash
# Build image
make docker-build

# Run with docker-compose
docker-compose up -d

# View logs
docker-compose logs -f app

# Stop all services
docker-compose down
```

### Available Make Commands

```bash
make help           # Show all commands
make build          # Build application
make run            # Run locally
make test           # Run all tests
make test-unit      # Unit tests only
make docker-up      # Start all containers
make db-up          # Start PostgreSQL only
make db-shell       # Connect to database
make lint           # Run linter
make ci             # Full CI pipeline
```

## 🔒 Security Features

**Rating: 6.5/10** - Production-ready with recent security improvements

### Implemented Security Controls
- ✅ **JWT Authentication** - Production HS256 implementation
- ✅ **Wallet Ownership Validation** - IDOR protection
- ✅ **SQL Injection Prevention** - Parameterized queries throughout
- ✅ **Optimistic Locking** - Concurrent update protection
- ✅ **Idempotency Keys** - Duplicate transaction prevention
- ✅ **Rate Limiting** - DDoS protection (100 req/min, 30 for financial ops)
- ✅ **Input Validation** - Comprehensive validation framework
- ✅ **Audit Logging** - Structured logging with context
- ✅ **Non-root Docker** - Container security

### Security Development Process
- 📋 [Security Quick Start](SECURITY_QUICK_START.md) - New developer onboarding
- ✅ [Security Checklist](SECURITY_CHECKLIST.md) - PR review requirements
- 🧪 [Security Testing Guide](SECURITY_TESTING.md) - Testing practices
- 📖 [Security Guidelines](docs/SECURITY_GUIDELINES.md) - Development standards
- 🔍 [Security Audit Report](docs/security-audit.md) - Latest findings

### Running Security Checks
```powershell
# Full security audit (recommended before each commit)
.\scripts\security_audit.ps1

# Quick vulnerability scan
govulncheck ./...

# Static analysis
gosec ./...

# Run security tests
go test -tags=security ./internal/adapters/http/...
```

### For New Developers
**Required:** Read [SECURITY_QUICK_START.md](SECURITY_QUICK_START.md) before contributing

## 🎯 Production Ready

### Testing & Quality
- ✅ 178 comprehensive tests (unit + integration)
- ✅ 61.3% code coverage with detailed reports
- ✅ Race detector validated (0 race conditions)
- ✅ Concurrent operations tested with retry mechanism
- ✅ testcontainers-go for isolated integration tests

### CI/CD Pipeline
- ✅ GitHub Actions workflows (tests + lint + security)
- ✅ Parallel test execution (unit + integration)
- ✅ golangci-lint with custom rules
- ✅ Gosec security scanner
- ✅ Automated PostgreSQL provisioning in CI

### Reliability Features
- ✅ Optimistic locking for wallet balance
- ✅ Transaction state machine with validation
- ✅ Idempotency keys for duplicate prevention
- ✅ Exponential backoff retry mechanism
- ✅ Comprehensive error handling & recovery

## 📊 Performance

### Test Execution
- Unit tests: ~3.6s (19 tests in transaction package)
- Integration tests: ~10s (29 tests with real PostgreSQL)
- CI parallel execution: ~12s (unit + integration in parallel)
- Total: 178 tests across entire codebase

### Coverage by Module
- Transaction Creation: 65.6%
- Transaction Transfer: 63.6%
- Transaction Processing: 71.2%
- Transaction Cancellation: 52.4%
- Overall: 61.3%

### Reliability
- 100% success rate on concurrent wallet operations
- Automatic retry on transient failures (deadlocks, timeouts)
- Zero race conditions in stress testing
- PostgreSQL connection pooling optimized

## 🤝 Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request

## 📝 License

This project is licensed under the MIT License.

## 🔗 Links

- API Documentation: `/api/docs`
- OpenAPI Spec: `api/openapi.yaml`
- GitHub: https://github.com/Haleralex/paybridge
