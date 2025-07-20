package tracing

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware creates an OpenTelemetry HTTP middleware
func HTTPMiddleware(serviceName string) func(http.Handler) http.Handler {
	return otelhttp.NewMiddleware(serviceName)
}

// StartHTTPSpan starts a new span for HTTP requests with common attributes
func StartHTTPSpan(r *http.Request, operationName string) (oteltrace.Span, *http.Request) {
	tracer := otel.Tracer("http-server")
	ctx, span := tracer.Start(r.Context(), operationName)
	
	// Add common HTTP attributes
	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.url", r.URL.String()),
		attribute.String("http.route", r.URL.Path),
		attribute.String("http.user_agent", r.UserAgent()),
	)
	
	// Create new request with traced context
	r = r.WithContext(ctx)
	
	return span, r
}

// EndHTTPSpan ends an HTTP span with response information
func EndHTTPSpan(span oteltrace.Span, statusCode int, responseSize int64) {
	span.SetAttributes(
		attribute.Int("http.status_code", statusCode),
		attribute.Int64("http.response_size", responseSize),
	)
	
	// Set span status based on HTTP status code
	if statusCode >= 400 {
		span.SetAttributes(attribute.Bool("error", true))
	}
	
	span.End()
}