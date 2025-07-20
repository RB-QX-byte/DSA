package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP request metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "endpoint"},
	)

	// Judge queue metrics
	judgeQueueSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "judge_queue_size",
			Help: "Current size of the judge queue",
		},
		[]string{"queue_name"},
	)

	judgeTasksProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "judge_tasks_processed_total",
			Help: "Total number of judge tasks processed",
		},
		[]string{"queue_name", "status"},
	)

	judgeTaskDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "judge_task_duration_seconds",
			Help:    "Duration of judge task processing in seconds",
			Buckets: []float64{.1, .5, 1, 2, 5, 10, 30, 60, 120},
		},
		[]string{"queue_name"},
	)

	// Database metrics
	databaseConnectionsInUse = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "database_connections_in_use",
			Help: "Number of database connections currently in use",
		},
	)

	databaseConnectionsIdle = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "database_connections_idle",
			Help: "Number of idle database connections",
		},
	)

	databaseQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "database_query_duration_seconds",
			Help:    "Duration of database queries in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"operation"},
	)

	// Application-specific metrics
	activeConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_connections",
			Help: "Number of active connections to the service",
		},
	)

	submissionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "submissions_total",
			Help: "Total number of submissions processed",
		},
		[]string{"status", "language"},
	)

	realtimeConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "realtime_connections",
			Help: "Number of active real-time connections (SSE)",
		},
	)
)

func init() {
	// Register all metrics with Prometheus
	prometheus.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		judgeQueueSize,
		judgeTasksProcessed,
		judgeTaskDuration,
		databaseConnectionsInUse,
		databaseConnectionsIdle,
		databaseQueryDuration,
		activeConnections,
		submissionsTotal,
		realtimeConnections,
	)
}

// MetricsHandler returns a Prometheus HTTP handler for metrics endpoint
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// HTTPMiddleware creates a middleware that records HTTP request metrics
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Wrap the ResponseWriter to capture status code
		wrapper := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		// Process the request
		next.ServeHTTP(wrapper, r)
		
		// Record metrics
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(wrapper.statusCode)
		
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// JudgeMetrics provides methods to update judge-related metrics
type JudgeMetrics struct{}

func NewJudgeMetrics() *JudgeMetrics {
	return &JudgeMetrics{}
}

func (jm *JudgeMetrics) SetQueueSize(queueName string, size int) {
	judgeQueueSize.WithLabelValues(queueName).Set(float64(size))
}

func (jm *JudgeMetrics) IncrementTasksProcessed(queueName, status string) {
	judgeTasksProcessed.WithLabelValues(queueName, status).Inc()
}

func (jm *JudgeMetrics) ObserveTaskDuration(queueName string, duration time.Duration) {
	judgeTaskDuration.WithLabelValues(queueName).Observe(duration.Seconds())
}

// DatabaseMetrics provides methods to update database-related metrics
type DatabaseMetrics struct{}

func NewDatabaseMetrics() *DatabaseMetrics {
	return &DatabaseMetrics{}
}

func (dm *DatabaseMetrics) SetConnectionsInUse(count int) {
	databaseConnectionsInUse.Set(float64(count))
}

func (dm *DatabaseMetrics) SetConnectionsIdle(count int) {
	databaseConnectionsIdle.Set(float64(count))
}

func (dm *DatabaseMetrics) ObserveQueryDuration(operation string, duration time.Duration) {
	databaseQueryDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// ApplicationMetrics provides methods to update application-specific metrics
type ApplicationMetrics struct{}

func NewApplicationMetrics() *ApplicationMetrics {
	return &ApplicationMetrics{}
}

func (am *ApplicationMetrics) SetActiveConnections(count int) {
	activeConnections.Set(float64(count))
}

func (am *ApplicationMetrics) IncrementSubmissions(status, language string) {
	submissionsTotal.WithLabelValues(status, language).Inc()
}

func (am *ApplicationMetrics) SetRealtimeConnections(count int) {
	realtimeConnections.Set(float64(count))
}