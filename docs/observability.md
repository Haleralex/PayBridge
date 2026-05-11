# Observability в PayBridge

Гайд по трём столпам observability на примере этого проекта. Формат: концепция → где в коде → что смотреть в Grafana Cloud.

---

## Три столпа

```
Logs     → ЧТО произошло (текст события с контекстом)
Metrics  → СКОЛЬКО/КАК ЧАСТО происходит (числа во времени)
Traces   → КАК ДОЛГО и ЧЕРЕЗ ЧТО прошёл запрос (путь через код)
```

Все три связаны через **`trace_id`** — по нему можно перейти от лога к трейсу и обратно.

---

## Стек: LGTM + Grafana Alloy

```
PayBridge App
    ├─ Logs    → stdout JSON ──────► Grafana Alloy ──► Grafana Cloud Loki   (L)
    ├─ Metrics → GET /metrics ─────► Grafana Alloy ──► Grafana Cloud Mimir  (M)
    └─ Traces  → OTel SDK (push) ──────────────────► Grafana Cloud Tempo    (T)
                                                           │
                                                    Grafana Cloud UI         (G)

На Fly.io (нет Alloy):
    ├─ Logs    → Fly.io log drain ──────────────────► Grafana Cloud Loki
    └─ Metrics → OTel MeterProvider (push, 30s) ───► Grafana Cloud Mimir
```

**Grafana Alloy** — агент, который работает в docker-compose. Его конфиг: [`alloy-config.river`](../alloy-config.river).

---

## Столп 1: Logs

### Концепция

**Structured log** — это JSON-объект вместо строки. Каждое поле можно фильтровать в Loki.

```
Plain text:  "GET /wallets/me 200 12ms"
Structured:  {"time":"...","level":"INFO","method":"GET","path":"/wallets/me","status":200,"duration_ms":12,"trace_id":"abc123","user_id":"uuid"}
```

**Correlation ID / Request ID** — уникальный ID запроса, который проходит через все логи одного HTTP-вызова. Позволяет найти все логи конкретного запроса.

**Trace ID** — ID из OpenTelemetry. Если он есть в логе — можно кликнуть и перейти в Tempo.

### Где в коде

| Файл | Что делает |
|------|-----------|
| [`internal/pkg/logger/logger.go`](../internal/pkg/logger/logger.go) | Инициализация `slog`, JSON/text формат |
| [`internal/adapters/http/middleware/logging.go`](../internal/adapters/http/middleware/logging.go) | Middleware: логирует каждый запрос — метод, путь, статус, duration, `trace_id`, `span_id`, `user_id` |

Ключевой момент в `logging.go` — `trace_id` и `span_id` добавляются из контекста OTel:
```go
span := trace.SpanFromContext(c.Request.Context())
traceID := span.SpanContext().TraceID().String()
```

### В Grafana Cloud (Loki)

```logql
// Все логи PayBridge API
{container="paybridge-api"}

// Только ошибки
{container="paybridge-api"} | json | level="ERROR"

// Конкретный запрос по trace_id
{container="paybridge-api"} | json | trace_id="abc123def456..."

// Медленные запросы (>500ms)
{container="paybridge-api"} | json | duration_ms > 500
```

---

## Столп 2: Metrics

### Концепция

**Метрика** — число, которое меняется со временем. Три основных типа:

| Тип | Что считает | Пример |
|-----|-------------|--------|
| **Counter** | Только растёт. Количество событий. | `http_requests_total` |
| **Gauge** | Может расти и падать. Текущее состояние. | `requests_in_flight`, `wallets_total` |
| **Histogram** | Распределение значений по бакетам. | `request_duration_seconds` |

**Labels** — теги на метрике для группировки. `http_requests_total{method="POST", path="/wallets", status="201"}`. Важно: не используй в labels значения с высокой кардинальностью (UUID, user_id) — это убивает Prometheus.

**PromQL** — язык запросов. Примеры:
```promql
rate(paybridge_http_requests_total[5m])          // RPS за последние 5 минут
histogram_quantile(0.99, rate(paybridge_http_request_duration_seconds_bucket[5m]))  // P99 latency
```

### Где в коде

| Файл | Что делает |
|------|-----------|
| [`internal/adapters/http/middleware/metrics.go`](../internal/adapters/http/middleware/metrics.go) | Регистрирует все метрики, middleware для HTTP |
| [`internal/adapters/http/router.go:175`](../internal/adapters/http/router.go) | Эндпоинт `GET /metrics` — отдаёт все метрики в Prometheus формате |
| [`internal/infrastructure/telemetry/metrics_provider.go`](../internal/infrastructure/telemetry/metrics_provider.go) | OTel MeterProvider для push на Fly.io |

Метрики в `metrics.go`:
```
paybridge_http_requests_total          — счётчик запросов (labels: method, path, status)
paybridge_http_request_duration_seconds — histogram задержек
paybridge_http_requests_in_flight      — gauge активных запросов
paybridge_http_response_size_bytes     — histogram размеров ответов

paybridge_business_transactions_total  — счётчик транзакций (labels: type, status, currency)
paybridge_business_transaction_amount  — histogram сумм транзакций
paybridge_business_wallets_total       — gauge количества кошельков
paybridge_business_users_total         — gauge количества пользователей
```

### В Grafana Cloud (Mimir/Prometheus)

```promql
// RPS по эндпоинтам
sum by (path) (rate(paybridge_http_requests_total[5m]))

// P99 latency
histogram_quantile(0.99,
  sum by (le, path) (rate(paybridge_http_request_duration_seconds_bucket[5m]))
)

// Error rate (4xx + 5xx)
sum(rate(paybridge_http_requests_total{status=~"[45].."}[5m]))
  /
sum(rate(paybridge_http_requests_total[5m]))

// Активные кошельки по валюте
paybridge_business_wallets_total{status="active"}
```

---

## Столп 3: Traces

### Концепция

**Trace** — полный путь одного запроса через систему. Состоит из **spans**.

**Span** — единица работы: HTTP handler, DB запрос, вызов внешнего сервиса. Каждый span знает:
- Имя (`WalletRepository.FindByID`)
- Время начала и конца (можно посчитать duration)
- Parent span ID (для построения дерева)
- Attributes (`wallet.id`, `db.system`, `db.operation`)
- Status (OK / Error)

**Context propagation** — `context.Context` в Go несёт активный span. Каждая функция, принимающая `ctx`, может создать child span. Так строится дерево.

```
HTTP GET /api/v1/wallets/me                    [root span, создан otelgin]
  └─ Auth middleware                           [child span]
  └─ WalletRepository.FindByID                [child span — добавлен нами]
       wallet.id = "uuid"
       db.system = "postgresql"
```

### Где в коде

| Файл | Что делает |
|------|-----------|
| [`internal/infrastructure/telemetry/tracer.go`](../internal/infrastructure/telemetry/tracer.go) | Инициализация OTel TracerProvider, OTLP HTTP экспортёр → Grafana Cloud Tempo |
| [`internal/adapters/http/router.go:143`](../internal/adapters/http/router.go) | `otelgin.Middleware("paybridge-api")` — создаёт root span для каждого запроса |
| [`internal/infrastructure/persistence/postgres/wallet_repository.go:158`](../internal/infrastructure/persistence/postgres/wallet_repository.go) | Ручной span на `FindByID` — пример DB instrumentation |
| [`internal/application/cqrs/`](../internal/application/cqrs/) | `TracingMiddleware()` в CQRS buses — spans на use cases |
| [`internal/container/container.go`](../internal/container/container.go) | `initTracing()` и `initMetrics()` — инициализация в Composition Root |

Как добавить span вручную (паттерн из `wallet_repository.go`):
```go
ctx, span := otel.Tracer("paybridge/wallet-repository").Start(ctx, "WalletRepository.FindByID",
    trace.WithAttributes(
        attribute.String("wallet.id", id.String()),
        attribute.String("db.system", "postgresql"),
    ),
)
defer span.End()

// ... работа ...

if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, err.Error())
    return nil, err
}
```

### В Grafana Cloud (Tempo)

1. **Explore → Tempo → Search**
   - Service Name: `paybridge-api`
   - Span Name: `WalletRepository.FindByID`
   - Duration: `> 100ms` (для поиска медленных запросов)

2. **TraceQL** — язык запросов Tempo:
```traceql
// Все трейсы с DB ошибками
{span.db.system="postgresql" && status=error}

// Медленные wallet запросы
{resource.service.name="paybridge-api" && name=~"Wallet.*" && duration > 200ms}

// Трейс по конкретному trace_id (из лога)
{traceID="abc123..."}
```

---

## Correlating the Three Pillars

Связка логов, метрик и трейсов — главная суперсила observability.

### Сценарий: расследуем инцидент

**1. Замечаем аномалию в метриках**
```promql
// Видим spike error rate
sum(rate(paybridge_http_requests_total{status=~"5.."}[1m]))
```

**2. Находим конкретные ошибки в логах**
```logql
{container="paybridge-api"} | json | level="ERROR" | status >= 500
```

**3. Берём `trace_id` из лога → идём в Tempo**

Кликаем по trace_id в Grafana Loki → Tempo открывает waterfall:
```
HTTP POST /api/v1/wallets/uuid/transfer  [450ms — SLOW]
  └─ Auth middleware                     [2ms]
  └─ TransferBetweenWalletsUseCase       [445ms]
       └─ WalletRepository.FindByID      [440ms — SLOW!]  ← вот проблема
```

**4. По span attributes находим проблему**
```
wallet.id = "specific-uuid"
db.system = "postgresql"
db.operation = "SELECT"
```
→ Идём в pgAdmin, смотрим EXPLAIN для этого запроса → находим missing index.

### Связка в Grafana Cloud

В Loki и Tempo можно настроить **derived fields** и **trace links**:
- В Loki datasource: поле `trace_id` → ссылка на Tempo
- В Tempo: span attributes → ссылка на Loki logs

---

## Переменные окружения

```env
# Traces → Grafana Cloud Tempo
PAYBRIDGE_TELEMETRY_ENABLED=true
PAYBRIDGE_TELEMETRY_OTLP_ENDPOINT=https://otlp-gateway-prod-eu-west-0.grafana.net/otlp
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Basic <base64(instanceId:apiKey)>

# Metrics + Logs (локально через Alloy, см. alloy-config.river)
GRAFANA_CLOUD_METRICS_URL=https://prometheus-prod-XX.grafana.net/api/prom/push
GRAFANA_CLOUD_METRICS_USER=<prometheus_instance_id>
GRAFANA_CLOUD_LOGS_URL=https://logs-prod-XX.grafana.net/loki/api/v1/push
GRAFANA_CLOUD_LOGS_USER=<loki_instance_id>
GRAFANA_CLOUD_API_KEY=<api_key>
```

---

## Быстрый старт локально

```bash
# 1. Заполни .env из .env.example (нужны Grafana Cloud credentials)
cp .env.example .env
# отредактируй .env

# 2. Запусти стек
docker-compose up -d

# 3. Проверь что Alloy поднялся
docker-compose logs -f alloy

# 4. Сделай запрос чтобы сгенерировать данные
curl http://localhost:8080/health
curl http://localhost:8080/metrics  # должен вернуть prometheus-формат

# 5. Grafana Cloud → Explore
#    Loki:   {container="paybridge-api"}
#    Mimir:  paybridge_http_requests_total
#    Tempo:  Service Name = paybridge-api
```

---

## Что добавить потом

- **pgx instrumentation** — автоматические spans для всех DB запросов (сейчас только `FindByID` вручную). Пакет: `github.com/exaring/otelpgx`
- **NATS tracing** — propagation через message headers для трейсов через очередь
- **Alerting** — Grafana Cloud alerts на error rate > 1%, P99 > 500ms
- **Exemplars** — связь histogram bucket → конкретный trace (нужна поддержка в `metrics.go`)
- **Grafana dashboards** — готовые дашборды для Go apps: [grafana.com/grafana/dashboards](https://grafana.com/grafana/dashboards/?search=golang)
