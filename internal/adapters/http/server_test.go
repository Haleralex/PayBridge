package http

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestDefaultServerConfig(t *testing.T) {
	cfg := DefaultServerConfig()

	assert.Equal(t, "0.0.0.0", cfg.Host)
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, 15*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 15*time.Second, cfg.WriteTimeout)
	assert.Equal(t, 60*time.Second, cfg.IdleTimeout)
	assert.Equal(t, 30*time.Second, cfg.ShutdownTimeout)
	assert.NotNil(t, cfg.Logger)
}

func TestServerConfig_Address(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     string
		expected string
	}{
		{"localhost", "localhost", "8080", "localhost:8080"},
		{"all interfaces", "0.0.0.0", "3000", "0.0.0.0:3000"},
		{"empty host", "", "8080", ":8080"},
		{"ipv4", "192.168.1.1", "9000", "192.168.1.1:9000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ServerConfig{Host: tt.host, Port: tt.port}
			assert.Equal(t, tt.expected, cfg.Address())
		})
	}
}

func TestNewServer_WithConfig(t *testing.T) {
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	cfg := &ServerConfig{
		Host:            "localhost",
		Port:            "9999",
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     30 * time.Second,
		ShutdownTimeout: 15 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	server := NewServer(cfg, router)

	require.NotNil(t, server)
	assert.Equal(t, cfg, server.config)
	assert.NotNil(t, server.httpServer)
	assert.Equal(t, router, server.router)
}

func TestNewServer_NilConfig(t *testing.T) {
	router := gin.New()

	server := NewServer(nil, router)

	require.NotNil(t, server)
	assert.NotNil(t, server.config)
	assert.Equal(t, "0.0.0.0", server.config.Host)
	assert.Equal(t, "8080", server.config.Port)
}

func TestNewServer_HttpServerConfiguration(t *testing.T) {
	router := gin.New()
	cfg := &ServerConfig{
		Host:         "localhost",
		Port:         "8080",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  20 * time.Second,
		Logger:       slog.Default(),
	}

	server := NewServer(cfg, router)

	assert.Equal(t, "localhost:8080", server.httpServer.Addr)
	assert.Equal(t, 5*time.Second, server.httpServer.ReadTimeout)
	assert.Equal(t, 10*time.Second, server.httpServer.WriteTimeout)
	assert.Equal(t, 20*time.Second, server.httpServer.IdleTimeout)
}

func TestServer_Shutdown(t *testing.T) {
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	cfg := &ServerConfig{
		Host:            "localhost",
		Port:            "0", // random port
		ShutdownTimeout: 5 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	server := NewServer(cfg, router)

	// Start server in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestParseAddress(t *testing.T) {
	tests := []struct {
		addr     string
		wantHost string
		wantPort string
	}{
		{":8080", "", "8080"},
		{"localhost:3000", "localhost", "3000"},
		{"192.168.1.1:9000", "192.168.1.1", "9000"},
		{"8080", "", "8080"},
		{"", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			host, port := parseAddress(tt.addr)
			assert.Equal(t, tt.wantHost, host)
			assert.Equal(t, tt.wantPort, port)
		})
	}
}

func TestServer_RouterIntegration(t *testing.T) {
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "test response")
	})

	cfg := &ServerConfig{
		Host:   "localhost",
		Port:   "8080",
		Logger: slog.Default(),
	}

	server := NewServer(cfg, router)

	// Test through httptest
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test response", w.Body.String())
}

func TestServer_RunWithContext_Cancellation(t *testing.T) {
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	cfg := &ServerConfig{
		Host:            "localhost",
		Port:            "0",
		ShutdownTimeout: 1 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	server := NewServer(cfg, router)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- server.RunWithContext(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shutdown in time")
	}
}

func TestServerConfig_CustomValues(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &ServerConfig{
		Host:            "custom-host",
		Port:            "1234",
		ReadTimeout:     1 * time.Second,
		WriteTimeout:    2 * time.Second,
		IdleTimeout:     3 * time.Second,
		ShutdownTimeout: 4 * time.Second,
		Logger:          logger,
	}

	assert.Equal(t, "custom-host", cfg.Host)
	assert.Equal(t, "1234", cfg.Port)
	assert.Equal(t, 1*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 2*time.Second, cfg.WriteTimeout)
	assert.Equal(t, 3*time.Second, cfg.IdleTimeout)
	assert.Equal(t, 4*time.Second, cfg.ShutdownTimeout)
	assert.Equal(t, logger, cfg.Logger)
}
