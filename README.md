# PayBridge - Digital Payment Platform

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
- **PostgreSQL** - Primary database
- **Redis** - Caching layer
- **Kafka** - Event streaming
- **Docker** - Containerization

## 🧪 Testing

```bash
# Unit tests
go test ./internal/application/usecases/transaction/...

# Integration tests (requires PostgreSQL)
go test -tags=integration ./internal/application/usecases/transaction/...

# With coverage
go test -tags=integration -coverprofile=coverage.out ./...
```

**Current Status:**
- 178 tests passing
- 61.3% coverage
- 0 race conditions
- CI/CD ready

## 🏃 Quick Start

### Prerequisites
- Go 1.21+
- Docker & Docker Compose
- PostgreSQL 15+

### Setup

```bash
# Clone repository
git clone https://github.com/yourusername/paybridge.git
cd paybridge

# Start PostgreSQL
docker run -d \
  --name paybridge-postgres \
  -e POSTGRES_PASSWORD=postgres \
  -p 5432:5432 \
  postgres:15-alpine

# Run migrations
for f in internal/infrastructure/persistence/migrations/*_up.sql; do
  psql -h localhost -U postgres -f "$f"
done

# Run application
go run cmd/api/main.go
```

### Configuration

Set environment variables:

```bash
DATABASE_URL=postgresql://postgres:postgres@localhost:5432/paybridge?sslmode=disable
REDIS_URL=redis://localhost:6379
KAFKA_BROKERS=localhost:9092
```

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
.\test.ps1 test-all

# Make (Linux/Mac)
make test-all
```

### Database Migrations

Migrations are located in `internal/infrastructure/persistence/migrations/`

Apply manually:
```bash
psql -h localhost -U postgres -d paybridge -f migrations/001_create_users_up.sql
```

## 🐳 Docker

```bash
# Build image
docker build -t paybridge:latest .

# Run with docker-compose
docker-compose up -d
```

## 🔒 Security Features

- Optimistic locking for wallet balance
- Idempotency keys for duplicate prevention
- Transaction state machine
- Retry mechanism with exponential backoff
- Rate limiting on sensitive endpoints

## 🎯 Production Ready

- ✅ Comprehensive test coverage
- ✅ CI/CD with GitHub Actions
- ✅ Race detector validated
- ✅ Concurrent operations tested
- ✅ PostgreSQL integration verified
- ✅ Error handling & recovery

## 📊 Performance

- Unit tests: ~5s
- Integration tests: ~10s
- Supports concurrent operations with retry mechanism
- 100% success rate on concurrent wallet operations

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
- GitHub: https://github.com/yourusername/paybridge
