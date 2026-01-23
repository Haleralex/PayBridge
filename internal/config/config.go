// Package config - Application configuration management.
//
// Использует Viper для:
// - Загрузки из YAML файлов
// - Переменных окружения
// - Значений по умолчанию
//
// Порядок приоритета (от высшего к низшему):
// 1. Environment variables
// 2. Config file
// 3. Default values
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// ============================================
// Main Configuration
// ============================================

// Config - главная структура конфигурации приложения.
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Auth     AuthConfig     `mapstructure:"auth"`
	CORS     CORSConfig     `mapstructure:"cors"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
	Log      LogConfig      `mapstructure:"log"`
}

// ============================================
// App Configuration
// ============================================

// AppConfig - конфигурация приложения.
type AppConfig struct {
	Name        string `mapstructure:"name"`
	Version     string `mapstructure:"version"`
	Environment string `mapstructure:"environment"` // development, staging, production
	Debug       bool   `mapstructure:"debug"`
	BuildTime   string `mapstructure:"build_time"`
	GitCommit   string `mapstructure:"git_commit"`
}

// IsDevelopment возвращает true если окружение development.
func (c *AppConfig) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction возвращает true если окружение production.
func (c *AppConfig) IsProduction() bool {
	return c.Environment == "production"
}

// ============================================
// Server Configuration
// ============================================

// ServerConfig - конфигурация HTTP сервера.
type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// Address возвращает полный адрес сервера.
func (c *ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// ============================================
// Database Configuration
// ============================================

// DatabaseConfig - конфигурация базы данных.
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Database        string        `mapstructure:"database"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxConnections  int32         `mapstructure:"max_connections"`
	MinConnections  int32         `mapstructure:"min_connections"`
	MaxConnLifetime time.Duration `mapstructure:"max_conn_lifetime"`
	MaxConnIdleTime time.Duration `mapstructure:"max_conn_idle_time"`
}

// DSN возвращает строку подключения к PostgreSQL.
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User,
		c.Password,
		c.Host,
		c.Port,
		c.Database,
		c.SSLMode,
	)
}

// ============================================
// Auth Configuration
// ============================================

// AuthConfig - конфигурация аутентификации.
type AuthConfig struct {
	JWTSecret          string        `mapstructure:"jwt_secret"`
	JWTIssuer          string        `mapstructure:"jwt_issuer"`
	AccessTokenExpiry  time.Duration `mapstructure:"access_token_expiry"`
	RefreshTokenExpiry time.Duration `mapstructure:"refresh_token_expiry"`
	EnableMockAuth     bool          `mapstructure:"enable_mock_auth"` // Только для development!
}

// ============================================
// CORS Configuration
// ============================================

// CORSConfig - конфигурация CORS.
type CORSConfig struct {
	AllowedOrigins   []string      `mapstructure:"allowed_origins"`
	AllowedMethods   []string      `mapstructure:"allowed_methods"`
	AllowedHeaders   []string      `mapstructure:"allowed_headers"`
	ExposedHeaders   []string      `mapstructure:"exposed_headers"`
	AllowCredentials bool          `mapstructure:"allow_credentials"`
	MaxAge           time.Duration `mapstructure:"max_age"`
}

// ============================================
// Rate Limit Configuration
// ============================================

// RateLimitConfig - конфигурация rate limiting.
type RateLimitConfig struct {
	Enabled              bool          `mapstructure:"enabled"`
	RequestsPerMinute    int           `mapstructure:"requests_per_minute"`
	BurstSize            int           `mapstructure:"burst_size"`
	FinancialOpsPerMin   int           `mapstructure:"financial_ops_per_min"`
	CleanupInterval      time.Duration `mapstructure:"cleanup_interval"`
}

// ============================================
// Log Configuration
// ============================================

// LogConfig - конфигурация логирования.
type LogConfig struct {
	Level      string `mapstructure:"level"`  // debug, info, warn, error
	Format     string `mapstructure:"format"` // json, text
	Output     string `mapstructure:"output"` // stdout, stderr, file
	FilePath   string `mapstructure:"file_path"`
	MaxSize    int    `mapstructure:"max_size"`    // MB
	MaxBackups int    `mapstructure:"max_backups"` // количество файлов
	MaxAge     int    `mapstructure:"max_age"`     // дней
	Compress   bool   `mapstructure:"compress"`
}

// ============================================
// Configuration Loading
// ============================================

// Load загружает конфигурацию из файла и переменных окружения.
//
// configPath - путь к директории с конфигурацией (например, "configs")
// configName - имя файла конфигурации без расширения (например, "config")
//
// Поддерживаемые форматы: yaml, json, toml
func Load(configPath, configName string) (*Config, error) {
	v := viper.New()

	// Устанавливаем defaults
	setDefaults(v)

	// Настраиваем Viper
	v.SetConfigName(configName)
	v.SetConfigType("yaml")
	v.AddConfigPath(configPath)
	v.AddConfigPath(".")
	v.AddConfigPath("./configs")
	v.AddConfigPath("/etc/paybridge")

	// Переменные окружения
	v.SetEnvPrefix("PAYBRIDGE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Читаем конфигурационный файл
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Файл не найден - используем defaults и env vars
	}

	// Парсим в структуру
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Валидируем конфигурацию
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// LoadFromEnv загружает конфигурацию только из переменных окружения.
func LoadFromEnv() (*Config, error) {
	v := viper.New()

	// Устанавливаем defaults
	setDefaults(v)

	// Переменные окружения
	v.SetEnvPrefix("PAYBRIDGE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Bind specific env vars
	bindEnvVars(v)

	// Парсим в структуру
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Валидируем конфигурацию
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// setDefaults устанавливает значения по умолчанию.
func setDefaults(v *viper.Viper) {
	// App defaults
	v.SetDefault("app.name", "PayBridge")
	v.SetDefault("app.version", "1.0.0")
	v.SetDefault("app.environment", "development")
	v.SetDefault("app.debug", true)

	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "15s")
	v.SetDefault("server.write_timeout", "15s")
	v.SetDefault("server.idle_timeout", "60s")
	v.SetDefault("server.shutdown_timeout", "30s")

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "postgres")
	v.SetDefault("database.database", "paybridge")
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.max_connections", 25)
	v.SetDefault("database.min_connections", 5)
	v.SetDefault("database.max_conn_lifetime", "1h")
	v.SetDefault("database.max_conn_idle_time", "30m")

	// Auth defaults
	v.SetDefault("auth.jwt_secret", "change-me-in-production")
	v.SetDefault("auth.jwt_issuer", "paybridge")
	v.SetDefault("auth.access_token_expiry", "15m")
	v.SetDefault("auth.refresh_token_expiry", "168h") // 7 days
	v.SetDefault("auth.enable_mock_auth", true)

	// CORS defaults
	v.SetDefault("cors.allowed_origins", []string{"*"})
	v.SetDefault("cors.allowed_methods", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"})
	v.SetDefault("cors.allowed_headers", []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"})
	v.SetDefault("cors.exposed_headers", []string{"X-Request-ID", "X-RateLimit-Limit", "X-RateLimit-Remaining"})
	v.SetDefault("cors.allow_credentials", true)
	v.SetDefault("cors.max_age", "12h")

	// Rate Limit defaults
	v.SetDefault("rate_limit.enabled", true)
	v.SetDefault("rate_limit.requests_per_minute", 100)
	v.SetDefault("rate_limit.burst_size", 20)
	v.SetDefault("rate_limit.financial_ops_per_min", 30)
	v.SetDefault("rate_limit.cleanup_interval", "1m")

	// Log defaults
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
	v.SetDefault("log.output", "stdout")
}

// bindEnvVars привязывает переменные окружения.
func bindEnvVars(v *viper.Viper) {
	// Database (обычно передаётся через env в production)
	_ = v.BindEnv("database.host", "PAYBRIDGE_DATABASE_HOST", "DB_HOST")
	_ = v.BindEnv("database.port", "PAYBRIDGE_DATABASE_PORT", "DB_PORT")
	_ = v.BindEnv("database.user", "PAYBRIDGE_DATABASE_USER", "DB_USER")
	_ = v.BindEnv("database.password", "PAYBRIDGE_DATABASE_PASSWORD", "DB_PASSWORD")
	_ = v.BindEnv("database.database", "PAYBRIDGE_DATABASE_DATABASE", "DB_NAME")

	// Auth
	_ = v.BindEnv("auth.jwt_secret", "PAYBRIDGE_AUTH_JWT_SECRET", "JWT_SECRET")

	// Server
	_ = v.BindEnv("server.port", "PAYBRIDGE_SERVER_PORT", "PORT")

	// App
	_ = v.BindEnv("app.environment", "PAYBRIDGE_APP_ENVIRONMENT", "ENVIRONMENT", "ENV")
}

// ============================================
// Configuration Validation
// ============================================

// Validate валидирует конфигурацию.
func (c *Config) Validate() error {
	// Проверяем критичные настройки в production
	if c.App.IsProduction() {
		if c.Auth.JWTSecret == "change-me-in-production" {
			return fmt.Errorf("JWT secret must be changed in production")
		}

		if c.Auth.EnableMockAuth {
			return fmt.Errorf("mock auth must be disabled in production")
		}

		if c.Database.SSLMode == "disable" {
			// Warning, но не error
			// В реальном приложении можно добавить логирование
		}
	}

	// Проверяем обязательные поля
	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}

	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	return nil
}

// ============================================
// Development Helpers
// ============================================

// Development возвращает конфигурацию для разработки.
func Development() *Config {
	return &Config{
		App: AppConfig{
			Name:        "PayBridge",
			Version:     "dev",
			Environment: "development",
			Debug:       true,
		},
		Server: ServerConfig{
			Host:            "localhost",
			Port:            8080,
			ReadTimeout:     15 * time.Second,
			WriteTimeout:    15 * time.Second,
			IdleTimeout:     60 * time.Second,
			ShutdownTimeout: 30 * time.Second,
		},
		Database: DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			User:            "postgres",
			Password:        "postgres",
			Database:        "paybridge",
			SSLMode:         "disable",
			MaxConnections:  10,
			MinConnections:  2,
			MaxConnLifetime: time.Hour,
			MaxConnIdleTime: 30 * time.Minute,
		},
		Auth: AuthConfig{
			JWTSecret:          "dev-secret-key",
			JWTIssuer:          "paybridge-dev",
			AccessTokenExpiry:  15 * time.Minute,
			RefreshTokenExpiry: 168 * time.Hour,
			EnableMockAuth:     true,
		},
		CORS: CORSConfig{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"*"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		},
		RateLimit: RateLimitConfig{
			Enabled:            true,
			RequestsPerMinute:  100,
			BurstSize:          20,
			FinancialOpsPerMin: 30,
			CleanupInterval:    time.Minute,
		},
		Log: LogConfig{
			Level:  "debug",
			Format: "text",
			Output: "stdout",
		},
	}
}

// Test возвращает конфигурацию для тестов.
func Test() *Config {
	cfg := Development()
	cfg.App.Environment = "test"
	cfg.Database.Database = "paybridge_test"
	cfg.Log.Level = "error" // Меньше шума в тестах
	return cfg
}
