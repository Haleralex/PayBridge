.PHONY: help build run test migrate-up migrate-down docker-up docker-down

help: ## Show this help
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the application
	go build -o bin/paybridge-api cmd/api/main.go

run: ## Run the application
	go run cmd/api/main.go

test: ## Run all tests
	go test -v -race -cover ./...

test-unit: ## Run only unit tests (fast, ~5s)
	go test -v -race -coverprofile=coverage_unit.out ./internal/application/usecases/transaction/...
	@echo "\n✅ Unit tests completed in ~5 seconds"

test-integration: ## Run integration tests (requires PostgreSQL, ~10s)
	go test -tags=integration -v -race -coverprofile=coverage_integration.out ./internal/application/usecases/transaction/...
	@echo "\n✅ Integration tests completed in ~10 seconds"

test-all: ## Run both unit and integration tests (~15s total)
	@echo "Running unit tests..."
	@$(MAKE) test-unit
	@echo "\nRunning integration tests..."
	@$(MAKE) test-integration
	@echo "\n✅ All tests completed"

test-coverage: ## Run tests with coverage report
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: coverage.html"

test-coverage-func: ## Show coverage by function
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

test-bench: ## Run benchmarks
	go test -bench=. -benchmem ./...

test-verbose: ## Run tests with verbose output
	go test -v -race -cover -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

test-watch: ## Watch and run tests on change (requires entr)
	find . -name "*.go" | entr -c go test -v ./...

test-ci: ## Run tests in CI environment
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

migrate-up: ## Run database migrations
	@echo "Running migrations..."
	psql $(DATABASE_URL) -f internal/infrastructure/persistence/migrations/001_create_users.sql
	psql $(DATABASE_URL) -f internal/infrastructure/persistence/migrations/002_create_wallets.sql
	psql $(DATABASE_URL) -f internal/infrastructure/persistence/migrations/003_create_transactions.sql
	psql $(DATABASE_URL) -f internal/infrastructure/persistence/migrations/004_create_outbox.sql
	@echo "✅ Migrations completed"

migrate-down: ## Rollback database migrations
	@echo "Rolling back migrations..."
	psql $(DATABASE_URL) -c "DROP TABLE IF EXISTS outbox_events CASCADE;"
	psql $(DATABASE_URL) -c "DROP TABLE IF EXISTS transactions CASCADE;"
	psql $(DATABASE_URL) -c "DROP TABLE IF EXISTS wallets CASCADE;"
	psql $(DATABASE_URL) -c "DROP TABLE IF EXISTS users CASCADE;"
	@echo "✅ Rollback completed"

docker-up: ## Start PostgreSQL in Docker
	docker run -d \
		--name paybridge-postgres \
		-e POSTGRES_USER=postgres \
		-e POSTGRES_PASSWORD=postgres \
		-e POSTGRES_DB=paybridge \
		-p 5432:5432 \
		postgres:15-alpine
	@echo "✅ PostgreSQL started on localhost:5432"
	@echo "   Database: paybridge"
	@echo "   User: postgres"
	@echo "   Password: postgres"

docker-down: ## Stop PostgreSQL container
	docker stop paybridge-postgres || true
	docker rm paybridge-postgres || true
	@echo "✅ PostgreSQL stopped"

docker-logs: ## Show PostgreSQL logs
	docker logs -f paybridge-postgres

install: ## Install dependencies
	go mod download
	go mod tidy

fmt: ## Format code
	go fmt ./...

lint: ## Run linter
	golangci-lint run --timeout=5m

ci-test: ## Run tests as in CI (with race detector and coverage)
	@echo "Running CI tests..."
	go test -tags=integration -v -race -coverprofile=coverage_ci.out -covermode=atomic ./internal/application/usecases/transaction/...
	@echo "\n📊 Coverage Summary:"
	@go tool cover -func=coverage_ci.out | tail -20
	@echo "\n✅ CI tests completed"

ci-setup-db: ## Setup test database for CI
	@echo "Setting up test database..."
	psql -h localhost -U postgres -c "CREATE DATABASE wallethub_test;" || true
	@for migration in internal/infrastructure/persistence/migrations/*_up.sql; do \
		echo "Applying $$migration..."; \
		psql -h localhost -U postgres -d wallethub_test -f "$$migration"; \
	done
	@echo "✅ Database ready"

ci-local: ## Run full CI pipeline locally
	@echo "🚀 Running full CI pipeline locally...\n"
	@$(MAKE) fmt
	@$(MAKE) lint
	@$(MAKE) ci-setup-db
	@$(MAKE) ci-test
	@echo "\n✅ Local CI pipeline completed successfully!"
	go fmt ./...

lint: ## Run linter
	golangci-lint run

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html
