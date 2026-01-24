# Phase 6: Production Readiness - Deep Dive

## Обзор

Phase 6 превращает приложение в production-ready систему с:
- Миграциями базы данных (golang-migrate)
- Метриками Prometheus
- Структурированным логированием с correlation IDs
- Улучшенными health checks

---

## 1. Database Migrations

### 1.1 Почему golang-migrate?

**Альтернативы:**
| Инструмент | Плюсы | Минусы |
|------------|-------|--------|
| goose | Go-native, SQL+Go миграции | Менее популярен |
| golang-migrate | Популярный, CLI + библиотека | Только SQL |
| Atlas | Декларативный подход | Сложнее в освоении |
| GORM Migrate | Интегрирован с GORM | Привязка к ORM |

**Выбор golang-migrate:**
- Наиболее популярен в Go-экосистеме
- Поддерживает 20+ баз данных
- Можно использовать как CLI и как библиотеку
- Простая схема версионирования

### 1.2 Структура миграций

```
migrations/
├── 000001_create_users.up.sql      # Применение
├── 000001_create_users.down.sql    # Откат
├── 000002_create_wallets.up.sql
├── 000002_create_wallets.down.sql
├── 000003_create_transactions.up.sql
├── 000003_create_transactions.down.sql
├── 000004_create_outbox.up.sql
└── 000004_create_outbox.down.sql
```

**Формат имени:** `{VERSION}_{NAME}.{up|down}.sql`
- VERSION - 6 цифр (000001, 000002, ...)
- NAME - описание миграции (snake_case)
- up.sql - применение изменений
- down.sql - откат изменений

### 1.3 Migration CLI

```go
// cmd/migrate/main.go
func main() {
    // Флаги командной строки
    flag.StringVar(&migrationsPath, "path", "./migrations", "Path to migrations")
    flag.StringVar(&databaseURL, "database-url", "", "Database URL")
    flag.StringVar(&command, "command", "up", "Migration command")

    // Создаём экземпляр migrate
    m, err := migrate.New(sourceURL, databaseURL)

    switch command {
    case "up":
        err = m.Up()  // Применить все pending миграции
    case "down":
        err = m.Steps(-steps)  // Откатить N миграций
    case "force":
        err = m.Force(version)  // Принудительно установить версию
    case "version":
        version, dirty, err := m.Version()  // Текущая версия
    }
}
```

### 1.4 Команды Makefile

```bash
# Применить все миграции
make migrate-up

# Откатить последнюю миграцию
make migrate-down

# Откатить все миграции
make migrate-down-all

# Показать текущую версию
make migrate-version

# Создать новую миграцию
make migrate-create NAME=add_user_phone

# Принудительно установить версию (для recovery)
make migrate-force V=3
```

### 1.5 Dirty State Recovery

Если миграция упала посередине, БД переходит в "dirty" состояние:

```bash
# Проверяем версию
make migrate-version
# Output: Current version: 3 (dirty: true)

# Вручную исправляем БД, затем:
make migrate-force V=3  # Снимаем dirty flag

# Или откатываем и применяем заново:
make migrate-force V=2
make migrate-up
```

---

## 2. Prometheus Metrics

### 2.1 Архитектура метрик

```
┌─────────────────────────────────────────────────────────────┐
│                     PayBridge API                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │   Handler   │  │  Use Case   │  │ Repository  │        │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘        │
│         │                │                │                │
│         ▼                ▼                ▼                │
│  ┌─────────────────────────────────────────────────┐      │
│  │              Metrics Middleware                  │      │
│  │  - HTTP requests (count, latency, size)         │      │
│  │  - Business metrics (transactions, users)       │      │
│  │  - Database metrics (connections, queries)      │      │
│  └──────────────────────────────────────────────────┘      │
│                          │                                  │
│                          ▼                                  │
│                   /metrics endpoint                         │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │  Prometheus  │
                    └──────┬───────┘
                           │
                           ▼
                    ┌──────────────┐
                    │   Grafana    │
                    └──────────────┘
```

### 2.2 Типы метрик

#### Counter (счётчик)
Монотонно растущее значение. Используется для подсчёта событий.

```go
httpRequestsTotal = promauto.NewCounterVec(
    prometheus.CounterOpts{
        Namespace: "paybridge",
        Subsystem: "http",
        Name:      "requests_total",
        Help:      "Total number of HTTP requests",
    },
    []string{"method", "path", "status"},  // Labels
)

// Использование
httpRequestsTotal.WithLabelValues("GET", "/api/v1/users", "200").Inc()
```

#### Histogram (гистограмма)
Распределение значений по buckets. Для latency, размеров.

```go
httpRequestDuration = promauto.NewHistogramVec(
    prometheus.HistogramOpts{
        Namespace: "paybridge",
        Subsystem: "http",
        Name:      "request_duration_seconds",
        Help:      "HTTP request latency",
        Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
    },
    []string{"method", "path"},
)

// Использование
httpRequestDuration.WithLabelValues("GET", "/api/v1/users").Observe(0.042)
```

#### Gauge (датчик)
Значение, которое может увеличиваться и уменьшаться.

```go
httpRequestsInFlight = promauto.NewGauge(
    prometheus.GaugeOpts{
        Namespace: "paybridge",
        Subsystem: "http",
        Name:      "requests_in_flight",
        Help:      "Number of requests currently being processed",
    },
)

// Использование
httpRequestsInFlight.Inc()
defer httpRequestsInFlight.Dec()
```

### 2.3 HTTP Middleware

```go
func Metrics() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Skip /metrics endpoint
        if c.Request.URL.Path == "/metrics" {
            c.Next()
            return
        }

        start := time.Now()
        path := c.FullPath()
        method := c.Request.Method

        // Track in-flight requests
        httpRequestsInFlight.Inc()
        defer httpRequestsInFlight.Dec()

        c.Next()

        // Record metrics after request
        status := strconv.Itoa(c.Writer.Status())
        duration := time.Since(start).Seconds()

        httpRequestsTotal.WithLabelValues(method, path, status).Inc()
        httpRequestDuration.WithLabelValues(method, path).Observe(duration)
        httpResponseSize.WithLabelValues(method, path).Observe(float64(c.Writer.Size()))
    }
}
```

### 2.4 Business Metrics

```go
// Записываем метрику транзакции
middleware.RecordTransaction("DEPOSIT", "COMPLETED", "USD", 10000)

// Записываем метрику запроса к БД
start := time.Now()
// ... выполнение запроса ...
middleware.RecordDBQuery("SELECT", "users", time.Since(start))

// Обновляем метрики соединений БД
stats := pool.Stat()
middleware.UpdateDBConnections(stats.IdleConns(), stats.AcquiredConns(), stats.MaxConns())
```

### 2.5 Prometheus Queries (PromQL)

```promql
# Requests per second
rate(paybridge_http_requests_total[5m])

# 99th percentile latency
histogram_quantile(0.99, rate(paybridge_http_request_duration_seconds_bucket[5m]))

# Error rate
sum(rate(paybridge_http_requests_total{status=~"5.."}[5m]))
  / sum(rate(paybridge_http_requests_total[5m])) * 100

# Transaction volume
sum(rate(paybridge_business_transactions_total[1h])) by (type)

# Database connection utilization
paybridge_db_connections{state="in_use"}
  / paybridge_db_connections{state="max"} * 100
```

### 2.6 Alerting Rules

```yaml
# prometheus/alerts.yml
groups:
  - name: paybridge
    rules:
      - alert: HighErrorRate
        expr: |
          sum(rate(paybridge_http_requests_total{status=~"5.."}[5m]))
          / sum(rate(paybridge_http_requests_total[5m])) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"

      - alert: HighLatency
        expr: |
          histogram_quantile(0.99, rate(paybridge_http_request_duration_seconds_bucket[5m])) > 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High latency detected (p99 > 2s)"

      - alert: DatabaseConnectionsExhausted
        expr: |
          paybridge_db_connections{state="in_use"}
          / paybridge_db_connections{state="max"} > 0.9
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Database connection pool near exhaustion"
```

---

## 3. Structured Logging

### 3.1 Проблема

Без correlation IDs сложно отследить запрос через систему:

```
// Плохо - логи без связи
2024-01-15 10:30:00 INFO  Processing payment
2024-01-15 10:30:00 INFO  Validating user
2024-01-15 10:30:01 ERROR Database connection failed
2024-01-15 10:30:01 INFO  Processing payment
```

### 3.2 Решение - Correlation IDs

```go
// internal/pkg/logger/logger.go

// Ключи контекста
const (
    CorrelationIDKey contextKey = "correlation_id"
    RequestIDKey     contextKey = "request_id"
    UserIDKey        contextKey = "user_id"
    TraceIDKey       contextKey = "trace_id"
)

// ContextHandler автоматически добавляет IDs из контекста в логи
type ContextHandler struct {
    handler slog.Handler
}

func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
    // Извлекаем IDs из контекста
    if correlationID := GetCorrelationID(ctx); correlationID != "" {
        r.AddAttrs(slog.String("correlation_id", correlationID))
    }
    if requestID := GetRequestID(ctx); requestID != "" {
        r.AddAttrs(slog.String("request_id", requestID))
    }
    if userID := GetUserID(ctx); userID != "" {
        r.AddAttrs(slog.String("user_id", userID))
    }
    return h.handler.Handle(ctx, r)
}
```

### 3.3 Middleware Integration

```go
// middleware/logging.go
func Logging(config *LoggingConfig) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Добавляем correlation IDs в контекст
        requestID := GetRequestID(c)
        ctx := c.Request.Context()
        ctx = logger.WithRequestID(ctx, requestID)
        ctx = logger.WithCorrelationID(ctx, requestID)

        if userID, exists := c.Get("user_id"); exists {
            ctx = logger.WithUserID(ctx, userID.(string))
        }

        c.Request = c.Request.WithContext(ctx)
        c.Next()
    }
}
```

### 3.4 Использование в коде

```go
// В любом месте приложения
func (uc *CreateTransactionUseCase) Execute(ctx context.Context, cmd CreateTransactionCommand) error {
    // Логи автоматически включают correlation_id, request_id, user_id
    slog.InfoContext(ctx, "Creating transaction",
        "amount", cmd.Amount,
        "currency", cmd.Currency,
    )

    // ... бизнес-логика ...

    slog.InfoContext(ctx, "Transaction created",
        "transaction_id", tx.ID,
    )
    return nil
}
```

### 3.5 Результат

```json
{"time":"2024-01-15T10:30:00Z","level":"INFO","msg":"Creating transaction","correlation_id":"abc-123","request_id":"abc-123","user_id":"user-456","amount":10000,"currency":"USD"}
{"time":"2024-01-15T10:30:00Z","level":"INFO","msg":"Validating user","correlation_id":"abc-123","request_id":"abc-123","user_id":"user-456"}
{"time":"2024-01-15T10:30:01Z","level":"INFO","msg":"Transaction created","correlation_id":"abc-123","request_id":"abc-123","user_id":"user-456","transaction_id":"tx-789"}
```

Теперь легко отфильтровать все логи одного запроса:
```bash
cat logs.json | jq 'select(.correlation_id == "abc-123")'
```

---

## 4. Health Checks

### 4.1 Liveness vs Readiness

| Аспект | Liveness | Readiness |
|--------|----------|-----------|
| Вопрос | Жив ли процесс? | Готов ли принимать трафик? |
| При fail | Kubernetes перезапускает pod | Kubernetes убирает из LB |
| Проверки | Минимальные | Все зависимости |
| Скорость | Очень быстро | Может быть медленнее |

### 4.2 Endpoints

```go
// GET /live - Liveness probe
func (h *HealthHandler) Live(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

// GET /ready - Readiness probe
func (h *HealthHandler) Ready(c *gin.Context) {
    checks := make(map[string]string)
    allReady := true

    // Проверяем PostgreSQL
    if h.pool != nil {
        ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
        defer cancel()

        if err := h.pool.Ping(ctx); err != nil {
            checks["database"] = "unhealthy: " + err.Error()
            allReady = false
        } else {
            checks["database"] = "healthy"
        }
    }

    statusCode := http.StatusOK
    if !allReady {
        statusCode = http.StatusServiceUnavailable  // 503
    }

    c.JSON(statusCode, ReadinessResponse{
        Ready:  allReady,
        Checks: checks,
    })
}

// GET /health/detailed - Детальная информация
func (h *HealthHandler) DetailedHealth(c *gin.Context) {
    // Включает статистику пула соединений
    stats := h.pool.Stat()
    checks["db_total_conns"] = strconv.Itoa(int(stats.TotalConns()))
    checks["db_idle_conns"] = strconv.Itoa(int(stats.IdleConns()))

    // Обновляем Prometheus метрики
    middleware.UpdateDBConnections(stats.IdleConns(), stats.AcquiredConns(), stats.MaxConns())
}
```

### 4.3 Kubernetes Configuration

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: paybridge-api
spec:
  template:
    spec:
      containers:
        - name: api
          image: paybridge-api:latest
          ports:
            - containerPort: 8080

          # Liveness: перезапустить если процесс завис
          livenessProbe:
            httpGet:
              path: /live
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 3
            failureThreshold: 3

          # Readiness: не отправлять трафик пока не готов
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
            timeoutSeconds: 5
            failureThreshold: 3

          # Startup: дать время на инициализацию
          startupProbe:
            httpGet:
              path: /live
              port: 8080
            failureThreshold: 30
            periodSeconds: 2
```

---

## 5. Итоговая архитектура

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           PayBridge API                                  │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                         HTTP Layer                                  │ │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐     │ │
│  │  │Recovery │ │RequestID│ │ Metrics │ │ Logging │ │  Auth   │     │ │
│  │  └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘     │ │
│  │       └───────────┴───────────┴───────────┴───────────┘           │ │
│  └───────────────────────────────┬────────────────────────────────────┘ │
│                                  │                                       │
│  ┌───────────────────────────────▼────────────────────────────────────┐ │
│  │                        Endpoints                                    │ │
│  │  /metrics    /health    /ready    /live    /api/v1/*               │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                  │                                       │
│  ┌───────────────────────────────▼────────────────────────────────────┐ │
│  │                     Application Layer                               │ │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐    │ │
│  │  │   User Cases    │  │  Wallet Cases   │  │Transaction Cases│    │ │
│  │  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘    │ │
│  │           └───────────────────┬┴───────────────────┘              │ │
│  └───────────────────────────────┬────────────────────────────────────┘ │
│                                  │                                       │
│  ┌───────────────────────────────▼────────────────────────────────────┐ │
│  │                   Infrastructure Layer                              │ │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐    │ │
│  │  │   PostgreSQL    │  │     Logger      │  │   Prometheus    │    │ │
│  │  │  (+ Migrations) │  │ (Correlation ID)│  │   (Metrics)     │    │ │
│  │  └─────────────────┘  └─────────────────┘  └─────────────────┘    │ │
│  └────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 6. Production Checklist

### Мониторинг
- [ ] Развернуть Prometheus
- [ ] Создать Grafana dashboards
- [ ] Настроить alerting rules
- [ ] Настроить log aggregation (ELK/Loki/CloudWatch)

### База данных
- [ ] Запустить миграции в production
- [ ] Настроить backup schedule
- [ ] Настроить connection pool limits
- [ ] Включить SSL для соединений

### Health Checks
- [ ] Настроить Kubernetes probes
- [ ] Настроить uptime monitoring
- [ ] Настроить health checks в load balancer

### Безопасность
- [ ] Ограничить доступ к /metrics
- [ ] Включить TLS
- [ ] Настроить rate limits для production
