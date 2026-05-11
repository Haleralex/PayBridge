# Observability Design: LGTM Stack for PayBridge

**Date:** 2026-05-11  
**Goal:** Поднять знания по observability (OTel, Prometheus, Grafana) через изучение реального кода проекта. Попутно довести стек до рабочего состояния локально и в проде на Fly.io.

---

## 1. Контекст

Проект уже содержит заготовки:
- OTel SDK с OTLP HTTP экспортёром (`internal/infrastructure/telemetry/tracer.go`)
- Prometheus metrics middleware с HTTP + бизнес-метриками (`internal/adapters/http/middleware/metrics.go`)
- Structured logging через `slog` с correlation ID, trace ID, span ID
- Jaeger в docker-compose (для локальной разработки)
- Grafana Cloud уже подключён (трейсы летят в Tempo)

**Что не хватает:**
- Grafana Alloy в docker-compose (нет scrape метрик и сбора логов локально)
- OTel Metrics Provider для Fly.io (нет push метрик в прод)
- Spans на уровне DB (pgx) и NATS
- `/metrics` endpoint нужно верифицировать в роутере

---

## 2. Целевая Архитектура

### Локально (docker-compose)

```
PayBridge App
    ├─ Traces  → OTel SDK (OTLP HTTP) ──────────────────► Grafana Cloud Tempo
    ├─ Metrics → /metrics endpoint ──► Grafana Alloy ──► Grafana Cloud Mimir
    └─ Logs    → stdout JSON ─────────► Grafana Alloy ──► Grafana Cloud Loki
```

> **Jaeger:** сервис `jaeger` в docker-compose удаляется — трейсы идут напрямую в Grafana Cloud Tempo. Локального UI для трейсов нет, используется Grafana Cloud.

### Прод (Fly.io)

```
PayBridge App
    ├─ Traces  → OTel SDK (OTLP HTTP push) ──► Grafana Cloud Tempo
    ├─ Metrics → OTel Metrics SDK (OTLP push) ► Grafana Cloud Mimir
    └─ Logs    → Fly.io log drain (syslog) ───► Grafana Cloud Loki
```

На Fly.io нет сайдкаров, поэтому метрики шлются через OTel push вместо Prometheus scrape. Логи дренируются через встроенный механизм Fly.io (syslog drain → Loki).

---

## 3. Компоненты для добавления

### 3.1 docker-compose.yml — Grafana Alloy

Новый сервис `alloy` с конфигом `alloy-config.river`:
- Scrapes `http://app:8080/metrics` каждые 15 секунд
- Читает Docker-логи контейнеров через `loki.source.docker`
- Шлёт метрики в Grafana Cloud Mimir (remote_write)
- Шлёт логи в Grafana Cloud Loki

### 3.2 alloy-config.river

Конфиг Alloy в формате River (язык конфигурации Grafana):
- `prometheus.scrape` — pull метрик с `/metrics`
- `prometheus.remote_write` — push в Mimir с Basic Auth
- `loki.source.docker` — сбор логов из Docker
- `loki.write` — push в Loki с Basic Auth

Credentials берутся из переменных окружения (`GRAFANA_CLOUD_*`).

### 3.3 internal/infrastructure/telemetry/metrics_provider.go

OTel Metrics Provider для Fly.io:
- Инициализирует `MeterProvider` с OTLP HTTP экспортёром
- Interval: 30 секунд (PeriodicReader)
- Endpoint: `PAYBRIDGE_TELEMETRY_OTLP_ENDPOINT` (тот же что и для трейсов)
- Регистрируется в container.go рядом с TracerProvider
- Graceful shutdown

### 3.4 Fly.io log drain

Fly.io позволяет настроить log drain через CLI:
```
fly log-destination create --app paybridge-api \
  --name loki \
  --type https \
  --url "https://logs-prod-xx.grafana.net/loki/api/v1/push" \
  --header "Authorization=Basic <base64(user:apikey)>"
```
Все stdout/stderr логи приложения автоматически поступают в Loki с метками `app`, `region`, `instance`.

### 3.5 Верификация /metrics endpoint

Проверить что в `internal/adapters/http/router.go` есть:
```go
r.GET("/metrics", gin.WrapH(promhttp.Handler()))
```
Если нет — добавить.

### 3.6 Spans для pgx и NATS (расширение после базового обучения)

- pgx: использовать `otelpgx` instrumentation или ручные spans в repository слое
- NATS: обернуть publish/subscribe в spans с propagation через message headers

---

## 4. Путь обучения — Жизнь HTTP-запроса

Каждый модуль: концепция (2-3 мин) → код → задание.

### Модуль 1: Logging — Что это и зачем
**Файлы:** `internal/pkg/logger/logger.go`, `internal/adapters/http/middleware/logging.go`  
**Концепция:** Structured logs vs plain text. Что такое correlation ID и зачем он нужен.  
**Задание:** Найти в коде где trace_id попадает в лог и объяснить зачем это поле.

### Модуль 2: Metrics — Считаем всё
**Файлы:** `internal/adapters/http/middleware/metrics.go`  
**Концепция:** Counter vs Gauge vs Histogram. Labels. Cardinality.  
**Задание:** Добавить метрику `paybridge_auth_failures_total` с label `reason` (invalid_token, expired, missing).

### Модуль 3: Traces — Путь запроса через сервис
**Файлы:** `internal/infrastructure/telemetry/tracer.go`, `internal/adapters/http/router.go`  
**Концепция:** Span, Trace, TraceID, SpanID. Parent/child spans. Context propagation.  
**Задание:** Открыть Grafana Cloud Tempo, найти trace для своего запроса, разобрать waterfall.

### Модуль 4: Correlating the Three Pillars
**Концепция:** Как trace_id связывает лог, метрику и трейс в единую картину инцидента.  
**Задание:** Найти в Grafana лог → по trace_id перейти в Tempo → посмотреть span.

### Модуль 5: Spans на уровне БД (расширение)
**Файлы:** `internal/infrastructure/persistence/` (repository слой)  
**Концепция:** Distributed tracing через слои приложения. Почему важно видеть DB-запросы в трейсе.  
**Задание:** Добавить span для одного pgx-запроса вручную, проверить в Tempo.

---

## 5. Переменные окружения

```env
# Уже есть (трейсы)
PAYBRIDGE_TELEMETRY_OTLP_ENDPOINT=https://otlp-gateway-prod-eu-west-0.grafana.net/otlp
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Basic <base64>

# Для Alloy (локально)
GRAFANA_CLOUD_METRICS_URL=https://prometheus-prod-xx.grafana.net/api/prom/push
GRAFANA_CLOUD_METRICS_USER=<user_id>
GRAFANA_CLOUD_LOGS_URL=https://logs-prod-xx.grafana.net/loki/api/v1/push
GRAFANA_CLOUD_LOGS_USER=<user_id>
GRAFANA_CLOUD_API_KEY=<api_key>
```

---

## 6. Что НЕ входит в этот дизайн

- Alerting rules в Grafana (отдельная тема)
- SLO/Error Budget dashboards
- Profiling (Grafana Pyroscope)
- Exemplars (связь metric → trace) — можно добавить позже

---

## 7. Порядок реализации

1. Верифицировать `/metrics` endpoint в роутере
2. Добавить Alloy в docker-compose + alloy-config.river
3. Проверить локально: метрики в Mimir, логи в Loki, трейсы в Tempo
4. Добавить OTel Metrics Provider для Fly.io прода
5. Настроить log drain на Fly.io → Loki
6. Учебные модули 1-4 (параллельно с п.1-3)
7. Модуль 5 — spans для pgx (после базы)
