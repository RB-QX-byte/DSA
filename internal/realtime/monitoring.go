package realtime

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// PerformanceMonitor tracks real-time performance metrics
type PerformanceMonitor struct {
	hub       *Hub
	metrics   *Metrics
	startTime time.Time
	mu        sync.RWMutex
}

// Metrics holds performance metrics
type Metrics struct {
	ConnectedClients      int64     `json:"connected_clients"`
	TotalConnections      int64     `json:"total_connections"`
	TotalDisconnections   int64     `json:"total_disconnections"`
	MessagesPerSecond     float64   `json:"messages_per_second"`
	TotalMessagesSent     int64     `json:"total_messages_sent"`
	MemoryUsageMB         float64   `json:"memory_usage_mb"`
	CPUUsagePercent       float64   `json:"cpu_usage_percent"`
	GoroutineCount        int       `json:"goroutine_count"`
	UpTime                string    `json:"uptime"`
	LastUpdated           time.Time `json:"last_updated"`
	
	// Connection metrics by contest
	ContestConnections    map[string]int64 `json:"contest_connections"`
	
	// Latency metrics
	AvgLatencyMs          float64   `json:"avg_latency_ms"`
	MaxLatencyMs          float64   `json:"max_latency_ms"`
	
	// Error metrics
	ConnectionErrors      int64     `json:"connection_errors"`
	MessageErrors         int64     `json:"message_errors"`
	
	// Throughput metrics
	ConnectionsPerSecond  float64   `json:"connections_per_second"`
	DisconnectionsPerSecond float64 `json:"disconnections_per_second"`
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(hub *Hub) *PerformanceMonitor {
	return &PerformanceMonitor{
		hub:       hub,
		startTime: time.Now(),
		metrics: &Metrics{
			ContestConnections: make(map[string]int64),
			LastUpdated:       time.Now(),
		},
	}
}

// Start begins monitoring performance metrics
func (pm *PerformanceMonitor) Start(ctx context.Context) {
	log.Println("Starting performance monitor")
	
	// Update metrics every second
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	// Track message counts
	var lastMessageCount int64
	var lastConnectionCount int64
	var lastDisconnectionCount int64
	
	for {
		select {
		case <-ticker.C:
			pm.updateMetrics(
				&lastMessageCount,
				&lastConnectionCount, 
				&lastDisconnectionCount,
			)
			
		case <-ctx.Done():
			log.Println("Performance monitor shutting down")
			return
		}
	}
}

// updateMetrics updates all performance metrics
func (pm *PerformanceMonitor) updateMetrics(
	lastMessageCount *int64,
	lastConnectionCount *int64,
	lastDisconnectionCount *int64,
) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	// System metrics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	pm.metrics.ConnectedClients = int64(pm.hub.GetClientCount())
	pm.metrics.MemoryUsageMB = float64(memStats.Alloc) / 1024 / 1024
	pm.metrics.GoroutineCount = runtime.NumGoroutine()
	pm.metrics.UpTime = time.Since(pm.startTime).String()
	pm.metrics.LastUpdated = time.Now()
	
	// Message throughput
	currentMessages := pm.metrics.TotalMessagesSent
	messagesThisSecond := float64(currentMessages - *lastMessageCount)
	pm.metrics.MessagesPerSecond = messagesThisSecond
	*lastMessageCount = currentMessages
	
	// Connection throughput
	connectionsThisSecond := float64(pm.metrics.TotalConnections - *lastConnectionCount)
	pm.metrics.ConnectionsPerSecond = connectionsThisSecond
	*lastConnectionCount = pm.metrics.TotalConnections
	
	disconnectionsThisSecond := float64(pm.metrics.TotalDisconnections - *lastDisconnectionCount)
	pm.metrics.DisconnectionsPerSecond = disconnectionsThisSecond
	*lastDisconnectionCount = pm.metrics.TotalDisconnections
	
	// Update contest-specific metrics
	pm.updateContestMetrics()
}

// updateContestMetrics updates per-contest connection metrics
func (pm *PerformanceMonitor) updateContestMetrics() {
	// This would need to be implemented based on how contests are tracked
	// For now, we'll use a simplified approach
	pm.metrics.ContestConnections = make(map[string]int64)
	
	// In a real implementation, you would iterate through connected clients
	// and group them by contest ID
}

// GetMetrics returns the current metrics
func (pm *PerformanceMonitor) GetMetrics() *Metrics {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	metricsCopy := *pm.metrics
	metricsCopy.ContestConnections = make(map[string]int64)
	for k, v := range pm.metrics.ContestConnections {
		metricsCopy.ContestConnections[k] = v
	}
	
	return &metricsCopy
}

// IncrementConnections increments the connection count
func (pm *PerformanceMonitor) IncrementConnections() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.metrics.TotalConnections++
}

// IncrementDisconnections increments the disconnection count
func (pm *PerformanceMonitor) IncrementDisconnections() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.metrics.TotalDisconnections++
}

// IncrementMessages increments the message count
func (pm *PerformanceMonitor) IncrementMessages() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.metrics.TotalMessagesSent++
}

// IncrementConnectionErrors increments the connection error count
func (pm *PerformanceMonitor) IncrementConnectionErrors() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.metrics.ConnectionErrors++
}

// IncrementMessageErrors increments the message error count
func (pm *PerformanceMonitor) IncrementMessageErrors() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.metrics.MessageErrors++
}

// UpdateLatency updates latency metrics
func (pm *PerformanceMonitor) UpdateLatency(latencyMs float64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	if latencyMs > pm.metrics.MaxLatencyMs {
		pm.metrics.MaxLatencyMs = latencyMs
	}
	
	// Simple moving average (in production, use proper statistical methods)
	if pm.metrics.AvgLatencyMs == 0 {
		pm.metrics.AvgLatencyMs = latencyMs
	} else {
		pm.metrics.AvgLatencyMs = (pm.metrics.AvgLatencyMs + latencyMs) / 2
	}
}

// GetMetricsHandler returns an HTTP handler for metrics
func (pm *PerformanceMonitor) GetMetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics := pm.GetMetrics()
		
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(metrics); err != nil {
			http.Error(w, "Failed to encode metrics", http.StatusInternalServerError)
			return
		}
	}
}

// GetHealthHandler returns an HTTP handler for health checks
func (pm *PerformanceMonitor) GetHealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics := pm.GetMetrics()
		
		// Determine health status based on metrics
		health := map[string]interface{}{
			"status":            "healthy",
			"timestamp":         time.Now(),
			"connected_clients": metrics.ConnectedClients,
			"memory_usage_mb":   metrics.MemoryUsageMB,
			"uptime":           metrics.UpTime,
		}
		
		// Add warnings if metrics indicate issues
		warnings := []string{}
		
		if metrics.MemoryUsageMB > 1000 { // More than 1GB
			warnings = append(warnings, "High memory usage")
		}
		
		if metrics.GoroutineCount > 10000 {
			warnings = append(warnings, "High goroutine count")
		}
		
		if metrics.ConnectionErrors > 100 {
			warnings = append(warnings, "High connection error rate")
		}
		
		if len(warnings) > 0 {
			health["status"] = "warning"
			health["warnings"] = warnings
		}
		
		// Set HTTP status based on health
		statusCode := http.StatusOK
		if health["status"] == "warning" {
			statusCode = http.StatusOK // Still OK, just with warnings
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(health)
	}
}

// Alert represents a performance alert
type Alert struct {
	Type        string    `json:"type"`
	Message     string    `json:"message"`
	Severity    string    `json:"severity"`
	Timestamp   time.Time `json:"timestamp"`
	MetricValue float64   `json:"metric_value"`
	Threshold   float64   `json:"threshold"`
}

// AlertManager manages performance alerts
type AlertManager struct {
	monitor   *PerformanceMonitor
	alerts    []Alert
	thresholds map[string]float64
	mu        sync.RWMutex
}

// NewAlertManager creates a new alert manager
func NewAlertManager(monitor *PerformanceMonitor) *AlertManager {
	return &AlertManager{
		monitor: monitor,
		alerts:  make([]Alert, 0),
		thresholds: map[string]float64{
			"memory_usage_mb":       2000,  // 2GB
			"goroutine_count":       15000, // 15k goroutines
			"connection_errors":     500,   // 500 errors
			"message_errors":        100,   // 100 message errors
			"avg_latency_ms":        1000,  // 1 second
			"connected_clients":     25000, // 25k clients
		},
	}
}

// CheckAlerts checks for alert conditions
func (am *AlertManager) CheckAlerts() {
	metrics := am.monitor.GetMetrics()
	
	am.checkThreshold("memory_usage_mb", metrics.MemoryUsageMB, "Memory usage exceeds threshold")
	am.checkThreshold("goroutine_count", float64(metrics.GoroutineCount), "Goroutine count exceeds threshold")
	am.checkThreshold("connection_errors", float64(metrics.ConnectionErrors), "Connection errors exceed threshold")
	am.checkThreshold("message_errors", float64(metrics.MessageErrors), "Message errors exceed threshold")
	am.checkThreshold("avg_latency_ms", metrics.AvgLatencyMs, "Average latency exceeds threshold")
	am.checkThreshold("connected_clients", float64(metrics.ConnectedClients), "Connected clients exceed threshold")
}

// checkThreshold checks a specific metric against its threshold
func (am *AlertManager) checkThreshold(metricName string, value float64, message string) {
	threshold, exists := am.thresholds[metricName]
	if !exists {
		return
	}
	
	if value > threshold {
		severity := "warning"
		if value > threshold*1.5 {
			severity = "critical"
		}
		
		alert := Alert{
			Type:        metricName,
			Message:     message,
			Severity:    severity,
			Timestamp:   time.Now(),
			MetricValue: value,
			Threshold:   threshold,
		}
		
		am.addAlert(alert)
	}
}

// addAlert adds an alert to the list
func (am *AlertManager) addAlert(alert Alert) {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	am.alerts = append(am.alerts, alert)
	
	// Keep only last 100 alerts
	if len(am.alerts) > 100 {
		am.alerts = am.alerts[len(am.alerts)-100:]
	}
	
	log.Printf("ALERT [%s]: %s (%.2f > %.2f)", 
		alert.Severity, alert.Message, alert.MetricValue, alert.Threshold)
}

// GetAlerts returns recent alerts
func (am *AlertManager) GetAlerts() []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	alertsCopy := make([]Alert, len(am.alerts))
	copy(alertsCopy, am.alerts)
	return alertsCopy
}

// GetAlertsHandler returns an HTTP handler for alerts
func (am *AlertManager) GetAlertsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		alerts := am.GetAlerts()
		
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(alerts); err != nil {
			http.Error(w, "Failed to encode alerts", http.StatusInternalServerError)
			return
		}
	}
}

// StartAlertMonitoring starts the alert monitoring loop
func (am *AlertManager) StartAlertMonitoring(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			am.CheckAlerts()
		case <-ctx.Done():
			return
		}
	}
}