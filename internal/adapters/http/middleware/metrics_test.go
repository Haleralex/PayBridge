package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestMetrics_BasicRequest(t *testing.T) {
	router := gin.New()
	router.Use(Metrics())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestMetrics_SkipMetricsEndpoint(t *testing.T) {
	router := gin.New()
	router.Use(Metrics())
	router.GET("/metrics", func(c *gin.Context) {
		c.String(http.StatusOK, "metrics")
	})

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMetrics_DifferentStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"201 Created", http.StatusCreated},
		{"400 Bad Request", http.StatusBadRequest},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(Metrics())
			router.GET("/test", func(c *gin.Context) {
				c.Status(tt.statusCode)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.statusCode, w.Code)
		})
	}
}

func TestMetrics_DifferentMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			router := gin.New()
			router.Use(Metrics())
			router.Handle(method, "/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(method, "/test", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestMetrics_UnknownPath(t *testing.T) {
	router := gin.New()
	router.Use(Metrics())

	// No routes defined, path will be "unknown"
	req := httptest.NewRequest("GET", "/unknown-path", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRecordTransaction(t *testing.T) {
	// Should not panic
	RecordTransaction("DEPOSIT", "COMPLETED", "USD", 10000)
	RecordTransaction("WITHDRAW", "FAILED", "EUR", 5000)
	RecordTransaction("TRANSFER", "PENDING", "BTC", 100000000)
}

func TestRecordDBQuery(t *testing.T) {
	// Should not panic
	RecordDBQuery("SELECT", "users", 10*time.Millisecond)
	RecordDBQuery("INSERT", "transactions", 50*time.Millisecond)
	RecordDBQuery("UPDATE", "wallets", 5*time.Millisecond)
}

func TestRecordDBError(t *testing.T) {
	// Should not panic
	RecordDBError("SELECT", "connection_error")
	RecordDBError("INSERT", "constraint_violation")
	RecordDBError("UPDATE", "timeout")
}

func TestUpdateDBConnections(t *testing.T) {
	// Should not panic
	UpdateDBConnections(5, 10, 25)
	UpdateDBConnections(0, 0, 0)
	UpdateDBConnections(25, 0, 25)
}

func TestMetrics_ResponseSize(t *testing.T) {
	router := gin.New()
	router.Use(Metrics())
	router.GET("/large", func(c *gin.Context) {
		// Return a larger response
		c.String(http.StatusOK, "This is a larger response body for testing")
	})

	req := httptest.NewRequest("GET", "/large", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Greater(t, w.Body.Len(), 0)
}

func TestMetrics_SlowRequest(t *testing.T) {
	router := gin.New()
	router.Use(Metrics())
	router.GET("/slow", func(c *gin.Context) {
		time.Sleep(10 * time.Millisecond)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/slow", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	router.ServeHTTP(w, req)
	duration := time.Since(start)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.GreaterOrEqual(t, duration.Milliseconds(), int64(10))
}

func TestMetricsCollectors_Registered(t *testing.T) {
	// Verify that metrics are registered without panic
	// The promauto package auto-registers metrics

	// Try to describe each metric (this verifies they exist)
	ch := make(chan *prometheus.Desc, 100)

	httpRequestsTotal.Describe(ch)
	assert.NotEmpty(t, ch)
	<-ch

	httpRequestDuration.Describe(ch)
	assert.NotEmpty(t, ch)
	<-ch

	httpRequestsInFlight.Describe(ch)
	assert.NotEmpty(t, ch)
	<-ch

	httpResponseSize.Describe(ch)
	assert.NotEmpty(t, ch)
	<-ch
}

func TestBusinessMetrics_Registered(t *testing.T) {
	ch := make(chan *prometheus.Desc, 100)

	TransactionsTotal.Describe(ch)
	assert.NotEmpty(t, ch)
	<-ch

	TransactionAmount.Describe(ch)
	assert.NotEmpty(t, ch)
	<-ch

	WalletsTotal.Describe(ch)
	assert.NotEmpty(t, ch)
	<-ch

	UsersTotal.Describe(ch)
	assert.NotEmpty(t, ch)
	<-ch
}

func TestDBMetrics_Registered(t *testing.T) {
	ch := make(chan *prometheus.Desc, 100)

	DBQueryDuration.Describe(ch)
	assert.NotEmpty(t, ch)
	<-ch

	DBConnectionsTotal.Describe(ch)
	assert.NotEmpty(t, ch)
	<-ch

	DBErrorsTotal.Describe(ch)
	assert.NotEmpty(t, ch)
	<-ch
}

func TestMetrics_ConcurrentRequests(t *testing.T) {
	router := gin.New()
	router.Use(Metrics())
	router.GET("/concurrent", func(c *gin.Context) {
		time.Sleep(5 * time.Millisecond)
		c.Status(http.StatusOK)
	})

	// Make concurrent requests
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/concurrent", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			done <- true
		}()
	}

	// Wait for all requests
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestMetrics_PathWithParams(t *testing.T) {
	router := gin.New()
	router.Use(Metrics())
	router.GET("/users/:id", func(c *gin.Context) {
		c.String(http.StatusOK, c.Param("id"))
	})

	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "123", w.Body.String())
}

func TestRecordTransaction_AllTypes(t *testing.T) {
	types := []string{"DEPOSIT", "WITHDRAW", "PAYOUT", "TRANSFER", "FEE", "REFUND", "ADJUSTMENT"}
	statuses := []string{"PENDING", "PROCESSING", "COMPLETED", "FAILED", "CANCELLED"}
	currencies := []string{"USD", "EUR", "GBP", "BTC", "ETH"}

	for _, txType := range types {
		for _, status := range statuses {
			for _, currency := range currencies {
				// Should not panic
				RecordTransaction(txType, status, currency, 1000)
			}
		}
	}
}
