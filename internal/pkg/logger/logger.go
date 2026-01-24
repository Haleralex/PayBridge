// Package logger provides structured logging with correlation ID support.
//
// Features:
// - Context-aware logging with automatic correlation ID extraction
// - JSON and text output formats
// - Log level configuration
// - Consistent log structure across the application
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Context keys for correlation data
type contextKey string

const (
	// CorrelationIDKey is the context key for correlation ID
	CorrelationIDKey contextKey = "correlation_id"
	// RequestIDKey is the context key for request ID
	RequestIDKey contextKey = "request_id"
	// UserIDKey is the context key for user ID
	UserIDKey contextKey = "user_id"
	// TraceIDKey is the context key for trace ID (OpenTelemetry)
	TraceIDKey contextKey = "trace_id"
	// SpanIDKey is the context key for span ID (OpenTelemetry)
	SpanIDKey contextKey = "span_id"
)

// Config holds logger configuration
type Config struct {
	Level      string // debug, info, warn, error
	Format     string // json, text
	Output     io.Writer
	AddSource  bool
	TimeFormat string
}

// DefaultConfig returns default logger configuration
func DefaultConfig() *Config {
	return &Config{
		Level:      "info",
		Format:     "json",
		Output:     os.Stdout,
		AddSource:  false,
		TimeFormat: "2006-01-02T15:04:05.000Z07:00",
	}
}

// New creates a new slog.Logger with the given configuration
func New(cfg *Config) *slog.Logger {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Parse log level
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
	}

	output := cfg.Output
	if output == nil {
		output = os.Stdout
	}

	var handler slog.Handler
	if strings.ToLower(cfg.Format) == "text" {
		handler = slog.NewTextHandler(output, opts)
	} else {
		handler = slog.NewJSONHandler(output, opts)
	}

	// Wrap with context handler
	return slog.New(&ContextHandler{handler: handler})
}

// ContextHandler wraps a slog.Handler to extract correlation data from context
type ContextHandler struct {
	handler slog.Handler
}

// Enabled returns whether the handler is enabled for the given level
func (h *ContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle adds correlation data from context to the log record
func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	// Extract correlation data from context
	if correlationID := GetCorrelationID(ctx); correlationID != "" {
		r.AddAttrs(slog.String("correlation_id", correlationID))
	}
	if requestID := GetRequestID(ctx); requestID != "" {
		r.AddAttrs(slog.String("request_id", requestID))
	}
	if userID := GetUserID(ctx); userID != "" {
		r.AddAttrs(slog.String("user_id", userID))
	}
	if traceID := GetTraceID(ctx); traceID != "" {
		r.AddAttrs(slog.String("trace_id", traceID))
	}
	if spanID := GetSpanID(ctx); spanID != "" {
		r.AddAttrs(slog.String("span_id", spanID))
	}

	return h.handler.Handle(ctx, r)
}

// WithAttrs returns a new handler with the given attributes
func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{handler: h.handler.WithAttrs(attrs)}
}

// WithGroup returns a new handler with the given group
func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{handler: h.handler.WithGroup(name)}
}

// Context helpers

// WithCorrelationID adds correlation ID to context
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, id)
}

// GetCorrelationID extracts correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return id
	}
	return ""
}

// WithRequestID adds request ID to context
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDKey, id)
}

// GetRequestID extracts request ID from context
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// WithUserID adds user ID to context
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, UserIDKey, id)
}

// GetUserID extracts user ID from context
func GetUserID(ctx context.Context) string {
	if id, ok := ctx.Value(UserIDKey).(string); ok {
		return id
	}
	return ""
}

// WithTraceID adds trace ID to context
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, TraceIDKey, id)
}

// GetTraceID extracts trace ID from context
func GetTraceID(ctx context.Context) string {
	if id, ok := ctx.Value(TraceIDKey).(string); ok {
		return id
	}
	return ""
}

// WithSpanID adds span ID to context
func WithSpanID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, SpanIDKey, id)
}

// GetSpanID extracts span ID from context
func GetSpanID(ctx context.Context) string {
	if id, ok := ctx.Value(SpanIDKey).(string); ok {
		return id
	}
	return ""
}

// WithAllIDs adds all correlation IDs to context at once
func WithAllIDs(ctx context.Context, correlationID, requestID, userID string) context.Context {
	if correlationID != "" {
		ctx = WithCorrelationID(ctx, correlationID)
	}
	if requestID != "" {
		ctx = WithRequestID(ctx, requestID)
	}
	if userID != "" {
		ctx = WithUserID(ctx, userID)
	}
	return ctx
}

// L is a convenience function to get the default logger
func L() *slog.Logger {
	return slog.Default()
}

// FromContext returns a logger with context attributes
// This is useful when you want to log with correlation data
func FromContext(ctx context.Context) *slog.Logger {
	return slog.Default()
}

// Setup initializes the global logger
func Setup(cfg *Config) {
	logger := New(cfg)
	slog.SetDefault(logger)
}
