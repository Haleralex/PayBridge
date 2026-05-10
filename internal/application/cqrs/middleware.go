package cqrs

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"runtime/debug"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// requestName extracts the short type name from a request value.
func requestName(request any) string {
	t := reflect.TypeOf(request)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

// LoggingMiddleware logs every command/query dispatch with name, duration, and errors.
func LoggingMiddleware(logger *slog.Logger) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, request any) (any, error) {
			name := requestName(request)
			start := time.Now()

			logger.InfoContext(ctx, "CQRS dispatch",
				slog.String("name", name),
			)

			result, err := next(ctx, request)
			duration := time.Since(start)

			if err != nil {
				logger.ErrorContext(ctx, "CQRS failed",
					slog.String("name", name),
					slog.Duration("duration", duration),
					slog.String("error", err.Error()),
				)
			} else {
				logger.InfoContext(ctx, "CQRS completed",
					slog.String("name", name),
					slog.Duration("duration", duration),
				)
			}

			return result, err
		}
	}
}

// TracingMiddleware creates an OpenTelemetry span for each command/query dispatch.
func TracingMiddleware() Middleware {
	tracer := otel.Tracer("cqrs")

	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, request any) (any, error) {
			name := requestName(request)

			// Determine operation type by suffix convention
			opType := "command"
			if isQuery(name) {
				opType = "query"
			}

			ctx, span := tracer.Start(ctx, fmt.Sprintf("cqrs.%s", name),
				trace.WithAttributes(
					attribute.String("cqrs.type", opType),
					attribute.String("cqrs.name", name),
				),
			)
			defer span.End()

			result, err := next(ctx, request)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			} else {
				span.SetStatus(codes.Ok, "")
			}

			return result, err
		}
	}
}

// isQuery checks if the request name matches query naming convention.
func isQuery(name string) bool {
	n := len(name)
	if n >= 5 && name[n-5:] == "Query" {
		return true
	}
	return false
}

// RecoveryMiddleware catches panics during handler execution and converts them to errors.
func RecoveryMiddleware(logger *slog.Logger) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, request any) (result any, err error) {
			defer func() {
				if r := recover(); r != nil {
					name := requestName(request)
					stack := string(debug.Stack())
					logger.ErrorContext(ctx, "CQRS panic recovered",
						slog.String("name", name),
						slog.Any("panic", r),
						slog.String("stack", stack),
					)
					err = fmt.Errorf("cqrs: panic in handler %s: %v", name, r)
				}
			}()
			return next(ctx, request)
		}
	}
}
