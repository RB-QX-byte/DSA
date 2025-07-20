package tracing

import (
	"context"
	"log"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// TracingConfig holds configuration for OpenTelemetry setup
type TracingConfig struct {
	ServiceName        string
	ServiceVersion     string
	ServiceEnvironment string
	OTLPEndpoint      string
}

// DefaultConfig returns a default tracing configuration
func DefaultConfig() TracingConfig {
	return TracingConfig{
		ServiceName:        getEnvOrDefault("OTEL_SERVICE_NAME", "judge-system"),
		ServiceVersion:     getEnvOrDefault("OTEL_SERVICE_VERSION", "1.0.0"),
		ServiceEnvironment: getEnvOrDefault("OTEL_ENVIRONMENT", "development"),
		OTLPEndpoint:      getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel-collector:4318"),
	}
}

// InitTracing initializes OpenTelemetry tracing
func InitTracing(config TracingConfig) func(context.Context) error {
	ctx := context.Background()

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(config.ServiceEnvironment),
		),
	)
	if err != nil {
		log.Printf("Failed to create resource: %v", err)
		return nil
	}

	// Create OTLP exporter
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(config.OTLPEndpoint),
		otlptracehttp.WithInsecure(), // Use HTTP instead of HTTPS for local development
	)
	if err != nil {
		log.Printf("Failed to create OTLP exporter: %v", err)
		return nil
	}

	// Create trace provider
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
		trace.WithSampler(trace.AlwaysSample()), // Sample all traces in development
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)

	// Set global propagator for context propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Printf("OpenTelemetry tracing initialized for service: %s", config.ServiceName)

	// Return cleanup function
	return tp.Shutdown
}

// GetTracer returns a tracer for the given name
func GetTracer(name string) oteltrace.Tracer {
	return otel.Tracer(name)
}

// getEnvOrDefault returns environment variable value or default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}