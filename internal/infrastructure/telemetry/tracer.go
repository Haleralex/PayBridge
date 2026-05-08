package telemetry

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitTracer initializes OpenTelemetry TracerProvider with OTLP HTTP exporter.
// Supports both plain HTTP (Jaeger) and HTTPS with auth headers (Grafana Cloud).
// Set OTEL_EXPORTER_OTLP_HEADERS="Authorization=Basic <key>" for authenticated endpoints.
// Returns TracerProvider for graceful shutdown via tp.Shutdown(ctx).
func InitTracer(ctx context.Context, serviceName, otlpEndpoint string) (*sdktrace.TracerProvider, error) {
	endpoint := normalizeEndpoint(otlpEndpoint)

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpointURL(endpoint),
	}

	if headersEnv := os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"); headersEnv != "" {
		opts = append(opts, otlptracehttp.WithHeaders(parseOTLPHeaders(headersEnv)))
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}

// normalizeEndpoint ensures the endpoint has a URL scheme.
// "jaeger:4318" → "http://jaeger:4318" (backward compat with docker-compose env var)
// "https://..." → unchanged
func normalizeEndpoint(endpoint string) string {
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	return "http://" + endpoint
}

// parseOTLPHeaders parses "key=value,key2=value2" format (OTel standard).
func parseOTLPHeaders(raw string) map[string]string {
	headers := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 && kv[0] != "" {
			headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return headers
}
