package judge

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// ResourceMonitor monitors system resources and security violations
type ResourceMonitor struct {
	config    MonitorConfig
	logger    SandboxLogger
	mu        sync.RWMutex
	running   bool
	stopChan  chan struct{}
	
	// Metrics
	metrics   ResourceMetrics
	violations []SecurityViolation
}

// MonitorConfig represents resource monitoring configuration
type MonitorConfig struct {
	MonitorInterval     time.Duration `json:"monitor_interval"`
	AlertThresholds     AlertThresholds `json:"alert_thresholds"`
	EnableMemoryMonitor bool          `json:"enable_memory_monitor"`
	EnableCPUMonitor    bool          `json:"enable_cpu_monitor"`
	EnableDiskMonitor   bool          `json:"enable_disk_monitor"`
	EnableNetworkMonitor bool         `json:"enable_network_monitor"`
	EnableSecurityMonitor bool        `json:"enable_security_monitor"`
	MaxViolationHistory int           `json:"max_violation_history"`
	MetricsRetention    time.Duration `json:"metrics_retention"`
}

// AlertThresholds defines thresholds for resource alerts
type AlertThresholds struct {
	MemoryUsagePercent    float64 `json:"memory_usage_percent"`
	CPUUsagePercent       float64 `json:"cpu_usage_percent"`
	DiskUsagePercent      float64 `json:"disk_usage_percent"`
	NetworkBytesPerSecond int64   `json:"network_bytes_per_second"`
	ProcessCount          int     `json:"process_count"`
	FileDescriptorCount   int     `json:"file_descriptor_count"`
}

// ResourceMetrics represents current resource usage metrics
type ResourceMetrics struct {
	Timestamp       time.Time   `json:"timestamp"`
	MemoryUsage     MemoryStats `json:"memory_usage"`
	CPUUsage        CPUStats    `json:"cpu_usage"`
	DiskUsage       DiskStats   `json:"disk_usage"`
	NetworkUsage    NetworkStats `json:"network_usage"`
	ProcessStats    ProcessStats `json:"process_stats"`
	SecurityStats   SecurityStats `json:"security_stats"`
}

// MemoryStats represents memory usage statistics
type MemoryStats struct {
	TotalBytes     int64   `json:"total_bytes"`
	UsedBytes      int64   `json:"used_bytes"`
	FreeBytes      int64   `json:"free_bytes"`
	CachedBytes    int64   `json:"cached_bytes"`
	BufferedBytes  int64   `json:"buffered_bytes"`
	UsagePercent   float64 `json:"usage_percent"`
	SwapUsedBytes  int64   `json:"swap_used_bytes"`
	SwapTotalBytes int64   `json:"swap_total_bytes"`
}

// CPUStats represents CPU usage statistics
type CPUStats struct {
	UsagePercent    float64 `json:"usage_percent"`
	UserPercent     float64 `json:"user_percent"`
	SystemPercent   float64 `json:"system_percent"`
	IdlePercent     float64 `json:"idle_percent"`
	IOWaitPercent   float64 `json:"iowait_percent"`
	LoadAverage1m   float64 `json:"load_average_1m"`
	LoadAverage5m   float64 `json:"load_average_5m"`
	LoadAverage15m  float64 `json:"load_average_15m"`
}

// DiskStats represents disk usage statistics
type DiskStats struct {
	TotalBytes     int64   `json:"total_bytes"`
	UsedBytes      int64   `json:"used_bytes"`
	FreeBytes      int64   `json:"free_bytes"`
	UsagePercent   float64 `json:"usage_percent"`
	InodesTotal    int64   `json:"inodes_total"`
	InodesUsed     int64   `json:"inodes_used"`
	InodesFree     int64   `json:"inodes_free"`
	InodesPercent  float64 `json:"inodes_percent"`
}

// NetworkStats represents network usage statistics
type NetworkStats struct {
	BytesReceived    int64 `json:"bytes_received"`
	BytesSent        int64 `json:"bytes_sent"`
	PacketsReceived  int64 `json:"packets_received"`
	PacketsSent      int64 `json:"packets_sent"`
	ErrorsReceived   int64 `json:"errors_received"`
	ErrorsSent       int64 `json:"errors_sent"`
	DroppedReceived  int64 `json:"dropped_received"`
	DroppedSent      int64 `json:"dropped_sent"`
}

// ProcessStats represents process statistics
type ProcessStats struct {
	TotalProcesses    int `json:"total_processes"`
	RunningProcesses  int `json:"running_processes"`
	SleepingProcesses int `json:"sleeping_processes"`
	ZombieProcesses   int `json:"zombie_processes"`
	StoppedProcesses  int `json:"stopped_processes"`
	FileDescriptors   int `json:"file_descriptors"`
}

// SecurityStats represents security-related statistics
type SecurityStats struct {
	SyscallViolations    int `json:"syscall_violations"`
	CapabilityViolations int `json:"capability_violations"`
	NamespaceViolations  int `json:"namespace_violations"`
	QuotaViolations      int `json:"quota_violations"`
	PermissionViolations int `json:"permission_violations"`
}

// SecurityViolation represents a security violation event
type SecurityViolation struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`        // "syscall", "capability", "quota", etc.
	Severity    string    `json:"severity"`    // "low", "medium", "high", "critical"
	Description string    `json:"description"`
	ProcessID   int       `json:"process_id"`
	UserID      int       `json:"user_id"`
	Details     map[string]interface{} `json:"details"`
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor(config MonitorConfig) *ResourceMonitor {
	return &ResourceMonitor{
		config:     config,
		logger:     &DefaultSandboxLogger{},
		stopChan:   make(chan struct{}),
		violations: make([]SecurityViolation, 0),
	}
}

// SetLogger sets a custom logger
func (rm *ResourceMonitor) SetLogger(logger SandboxLogger) {
	rm.logger = logger
}

// Start starts the resource monitoring
func (rm *ResourceMonitor) Start(ctx context.Context) error {
	rm.mu.Lock()
	if rm.running {
		rm.mu.Unlock()
		return fmt.Errorf("resource monitor is already running")
	}
	rm.running = true
	rm.mu.Unlock()

	rm.logger.LogInfo("Starting resource monitor with interval %v", rm.config.MonitorInterval)

	go rm.monitoringLoop(ctx)
	return nil
}

// Stop stops the resource monitoring
func (rm *ResourceMonitor) Stop() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.running {
		rm.running = false
		close(rm.stopChan)
		rm.logger.LogInfo("Resource monitor stopped")
	}
}

// monitoringLoop runs the main monitoring loop
func (rm *ResourceMonitor) monitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(rm.config.MonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-rm.stopChan:
			return
		case <-ticker.C:
			if err := rm.collectMetrics(); err != nil {
				rm.logger.LogError("Failed to collect metrics: %v", err)
			}
			rm.checkAlerts()
		}
	}
}

// collectMetrics collects current resource metrics
func (rm *ResourceMonitor) collectMetrics() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	metrics := ResourceMetrics{
		Timestamp: time.Now(),
	}

	var err error

	// Collect memory metrics
	if rm.config.EnableMemoryMonitor {
		metrics.MemoryUsage, err = rm.collectMemoryStats()
		if err != nil {
			rm.logger.LogError("Failed to collect memory stats: %v", err)
		}
	}

	// Collect CPU metrics
	if rm.config.EnableCPUMonitor {
		metrics.CPUUsage, err = rm.collectCPUStats()
		if err != nil {
			rm.logger.LogError("Failed to collect CPU stats: %v", err)
		}
	}

	// Collect disk metrics
	if rm.config.EnableDiskMonitor {
		metrics.DiskUsage, err = rm.collectDiskStats()
		if err != nil {
			rm.logger.LogError("Failed to collect disk stats: %v", err)
		}
	}

	// Collect network metrics
	if rm.config.EnableNetworkMonitor {
		metrics.NetworkUsage, err = rm.collectNetworkStats()
		if err != nil {
			rm.logger.LogError("Failed to collect network stats: %v", err)
		}
	}

	// Collect process metrics
	metrics.ProcessStats, err = rm.collectProcessStats()
	if err != nil {
		rm.logger.LogError("Failed to collect process stats: %v", err)
	}

	// Collect security metrics
	if rm.config.EnableSecurityMonitor {
		metrics.SecurityStats, err = rm.collectSecurityStats()
		if err != nil {
			rm.logger.LogError("Failed to collect security stats: %v", err)
		}
	}

	rm.metrics = metrics
	return nil
}

// collectMemoryStats collects memory usage statistics
func (rm *ResourceMonitor) collectMemoryStats() (MemoryStats, error) {
	data, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		return MemoryStats{}, err
	}

	stats := MemoryStats{}
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}

		// Convert from KB to bytes
		value *= 1024

		switch key {
		case "MemTotal":
			stats.TotalBytes = value
		case "MemFree":
			stats.FreeBytes = value
		case "Cached":
			stats.CachedBytes = value
		case "Buffers":
			stats.BufferedBytes = value
		case "SwapTotal":
			stats.SwapTotalBytes = value
		case "SwapFree":
			swapFree := value
			stats.SwapUsedBytes = stats.SwapTotalBytes - swapFree
		}
	}

	stats.UsedBytes = stats.TotalBytes - stats.FreeBytes - stats.CachedBytes - stats.BufferedBytes
	if stats.TotalBytes > 0 {
		stats.UsagePercent = float64(stats.UsedBytes) / float64(stats.TotalBytes) * 100
	}

	return stats, nil
}

// collectCPUStats collects CPU usage statistics
func (rm *ResourceMonitor) collectCPUStats() (CPUStats, error) {
	data, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return CPUStats{}, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return CPUStats{}, fmt.Errorf("empty /proc/stat")
	}

	// Parse first line (overall CPU stats)
	fields := strings.Fields(lines[0])
	if len(fields) < 8 || fields[0] != "cpu" {
		return CPUStats{}, fmt.Errorf("invalid /proc/stat format")
	}

	var values []int64
	for i := 1; i < 8; i++ {
		val, err := strconv.ParseInt(fields[i], 10, 64)
		if err != nil {
			return CPUStats{}, err
		}
		values = append(values, val)
	}

	user := values[0]
	nice := values[1]
	system := values[2]
	idle := values[3]
	iowait := values[4]
	
	total := user + nice + system + idle + iowait
	if total == 0 {
		return CPUStats{}, nil
	}

	stats := CPUStats{
		UserPercent:   float64(user+nice) / float64(total) * 100,
		SystemPercent: float64(system) / float64(total) * 100,
		IdlePercent:   float64(idle) / float64(total) * 100,
		IOWaitPercent: float64(iowait) / float64(total) * 100,
	}

	stats.UsagePercent = 100 - stats.IdlePercent

	// Get load averages
	loadData, err := ioutil.ReadFile("/proc/loadavg")
	if err == nil {
		loadFields := strings.Fields(string(loadData))
		if len(loadFields) >= 3 {
			stats.LoadAverage1m, _ = strconv.ParseFloat(loadFields[0], 64)
			stats.LoadAverage5m, _ = strconv.ParseFloat(loadFields[1], 64)
			stats.LoadAverage15m, _ = strconv.ParseFloat(loadFields[2], 64)
		}
	}

	return stats, nil
}

// collectDiskStats collects disk usage statistics
func (rm *ResourceMonitor) collectDiskStats() (DiskStats, error) {
	// Get disk usage for /tmp (where judge operations happen)
	var stat unix.Statfs_t
	err := unix.Statfs("/tmp", &stat)
	if err != nil {
		return DiskStats{}, err
	}

	blockSize := stat.Bsize
	totalBytes := int64(stat.Blocks) * blockSize
	freeBytes := int64(stat.Bavail) * blockSize
	usedBytes := totalBytes - freeBytes

	stats := DiskStats{
		TotalBytes:    totalBytes,
		UsedBytes:     usedBytes,
		FreeBytes:     freeBytes,
		InodesTotal:   int64(stat.Files),
		InodesFree:    int64(stat.Ffree),
	}

	stats.InodesUsed = stats.InodesTotal - stats.InodesFree

	if totalBytes > 0 {
		stats.UsagePercent = float64(usedBytes) / float64(totalBytes) * 100
	}

	if stats.InodesTotal > 0 {
		stats.InodesPercent = float64(stats.InodesUsed) / float64(stats.InodesTotal) * 100
	}

	return stats, nil
}

// collectNetworkStats collects network usage statistics
func (rm *ResourceMonitor) collectNetworkStats() (NetworkStats, error) {
	data, err := ioutil.ReadFile("/proc/net/dev")
	if err != nil {
		return NetworkStats{}, err
	}

	lines := strings.Split(string(data), "\n")
	stats := NetworkStats{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, ":") {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		interface_name := strings.TrimSpace(parts[0])
		if interface_name == "lo" {
			continue // Skip loopback
		}

		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}

		// Parse receive stats
		bytesRx, _ := strconv.ParseInt(fields[0], 10, 64)
		packetsRx, _ := strconv.ParseInt(fields[1], 10, 64)
		errorsRx, _ := strconv.ParseInt(fields[2], 10, 64)
		droppedRx, _ := strconv.ParseInt(fields[3], 10, 64)

		// Parse transmit stats
		bytesTx, _ := strconv.ParseInt(fields[8], 10, 64)
		packetsTx, _ := strconv.ParseInt(fields[9], 10, 64)
		errorsTx, _ := strconv.ParseInt(fields[10], 10, 64)
		droppedTx, _ := strconv.ParseInt(fields[11], 10, 64)

		// Aggregate stats from all interfaces
		stats.BytesReceived += bytesRx
		stats.PacketsReceived += packetsRx
		stats.ErrorsReceived += errorsRx
		stats.DroppedReceived += droppedRx
		stats.BytesSent += bytesTx
		stats.PacketsSent += packetsTx
		stats.ErrorsSent += errorsTx
		stats.DroppedSent += droppedTx
	}

	return stats, nil
}

// collectProcessStats collects process statistics
func (rm *ResourceMonitor) collectProcessStats() (ProcessStats, error) {
	data, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return ProcessStats{}, err
	}

	lines := strings.Split(string(data), "\n")
	stats := ProcessStats{}

	for _, line := range lines {
		if strings.HasPrefix(line, "processes ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				stats.TotalProcesses, _ = strconv.Atoi(fields[1])
			}
		} else if strings.HasPrefix(line, "procs_running ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				stats.RunningProcesses, _ = strconv.Atoi(fields[1])
			}
		} else if strings.HasPrefix(line, "procs_blocked ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				// Add blocked processes to sleeping count
				blocked, _ := strconv.Atoi(fields[1])
				stats.SleepingProcesses += blocked
			}
		}
	}

	// Count file descriptors
	if fds, err := rm.countFileDescriptors(); err == nil {
		stats.FileDescriptors = fds
	}

	return stats, nil
}

// collectSecurityStats collects security-related statistics
func (rm *ResourceMonitor) collectSecurityStats() (SecurityStats, error) {
	stats := SecurityStats{}

	// This would integrate with audit logs, seccomp violations, etc.
	// For now, return basic stats based on violation history
	rm.mu.RLock()
	for _, violation := range rm.violations {
		switch violation.Type {
		case "syscall":
			stats.SyscallViolations++
		case "capability":
			stats.CapabilityViolations++
		case "namespace":
			stats.NamespaceViolations++
		case "quota":
			stats.QuotaViolations++
		case "permission":
			stats.PermissionViolations++
		}
	}
	rm.mu.RUnlock()

	return stats, nil
}

// countFileDescriptors counts open file descriptors
func (rm *ResourceMonitor) countFileDescriptors() (int, error) {
	entries, err := ioutil.ReadDir("/proc/self/fd")
	if err != nil {
		return 0, err
	}
	return len(entries), nil
}

// checkAlerts checks if any thresholds are exceeded
func (rm *ResourceMonitor) checkAlerts() {
	rm.mu.RLock()
	metrics := rm.metrics
	thresholds := rm.config.AlertThresholds
	rm.mu.RUnlock()

	// Memory alerts
	if rm.config.EnableMemoryMonitor && metrics.MemoryUsage.UsagePercent > thresholds.MemoryUsagePercent {
		rm.triggerAlert("memory", "high",
			fmt.Sprintf("Memory usage %.1f%% exceeds threshold %.1f%%",
				metrics.MemoryUsage.UsagePercent, thresholds.MemoryUsagePercent))
	}

	// CPU alerts
	if rm.config.EnableCPUMonitor && metrics.CPUUsage.UsagePercent > thresholds.CPUUsagePercent {
		rm.triggerAlert("cpu", "high",
			fmt.Sprintf("CPU usage %.1f%% exceeds threshold %.1f%%",
				metrics.CPUUsage.UsagePercent, thresholds.CPUUsagePercent))
	}

	// Disk alerts
	if rm.config.EnableDiskMonitor && metrics.DiskUsage.UsagePercent > thresholds.DiskUsagePercent {
		rm.triggerAlert("disk", "high",
			fmt.Sprintf("Disk usage %.1f%% exceeds threshold %.1f%%",
				metrics.DiskUsage.UsagePercent, thresholds.DiskUsagePercent))
	}

	// Process count alerts
	if metrics.ProcessStats.TotalProcesses > thresholds.ProcessCount {
		rm.triggerAlert("process", "medium",
			fmt.Sprintf("Process count %d exceeds threshold %d",
				metrics.ProcessStats.TotalProcesses, thresholds.ProcessCount))
	}

	// File descriptor alerts
	if metrics.ProcessStats.FileDescriptors > thresholds.FileDescriptorCount {
		rm.triggerAlert("fd", "medium",
			fmt.Sprintf("File descriptor count %d exceeds threshold %d",
				metrics.ProcessStats.FileDescriptors, thresholds.FileDescriptorCount))
	}
}

// triggerAlert triggers a security alert
func (rm *ResourceMonitor) triggerAlert(alertType, severity, description string) {
	violation := SecurityViolation{
		Timestamp:   time.Now(),
		Type:        alertType,
		Severity:    severity,
		Description: description,
		Details:     make(map[string]interface{}),
	}

	rm.mu.Lock()
	rm.violations = append(rm.violations, violation)

	// Limit violation history
	if len(rm.violations) > rm.config.MaxViolationHistory {
		rm.violations = rm.violations[len(rm.violations)-rm.config.MaxViolationHistory:]
	}
	rm.mu.Unlock()

	rm.logger.LogError("Security alert [%s]: %s", severity, description)
}

// GetCurrentMetrics returns the current resource metrics
func (rm *ResourceMonitor) GetCurrentMetrics() ResourceMetrics {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.metrics
}

// GetViolations returns recent security violations
func (rm *ResourceMonitor) GetViolations() []SecurityViolation {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	violations := make([]SecurityViolation, len(rm.violations))
	copy(violations, rm.violations)
	return violations
}

// IsRunning returns whether the monitor is currently running
func (rm *ResourceMonitor) IsRunning() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.running
}

// GetMonitorStatus returns the status of the resource monitor
func (rm *ResourceMonitor) GetMonitorStatus() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return map[string]interface{}{
		"running":           rm.running,
		"monitor_interval":  rm.config.MonitorInterval.String(),
		"violations_count":  len(rm.violations),
		"last_collection":   rm.metrics.Timestamp,
		"memory_monitoring": rm.config.EnableMemoryMonitor,
		"cpu_monitoring":    rm.config.EnableCPUMonitor,
		"disk_monitoring":   rm.config.EnableDiskMonitor,
		"network_monitoring": rm.config.EnableNetworkMonitor,
		"security_monitoring": rm.config.EnableSecurityMonitor,
	}
}

// DefaultMonitorConfig returns a default monitoring configuration
func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		MonitorInterval:       1 * time.Second,
		EnableMemoryMonitor:   true,
		EnableCPUMonitor:      true,
		EnableDiskMonitor:     true,
		EnableNetworkMonitor:  true,
		EnableSecurityMonitor: true,
		MaxViolationHistory:   100,
		MetricsRetention:      1 * time.Hour,
		AlertThresholds: AlertThresholds{
			MemoryUsagePercent:    90.0,
			CPUUsagePercent:       95.0,
			DiskUsagePercent:      85.0,
			NetworkBytesPerSecond: 100 * 1024 * 1024, // 100MB/s
			ProcessCount:          100,
			FileDescriptorCount:   1000,
		},
	}
}