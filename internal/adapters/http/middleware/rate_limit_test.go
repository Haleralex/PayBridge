package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestDefaultRateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()

	assert.Equal(t, 100, config.Limit)
	assert.Equal(t, time.Minute, config.Window)
	assert.NotNil(t, config.KeyFunc)
	assert.Nil(t, config.OnLimitReached)
}

func TestRateLimit_AllowsRequestsUnderLimit(t *testing.T) {
	router := gin.New()
	router.Use(RateLimit(&RateLimitConfig{
		Limit:  5,
		Window: time.Minute,
		KeyFunc: func(c *gin.Context) string {
			return "test-key"
		},
	}))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Make 5 requests - all should succeed
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
	}
}

func TestRateLimit_BlocksRequestsOverLimit(t *testing.T) {
	router := gin.New()
	router.Use(RateLimit(&RateLimitConfig{
		Limit:  3,
		Window: time.Minute,
		KeyFunc: func(c *gin.Context) string {
			return "test-key"
		},
	}))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Make 3 requests - all should succeed
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// 4th request should be blocked
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestRateLimit_Headers(t *testing.T) {
	router := gin.New()
	router.Use(RateLimit(&RateLimitConfig{
		Limit:  10,
		Window: time.Minute,
		KeyFunc: func(c *gin.Context) string {
			return "test-key"
		},
	}))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, "10", w.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "9", w.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
}

func TestRateLimit_RetryAfterHeader(t *testing.T) {
	router := gin.New()
	router.Use(RateLimit(&RateLimitConfig{
		Limit:  1,
		Window: time.Minute,
		KeyFunc: func(c *gin.Context) string {
			return "test-key"
		},
	}))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// First request succeeds
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Second request blocked
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
}

func TestRateLimit_DifferentKeys(t *testing.T) {
	counter := 0
	router := gin.New()
	router.Use(RateLimit(&RateLimitConfig{
		Limit:  2,
		Window: time.Minute,
		KeyFunc: func(c *gin.Context) string {
			counter++
			return c.Query("key")
		},
	}))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Key1: 2 requests
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test?key=key1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Key1: 3rd request blocked
	req := httptest.NewRequest("GET", "/test?key=key1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Key2: Should still work (different key)
	req = httptest.NewRequest("GET", "/test?key=key2", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimit_OnLimitReachedCallback(t *testing.T) {
	callbackCalled := false
	router := gin.New()
	router.Use(RateLimit(&RateLimitConfig{
		Limit:  1,
		Window: time.Minute,
		KeyFunc: func(c *gin.Context) string {
			return "test-key"
		},
		OnLimitReached: func(c *gin.Context) {
			callbackCalled = true
		},
	}))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// First request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.False(t, callbackCalled)

	// Second request triggers callback
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.True(t, callbackCalled)
}

func TestRateLimit_NilConfig(t *testing.T) {
	router := gin.New()
	router.Use(RateLimit(nil)) // Should use default config
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimit_ConcurrentRequests(t *testing.T) {
	router := gin.New()
	router.Use(RateLimit(&RateLimitConfig{
		Limit:  50,
		Window: time.Minute,
		KeyFunc: func(c *gin.Context) string {
			return "test-key"
		},
	}))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// Send 100 concurrent requests
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code == http.StatusOK {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, 50, successCount, "Exactly 50 requests should succeed")
}

func TestSensitiveEndpointRateLimit(t *testing.T) {
	router := gin.New()
	router.Use(SensitiveEndpointRateLimit())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Make requests up to limit (10)
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// 11th request should be blocked
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestTransactionRateLimit(t *testing.T) {
	router := gin.New()
	router.Use(TransactionRateLimit())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Should work with IP-based limiting
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{100, "100"},
		{12345, "12345"},
		{-1, "-1"},
		{-100, "-100"},
		{-12345, "-12345"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := itoa(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRateLimiter_BucketReset(t *testing.T) {
	config := &RateLimitConfig{
		Limit:  2,
		Window: 50 * time.Millisecond, // Short window for testing
		KeyFunc: func(c *gin.Context) string {
			return "test"
		},
	}

	limiter := newRateLimiter(config)

	// Use 2 tokens
	allowed, remaining, _ := limiter.allow("test")
	assert.True(t, allowed)
	assert.Equal(t, 1, remaining)

	allowed, remaining, _ = limiter.allow("test")
	assert.True(t, allowed)
	assert.Equal(t, 0, remaining)

	// Should be blocked
	allowed, _, _ = limiter.allow("test")
	assert.False(t, allowed)

	// Wait for window to reset
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again
	allowed, remaining, _ = limiter.allow("test")
	assert.True(t, allowed)
	assert.Equal(t, 1, remaining)
}

func TestRateLimit_ResponseBody(t *testing.T) {
	router := gin.New()
	router.Use(RateLimit(&RateLimitConfig{
		Limit:  1,
		Window: time.Minute,
		KeyFunc: func(c *gin.Context) string {
			return "test-key"
		},
	}))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// First request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Second request - check response body
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "TOO_MANY_REQUESTS")
	assert.Contains(t, body, "Rate limit exceeded")
}

func TestRateLimit_WindowBoundary(t *testing.T) {
	config := &RateLimitConfig{
		Limit:  5,
		Window: 100 * time.Millisecond,
		KeyFunc: func(c *gin.Context) string {
			return "test"
		},
	}

	limiter := newRateLimiter(config)

	// Use all tokens
	for i := 0; i < 5; i++ {
		allowed, _, _ := limiter.allow("test")
		assert.True(t, allowed)
	}

	// Should be blocked
	allowed, _, retryAfter := limiter.allow("test")
	assert.False(t, allowed)
	assert.True(t, retryAfter > 0)
	assert.True(t, retryAfter <= config.Window)
}
