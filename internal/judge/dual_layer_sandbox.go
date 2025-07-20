package judge

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-units"
	"github.com/google/uuid"
)

// DualLayerSandboxConfig represents configuration for the dual-layer sandbox
type DualLayerSandboxConfig struct {
	// Docker layer configuration
	DockerImage     string        `json:"docker_image"`
	ContainerName   string        `json:"container_name"`
	CPULimit        float64       `json:"cpu_limit"`        // CPU cores
	MemoryLimit     int64         `json:"memory_limit"`     // Memory in bytes
	NetworkMode     string        `json:"network_mode"`     // "none", "bridge", etc.
	ReadOnlyRootFS  bool          `json:"read_only_rootfs"`
	SecurityOpts    []string      `json:"security_opts"`
	
	// Isolate layer configuration  
	IsolateBoxID    int           `json:"isolate_box_id"`
	TimeLimit       time.Duration `json:"time_limit"`
	WallTimeLimit   time.Duration `json:"wall_time_limit"`
	IsolateMemLimit int           `json:"isolate_mem_limit"` // Memory in KB
	ProcessLimit    int           `json:"process_limit"`
	DiskQuota       int           `json:"disk_quota"`        // Disk quota in KB
	
	// Security features
	SeccompProfile  string        `json:"seccomp_profile"`
	UseAppArmor     bool          `json:"use_apparmor"`
	DropCapabilities []string     `json:"drop_capabilities"`
	NoNewPrivileges bool          `json:"no_new_privileges"`
	
	// Resource monitoring
	EnableCgroups   bool          `json:"enable_cgroups"`
	LogExecution    bool          `json:"log_execution"`
}

// DualLayerSandboxManager manages both Docker and Isolate layers
type DualLayerSandboxManager struct {
	config       DualLayerSandboxConfig
	dockerClient *client.Client
	containerID  string
	sandboxPath  string
	logger       SandboxLogger
}

// SandboxLogger provides logging for sandbox operations
type SandboxLogger interface {
	LogInfo(message string, args ...interface{})
	LogError(message string, args ...interface{})
	LogDebug(message string, args ...interface{})
}

// DefaultSandboxLogger provides a default logger implementation
type DefaultSandboxLogger struct{}

func (l *DefaultSandboxLogger) LogInfo(message string, args ...interface{}) {
	fmt.Printf("[INFO] "+message+"\n", args...)
}

func (l *DefaultSandboxLogger) LogError(message string, args ...interface{}) {
	fmt.Printf("[ERROR] "+message+"\n", args...)
}

func (l *DefaultSandboxLogger) LogDebug(message string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+message+"\n", args...)
}

// NewDualLayerSandboxManager creates a new dual-layer sandbox manager
func NewDualLayerSandboxManager(config DualLayerSandboxConfig) (*DualLayerSandboxManager, error) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	if config.ContainerName == "" {
		config.ContainerName = fmt.Sprintf("judge-sandbox-%s", uuid.New().String()[:8])
	}

	return &DualLayerSandboxManager{
		config:       config,
		dockerClient: dockerClient,
		logger:       &DefaultSandboxLogger{},
	}, nil
}

// SetLogger sets a custom logger
func (dsm *DualLayerSandboxManager) SetLogger(logger SandboxLogger) {
	dsm.logger = logger
}

// InitializeSandbox initializes both Docker and Isolate layers
func (dsm *DualLayerSandboxManager) InitializeSandbox(ctx context.Context) error {
	dsm.logger.LogInfo("Initializing dual-layer sandbox: %s", dsm.config.ContainerName)

	// Step 1: Create and start Docker container
	if err := dsm.createDockerContainer(ctx); err != nil {
		return fmt.Errorf("failed to create Docker container: %w", err)
	}

	if err := dsm.startDockerContainer(ctx); err != nil {
		dsm.cleanupDockerContainer(ctx) // Cleanup on failure
		return fmt.Errorf("failed to start Docker container: %w", err)
	}

	// Step 2: Initialize Isolate sandbox inside the container
	if err := dsm.initializeIsolateSandbox(ctx); err != nil {
		dsm.cleanupDockerContainer(ctx) // Cleanup on failure
		return fmt.Errorf("failed to initialize Isolate sandbox: %w", err)
	}

	dsm.logger.LogInfo("Dual-layer sandbox initialized successfully")
	return nil
}

// createDockerContainer creates the Docker container with security constraints
func (dsm *DualLayerSandboxManager) createDockerContainer(ctx context.Context) error {
	// Security options
	securityOpts := []string{
		"no-new-privileges:true",
	}

	if dsm.config.SeccompProfile != "" {
		securityOpts = append(securityOpts, fmt.Sprintf("seccomp:%s", dsm.config.SeccompProfile))
	}

	if dsm.config.UseAppArmor {
		securityOpts = append(securityOpts, "apparmor:docker-default")
	}

	securityOpts = append(securityOpts, dsm.config.SecurityOpts...)

	// Resource limits
	resources := container.Resources{
		Memory:   dsm.config.MemoryLimit,
		NanoCPUs: int64(dsm.config.CPULimit * 1e9), // Convert to nano CPUs
		PidsLimit: func() *int64 { limit := int64(dsm.config.ProcessLimit); return &limit }(),
	}

	// Drop capabilities
	capDrop := []string{
		"ALL", // Drop all capabilities first
	}
	
	// Only add back essential capabilities
	capAdd := []string{
		"SETUID",
		"SETGID", 
		"SYS_CHROOT",
	}

	hostConfig := &container.HostConfig{
		Resources:     resources,
		SecurityOpt:   securityOpts,
		NetworkMode:   container.NetworkMode(dsm.config.NetworkMode),
		ReadonlyRootfs: dsm.config.ReadOnlyRootFS,
		CapDrop:       capDrop,
		CapAdd:        capAdd,
		Tmpfs: map[string]string{
			"/tmp":     "rw,noexec,nosuid,size=100m",
			"/var/tmp": "rw,noexec,nosuid,size=100m",
		},
		Ulimits: []*units.Ulimit{
			{Name: "nproc", Soft: 64, Hard: 64},
			{Name: "fsize", Soft: 32 * 1024 * 1024, Hard: 32 * 1024 * 1024}, // 32MB
			{Name: "cpu", Soft: 30, Hard: 30}, // 30 seconds
		},
	}

	// Container configuration
	containerConfig := &container.Config{
		Image:        dsm.config.DockerImage,
		Cmd:          []string{"sleep", "3600"}, // Keep container alive
		WorkingDir:   "/tmp/sandbox",
		User:         "judge:judge",
		AttachStdout: false,
		AttachStderr: false,
		AttachStdin:  false,
		Tty:          false,
		Env: []string{
			"HOME=/tmp",
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			fmt.Sprintf("ISOLATE_BOX_ID=%d", dsm.config.IsolateBoxID),
		},
	}

	resp, err := dsm.dockerClient.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		nil,
		nil,
		dsm.config.ContainerName,
	)

	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	dsm.containerID = resp.ID
	dsm.logger.LogInfo("Docker container created: %s", dsm.containerID[:12])

	return nil
}

// startDockerContainer starts the Docker container
func (dsm *DualLayerSandboxManager) startDockerContainer(ctx context.Context) error {
	err := dsm.dockerClient.ContainerStart(ctx, dsm.containerID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	dsm.logger.LogInfo("Docker container started: %s", dsm.containerID[:12])
	return nil
}

// initializeIsolateSandbox initializes the Isolate sandbox inside the container
func (dsm *DualLayerSandboxManager) initializeIsolateSandbox(ctx context.Context) error {
	// Execute isolate --init inside the container
	cmd := []string{
		"isolate",
		"--box-id", strconv.Itoa(dsm.config.IsolateBoxID),
		"--init",
	}

	execConfig := types.ExecConfig{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := dsm.dockerClient.ContainerExecCreate(ctx, dsm.containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create exec instance: %w", err)
	}

	execStartConfig := types.ExecStartConfig{
		Detach: false,
	}

	resp, err := dsm.dockerClient.ContainerExecAttach(ctx, execID.ID, execStartConfig)
	if err != nil {
		return fmt.Errorf("failed to attach to exec instance: %w", err)
	}
	defer resp.Close()

	// Start the exec instance
	err = dsm.dockerClient.ContainerExecStart(ctx, execID.ID, execStartConfig)
	if err != nil {
		return fmt.Errorf("failed to start exec instance: %w", err)
	}

	// Check exit code
	inspectResp, err := dsm.dockerClient.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect exec instance: %w", err)
	}

	if inspectResp.ExitCode != 0 {
		return fmt.Errorf("isolate initialization failed with exit code: %d", inspectResp.ExitCode)
	}

	dsm.logger.LogInfo("Isolate sandbox initialized in container")
	return nil
}

// ExecuteCode executes code with dual-layer sandboxing
func (dsm *DualLayerSandboxManager) ExecuteCode(ctx context.Context, language, sourceCode, input string) (*ExecutionResult, error) {
	dsm.logger.LogInfo("Executing code in dual-layer sandbox")

	// Validate language support
	langConfig, exists := SupportedLanguages[language]
	if !exists {
		return &ExecutionResult{
			Status:  "CE",
			Message: fmt.Sprintf("Unsupported language: %s", language),
		}, nil
	}

	// Write source code to container
	sourceFile := "main" + langConfig.FileExtension
	if err := dsm.writeFileToSandbox(ctx, sourceFile, sourceCode); err != nil {
		return &ExecutionResult{
			Status:  "IE",
			Message: fmt.Sprintf("Failed to write source file: %v", err),
		}, nil
	}

	// Compile if needed
	if langConfig.CompileCommand != "" {
		compileResult, err := dsm.compileInSandbox(ctx, langConfig, sourceFile)
		if err != nil || compileResult.ExitCode != 0 {
			return &ExecutionResult{
				Status:  "CE",
				Message: fmt.Sprintf("Compilation failed: %s", compileResult.Stderr),
				Stderr:  compileResult.Stderr,
			}, nil
		}
	}

	// Execute with both layers of security
	return dsm.executeInSandbox(ctx, langConfig, sourceFile, input)
}

// writeFileToSandbox writes a file to the sandbox
func (dsm *DualLayerSandboxManager) writeFileToSandbox(ctx context.Context, filename, content string) error {
	// Create a temporary file on host
	tmpFile, err := ioutil.TempFile("", "sandbox-file-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write content to temporary file
	if _, err := tmpFile.WriteString(content); err != nil {
		return fmt.Errorf("failed to write to temporary file: %w", err)
	}
	tmpFile.Close()

	// Copy file to container using docker cp
	cmd := exec.Command("docker", "cp", tmpFile.Name(), fmt.Sprintf("%s:/tmp/sandbox/%s", dsm.containerID, filename))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy file to container: %w", err)
	}

	return nil
}

// compileInSandbox compiles code inside the sandbox
func (dsm *DualLayerSandboxManager) compileInSandbox(ctx context.Context, langConfig Language, sourceFile string) (*ExecutionResult, error) {
	compileCmd := langConfig.CompileCommand
	compileCmd = replacePlaceholders(compileCmd, map[string]string{
		"{source}": sourceFile,
		"{output}": "program",
	})

	parts := splitCommand(compileCmd)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty compile command")
	}

	// Build isolate command for compilation
	isolateArgs := []string{
		"isolate",
		"--box-id", strconv.Itoa(dsm.config.IsolateBoxID),
		"--time", "10", // 10 seconds for compilation
		"--wall-time", "20",
		"--memory", "512000", // 512MB for compilation
		"--processes", "10",
		"--quota", fmt.Sprintf("%d,1000", dsm.config.DiskQuota), // blocks, inodes
		"--run",
	}

	isolateArgs = append(isolateArgs, parts...)

	return dsm.executeInContainer(ctx, isolateArgs, "")
}

// executeInSandbox executes the program inside the sandbox
func (dsm *DualLayerSandboxManager) executeInSandbox(ctx context.Context, langConfig Language, sourceFile, input string) (*ExecutionResult, error) {
	runCmd := langConfig.RunCommand
	runCmd = replacePlaceholders(runCmd, map[string]string{
		"{source}": sourceFile,
		"{output}": "program",
		"{class}":  getClassNameFromFile(sourceFile),
	})

	parts := splitCommand(runCmd)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty run command")
	}

	// Build isolate command for execution
	isolateArgs := []string{
		"isolate",
		"--box-id", strconv.Itoa(dsm.config.IsolateBoxID),
		"--time", fmt.Sprintf("%.3f", dsm.config.TimeLimit.Seconds()),
		"--wall-time", fmt.Sprintf("%.3f", dsm.config.WallTimeLimit.Seconds()),
		"--memory", strconv.Itoa(dsm.config.IsolateMemLimit),
		"--processes", strconv.Itoa(dsm.config.ProcessLimit),
		"--quota", fmt.Sprintf("%d,1000", dsm.config.DiskQuota),
		"--meta", "/tmp/meta.txt",
		"--run",
	}

	isolateArgs = append(isolateArgs, parts...)

	result, err := dsm.executeInContainer(ctx, isolateArgs, input)
	if err != nil {
		return result, err
	}

	// Parse meta file for accurate resource usage
	if err := dsm.parseMetaFile(ctx, result); err != nil {
		dsm.logger.LogError("Failed to parse meta file: %v", err)
	}

	return result, nil
}

// executeInContainer executes a command inside the Docker container
func (dsm *DualLayerSandboxManager) executeInContainer(ctx context.Context, cmd []string, input string) (*ExecutionResult, error) {
	execConfig := types.ExecConfig{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  input != "",
	}

	execID, err := dsm.dockerClient.ContainerExecCreate(ctx, dsm.containerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec instance: %w", err)
	}

	execStartConfig := types.ExecStartConfig{
		Detach: false,
	}

	resp, err := dsm.dockerClient.ContainerExecAttach(ctx, execID.ID, execStartConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to attach to exec instance: %w", err)
	}
	defer resp.Close()

	// Write input if provided
	if input != "" {
		if _, err := resp.Conn.Write([]byte(input)); err != nil {
			return nil, fmt.Errorf("failed to write input: %w", err)
		}
		resp.CloseWrite()
	}

	// Start execution
	if err := dsm.dockerClient.ContainerExecStart(ctx, execID.ID, execStartConfig); err != nil {
		return nil, fmt.Errorf("failed to start exec instance: %w", err)
	}

	// Read output
	output, err := ioutil.ReadAll(resp.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read output: %w", err)
	}

	// Get exit code
	inspectResp, err := dsm.dockerClient.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect exec instance: %w", err)
	}

	result := &ExecutionResult{
		ExitCode: inspectResp.ExitCode,
		Stdout:   string(output),
		Status:   "OK",
	}

	if inspectResp.ExitCode != 0 {
		result.Status = "RE"
		result.Stderr = string(output)
	}

	return result, nil
}

// parseMetaFile parses the isolate meta file for resource usage information
func (dsm *DualLayerSandboxManager) parseMetaFile(ctx context.Context, result *ExecutionResult) error {
	// Read meta file from container
	cmd := []string{"cat", "/tmp/meta.txt"}
	metaResult, err := dsm.executeInContainer(ctx, cmd, "")
	if err != nil {
		return err
	}

	// Parse meta file content
	lines := strings.Split(metaResult.Stdout, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "time:") {
			if timeStr := strings.TrimPrefix(line, "time:"); timeStr != "" {
				if timeFloat, err := strconv.ParseFloat(timeStr, 64); err == nil {
					result.TimeUsed = time.Duration(timeFloat * float64(time.Second))
				}
			}
		} else if strings.HasPrefix(line, "max-rss:") {
			if memStr := strings.TrimPrefix(line, "max-rss:"); memStr != "" {
				if memInt, err := strconv.Atoi(memStr); err == nil {
					result.MemoryUsed = memInt
				}
			}
		} else if strings.HasPrefix(line, "status:") {
			statusStr := strings.TrimPrefix(line, "status:")
			switch statusStr {
			case "TO":
				result.Status = "TLE" // Time Limit Exceeded
			case "SG":
				result.Status = "RTE" // Runtime Error (Signal)
			case "RE":
				result.Status = "RTE" // Runtime Error
			case "XX":
				result.Status = "IE"  // Internal Error
			}
		}
	}

	return nil
}

// CleanupSandbox cleans up both Docker and Isolate layers
func (dsm *DualLayerSandboxManager) CleanupSandbox(ctx context.Context) error {
	dsm.logger.LogInfo("Cleaning up dual-layer sandbox")

	// Cleanup Isolate sandbox first
	if err := dsm.cleanupIsolateSandbox(ctx); err != nil {
		dsm.logger.LogError("Failed to cleanup Isolate sandbox: %v", err)
	}

	// Cleanup Docker container
	if err := dsm.cleanupDockerContainer(ctx); err != nil {
		dsm.logger.LogError("Failed to cleanup Docker container: %v", err)
		return err
	}

	dsm.logger.LogInfo("Dual-layer sandbox cleanup completed")
	return nil
}

// cleanupIsolateSandbox cleans up the Isolate sandbox
func (dsm *DualLayerSandboxManager) cleanupIsolateSandbox(ctx context.Context) error {
	cmd := []string{
		"isolate",
		"--box-id", strconv.Itoa(dsm.config.IsolateBoxID),
		"--cleanup",
	}

	_, err := dsm.executeInContainer(ctx, cmd, "")
	return err
}

// cleanupDockerContainer stops and removes the Docker container
func (dsm *DualLayerSandboxManager) cleanupDockerContainer(ctx context.Context) error {
	if dsm.containerID == "" {
		return nil
	}

	// Stop container
	timeout := 10 * time.Second
	if err := dsm.dockerClient.ContainerStop(ctx, dsm.containerID, &timeout); err != nil {
		dsm.logger.LogError("Failed to stop container: %v", err)
	}

	// Remove container
	removeOptions := types.ContainerRemoveOptions{
		Force: true,
	}

	if err := dsm.dockerClient.ContainerRemove(ctx, dsm.containerID, removeOptions); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	dsm.containerID = ""
	return nil
}

// GetSandboxInfo returns information about the sandbox
func (dsm *DualLayerSandboxManager) GetSandboxInfo(ctx context.Context) (map[string]interface{}, error) {
	info := map[string]interface{}{
		"type":           "dual-layer",
		"docker_image":   dsm.config.DockerImage,
		"container_id":   dsm.containerID,
		"isolate_box_id": dsm.config.IsolateBoxID,
		"memory_limit":   dsm.config.MemoryLimit,
		"cpu_limit":      dsm.config.CPULimit,
		"time_limit":     dsm.config.TimeLimit.String(),
	}

	if dsm.containerID != "" {
		containerInfo, err := dsm.dockerClient.ContainerInspect(ctx, dsm.containerID)
		if err == nil {
			info["container_status"] = containerInfo.State.Status
			info["container_created"] = containerInfo.Created
		}
	}

	return info, nil
}

// DefaultDualLayerSandboxConfig returns a default configuration
func DefaultDualLayerSandboxConfig() DualLayerSandboxConfig {
	return DualLayerSandboxConfig{
		DockerImage:      "judge-worker:latest",
		CPULimit:         1.0, // 1 CPU core
		MemoryLimit:      512 * 1024 * 1024, // 512MB
		NetworkMode:      "none",
		ReadOnlyRootFS:   true,
		IsolateBoxID:     0,
		TimeLimit:        5 * time.Second,
		WallTimeLimit:    10 * time.Second,
		IsolateMemLimit:  256 * 1024, // 256MB in KB
		ProcessLimit:     32,
		DiskQuota:        10 * 1024, // 10MB in KB
		SeccompProfile:   "/opt/judge/seccomp-profile.json",
		UseAppArmor:      true,
		DropCapabilities: []string{"ALL"},
		NoNewPrivileges:  true,
		EnableCgroups:    true,
		LogExecution:     true,
		SecurityOpts: []string{
			"no-new-privileges:true",
		},
	}
}