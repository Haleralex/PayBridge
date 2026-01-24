package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// httpRequestsTotal counts total HTTP requests
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "paybridge",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// httpRequestDuration measures request latency
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "paybridge",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	// httpRequestsInFlight tracks concurrent requests
	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "paybridge",
			Subsystem: "http",
			Name:      "requests_in_flight",
			Help:      "Number of HTTP requests currently being processed",
		},
	)

	// httpResponseSize measures response body size
	httpResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "paybridge",
			Subsystem: "http",
			Name:      "response_size_bytes",
			Help:      "HTTP response size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8), // 100B to 10GB
		},
		[]string{"method", "path"},
	)
)

// Business metrics
var (
	// transactionsTotal counts transactions by type and status
	TransactionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "paybridge",
			Subsystem: "business",
			Name:      "transactions_total",
			Help:      "Total number of transactions",
		},
		[]string{"type", "status", "currency"},
	)

	// transactionAmount tracks transaction amounts
	TransactionAmount = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "paybridge",
			Subsystem: "business",
			Name:      "transaction_amount",
			Help:      "Transaction amounts in minor units",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 10), // 100 cents to $10M
		},
		[]string{"type", "currency"},
	)

	// walletsTotal counts total wallets by status
	WalletsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "paybridge",
			Subsystem: "business",
			Name:      "wallets_total",
			Help:      "Total number of wallets",
		},
		[]string{"status", "currency"},
	)

	// usersTotal counts total users by KYC status
	UsersTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "paybridge",
			Subsystem: "business",
			Name:      "users_total",
			Help:      "Total number of users",
		},
		[]string{"kyc_status"},
	)
)

// Database metrics
var (
	// dbQueryDuration measures database query latency
	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "paybridge",
			Subsystem: "db",
			Name:      "query_duration_seconds",
			Help:      "Database query duration in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"operation", "table"},
	)

	// dbConnectionsTotal tracks database connections
	DBConnectionsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "paybridge",
			Subsystem: "db",
			Name:      "connections",
			Help:      "Number of database connections",
		},
		[]string{"state"}, // idle, in_use, max
	)

	// dbErrorsTotal counts database errors
	DBErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "paybridge",
			Subsystem: "db",
			Name:      "errors_total",
			Help:      "Total number of database errors",
		},
		[]string{"operation", "error_type"},
	)
)

// Metrics returns Prometheus metrics middleware
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip metrics endpoint
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}
		method := c.Request.Method

		httpRequestsInFlight.Inc()
		defer httpRequestsInFlight.Dec()

		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		duration := time.Since(start).Seconds()

		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path).Observe(duration)
		httpResponseSize.WithLabelValues(method, path).Observe(float64(c.Writer.Size()))
	}
}

// RecordTransaction records a transaction metric
func RecordTransaction(txType, status, currency string, amount int64) {
	TransactionsTotal.WithLabelValues(txType, status, currency).Inc()
	TransactionAmount.WithLabelValues(txType, currency).Observe(float64(amount))
}

// RecordDBQuery records a database query metric
func RecordDBQuery(operation, table string, duration time.Duration) {
	DBQueryDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
}

// RecordDBError records a database error metric
func RecordDBError(operation, errorType string) {
	DBErrorsTotal.WithLabelValues(operation, errorType).Inc()
}

// UpdateDBConnections updates database connection metrics
func UpdateDBConnections(idle, inUse, max int32) {
	DBConnectionsTotal.WithLabelValues("idle").Set(float64(idle))
	DBConnectionsTotal.WithLabelValues("in_use").Set(float64(inUse))
	DBConnectionsTotal.WithLabelValues("max").Set(float64(max))
}
