package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	prometheusbridge "go.opentelemetry.io/contrib/bridges/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitMeterProvider initializes OpenTelemetry MeterProvider with OTLP HTTP exporter.
// Bridges the existing Prometheus default registry to OTel so all paybridge_* metrics
// are pushed to Grafana Cloud Mimir on Fly.io (no Alloy sidecar available there).
func InitMeterProvider(ctx context.Context, serviceName, otlpEndpoint string) (*sdkmetric.MeterProvider, error) {
	endpoint := normalizeEndpoint(otlpEndpoint)

	exporterOpts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpointURL(endpoint),
	}
	if headersEnv := os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"); headersEnv != "" {
		exporterOpts = append(exporterOpts, otlpmetrichttp.WithHeaders(parseOTLPHeaders(headersEnv)))
	}

	exporter, err := otlpmetrichttp.New(ctx, exporterOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceNameKey.String(serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Bridge reads from prometheus.DefaultGatherer (where metrics.go registers counters/histograms)
	// and produces OTel-format metric data for the periodic reader to export.
	bridge := prometheusbridge.NewMetricProducer()

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				exporter,
				sdkmetric.WithInterval(30*time.Second),
				sdkmetric.WithProducer(bridge),
			),
		),
	)

	otel.SetMeterProvider(mp)
	return mp, nil
}
