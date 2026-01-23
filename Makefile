# PayBridge Makefile
#
# Usage:
#   make help       - Show available commands
#   make build      - Build the application
#   make run        - Run the application
#   make test       - Run all tests
#   make docker-up  - Start Docker containers

# ============================================
# Variables
# ============================================

APP_NAME := paybridge
MAIN_PATH := ./cmd/api
BUILD_DIR := ./bin
CONFIG_PATH := ./configs

# Version info (can be overridden)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go settings
GO := go
GOFLAGS := -v
LDFLAGS := -ldflags="-w -s -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

# Docker settings
DOCKER_COMPOSE := docker-compose
DOCKER_IMAGE := paybridge-api

# Database URL
DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/paybridge?sslmode=disable

.PHONY: help build run test migrate-up migrate-down docker-up docker-down clean lint fmt

# ============================================
# Help
# ============================================

help: ## Show this help message
	@echo "PayBridge - Payment Gateway Service"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ============================================
# Build
# ============================================

build: ## Build the application
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME)"

build-linux: ## Build for Linux (amd64)
	@echo "Building $(APP_NAME) for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME)-linux-amd64"

build-windows: ## Build for Windows (amd64)
	@echo "Building $(APP_NAME) for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage*.out coverage*.html
	$(GO) clean
	@echo "Clean complete"

# ============================================
# Run
# ============================================

run: ## Run the application
	@echo "Starting $(APP_NAME)..."
	$(GO) run $(MAIN_PATH) -config $(CONFIG_PATH)

run-env: ## Run with environment variables only
	$(GO) run $(MAIN_PATH) -env-only

run-dev: ## Run with hot reload (requires air)
	@command -v air >/dev/null 2>&1 || { echo "Installing air..."; go install github.com/air-verse/air@latest; }
	air

# ============================================
# Test
# ============================================

test: ## Run all tests
	@echo "Running tests..."
	$(GO) test -v -race -cover ./...

test-unit: ## Run only unit tests (fast)
	$(GO) test -v -race -short ./...

test-integration: ## Run integration tests (requires Docker)
	$(GO) test -tags=integration -v -race ./...

test-coverage: ## Run tests with coverage report
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-coverage-func: ## Show coverage by function
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

test-bench: ## Run benchmarks
	$(GO) test -bench=. -benchmem ./...

test-ci: ## Run tests in CI environment
	$(GO) test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -func=coverage.out

# ============================================
# Lint & Format
# ============================================

fmt: ## Format code
	@echo "Formatting code..."
	$(GO) fmt ./...
	@echo "Format complete"

lint: ## Run linter (requires golangci-lint)
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Installing golangci-lint..."; go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; }
	golangci-lint run --timeout=5m ./...

vet: ## Run go vet
	@echo "Running go vet..."
	$(GO) vet ./...

tidy: ## Tidy go modules
	@echo "Tidying modules..."
	$(GO) mod tidy
	$(GO) mod verify

# ============================================
# Docker
# ============================================

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(DOCKER_IMAGE):$(VERSION) \
		-t $(DOCKER_IMAGE):latest \
		.

docker-up: ## Start all Docker containers
	@echo "Starting Docker containers..."
	$(DOCKER_COMPOSE) up -d

docker-down: ## Stop all Docker containers
	@echo "Stopping Docker containers..."
	$(DOCKER_COMPOSE) down

docker-logs: ## View Docker logs
	$(DOCKER_COMPOSE) logs -f

docker-ps: ## Show Docker container status
	$(DOCKER_COMPOSE) ps

docker-restart: docker-down docker-up ## Restart Docker containers

docker-clean: ## Remove Docker containers and volumes
	@echo "Cleaning Docker resources..."
	$(DOCKER_COMPOSE) down -v --remove-orphans
	docker rmi $(DOCKER_IMAGE):latest 2>/dev/null || true

# ============================================
# Database
# ============================================

db-up: ## Start PostgreSQL container only
	@echo "Starting PostgreSQL..."
	$(DOCKER_COMPOSE) up -d postgres
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 3
	@echo "PostgreSQL is ready on localhost:5432"

db-down: ## Stop PostgreSQL container
	@echo "Stopping PostgreSQL..."
	$(DOCKER_COMPOSE) stop postgres

db-shell: ## Connect to PostgreSQL shell
	$(DOCKER_COMPOSE) exec postgres psql -U postgres -d paybridge

db-logs: ## Show PostgreSQL logs
	$(DOCKER_COMPOSE) logs -f postgres

migrate-up: ## Run database migrations
	@echo "Running migrations..."
	@for migration in internal/infrastructure/persistence/migrations/*_up.sql; do \
		if [ -f "$$migration" ]; then \
			echo "Applying $$migration..."; \
			psql "$(DATABASE_URL)" -f "$$migration"; \
		fi \
	done
	@echo "Migrations completed"

migrate-down: ## Rollback database migrations
	@echo "Rolling back migrations..."
	@for migration in $$(ls -r internal/infrastructure/persistence/migrations/*_down.sql 2>/dev/null); do \
		if [ -f "$$migration" ]; then \
			echo "Reverting $$migration..."; \
			psql "$(DATABASE_URL)" -f "$$migration"; \
		fi \
	done
	@echo "Rollback completed"

db-reset: ## Reset database (drop and recreate)
	@echo "Resetting database..."
	$(DOCKER_COMPOSE) exec postgres psql -U postgres -c "DROP DATABASE IF EXISTS paybridge;"
	$(DOCKER_COMPOSE) exec postgres psql -U postgres -c "CREATE DATABASE paybridge;"
	@echo "Database reset complete"

# ============================================
# Development Tools
# ============================================

tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/air-verse/air@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	@echo "Tools installed"

swagger: ## Generate Swagger documentation
	@command -v swag >/dev/null 2>&1 || { echo "Installing swag..."; go install github.com/swaggo/swag/cmd/swag@latest; }
	swag init -g cmd/api/main.go -o ./docs/swagger

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod tidy
	@echo "Dependencies ready"

# ============================================
# CI/CD
# ============================================

ci: fmt lint test build ## Run CI pipeline (fmt, lint, test, build)
	@echo "CI pipeline complete"

ci-setup-db: ## Setup test database for CI
	@echo "Setting up test database..."
	psql -h localhost -U postgres -c "CREATE DATABASE paybridge_test;" || true
	@echo "Database ready"

version: ## Show version info
	@echo "Version:    $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"

# ============================================
# Default target
# ============================================

.DEFAULT_GOAL := help
