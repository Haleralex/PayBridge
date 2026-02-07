package container

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Haleralex/wallethub/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	cfg := config.Development()
	c := New(cfg)

	require.NotNil(t, c)
	assert.Equal(t, cfg, c.config)
}

func TestContainer_Config(t *testing.T) {
	cfg := config.Development()
	c := New(cfg)

	assert.Equal(t, cfg, c.Config())
}

func TestContainer_Logger_BeforeInit(t *testing.T) {
	cfg := config.Development()
	c := New(cfg)

	// Logger is nil before initialization
	assert.Nil(t, c.Logger())
}

func TestContainer_Pool_BeforeInit(t *testing.T) {
	cfg := config.Development()
	c := New(cfg)

	// Pool is nil before initialization
	assert.Nil(t, c.Pool())
}

func TestContainer_HTTPServer_BeforeInit(t *testing.T) {
	cfg := config.Development()
	c := New(cfg)

	// Server is nil before initialization
	assert.Nil(t, c.HTTPServer())
}

func TestContainer_UserRepository_BeforeInit(t *testing.T) {
	cfg := config.Development()
	c := New(cfg)

	// Repo is nil before initialization
	assert.Nil(t, c.UserRepository())
}

func TestContainer_WalletRepository_BeforeInit(t *testing.T) {
	cfg := config.Development()
	c := New(cfg)

	// Repo is nil before initialization
	assert.Nil(t, c.WalletRepository())
}

func TestContainer_TransactionRepository_BeforeInit(t *testing.T) {
	cfg := config.Development()
	c := New(cfg)

	// Repo is nil before initialization
	assert.Nil(t, c.TransactionRepository())
}

func TestContainer_UnitOfWork_BeforeInit(t *testing.T) {
	cfg := config.Development()
	c := New(cfg)

	// UoW is nil before initialization
	assert.Nil(t, c.UnitOfWork())
}

func TestContainer_CreateUserUseCase_BeforeInit(t *testing.T) {
	cfg := config.Development()
	c := New(cfg)

	assert.Nil(t, c.CreateUserUseCase())
}

func TestContainer_ApproveKYCUseCase_BeforeInit(t *testing.T) {
	cfg := config.Development()
	c := New(cfg)

	assert.Nil(t, c.ApproveKYCUseCase())
}

func TestContainer_CreateWalletUseCase_BeforeInit(t *testing.T) {
	cfg := config.Development()
	c := New(cfg)

	assert.Nil(t, c.CreateWalletUseCase())
}

func TestContainer_CreditWalletUseCase_BeforeInit(t *testing.T) {
	cfg := config.Development()
	c := New(cfg)

	assert.Nil(t, c.CreditWalletUseCase())
}

func TestContainer_TransferBetweenWalletsUseCase_BeforeInit(t *testing.T) {
	cfg := config.Development()
	c := New(cfg)

	assert.Nil(t, c.TransferBetweenWalletsUseCase())
}

func TestContainer_initLogger_DebugLevel(t *testing.T) {
	cfg := config.Development()
	cfg.Log.Level = "debug"
	cfg.Log.Format = "text"
	cfg.App.Debug = true

	c := New(cfg)
	logger := c.initLogger()

	require.NotNil(t, logger)
	assert.NotNil(t, logger.Handler())
}

func TestContainer_initLogger_InfoLevel(t *testing.T) {
	cfg := config.Development()
	cfg.Log.Level = "info"
	cfg.Log.Format = "json"

	c := New(cfg)
	logger := c.initLogger()

	require.NotNil(t, logger)
}

func TestContainer_initLogger_WarnLevel(t *testing.T) {
	cfg := config.Development()
	cfg.Log.Level = "warn"
	cfg.Log.Format = "text"

	c := New(cfg)
	logger := c.initLogger()

	require.NotNil(t, logger)
}

func TestContainer_initLogger_ErrorLevel(t *testing.T) {
	cfg := config.Development()
	cfg.Log.Level = "error"
	cfg.Log.Format = "json"

	c := New(cfg)
	logger := c.initLogger()

	require.NotNil(t, logger)
}

func TestContainer_initLogger_UnknownLevel(t *testing.T) {
	cfg := config.Development()
	cfg.Log.Level = "unknown"

	c := New(cfg)
	logger := c.initLogger()

	require.NotNil(t, logger)
	// Should default to info level
}

func TestContainer_initLogger_JSONFormat(t *testing.T) {
	cfg := config.Development()
	cfg.Log.Format = "json"

	c := New(cfg)
	logger := c.initLogger()

	require.NotNil(t, logger)
}

func TestContainer_initLogger_TextFormat(t *testing.T) {
	cfg := config.Development()
	cfg.Log.Format = "text"

	c := New(cfg)
	logger := c.initLogger()

	require.NotNil(t, logger)
}

// ContainerBuilder Tests

func TestNewBuilder(t *testing.T) {
	cfg := config.Development()
	builder := NewBuilder(cfg)

	require.NotNil(t, builder)
	assert.Equal(t, cfg, builder.cfg)
}

func TestContainerBuilder_WithLogger(t *testing.T) {
	cfg := config.Development()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	builder := NewBuilder(cfg).WithLogger(logger)

	assert.Equal(t, logger, builder.logger)
}

func TestContainerBuilder_WithPool(t *testing.T) {
	cfg := config.Development()

	// nil pool for testing builder chain
	builder := NewBuilder(cfg).WithPool(nil)

	assert.Nil(t, builder.pool)
}

func TestContainerBuilder_WithEventPublisher(t *testing.T) {
	cfg := config.Development()

	// nil publisher for testing builder chain
	builder := NewBuilder(cfg).WithEventPublisher(nil)

	assert.Nil(t, builder.eventPublisher)
}

func TestContainerBuilder_Chain(t *testing.T) {
	cfg := config.Development()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	builder := NewBuilder(cfg).
		WithLogger(logger).
		WithPool(nil).
		WithEventPublisher(nil)

	assert.Equal(t, cfg, builder.cfg)
	assert.Equal(t, logger, builder.logger)
}

// HealthStatus Tests

func TestHealthStatus_Structure(t *testing.T) {
	status := &HealthStatus{
		Status:  "healthy",
		Version: "1.0.0",
		Uptime:  time.Hour,
		Checks:  map[string]string{"database": "ok"},
	}

	assert.Equal(t, "healthy", status.Status)
	assert.Equal(t, "1.0.0", status.Version)
	assert.Equal(t, time.Hour, status.Uptime)
	assert.Equal(t, "ok", status.Checks["database"])
}

func TestHealthStatus_Unhealthy(t *testing.T) {
	status := &HealthStatus{
		Status:  "unhealthy",
		Version: "1.0.0",
		Checks:  map[string]string{"database": "error: connection refused"},
	}

	assert.Equal(t, "unhealthy", status.Status)
	assert.Contains(t, status.Checks["database"], "error")
}

// Shutdown Tests

func TestContainer_Shutdown_NilComponents(t *testing.T) {
	cfg := config.Development()
	c := New(cfg)
	c.logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Should not panic with nil components
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := c.Shutdown(ctx)
	assert.NoError(t, err)
}

// Initialize Tests (with expected failures for no DB)

func TestContainer_Initialize_NoDB(t *testing.T) {
	cfg := config.Development()
	cfg.Database.Host = "invalid-host-that-does-not-exist"
	cfg.Database.Port = 59999

	c := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := c.Initialize(ctx)

	// Should fail because database is not available
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize database")
}

// Edge Cases

func TestContainer_MultipleNew(t *testing.T) {
	cfg1 := config.Development()
	cfg2 := config.Test()

	c1 := New(cfg1)
	c2 := New(cfg2)

	assert.NotEqual(t, c1, c2)
	assert.Equal(t, cfg1, c1.Config())
	assert.Equal(t, cfg2, c2.Config())
}

func TestContainer_ConfigImmutability(t *testing.T) {
	cfg := config.Development()
	c := New(cfg)

	// Modifying the returned config should not affect the container
	// (depends on implementation - this tests the getter behavior)
	returnedCfg := c.Config()
	assert.Equal(t, cfg, returnedCfg)
}

func TestContainer_AllLogLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error", "unknown", ""}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			cfg := config.Development()
			cfg.Log.Level = level

			c := New(cfg)
			logger := c.initLogger()

			require.NotNil(t, logger)
		})
	}
}

func TestContainer_AllLogFormats(t *testing.T) {
	formats := []string{"json", "text", "unknown", ""}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			cfg := config.Development()
			cfg.Log.Format = format

			c := New(cfg)
			logger := c.initLogger()

			require.NotNil(t, logger)
		})
	}
}

func TestContainerBuilder_Build_WithoutPool(t *testing.T) {
	cfg := config.Development()
	cfg.Database.Host = "invalid-host"
	cfg.Database.Port = 59999

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := NewBuilder(cfg).
		WithLogger(logger).
		Build(ctx)

	// Should fail - no pool provided and DB connection fails
	assert.Error(t, err)
}
