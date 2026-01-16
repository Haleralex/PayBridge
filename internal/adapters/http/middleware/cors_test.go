package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("AllowsAllOrigins", func(t *testing.T) {
		router := gin.New()
		router.Use(CORS(DefaultCORSConfig()))
		router.GET("/test", func(c *gin.Context) {
			c.String(200, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "http://example.com")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"))
		assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Headers"))
	})

	t.Run("AllowsSpecificOrigin", func(t *testing.T) {
		config := &CORSConfig{
			AllowOrigins:     []string{"http://example.com", "http://test.com"},
			AllowMethods:     []string{http.MethodGet, http.MethodPost},
			AllowHeaders:     []string{"Content-Type"},
			ExposeHeaders:    []string{"X-Request-ID"},
			AllowCredentials: true,
			MaxAge:           3600,
		}

		router := gin.New()
		router.Use(CORS(config))
		router.GET("/test", func(c *gin.Context) {
			c.String(200, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "http://example.com")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, "http://example.com", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
	})

	t.Run("RejectsUnallowedOrigin", func(t *testing.T) {
		config := &CORSConfig{
			AllowOrigins:     []string{"http://example.com"},
			AllowMethods:     []string{http.MethodGet},
			AllowHeaders:     []string{"Content-Type"},
			ExposeHeaders:    []string{},
			AllowCredentials: false,
			MaxAge:           3600,
		}

		router := gin.New()
		router.Use(CORS(config))
		router.GET("/test", func(c *gin.Context) {
			c.String(200, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "http://malicious.com")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("HandlesPreflightRequest", func(t *testing.T) {
		router := gin.New()
		router.Use(CORS(DefaultCORSConfig()))
		router.OPTIONS("/test", func(c *gin.Context) {
			c.String(200, "should not reach here")
		})

		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		req.Header.Set("Origin", "http://example.com")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.NotContains(t, w.Body.String(), "should not reach here")
	})

	t.Run("AllowsActualRequestAfterPreflight", func(t *testing.T) {
		router := gin.New()
		router.Use(CORS(DefaultCORSConfig()))
		router.POST("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set("Origin", "http://example.com")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Contains(t, w.Body.String(), "ok")
	})

	t.Run("WithNilConfig", func(t *testing.T) {
		router := gin.New()
		router.Use(CORS(nil)) // Should use default
		router.GET("/test", func(c *gin.Context) {
			c.String(200, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "http://example.com")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("NoOriginHeader", func(t *testing.T) {
		router := gin.New()
		router.Use(CORS(DefaultCORSConfig()))
		router.GET("/test", func(c *gin.Context) {
			c.String(200, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		// No Origin header
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestDefaultCORSConfig(t *testing.T) {
	config := DefaultCORSConfig()

	assert.NotNil(t, config)
	assert.Equal(t, []string{"*"}, config.AllowOrigins)
	assert.Contains(t, config.AllowMethods, http.MethodGet)
	assert.Contains(t, config.AllowMethods, http.MethodPost)
	assert.Contains(t, config.AllowHeaders, "Authorization")
	assert.Contains(t, config.ExposeHeaders, "X-Request-ID")
	assert.False(t, config.AllowCredentials)
	assert.Equal(t, 86400, config.MaxAge)
}

func TestProductionCORSConfig(t *testing.T) {
	origins := []string{"https://app.example.com", "https://admin.example.com"}
	config := ProductionCORSConfig(origins)

	assert.NotNil(t, config)
	assert.Equal(t, origins, config.AllowOrigins)
	assert.True(t, config.AllowCredentials)
	assert.Contains(t, config.AllowMethods, http.MethodGet)
	assert.Contains(t, config.AllowHeaders, "Authorization")
}
