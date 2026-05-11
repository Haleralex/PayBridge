# Observability (LGTM) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up a complete LGTM observability stack: Grafana Alloy locally (scrapes metrics + collects Docker logs → Grafana Cloud), OTel Metrics push for Fly.io prod, and manual DB spans for deeper tracing.

**Architecture:** Locally, Grafana Alloy runs in docker-compose to scrape `/metrics` → Grafana Cloud Mimir and collect Docker container stdout → Grafana Cloud Loki. Traces already flow via OTel SDK → Grafana Cloud Tempo (no changes needed). On Fly.io prod, a new `InitMeterProvider` uses a Prometheus bridge to push existing metrics via OTLP; logs use Fly.io's native HTTPS log drain → Loki.

**Tech Stack:** Grafana Alloy (River config), OTel Go SDK v1.40.0, `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp`, `go.opentelemetry.io/contrib/bridges/prometheus`, Prometheus client_golang (unchanged), Fly.io HTTPS log drain.

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `docker-compose.yml` | Modify | Remove jaeger, add alloy service, move OTLP endpoint to env var |
| `alloy-config.river` | Create | Alloy: scrape `/metrics` → Mimir, Docker logs → Loki |
| `.env.example` | Modify | Add `GRAFANA_CLOUD_*` and `PAYBRIDGE_TELEMETRY_OTLP_ENDPOINT` vars |
| `go.mod` / `go.sum` | Modify | Add `otlpmetrichttp`, `sdk/metric`, `bridges/prometheus` |
| `internal/infrastructure/telemetry/metrics_provider.go` | Create | `InitMeterProvider` — OTLP push + Prometheus bridge |
| `internal/container/container.go` | Modify | Add `meterProvider` field, `initMetrics()`, shutdown hook |
| `internal/infrastructure/persistence/postgres/wallet_repository.go` | Modify | Add OTel span to `FindByID` (learning: DB tracing) |
| `fly.api.toml` | Modify | Document log drain setup as comment |

---

### Task 1: Replace Jaeger with Alloy in docker-compose

> **Concept — Traces:** Jaeger was a local trace collector running in docker-compose. We no longer need it because traces go directly to Grafana Cloud Tempo via OTLP. Alloy takes over two new responsibilities: scraping Prometheus metrics and collecting Docker container logs.

**Files:**
- Modify: `docker-compose.yml`

- [ ] **Step 1: Remove the entire `jaeger` service block**

Delete lines 174–184 from `docker-compose.yml` (the full `jaeger:` service):
```yaml
  # DELETE THIS ENTIRE BLOCK:
  jaeger:
    image: jaegertracing/all-in-one:latest
    container_name: paybridge-jaeger
    environment:
      - COLLECTOR_OTLP_ENABLED=true
    ports:
      - "16686:16686"
      - "4318:4318"
    networks:
      - paybridge-network
```

- [ ] **Step 2: Update `PAYBRIDGE_TELEMETRY_OTLP_ENDPOINT` in the `app` service**

Change the hardcoded jaeger address to an env var so `.env` controls the endpoint:
```yaml
      # BEFORE:
      - PAYBRIDGE_TELEMETRY_OTLP_ENDPOINT=jaeger:4318
      # AFTER:
      - PAYBRIDGE_TELEMETRY_OTLP_ENDPOINT=${PAYBRIDGE_TELEMETRY_OTLP_ENDPOINT}
```

- [ ] **Step 3: Same change for `notifier` and `fraud-detector` services**

In `notifier` environment section:
```yaml
      - PAYBRIDGE_TELEMETRY_OTLP_ENDPOINT=${PAYBRIDGE_TELEMETRY_OTLP_ENDPOINT}
```
In `fraud-detector` environment section:
```yaml
      - PAYBRIDGE_TELEMETRY_OTLP_ENDPOINT=${PAYBRIDGE_TELEMETRY_OTLP_ENDPOINT}
```

- [ ] **Step 4: Remove `jaeger` from `depends_on` in `app`, `notifier`, and `fraud-detector`**

In `app` depends_on, remove:
```yaml
      jaeger:
        condition: service_started
```
In `notifier` depends_on, remove the same block.
In `fraud-detector` depends_on, remove the same block.

- [ ] **Step 5: Add the `alloy` service (before `webapp:`)**

```yaml
  # ============================================
  # Grafana Alloy (Metrics scrape + Log collection)
  # ============================================
  alloy:
    image: grafana/alloy:latest
    container_name: paybridge-alloy
    volumes:
      - ./alloy-config.river:/etc/alloy/config.river:ro
      - /var/run/docker.sock:/var/run/docker.sock
    command: run --stability.level=experimental /etc/alloy/config.river
    environment:
      - GRAFANA_CLOUD_METRICS_URL=${GRAFANA_CLOUD_METRICS_URL}
      - GRAFANA_CLOUD_METRICS_USER=${GRAFANA_CLOUD_METRICS_USER}
      - GRAFANA_CLOUD_LOGS_URL=${GRAFANA_CLOUD_LOGS_URL}
      - GRAFANA_CLOUD_LOGS_USER=${GRAFANA_CLOUD_LOGS_USER}
      - GRAFANA_CLOUD_API_KEY=${GRAFANA_CLOUD_API_KEY}
    depends_on:
      app:
        condition: service_healthy
    networks:
      - paybridge-network
    restart: unless-stopped
```

- [ ] **Step 6: Commit**

```bash
git add docker-compose.yml
git commit -m "infra: replace Jaeger with Grafana Alloy in docker-compose"
```

---

### Task 2: Create Alloy configuration file

> **Concept — Metrics (Pull model):** Prometheus works on a *pull* model — a scraper periodically fetches `/metrics` from your app. Alloy plays the role of that scraper locally. The metrics endpoint at `GET /metrics` already exists in `router.go:175` and exposes everything defined in `middleware/metrics.go`.
>
> **Concept — Logs:** Alloy reads Docker container stdout directly from the Docker socket and forwards structured JSON log lines to Loki. Each log line gets labels (like `container="paybridge-api"`) automatically.
>
> **River syntax:** Each block is a component (`prometheus.scrape`, `loki.write`, etc.). Components connect via `forward_to` references. `env("VAR")` reads an environment variable.

**Files:**
- Create: `alloy-config.river`
- Modify: `.env.example`

- [ ] **Step 1: Create `alloy-config.river` in the project root**

```river
// Scrape PayBridge Prometheus metrics every 15 seconds.
// "app:8080" resolves via docker-compose network — same as how app connects to postgres.
prometheus.scrape "paybridge" {
  targets = [{
    __address__ = "app:8080",
  }]
  metrics_path    = "/metrics"
  scrape_interval = "15s"
  forward_to      = [prometheus.remote_write.grafana_cloud.receiver]
}

// Push scraped metrics to Grafana Cloud Mimir via remote_write protocol.
prometheus.remote_write "grafana_cloud" {
  endpoint {
    url = env("GRAFANA_CLOUD_METRICS_URL")

    basic_auth {
      username = env("GRAFANA_CLOUD_METRICS_USER")
      password = env("GRAFANA_CLOUD_API_KEY")
    }
  }
}

// Discover all running Docker containers so we can read their logs.
discovery.docker "containers" {
  host = "unix:///var/run/docker.sock"
}

// Tail stdout/stderr from every discovered container and ship to Loki.
loki.source.docker "default" {
  host       = "unix:///var/run/docker.sock"
  targets    = discovery.docker.containers.targets
  forward_to = [loki.write.grafana_cloud.receiver]
}

// Push log lines to Grafana Cloud Loki.
loki.write "grafana_cloud" {
  endpoint {
    url = env("GRAFANA_CLOUD_LOGS_URL")

    basic_auth {
      username = env("GRAFANA_CLOUD_LOGS_USER")
      password = env("GRAFANA_CLOUD_API_KEY")
    }
  }
}
```

- [ ] **Step 2: Add Grafana Cloud variables to `.env.example`**

Add this block (values come from Grafana Cloud → Your Stack → "Send Metrics" / "Send Logs"):
```env
# Grafana Cloud OTLP (traces → Tempo)
PAYBRIDGE_TELEMETRY_OTLP_ENDPOINT=https://otlp-gateway-prod-eu-west-0.grafana.net/otlp
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Basic <base64(instanceId:apiKey)>

# Grafana Cloud Alloy (metrics → Mimir, logs → Loki)
GRAFANA_CLOUD_METRICS_URL=https://prometheus-prod-XX-prod-eu-west-0.grafana.net/api/prom/push
GRAFANA_CLOUD_METRICS_USER=<prometheus_instance_id>
GRAFANA_CLOUD_LOGS_URL=https://logs-prod-XX-prod-eu-west-0.grafana.net/loki/api/v1/push
GRAFANA_CLOUD_LOGS_USER=<loki_instance_id>
GRAFANA_CLOUD_API_KEY=<grafana_cloud_api_key_with_metrics_and_logs_write_permissions>
```

- [ ] **Step 3: Start the stack and verify Alloy is running**

```bash
docker-compose up -d
docker-compose logs -f alloy
```

Expected in alloy logs (within 10 seconds):
```
level=info msg="Starting Grafana Alloy..."
level=info component=prometheus.scrape/paybridge msg="starting scrape loop"
level=info component=loki.source.docker/default msg="watching containers"
```

- [ ] **Step 4: Verify metrics appear in Grafana Cloud Mimir**

In Grafana Cloud → Explore → Select your Mimir/Prometheus datasource → run:
```
paybridge_http_requests_total
```
Expected: time series appear with labels `{method="GET", path="/health", status="200"}` within 30 seconds of the app receiving a request.

Send a test request to populate the metric:
```bash
curl http://localhost:8080/health
```

- [ ] **Step 5: Verify logs appear in Grafana Cloud Loki**

In Grafana Cloud → Explore → Select Loki datasource → run:
```logql
{container="paybridge-api"}
```
Expected: JSON log lines like `{"time":"...","level":"INFO","msg":"...","path":"/health"}` appear.

- [ ] **Step 6: Commit**

```bash
git add alloy-config.river .env.example
git commit -m "infra: add Grafana Alloy config for metrics scrape and log collection"
```

---

### Task 3: Add Go dependencies for OTel Metrics push

> **Concept — Push vs Pull:** Alloy uses *pull* (scrapes `/metrics`). On Fly.io there is no Alloy, so the app must *push* metrics to Grafana Cloud via OTLP. Two new packages:
> - `otlpmetrichttp` — OTLP HTTP exporter for metrics (same protocol as traces)
> - `bridges/prometheus` — reads existing Prometheus registry (all counters/histograms from `metrics.go`) and converts them to OTel format so the exporter can send them. **No changes to `metrics.go` needed.**

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the three packages**

```bash
go get go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp@v1.40.0
go get go.opentelemetry.io/otel/sdk/metric@v1.40.0
go get go.opentelemetry.io/contrib/bridges/prometheus@v0.65.0
```

- [ ] **Step 2: Verify go.mod contains the new direct dependencies**

```bash
grep -E "otlpmetrichttp|sdk/metric|bridges/prometheus" go.mod
```

Expected output (exact versions may differ if go get resolves newer patches):
```
go.opentelemetry.io/contrib/bridges/prometheus v0.65.0
go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.40.0
go.opentelemetry.io/otel/sdk/metric v1.40.0
```

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add OTel metric exporter and Prometheus bridge packages"
```

---

### Task 4: Create OTel MeterProvider

> **Concept — MeterProvider:** `MeterProvider` is to metrics what `TracerProvider` is to traces. It manages exporters and collection intervals. `NewPeriodicReader` flushes metrics every N seconds via the exporter. `WithProducer(bridge)` tells the reader to also collect from the Prometheus bridge — which reads `prometheus.DefaultGatherer` (where `metrics.go` registers all its counters and histograms). Result: every 30 seconds all paybridge_* metrics are pushed via OTLP to Grafana Cloud Mimir.

**Files:**
- Create: `internal/infrastructure/telemetry/metrics_provider.go`

- [ ] **Step 1: Create the file**

```go
package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	prometheusbridge "go.opentelemetry.io/contrib/bridges/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitMeterProvider initializes OpenTelemetry MeterProvider with OTLP HTTP exporter.
// Bridges the existing Prometheus default registry to OTel so all paybridge_* metrics
// are pushed to Grafana Cloud Mimir on Fly.io (no Alloy sidecar available there).
func InitMeterProvider(ctx context.Context, serviceName, otlpEndpoint string) (*sdkmetric.MeterProvider, error) {
	endpoint := normalizeEndpoint(otlpEndpoint)

	exporterOpts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpointURL(endpoint),
	}
	if headersEnv := os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"); headersEnv != "" {
		exporterOpts = append(exporterOpts, otlpmetrichttp.WithHeaders(parseOTLPHeaders(headersEnv)))
	}

	exporter, err := otlpmetrichttp.New(ctx, exporterOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceNameKey.String(serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Bridge reads from prometheus.DefaultGatherer (where metrics.go registers counters/histograms)
	// and produces OTel-format metric data for the periodic reader to export.
	bridge := prometheusbridge.NewMetricProducer()

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				exporter,
				sdkmetric.WithInterval(30*time.Second),
				sdkmetric.WithProducer(bridge),
			),
		),
	)

	otel.SetMeterProvider(mp)
	return mp, nil
}
```

- [ ] **Step 2: Verify it compiles (uses `normalizeEndpoint` and `parseOTLPHeaders` already defined in `tracer.go` — same package)**

```bash
go build ./internal/infrastructure/telemetry/...
```

Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add internal/infrastructure/telemetry/metrics_provider.go
git commit -m "feat: add OTel MeterProvider with Prometheus bridge for Fly.io metrics push"
```

---

### Task 5: Wire MeterProvider into container lifecycle

> **Concept — Composition Root:** `container.go` is the single place where all infrastructure is initialized, wired together, and shut down. MeterProvider follows exactly the same pattern as TracerProvider already does: initialize → store reference → shutdown gracefully.

**Files:**
- Modify: `internal/container/container.go`

- [ ] **Step 1: Add `sdkmetric` import**

In the import block (around line 37), alongside the existing `sdktrace` import, add:
```go
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
```

- [ ] **Step 2: Add `meterProvider` field to the `Container` struct**

In the `Container` struct (around line 51), after `tracerProvider`:
```go
	// Infrastructure
	pool           *pgxpool.Pool
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	redisClient    *redis.Client
```

- [ ] **Step 3: Add `initMetrics` call inside `Initialize()`**

In `Initialize()`, after the `initTracing` call (around line 119), add:
```go
	// 0b. Telemetry (metrics push for Fly.io — locally Alloy scrapes /metrics instead)
	if err := c.initMetrics(ctx); err != nil {
		c.logger.Warn("Failed to initialize metrics provider, continuing without it",
			slog.String("error", err.Error()))
	}
```

- [ ] **Step 4: Add the `initMetrics` method**

Place this after the existing `initTracing` method (around line 175):
```go
// initMetrics initializes OpenTelemetry MeterProvider for OTLP push to Grafana Cloud.
// On Fly.io (no Alloy sidecar), this pushes paybridge_* Prometheus metrics every 30s.
// Locally, Alloy scrapes /metrics — this provider is still initialized but harmless.
func (c *Container) initMetrics(ctx context.Context) error {
	if !c.config.Telemetry.Enabled {
		return nil
	}

	mp, err := telemetry.InitMeterProvider(ctx, c.config.App.Name, c.config.Telemetry.OTLPEndpoint)
	if err != nil {
		return err
	}

	c.meterProvider = mp
	c.logger.Info("Metrics provider initialized",
		slog.String("endpoint", c.config.Telemetry.OTLPEndpoint),
	)
	return nil
}
```

- [ ] **Step 5: Add MeterProvider shutdown in `Shutdown()`**

In `Shutdown()`, after the TracerProvider shutdown block (around line 568), add:
```go
	// 2b. Meter Provider
	if c.meterProvider != nil {
		if err := c.meterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter provider shutdown: %w", err))
		}
	}
```

- [ ] **Step 6: Verify it compiles**

```bash
go build ./internal/container/...
```

Expected: no output (success).

- [ ] **Step 7: Commit**

```bash
git add internal/container/container.go
git commit -m "feat: wire OTel MeterProvider into container lifecycle"
```

---

### Task 6: Add manual DB span to `WalletRepository.FindByID` (Learning Module 5)

> **Concept — Distributed Tracing across layers:** When `otelgin` middleware creates the root HTTP span for `GET /wallets/me`, it stores the span in `context.Context`. Every function receiving that `ctx` can create a *child span* — a sub-unit of work nested inside the parent. This is how you see in Grafana Cloud Tempo: `HTTP GET /wallets/me` → `WalletRepository.FindByID` → and the exact SQL execution time.
>
> **Pattern:** Always `defer span.End()` immediately after `Start`. If the operation errors, call `span.RecordError(err)` before returning — Tempo will mark the span red.

**Files:**
- Modify: `internal/infrastructure/persistence/postgres/wallet_repository.go`

- [ ] **Step 1: Add OTel imports to the file**

In the existing import block, add:
```go
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
```

- [ ] **Step 2: Wrap `FindByID` with a span**

The current `FindByID` is at line 158. Replace it with the span-instrumented version:
```go
func (r *WalletRepository) FindByID(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
	ctx, span := otel.Tracer("paybridge/wallet-repository").Start(ctx, "WalletRepository.FindByID",
		trace.WithAttributes(
			attribute.String("wallet.id", id.String()),
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "SELECT"),
			attribute.String("db.sql.table", "wallets"),
		),
	)
	defer span.End()

	q := r.getQuerier(ctx)

	query := `
		SELECT id, user_id, currency, wallet_type, status,
			   available_balance, pending_balance, balance_version,
			   daily_limit, monthly_limit, created_at, updated_at
		FROM wallets
		WHERE id = $1
	`

	wallet, err := r.scanWallet(q.QueryRow(ctx, query, id))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return wallet, nil
}
```

- [ ] **Step 3: Verify it compiles**

```bash
go build ./internal/infrastructure/persistence/postgres/...
```

Expected: no output (success).

- [ ] **Step 4: Run the stack and trigger a wallet request**

```bash
docker-compose up -d
# Get a JWT token via Telegram auth, then:
curl http://localhost:8080/api/v1/wallets/<wallet-uuid> \
  -H "Authorization: Bearer <your-jwt-token>"
```

- [ ] **Step 5: Verify the span appears in Grafana Cloud Tempo**

In Grafana Cloud → Explore → Tempo → Search:
- Service Name: `paybridge-api`
- Span Name: `WalletRepository.FindByID`

Expected: a trace waterfall showing the HTTP span with `WalletRepository.FindByID` as a child span. Click the DB span to see attributes: `wallet.id`, `db.system=postgresql`, `db.operation=SELECT`.

- [ ] **Step 6: Commit**

```bash
git add internal/infrastructure/persistence/postgres/wallet_repository.go
git commit -m "feat: add OTel span to WalletRepository.FindByID for DB query tracing"
```

---

### Task 7: Configure Fly.io log drain → Grafana Cloud Loki

> **Concept — Log drain:** Fly.io machines stream all stdout/stderr via the Fly.io log infrastructure. A "log drain" forwards those streams to an external HTTPS endpoint in real time. Grafana Cloud Loki accepts logs via its push API at `/loki/api/v1/push`. Once the drain is set up, Fly.io automatically adds labels like `app`, `region`, and `instance` to every log line.

**Files:**
- Modify: `fly.api.toml` (comment only)

- [ ] **Step 1: Get Loki credentials from Grafana Cloud**

In Grafana Cloud → Your Stack → Loki → "Send Logs":
- Note the **URL**: `https://logs-prod-XX.grafana.net/loki/api/v1/push`
- Note the **User** (Loki instance ID, numeric)
- Use the same **API key** as for metrics (needs Logs Write scope)

- [ ] **Step 2: Create the log drain via flyctl**

```bash
flyctl log-destination create \
  --app paybridge-api \
  --type https \
  --url "https://logs-prod-XX.grafana.net/loki/api/v1/push" \
  --header "Authorization=Basic $(echo -n '<loki_user_id>:<api_key>' | base64 -w0)"
```

Replace `XX`, `<loki_user_id>`, and `<api_key>` with your actual values from Step 1.

- [ ] **Step 3: Verify the drain was created**

```bash
flyctl log-destination list --app paybridge-api
```

Expected output includes an entry with `type=https` pointing to your Loki URL.

- [ ] **Step 4: Deploy and verify logs appear in Loki**

```bash
flyctl deploy --config fly.api.toml --remote-only
```

In Grafana Cloud → Explore → Loki:
```logql
{app="paybridge-api"}
```
Expected: JSON log lines from the Fly.io deployment appear within 30 seconds of deploy completing.

- [ ] **Step 5: Document the drain setup in `fly.api.toml`**

Add this comment block after the `[env]` section:
```toml
# Log drain to Grafana Cloud Loki is managed via flyctl (not stored in fly.toml).
# To recreate if lost:
#   flyctl log-destination create --app paybridge-api --type https \
#     --url "<GRAFANA_CLOUD_LOKI_URL>" \
#     --header "Authorization=Basic <base64(loki_user_id:api_key)>"
# Get credentials: Grafana Cloud → Your Stack → Loki → "Send Logs"
```

- [ ] **Step 6: Commit**

```bash
git add fly.api.toml
git commit -m "docs: document Fly.io log drain setup for Grafana Cloud Loki"
```

---

## Learning Checkpoints

After completing all tasks, you should be able to answer these from looking at the running system:

**Module 1 — Logs:** In `internal/adapters/http/middleware/logging.go`, find the line where `trace_id` is added to the log entry. Why is this field valuable? (Hint: search for it in Loki, then use the value in Tempo.)

**Module 2 — Metrics:** In Grafana Cloud Mimir, run `paybridge_http_request_duration_seconds_bucket`. What does a histogram bucket represent? Why does `paybridge_http_requests_in_flight` go up and down while `paybridge_http_requests_total` only increases?

**Module 3 — Traces:** In Tempo, open a trace for `POST /api/v1/wallets`. How many spans are in it? Which middleware creates the root span? (Hint: `router.go:143` — `otelgin.Middleware("paybridge-api")`.)

**Module 4 — Correlation:** In Loki, find a log line → copy the `trace_id` value → paste it into Tempo's TraceID search. You should land on the exact trace for that request.

**Module 5 — DB spans:** After Task 6, open a trace for a wallet request in Tempo. You should now see `WalletRepository.FindByID` as a child span. Note the `wallet.id` attribute and the duration — this is the raw DB query time.
