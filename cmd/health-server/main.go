package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"competitive-programming-platform/internal/judge"
	"competitive-programming-platform/internal/metrics"
)

// HealthStatus represents the health status of the judge worker
type HealthStatus struct {
	Status      string            `json:"status"`
	Timestamp   time.Time         `json:"timestamp"`
	Version     string            `json:"version"`
	Uptime      time.Duration     `json:"uptime"`
	System      SystemInfo        `json:"system"`
	Sandbox     SandboxInfo       `json:"sandbox"`
	Resources   ResourceInfo      `json:"resources"`
	Checks      map[string]string `json:"checks"`
}

// SystemInfo represents system information
type SystemInfo struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	NumCPU       int    `json:"num_cpu"`
	GoVersion    string `json:"go_version"`
}

// SandboxInfo represents sandbox status
type SandboxInfo struct {
	IsolateAvailable bool   `json:"isolate_available"`
	IsolateVersion   string `json:"isolate_version"`
	DockerAvailable  bool   `json:"docker_available"`
	SandboxType      string `json:"sandbox_type"`
}

// ResourceInfo represents resource usage
type ResourceInfo struct {
	MemoryUsage  runtime.MemStats `json:"memory_usage"`
	NumGoroutines int             `json:"num_goroutines"`
}

var startTime = time.Now()

func main() {
	port := os.Getenv("HEALTH_PORT")
	if port == "" {
		port = "8081"
	}

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/ready", readinessHandler)
	http.HandleFunc("/live", livenessHandler)
	http.Handle("/metrics", metrics.MetricsHandler())

	log.Printf("Health server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Health server failed:", err)
	}
}

// healthHandler provides comprehensive health information
func healthHandler(w http.ResponseWriter, r *http.Request) {
	status := getHealthStatus()
	
	w.Header().Set("Content-Type", "application/json")
	
	// Determine HTTP status code based on health
	statusCode := http.StatusOK
	if status.Status != "healthy" {
		statusCode = http.StatusServiceUnavailable
	}
	
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(status)
}

// readinessHandler checks if the service is ready to accept requests
func readinessHandler(w http.ResponseWriter, r *http.Request) {
	checks := performReadinessChecks()
	
	ready := true
	for _, check := range checks {
		if check != "ok" {
			ready = false
			break
		}
	}
	
	response := map[string]interface{}{
		"ready":     ready,
		"timestamp": time.Now(),
		"checks":    checks,
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	if ready {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	
	json.NewEncoder(w).Encode(response)
}

// livenessHandler checks if the service is alive
func livenessHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"alive":     true,
		"timestamp": time.Now(),
		"uptime":    time.Since(startTime).String(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// getHealthStatus returns comprehensive health status
func getHealthStatus() HealthStatus {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	checks := performHealthChecks()
	
	status := "healthy"
	for _, check := range checks {
		if check != "ok" {
			status = "unhealthy"
			break
		}
	}
	
	return HealthStatus{
		Status:    status,
		Timestamp: time.Now(),
		Version:   getVersion(),
		Uptime:    time.Since(startTime),
		System: SystemInfo{
			OS:           runtime.GOOS,
			Architecture: runtime.GOARCH,
			NumCPU:       runtime.NumCPU(),
			GoVersion:    runtime.Version(),
		},
		Sandbox: getSandboxInfo(),
		Resources: ResourceInfo{
			MemoryUsage:   memStats,
			NumGoroutines: runtime.NumGoroutine(),
		},
		Checks: checks,
	}
}

// performHealthChecks performs various health checks
func performHealthChecks() map[string]string {
	checks := make(map[string]string)
	
	// Check isolate availability
	if checkIsolateAvailable() {
		checks["isolate"] = "ok"
	} else {
		checks["isolate"] = "unavailable"
	}
	
	// Check Docker availability (if in dual-layer mode)
	if checkDockerAvailable() {
		checks["docker"] = "ok"
	} else {
		checks["docker"] = "unavailable"
	}
	
	// Check filesystem access
	if checkFilesystemAccess() {
		checks["filesystem"] = "ok"
	} else {
		checks["filesystem"] = "error"
	}
	
	// Check memory usage
	if checkMemoryUsage() {
		checks["memory"] = "ok"
	} else {
		checks["memory"] = "high"
	}
	
	// Check disk space
	if checkDiskSpace() {
		checks["disk_space"] = "ok"
	} else {
		checks["disk_space"] = "low"
	}
	
	return checks
}

// performReadinessChecks performs readiness checks
func performReadinessChecks() map[string]string {
	checks := make(map[string]string)
	
	// Essential components for readiness
	if checkIsolateAvailable() {
		checks["isolate_ready"] = "ok"
	} else {
		checks["isolate_ready"] = "not_ready"
	}
	
	if checkFilesystemAccess() {
		checks["filesystem_ready"] = "ok"
	} else {
		checks["filesystem_ready"] = "not_ready"
	}
	
	// Test sandbox initialization
	if testSandboxInitialization() {
		checks["sandbox_init"] = "ok"
	} else {
		checks["sandbox_init"] = "failed"
	}
	
	return checks
}

// getSandboxInfo returns sandbox information
func getSandboxInfo() SandboxInfo {
	return SandboxInfo{
		IsolateAvailable: checkIsolateAvailable(),
		IsolateVersion:   getIsolateVersion(),
		DockerAvailable:  checkDockerAvailable(),
		SandboxType:      getSandboxType(),
	}
}

// checkIsolateAvailable checks if isolate is available
func checkIsolateAvailable() bool {
	cmd := exec.Command("isolate", "--version")
	return cmd.Run() == nil
}

// checkDockerAvailable checks if Docker is available
func checkDockerAvailable() bool {
	cmd := exec.Command("docker", "--version")
	return cmd.Run() == nil
}

// checkFilesystemAccess checks filesystem access
func checkFilesystemAccess() bool {
	testFile := "/tmp/health-check-test"
	
	// Try to create a test file
	file, err := os.Create(testFile)
	if err != nil {
		return false
	}
	file.Close()
	
	// Try to remove the test file
	err = os.Remove(testFile)
	return err == nil
}

// checkMemoryUsage checks if memory usage is acceptable
func checkMemoryUsage() bool {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	// Consider unhealthy if using more than 80% of allocated memory
	usagePercent := float64(memStats.Alloc) / float64(memStats.Sys) * 100
	return usagePercent < 80.0
}

// checkDiskSpace checks available disk space
func checkDiskSpace() bool {
	// Simple check - ensure /tmp has some space
	testFile := "/tmp/disk-space-test"
	
	file, err := os.Create(testFile)
	if err != nil {
		return false
	}
	
	// Try to write 1MB
	data := make([]byte, 1024*1024)
	_, err = file.Write(data)
	file.Close()
	os.Remove(testFile)
	
	return err == nil
}

// testSandboxInitialization tests sandbox initialization
func testSandboxInitialization() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Try to initialize a test sandbox
	config := judge.SandboxConfig{
		BoxID:       255, // Use highest ID for testing
		TimeLimit:   time.Second,
		MemoryLimit: 64, // 64MB
		ProcessLimit: 10,
	}
	
	sandbox := judge.NewSandboxManager(config)
	
	if err := sandbox.InitializeSandbox(); err != nil {
		return false
	}
	
	defer sandbox.CleanupSandbox()
	
	// Try a simple execution test
	result, err := sandbox.ExecuteCommand("echo", []string{"test"}, "")
	if err != nil {
		return false
	}
	
	return result.ExitCode == 0
}

// getIsolateVersion returns isolate version
func getIsolateVersion() string {
	cmd := exec.Command("isolate", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

// getSandboxType returns the type of sandbox being used
func getSandboxType() string {
	if checkDockerAvailable() && checkIsolateAvailable() {
		return "dual-layer"
	} else if checkIsolateAvailable() {
		return "isolate-only"
	}
	return "none"
}

// getVersion returns the application version
func getVersion() string {
	version := os.Getenv("JUDGE_VERSION")
	if version == "" {
		return "unknown"
	}
	return version
}