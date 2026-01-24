package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppConfig_IsDevelopment(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		expected    bool
	}{
		{"development", "development", true},
		{"production", "production", false},
		{"staging", "staging", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AppConfig{Environment: tt.environment}
			assert.Equal(t, tt.expected, cfg.IsDevelopment())
		})
	}
}

func TestAppConfig_IsProduction(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		expected    bool
	}{
		{"production", "production", true},
		{"development", "development", false},
		{"staging", "staging", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AppConfig{Environment: tt.environment}
			assert.Equal(t, tt.expected, cfg.IsProduction())
		})
	}
}

func TestServerConfig_Address(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     int
		expected string
	}{
		{"localhost", "localhost", 8080, "localhost:8080"},
		{"all interfaces", "0.0.0.0", 3000, "0.0.0.0:3000"},
		{"custom host", "192.168.1.1", 9000, "192.168.1.1:9000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ServerConfig{Host: tt.host, Port: tt.port}
			assert.Equal(t, tt.expected, cfg.Address())
		})
	}
}

func TestDatabaseConfig_DSN(t *testing.T) {
	cfg := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "secret",
		Database: "paybridge",
		SSLMode:  "disable",
	}

	expected := "postgres://postgres:secret@localhost:5432/paybridge?sslmode=disable"
	assert.Equal(t, expected, cfg.DSN())
}

func TestConfig_Validate_Development(t *testing.T) {
	cfg := Development()
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_Production_DefaultJWTSecret(t *testing.T) {
	cfg := &Config{
		App: AppConfig{
			Environment: "production",
		},
		Auth: AuthConfig{
			JWTSecret:      "change-me-in-production",
			EnableMockAuth: false,
		},
		Database: DatabaseConfig{
			Host: "localhost",
		},
		Server: ServerConfig{
			Port: 8080,
		},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT secret must be changed")
}

func TestConfig_Validate_Production_MockAuthEnabled(t *testing.T) {
	cfg := &Config{
		App: AppConfig{
			Environment: "production",
		},
		Auth: AuthConfig{
			JWTSecret:      "super-secure-secret",
			EnableMockAuth: true,
		},
		Database: DatabaseConfig{
			Host: "localhost",
		},
		Server: ServerConfig{
			Port: 8080,
		},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mock auth must be disabled")
}

func TestConfig_Validate_EmptyDatabaseHost(t *testing.T) {
	cfg := &Config{
		App: AppConfig{
			Environment: "development",
		},
		Database: DatabaseConfig{
			Host: "",
		},
		Server: ServerConfig{
			Port: 8080,
		},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database host is required")
}

func TestConfig_Validate_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero", 0},
		{"negative", -1},
		{"too high", 70000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				App: AppConfig{
					Environment: "development",
				},
				Database: DatabaseConfig{
					Host: "localhost",
				},
				Server: ServerConfig{
					Port: tt.port,
				},
			}

			err := cfg.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid server port")
		})
	}
}

func TestConfig_Validate_Production_Valid(t *testing.T) {
	cfg := &Config{
		App: AppConfig{
			Environment: "production",
		},
		Auth: AuthConfig{
			JWTSecret:      "my-super-secure-production-secret",
			EnableMockAuth: false,
		},
		Database: DatabaseConfig{
			Host:    "db.example.com",
			SSLMode: "require",
		},
		Server: ServerConfig{
			Port: 8080,
		},
	}

	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestDevelopment(t *testing.T) {
	cfg := Development()

	assert.Equal(t, "PayBridge", cfg.App.Name)
	assert.Equal(t, "development", cfg.App.Environment)
	assert.True(t, cfg.App.Debug)
	assert.Equal(t, "localhost", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.True(t, cfg.Auth.EnableMockAuth)
	assert.Equal(t, "debug", cfg.Log.Level)
}

func TestTest(t *testing.T) {
	cfg := Test()

	assert.Equal(t, "test", cfg.App.Environment)
	assert.Equal(t, "paybridge_test", cfg.Database.Database)
	assert.Equal(t, "error", cfg.Log.Level)
}

func TestLoadFromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("PAYBRIDGE_APP_ENVIRONMENT", "staging")
	os.Setenv("PAYBRIDGE_SERVER_PORT", "9000")
	os.Setenv("PAYBRIDGE_DATABASE_HOST", "db.staging.local")
	defer func() {
		os.Unsetenv("PAYBRIDGE_APP_ENVIRONMENT")
		os.Unsetenv("PAYBRIDGE_SERVER_PORT")
		os.Unsetenv("PAYBRIDGE_DATABASE_HOST")
	}()

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.Equal(t, "staging", cfg.App.Environment)
	assert.Equal(t, 9000, cfg.Server.Port)
	assert.Equal(t, "db.staging.local", cfg.Database.Host)
}

func TestLoad_FileNotFound(t *testing.T) {
	// Should use defaults when file not found
	cfg, err := Load("/nonexistent/path", "nonexistent")
	require.NoError(t, err)

	// Should have default values
	assert.Equal(t, "PayBridge", cfg.App.Name)
	assert.Equal(t, 8080, cfg.Server.Port)
}

func TestLoad_WithEnvOverride(t *testing.T) {
	// Set environment variable to override config
	os.Setenv("PAYBRIDGE_SERVER_PORT", "3000")
	defer os.Unsetenv("PAYBRIDGE_SERVER_PORT")

	cfg, err := Load("/nonexistent/path", "nonexistent")
	require.NoError(t, err)

	// Env should override default
	assert.Equal(t, 3000, cfg.Server.Port)
}

func TestServerConfig_Timeouts(t *testing.T) {
	cfg := Development()

	assert.Equal(t, 15*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 15*time.Second, cfg.Server.WriteTimeout)
	assert.Equal(t, 60*time.Second, cfg.Server.IdleTimeout)
	assert.Equal(t, 30*time.Second, cfg.Server.ShutdownTimeout)
}

func TestDatabaseConfig_ConnectionPool(t *testing.T) {
	cfg := Development()

	assert.Equal(t, int32(10), cfg.Database.MaxConnections)
	assert.Equal(t, int32(2), cfg.Database.MinConnections)
	assert.Equal(t, time.Hour, cfg.Database.MaxConnLifetime)
	assert.Equal(t, 30*time.Minute, cfg.Database.MaxConnIdleTime)
}

func TestAuthConfig_TokenExpiry(t *testing.T) {
	cfg := Development()

	assert.Equal(t, 15*time.Minute, cfg.Auth.AccessTokenExpiry)
	assert.Equal(t, 168*time.Hour, cfg.Auth.RefreshTokenExpiry) // 7 days
}

func TestRateLimitConfig(t *testing.T) {
	cfg := Development()

	assert.True(t, cfg.RateLimit.Enabled)
	assert.Equal(t, 100, cfg.RateLimit.RequestsPerMinute)
	assert.Equal(t, 20, cfg.RateLimit.BurstSize)
	assert.Equal(t, 30, cfg.RateLimit.FinancialOpsPerMin)
	assert.Equal(t, time.Minute, cfg.RateLimit.CleanupInterval)
}

func TestCORSConfig(t *testing.T) {
	cfg := Development()

	assert.Contains(t, cfg.CORS.AllowedOrigins, "*")
	assert.Contains(t, cfg.CORS.AllowedMethods, "GET")
	assert.Contains(t, cfg.CORS.AllowedMethods, "POST")
	assert.True(t, cfg.CORS.AllowCredentials)
	assert.Equal(t, 12*time.Hour, cfg.CORS.MaxAge)
}

func TestLogConfig(t *testing.T) {
	cfg := Development()

	assert.Equal(t, "debug", cfg.Log.Level)
	assert.Equal(t, "text", cfg.Log.Format)
	assert.Equal(t, "stdout", cfg.Log.Output)
}
