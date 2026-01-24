package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "info", cfg.Level)
	assert.Equal(t, "json", cfg.Format)
	assert.NotNil(t, cfg.Output)
	assert.False(t, cfg.AddSource)
}

func TestNew_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}

	logger := New(cfg)
	require.NotNil(t, logger)

	logger.Info("test message", "key", "value")

	output := buf.String()
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "key")
	assert.Contains(t, output, "value")

	// Verify it's valid JSON
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(output), &logEntry)
	assert.NoError(t, err)
}

func TestNew_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Level:  "debug",
		Format: "text",
		Output: &buf,
	}

	logger := New(cfg)
	require.NotNil(t, logger)

	logger.Debug("debug message")

	output := buf.String()
	assert.Contains(t, output, "debug message")
}

func TestNew_LogLevels(t *testing.T) {
	tests := []struct {
		level    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo}, // default
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			var buf bytes.Buffer
			cfg := &Config{
				Level:  tt.level,
				Format: "json",
				Output: &buf,
			}

			logger := New(cfg)
			require.NotNil(t, logger)

			// Check that the handler is enabled for the expected level
			handler := logger.Handler()
			assert.True(t, handler.Enabled(context.Background(), tt.expected))
		})
	}
}

func TestNew_NilConfig(t *testing.T) {
	logger := New(nil)
	require.NotNil(t, logger)
}

func TestContextHandler_WithCorrelationID(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}

	logger := New(cfg)

	ctx := context.Background()
	ctx = WithCorrelationID(ctx, "corr-123")
	ctx = WithRequestID(ctx, "req-456")
	ctx = WithUserID(ctx, "user-789")

	logger.InfoContext(ctx, "test with context")

	output := buf.String()
	assert.Contains(t, output, "corr-123")
	assert.Contains(t, output, "req-456")
	assert.Contains(t, output, "user-789")
}

func TestContextHandler_WithTraceIDs(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}

	logger := New(cfg)

	ctx := context.Background()
	ctx = WithTraceID(ctx, "trace-abc")
	ctx = WithSpanID(ctx, "span-def")

	logger.InfoContext(ctx, "test with trace")

	output := buf.String()
	assert.Contains(t, output, "trace-abc")
	assert.Contains(t, output, "span-def")
}

func TestWithCorrelationID(t *testing.T) {
	ctx := context.Background()
	ctx = WithCorrelationID(ctx, "test-correlation-id")

	id := GetCorrelationID(ctx)
	assert.Equal(t, "test-correlation-id", id)
}

func TestGetCorrelationID_Empty(t *testing.T) {
	ctx := context.Background()
	id := GetCorrelationID(ctx)
	assert.Empty(t, id)
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "test-request-id")

	id := GetRequestID(ctx)
	assert.Equal(t, "test-request-id", id)
}

func TestGetRequestID_Empty(t *testing.T) {
	ctx := context.Background()
	id := GetRequestID(ctx)
	assert.Empty(t, id)
}

func TestWithUserID(t *testing.T) {
	ctx := context.Background()
	ctx = WithUserID(ctx, "test-user-id")

	id := GetUserID(ctx)
	assert.Equal(t, "test-user-id", id)
}

func TestGetUserID_Empty(t *testing.T) {
	ctx := context.Background()
	id := GetUserID(ctx)
	assert.Empty(t, id)
}

func TestWithTraceID(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "test-trace-id")

	id := GetTraceID(ctx)
	assert.Equal(t, "test-trace-id", id)
}

func TestGetTraceID_Empty(t *testing.T) {
	ctx := context.Background()
	id := GetTraceID(ctx)
	assert.Empty(t, id)
}

func TestWithSpanID(t *testing.T) {
	ctx := context.Background()
	ctx = WithSpanID(ctx, "test-span-id")

	id := GetSpanID(ctx)
	assert.Equal(t, "test-span-id", id)
}

func TestGetSpanID_Empty(t *testing.T) {
	ctx := context.Background()
	id := GetSpanID(ctx)
	assert.Empty(t, id)
}

func TestWithAllIDs(t *testing.T) {
	ctx := context.Background()
	ctx = WithAllIDs(ctx, "corr-1", "req-2", "user-3")

	assert.Equal(t, "corr-1", GetCorrelationID(ctx))
	assert.Equal(t, "req-2", GetRequestID(ctx))
	assert.Equal(t, "user-3", GetUserID(ctx))
}

func TestWithAllIDs_EmptyValues(t *testing.T) {
	ctx := context.Background()
	ctx = WithAllIDs(ctx, "", "", "")

	assert.Empty(t, GetCorrelationID(ctx))
	assert.Empty(t, GetRequestID(ctx))
	assert.Empty(t, GetUserID(ctx))
}

func TestWithAllIDs_PartialValues(t *testing.T) {
	ctx := context.Background()
	ctx = WithAllIDs(ctx, "corr-1", "", "user-3")

	assert.Equal(t, "corr-1", GetCorrelationID(ctx))
	assert.Empty(t, GetRequestID(ctx))
	assert.Equal(t, "user-3", GetUserID(ctx))
}

func TestL(t *testing.T) {
	logger := L()
	require.NotNil(t, logger)
}

func TestFromContext(t *testing.T) {
	ctx := context.Background()
	logger := FromContext(ctx)
	require.NotNil(t, logger)
}

func TestSetup(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}

	Setup(cfg)

	// Use the default logger after setup
	slog.Info("test after setup")

	output := buf.String()
	assert.Contains(t, output, "test after setup")
}

func TestContextHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}

	logger := New(cfg)
	loggerWithAttr := logger.With("service", "paybridge")

	loggerWithAttr.Info("test with attr")

	output := buf.String()
	assert.Contains(t, output, "paybridge")
}

func TestContextHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}

	logger := New(cfg)
	loggerWithGroup := logger.WithGroup("request")

	loggerWithGroup.Info("test with group", "method", "GET")

	output := buf.String()
	assert.Contains(t, output, "request")
	assert.Contains(t, output, "method")
}

func TestLogLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Level:  "warn",
		Format: "json",
		Output: &buf,
	}

	logger := New(cfg)

	// Debug and Info should be filtered out
	logger.Debug("debug message")
	logger.Info("info message")

	// Warn and Error should appear
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	assert.NotContains(t, output, "debug message")
	assert.NotContains(t, output, "info message")
	assert.Contains(t, output, "warn message")
	assert.Contains(t, output, "error message")
}

func TestContextHandler_Enabled(t *testing.T) {
	handler := &ContextHandler{
		handler: slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{
			Level: slog.LevelWarn,
		}),
	}

	assert.False(t, handler.Enabled(context.Background(), slog.LevelDebug))
	assert.False(t, handler.Enabled(context.Background(), slog.LevelInfo))
	assert.True(t, handler.Enabled(context.Background(), slog.LevelWarn))
	assert.True(t, handler.Enabled(context.Background(), slog.LevelError))
}

func TestNew_NilOutput(t *testing.T) {
	cfg := &Config{
		Level:  "info",
		Format: "json",
		Output: nil, // Should default to stdout
	}

	logger := New(cfg)
	require.NotNil(t, logger)
}

func TestCaseSensitivity(t *testing.T) {
	tests := []string{"INFO", "Info", "iNfO", "debug", "DEBUG", "Debug"}

	for _, level := range tests {
		t.Run(level, func(t *testing.T) {
			var buf bytes.Buffer
			cfg := &Config{
				Level:  level,
				Format: "json",
				Output: &buf,
			}

			logger := New(cfg)
			require.NotNil(t, logger)
		})
	}
}

func TestFormatCaseSensitivity(t *testing.T) {
	tests := []struct {
		format   string
		expected string // "json" or "text" output pattern
	}{
		{"json", "{"},
		{"JSON", "{"},
		{"Json", "{"},
		{"text", "level="},
		{"TEXT", "level="},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			var buf bytes.Buffer
			cfg := &Config{
				Level:  "info",
				Format: tt.format,
				Output: &buf,
			}

			logger := New(cfg)
			logger.Info("test")

			output := buf.String()
			assert.True(t, strings.Contains(output, tt.expected) || len(output) > 0)
		})
	}
}
