package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GeneratesNewRequestID", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID())
		router.GET("/test", func(c *gin.Context) {
			c.String(200, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		requestID := w.Header().Get(RequestIDHeader)
		assert.NotEmpty(t, requestID)
		assert.Len(t, requestID, 36) // UUID length
	})

	t.Run("UsesProvidedRequestID", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID())
		router.GET("/test", func(c *gin.Context) {
			c.String(200, "ok")
		})

		customID := "custom-request-123"
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set(RequestIDHeader, customID)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		requestID := w.Header().Get(RequestIDHeader)
		assert.Equal(t, customID, requestID)
	})

	t.Run("StoresRequestIDInContext", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID())

		var contextID string
		router.GET("/test", func(c *gin.Context) {
			contextID = GetRequestID(c)
			c.String(200, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		responseID := w.Header().Get(RequestIDHeader)
		assert.Equal(t, responseID, contextID)
		assert.NotEmpty(t, contextID)
	})
}

func TestGetRequestID(t *testing.T) {
	t.Run("ReturnsRequestID", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())

		expectedID := "test-id-123"
		c.Set(RequestIDContextKey, expectedID)

		actualID := GetRequestID(c)
		assert.Equal(t, expectedID, actualID)
	})

	t.Run("ReturnsEmptyWhenNotSet", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())

		actualID := GetRequestID(c)
		assert.Empty(t, actualID)
	})

	t.Run("ReturnsEmptyWhenWrongType", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())

		c.Set(RequestIDContextKey, 12345) // Wrong type

		actualID := GetRequestID(c)
		assert.Empty(t, actualID)
	})
}
