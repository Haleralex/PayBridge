package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("RecoversPanic", func(t *testing.T) {
		router := gin.New()
		router.Use(Recovery(DefaultRecoveryConfig()))
		router.GET("/panic", func(c *gin.Context) {
			panic("test panic")
		})

		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
		assert.Contains(t, w.Body.String(), "An unexpected error occurred")
	})

	t.Run("IncludesRequestID", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID())
		router.Use(Recovery(DefaultRecoveryConfig()))
		router.GET("/panic", func(c *gin.Context) {
			panic("test panic")
		})

		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "request_id")
	})

	t.Run("DoesNotAffectNormalRequests", func(t *testing.T) {
		router := gin.New()
		router.Use(Recovery(DefaultRecoveryConfig()))
		router.GET("/normal", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/normal", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "ok")
	})

	t.Run("WithNilConfig", func(t *testing.T) {
		router := gin.New()
		router.Use(Recovery(nil)) // Should use default config
		router.GET("/panic", func(c *gin.Context) {
			panic("test panic")
		})

		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("WithCustomConfig", func(t *testing.T) {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		config := &RecoveryConfig{
			Logger:           logger,
			EnableStackTrace: false,
			PrintStack:       false,
		}

		router := gin.New()
		router.Use(Recovery(config))
		router.GET("/panic", func(c *gin.Context) {
			panic("custom panic")
		})

		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("PanicWithStringError", func(t *testing.T) {
		router := gin.New()
		router.Use(Recovery(DefaultRecoveryConfig()))
		router.GET("/panic", func(c *gin.Context) {
			panic("string error")
		})

		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("PanicWithIntError", func(t *testing.T) {
		router := gin.New()
		router.Use(Recovery(DefaultRecoveryConfig()))
		router.GET("/panic", func(c *gin.Context) {
			panic(42)
		})

		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestDefaultRecoveryConfig(t *testing.T) {
	config := DefaultRecoveryConfig()

	assert.NotNil(t, config)
	assert.NotNil(t, config.Logger)
	assert.True(t, config.EnableStackTrace)
	assert.False(t, config.PrintStack)
}
