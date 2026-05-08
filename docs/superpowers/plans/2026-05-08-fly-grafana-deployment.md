# Fly.io + Grafana Cloud Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deploy PayBridge to Fly.io with Grafana Cloud tracing, replacing ngrok with a permanent HTTPS URL.

**Architecture:** Three Fly.io VMs (API, NATS, Notifier) + managed Fly Postgres + Grafana Cloud OTLP. Services communicate via Fly private network (`*.internal`). Only the API is publicly accessible at `paybridge-api.fly.dev`.

**Tech Stack:** Go 1.25, Fly.io (flyctl CLI), Grafana Cloud (free tier), GitHub Actions, NATS JetStream, PostgreSQL 16, OpenTelemetry OTLP HTTP.

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `Dockerfile` | Modify | Add migrate binary build + copy migrations dir |
| `internal/infrastructure/telemetry/tracer.go` | Modify | Support HTTPS endpoints + OTEL_EXPORTER_OTLP_HEADERS |
| `internal/infrastructure/telemetry/tracer_test.go` | Create | Unit tests for header parsing |
| `configs/config.production.yaml` | Modify | Fix CORS origins + add telemetry section |
| `fly.api.toml` | Create | Fly config for Go API |
| `fly.nats.toml` | Create | Fly config for NATS broker |
| `fly.notifier.toml` | Create | Fly config for Notifier service |
| `.github/workflows/deploy.yml` | Create | Auto-deploy on push to main |

---

## Task 1: Update Dockerfile — add migrate binary and migrations

**Files:**
- Modify: `Dockerfile`

The migrate binary (`cmd/migrate`) currently is not built in the Docker image, and the `/migrations` directory is not copied to the production stage. The Fly release command needs both.

- [ ] **Step 1: Add migrate build to Dockerfile builder stage**

Open `Dockerfile`. After the existing `go build` command (line 38), add:

```dockerfile
# Build migrate tool
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/migrate \
    ./cmd/migrate
```

- [ ] **Step 2: Copy migrate binary and migrations dir in production stage**

After `COPY --from=builder /app/webapp /app/webapp` (line 61), add:

```dockerfile
# Copy migrate binary and migrations
COPY --from=builder /app/migrate /app/migrate
COPY --from=builder /app/migrations /app/migrations
```

- [ ] **Step 3: Verify image builds locally**

```bash
docker build -t paybridge-test .
```

Expected: Build completes without error. Verify both binaries exist:

```bash
docker run --rm paybridge-test ls /app/
```

Expected output contains: `migrate  migrations  paybridge  configs  webapp`

- [ ] **Step 4: Commit**

```bash
git add Dockerfile
git commit -m "build: add migrate binary and migrations to Docker image"
```

---

## Task 2: Update tracer to support Grafana Cloud

**Files:**
- Modify: `internal/infrastructure/telemetry/tracer.go`
- Create: `internal/infrastructure/telemetry/tracer_test.go`

Currently `tracer.go` uses `WithInsecure()` and `WithEndpoint(host:port)`. Grafana Cloud requires TLS and `Authorization: Basic <key>` header. The fix: switch to `WithEndpointURL()` (auto-detects http/https) and read the standard `OTEL_EXPORTER_OTLP_HEADERS` env var.

- [ ] **Step 1: Write failing test for header parsing**

Create `internal/infrastructure/telemetry/tracer_test.go`:

```go
package telemetry

import (
	"testing"
)

func TestParseOTLPHeaders(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:     "single header",
			input:    "Authorization=Basic dXNlcjpwYXNz",
			expected: map[string]string{"Authorization": "Basic dXNlcjpwYXNz"},
		},
		{
			name:  "multiple headers",
			input: "Authorization=Basic abc,X-Scope-OrgID=1",
			expected: map[string]string{
				"Authorization":  "Basic abc",
				"X-Scope-OrgID": "1",
			},
		},
		{
			name:     "empty string",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:     "whitespace trimmed",
			input:    " Authorization=Basic abc , X-Scope-OrgID=1 ",
			expected: map[string]string{"Authorization": "Basic abc", "X-Scope-OrgID": "1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOTLPHeaders(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("got %d headers, want %d: %v", len(got), len(tt.expected), got)
			}
			for k, v := range tt.expected {
				if got[k] != v {
					t.Errorf("header %q: got %q, want %q", k, got[k], v)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/infrastructure/telemetry/... -v -run TestParseOTLPHeaders
```

Expected: FAIL — `parseOTLPHeaders` is not defined yet.

- [ ] **Step 3: Rewrite tracer.go**

Replace the entire contents of `internal/infrastructure/telemetry/tracer.go`:

```go
package telemetry

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitTracer initializes OpenTelemetry TracerProvider with OTLP HTTP exporter.
// Supports both plain HTTP (Jaeger) and HTTPS with auth headers (Grafana Cloud).
// Set OTEL_EXPORTER_OTLP_HEADERS="Authorization=Basic <key>" for authenticated endpoints.
// Returns TracerProvider for graceful shutdown via tp.Shutdown(ctx).
func InitTracer(ctx context.Context, serviceName, otlpEndpoint string) (*sdktrace.TracerProvider, error) {
	endpoint := normalizeEndpoint(otlpEndpoint)

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpointURL(endpoint),
	}

	if headersEnv := os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"); headersEnv != "" {
		opts = append(opts, otlptracehttp.WithHeaders(parseOTLPHeaders(headersEnv)))
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}

// normalizeEndpoint ensures the endpoint has a URL scheme.
// "jaeger:4318" → "http://jaeger:4318" (backward compat with docker-compose env var)
// "https://..." → unchanged
func normalizeEndpoint(endpoint string) string {
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	return "http://" + endpoint
}

// parseOTLPHeaders parses "key=value,key2=value2" format (OTel standard).
func parseOTLPHeaders(raw string) map[string]string {
	headers := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 && kv[0] != "" {
			headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return headers
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/infrastructure/telemetry/... -v -run TestParseOTLPHeaders
```

Expected: PASS

- [ ] **Step 5: Run full test suite to check for regressions**

```bash
go test -short ./...
```

Expected: all tests pass (integration tests are skipped with `-short`).

- [ ] **Step 6: Commit**

```bash
git add internal/infrastructure/telemetry/tracer.go internal/infrastructure/telemetry/tracer_test.go
git commit -m "feat(telemetry): support HTTPS endpoints and OTEL_EXPORTER_OTLP_HEADERS for Grafana Cloud"
```

---

## Task 3: Update production config

**Files:**
- Modify: `configs/config.production.yaml`

Fix the CORS origins to use the actual Fly.io domain, and add the telemetry section.

- [ ] **Step 1: Update configs/config.production.yaml**

Replace the `cors` section and add a `telemetry` section. The full updated file:

```yaml
# PayBridge Production Configuration
#
# IMPORTANT: Do NOT commit secrets to version control!
# Secrets are set via: fly secrets set KEY=VALUE

app:
  name: "PayBridge"
  version: "1.0.0"
  environment: "production"
  debug: false

server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "120s"
  shutdown_timeout: "60s"

database:
  host: ""        # Set via PAYBRIDGE_DATABASE_HOST secret
  port: 5432
  user: ""        # Set via PAYBRIDGE_DATABASE_USER secret
  password: ""    # Set via PAYBRIDGE_DATABASE_PASSWORD secret
  database: "paybridge"
  ssl_mode: "require"
  max_connections: 10
  min_connections: 2
  max_conn_lifetime: "1h"
  max_conn_idle_time: "30m"

auth:
  jwt_secret: ""      # Set via PAYBRIDGE_AUTH_JWT_SECRET secret
  jwt_issuer: "paybridge"
  access_token_expiry: "15m"
  refresh_token_expiry: "168h"
  enable_mock_auth: false
  telegram_bot_token: ""  # Set via PAYBRIDGE_AUTH_TELEGRAM_BOT_TOKEN secret

cors:
  allowed_origins:
    - "https://paybridge-api.fly.dev"
  allowed_methods:
    - "GET"
    - "POST"
    - "PUT"
    - "PATCH"
    - "DELETE"
    - "OPTIONS"
  allowed_headers:
    - "Origin"
    - "Content-Type"
    - "Accept"
    - "Authorization"
    - "X-Request-ID"
  exposed_headers:
    - "X-Request-ID"
    - "X-RateLimit-Limit"
    - "X-RateLimit-Remaining"
    - "X-RateLimit-Reset"
  allow_credentials: true
  max_age: "12h"

rate_limit:
  enabled: true
  requests_per_minute: 60
  burst_size: 10
  financial_ops_per_min: 20
  cleanup_interval: "5m"

nats:
  url: "nats://paybridge-nats.internal:4222"
  stream_name: "PAYBRIDGE"
  reconnect_wait: "2s"

telemetry:
  enabled: true
  otlp_endpoint: ""  # Set via PAYBRIDGE_TELEMETRY_OTLP_ENDPOINT secret

log:
  level: "info"
  format: "json"
  output: "stdout"
```

- [ ] **Step 2: Verify the config loads without errors**

```bash
go run ./cmd/api -config ./configs -config-name config.production -version
```

Expected: prints version info and exits 0 (config loads, no secrets required for `--version` flag).

- [ ] **Step 3: Commit**

```bash
git add configs/config.production.yaml
git commit -m "config: update production config for Fly.io deployment"
```

---

## Task 4: Create fly.api.toml

**Files:**
- Create: `fly.api.toml`

- [ ] **Step 1: Create fly.api.toml**

```toml
# Fly.io config for PayBridge API
app = "paybridge-api"
primary_region = "fra"

[build]
  dockerfile = "Dockerfile"

[experimental]
  cmd = ["-config", "/app/configs", "-config-name", "config.production"]

[deploy]
  release_command = "/app/migrate -path /app/migrations"

[env]
  PAYBRIDGE_DATABASE_SSL_MODE = "require"
  PAYBRIDGE_NATS_URL = "nats://paybridge-nats.internal:4222"
  PAYBRIDGE_TELEMETRY_ENABLED = "true"

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = false
  auto_start_machines = true
  min_machines_running = 1

  [http_service.concurrency]
    type = "connections"
    hard_limit = 25
    soft_limit = 20

[[vm]]
  memory = "256mb"
  cpu_kind = "shared"
  cpus = 1
```

- [ ] **Step 2: Commit**

```bash
git add fly.api.toml
git commit -m "deploy: add Fly.io config for API service"
```

---

## Task 5: Create fly.nats.toml

**Files:**
- Create: `fly.nats.toml`

NATS is deployed from the official Docker image. It is internal-only — no public ports exposed. Other Fly apps reach it at `paybridge-nats.internal:4222` via the Fly private network.

- [ ] **Step 1: Create fly.nats.toml**

```toml
# Fly.io config for NATS JetStream broker
# Internal-only service — not publicly accessible
app = "paybridge-nats"
primary_region = "fra"

[build]
  image = "nats:alpine"

[processes]
  app = "--jetstream --store_dir /data --http_port 8222"

[[mounts]]
  source = "nats_data"
  destination = "/data"

[checks]
  [checks.health]
    grace_period = "10s"
    interval = "15s"
    method = "get"
    path = "/healthz"
    port = 8222
    timeout = "5s"
    type = "http"

[[vm]]
  memory = "256mb"
  cpu_kind = "shared"
  cpus = 1
```

- [ ] **Step 2: Commit**

```bash
git add fly.nats.toml
git commit -m "deploy: add Fly.io config for NATS broker"
```

---

## Task 6: Create fly.notifier.toml

**Files:**
- Create: `fly.notifier.toml`

The Notifier is a long-running Go service that subscribes to NATS and sends Telegram notifications. It has no HTTP port — Fly.io runs it as a worker process.

- [ ] **Step 1: Create fly.notifier.toml**

```toml
# Fly.io config for PayBridge Notifier service
app = "paybridge-notifier"
primary_region = "fra"

[build]
  dockerfile = "Dockerfile.notifier"

[experimental]
  cmd = ["-config", "/app/configs", "-config-name", "config.production"]

[env]
  PAYBRIDGE_DATABASE_SSL_MODE = "require"
  PAYBRIDGE_NATS_URL = "nats://paybridge-nats.internal:4222"
  PAYBRIDGE_TELEMETRY_ENABLED = "true"

[[vm]]
  memory = "256mb"
  cpu_kind = "shared"
  cpus = 1
```

- [ ] **Step 2: Commit**

```bash
git add fly.notifier.toml
git commit -m "deploy: add Fly.io config for Notifier service"
```

---

## Task 7: Create GitHub Actions deploy workflow

**Files:**
- Create: `.github/workflows/deploy.yml`

Triggers on push to `main`. Deploys API and Notifier (NATS uses a static image and is deployed manually only once).

- [ ] **Step 1: Create .github/workflows/deploy.yml**

```yaml
name: Deploy to Fly.io

on:
  push:
    branches:
      - main

jobs:
  deploy-api:
    name: Deploy API
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: superfly/flyctl-actions/setup-flyctl@master

      - name: Deploy API
        run: flyctl deploy --config fly.api.toml --remote-only
        env:
          FLY_API_TOKEN: ${{ secrets.FLY_API_TOKEN }}

  deploy-notifier:
    name: Deploy Notifier
    runs-on: ubuntu-latest
    needs: deploy-api
    steps:
      - uses: actions/checkout@v4

      - uses: superfly/flyctl-actions/setup-flyctl@master

      - name: Deploy Notifier
        run: flyctl deploy --config fly.notifier.toml --remote-only
        env:
          FLY_API_TOKEN: ${{ secrets.FLY_API_TOKEN }}
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/deploy.yml
git commit -m "ci: add GitHub Actions workflow for Fly.io deployment"
```

---

## Task 8: First deploy — manual steps (one-time)

This task is a checklist of manual steps. No code to write — follow each step in order.

### Prerequisites

- [ ] **Step 1: Install flyctl**

```bash
# Windows (PowerShell)
iwr https://fly.io/install.ps1 -useb | iex
```

Verify: `flyctl version`

- [ ] **Step 2: Create Fly.io account and login**

```bash
flyctl auth signup   # or: flyctl auth login
```

- [ ] **Step 3: Create Grafana Cloud account**

Go to https://grafana.com/auth/sign-up — choose the free plan.

After signup:
1. Go to **Connections → Add new connection → OpenTelemetry**
2. Copy the **OTLP endpoint URL** (looks like: `https://otlp-gateway-prod-us-central-0.grafana.net/otlp`)
3. Create an **API token** (Grafana Cloud → My Account → Security → API Keys → Add)
4. Encode credentials: `echo -n "<instanceID>:<API-token>" | base64` — save the result as `<GRAFANA_BASE64>`

### Deploy NATS (once)

- [ ] **Step 4: Create NATS app**

```bash
flyctl apps create paybridge-nats --org personal
flyctl volumes create nats_data --app paybridge-nats --size 1 --region fra
flyctl deploy --config fly.nats.toml --app paybridge-nats
```

Expected: NATS running, accessible at `paybridge-nats.internal:4222`

### Deploy PostgreSQL (once)

- [ ] **Step 5: Create Fly Postgres**

```bash
flyctl postgres create \
  --name paybridge-db \
  --region fra \
  --initial-cluster-size 1 \
  --vm-size shared-cpu-1x \
  --volume-size 1 \
  --database-name paybridge
```

Save the connection string shown in the output — it looks like:
`postgres://paybridge_db:<password>@paybridge-db.internal:5432/paybridge`

Note the individual values: host = `paybridge-db.internal`, user = `paybridge_db`, password = `<password>`, database = `paybridge`

### Deploy API

- [ ] **Step 6: Create API app**

```bash
flyctl apps create paybridge-api --org personal
```

- [ ] **Step 7: Set API secrets**

Replace placeholders with your real values:

```bash
flyctl secrets set --app paybridge-api \
  PAYBRIDGE_DATABASE_HOST="paybridge-db.internal" \
  PAYBRIDGE_DATABASE_USER="paybridge_db" \
  PAYBRIDGE_DATABASE_PASSWORD="<password-from-step-5>" \
  PAYBRIDGE_DATABASE_DATABASE="paybridge" \
  PAYBRIDGE_AUTH_JWT_SECRET="$(openssl rand -hex 32)" \
  PAYBRIDGE_AUTH_TELEGRAM_BOT_TOKEN="<your-bot-token-from-BotFather>" \
  PAYBRIDGE_TELEMETRY_OTLP_ENDPOINT="<grafana-otlp-endpoint-from-step-3>" \
  OTEL_EXPORTER_OTLP_HEADERS="Authorization=Basic <GRAFANA_BASE64-from-step-3>" \
  DATABASE_URL="postgres://paybridge_db:<password>@paybridge-db.internal:5432/paybridge?sslmode=require"
```

- [ ] **Step 8: Deploy API (includes migrations via release_command)**

```bash
flyctl deploy --config fly.api.toml
```

Watch the logs — migration runs first, then the API starts.

- [ ] **Step 9: Verify API is live**

```bash
flyctl status --app paybridge-api
curl https://paybridge-api.fly.dev/health
```

Expected: `{"status":"ok","..."}` with HTTP 200

### Deploy Notifier

- [ ] **Step 10: Create Notifier app and set secrets**

```bash
flyctl apps create paybridge-notifier --org personal

flyctl secrets set --app paybridge-notifier \
  PAYBRIDGE_DATABASE_HOST="paybridge-db.internal" \
  PAYBRIDGE_DATABASE_USER="paybridge_db" \
  PAYBRIDGE_DATABASE_PASSWORD="<password-from-step-5>" \
  PAYBRIDGE_DATABASE_NAME="paybridge_db" \
  PAYBRIDGE_DATABASE_SSLMODE="require" \
  PAYBRIDGE_AUTH_TELEGRAM_BOT_TOKEN="<your-bot-token-from-BotFather>" \
  PAYBRIDGE_TELEMETRY_OTLP_ENDPOINT="<grafana-otlp-endpoint>" \
  OTEL_EXPORTER_OTLP_HEADERS="Authorization=Basic <GRAFANA_BASE64>"
```

- [ ] **Step 11: Deploy Notifier**

```bash
flyctl deploy --config fly.notifier.toml
```

- [ ] **Step 12: Set FLY_API_TOKEN in GitHub for CI/CD**

```bash
flyctl tokens create deploy -x 999999h
```

Copy the token, then go to: GitHub repo → Settings → Secrets → Actions → New secret
Name: `FLY_API_TOKEN`, Value: paste the token.

- [ ] **Step 13: Set Telegram Mini App webhook URL**

In BotFather or your Telegram Mini App settings, update the web app URL to:
```
https://paybridge-api.fly.dev/m/
```

- [ ] **Step 14: Smoke test**

Open the Telegram Mini App and verify:
1. App loads (no ngrok interstitial)
2. Auth works (Telegram login → JWT token returned)
3. Wallet operations work
4. Check Grafana Cloud → Explore → Traces — traces appear within 30 seconds
