package judge

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SandboxConfig represents configuration for the sandbox
type SandboxConfig struct {
	BoxID       int           // Isolate box ID
	TimeLimit   time.Duration // Time limit for execution
	MemoryLimit int           // Memory limit in MB
	ProcessLimit int          // Process limit
	WorkDir     string        // Working directory
}

// SandboxManager manages isolate sandbox operations
type SandboxManager struct {
	config SandboxConfig
}

// NewSandboxManager creates a new sandbox manager
func NewSandboxManager(config SandboxConfig) *SandboxManager {
	return &SandboxManager{
		config: config,
	}
}

// ExecutionResult represents the result of sandbox execution
type ExecutionResult struct {
	ExitCode     int
	TimeUsed     time.Duration
	MemoryUsed   int // in KB
	Stdout       string
	Stderr       string
	Status       string // OK, TO (timeout), SG (signal), RE (runtime error), XX (internal error)
	Message      string
}

// InitializeSandbox initializes the isolate sandbox
func (sm *SandboxManager) InitializeSandbox() error {
	cmd := exec.Command("isolate", "--box-id", strconv.Itoa(sm.config.BoxID), "--init")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to initialize sandbox (box %d): %v, output: %s", sm.config.BoxID, err, output)
	}
	
	return nil
}

// CleanupSandbox cleans up the isolate sandbox
func (sm *SandboxManager) CleanupSandbox() error {
	cmd := exec.Command("isolate", "--box-id", strconv.Itoa(sm.config.BoxID), "--cleanup")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to cleanup sandbox (box %d): %v, output: %s", sm.config.BoxID, err, output)
	}
	
	return nil
}

// GetSandboxPath returns the path to the sandbox directory
func (sm *SandboxManager) GetSandboxPath() (string, error) {
	cmd := exec.Command("isolate", "--box-id", strconv.Itoa(sm.config.BoxID), "--box-info")
	
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get sandbox path: %v", err)
	}
	
	// The output is the path to the sandbox directory
	path := string(output)
	path = path[:len(path)-1] // Remove trailing newline
	
	return path, nil
}

// WriteFile writes a file to the sandbox
func (sm *SandboxManager) WriteFile(filename, content string) error {
	sandboxPath, err := sm.GetSandboxPath()
	if err != nil {
		return err
	}
	
	boxDir := filepath.Join(sandboxPath, "box")
	filePath := filepath.Join(boxDir, filename)
	
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(boxDir, 0755); err != nil {
		return fmt.Errorf("failed to create sandbox directory: %v", err)
	}
	
	err = ioutil.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file to sandbox: %v", err)
	}
	
	return nil
}

// ReadFile reads a file from the sandbox
func (sm *SandboxManager) ReadFile(filename string) (string, error) {
	sandboxPath, err := sm.GetSandboxPath()
	if err != nil {
		return "", err
	}
	
	boxDir := filepath.Join(sandboxPath, "box")
	filePath := filepath.Join(boxDir, filename)
	
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file from sandbox: %v", err)
	}
	
	return string(content), nil
}

// ExecuteCommand executes a command in the sandbox
func (sm *SandboxManager) ExecuteCommand(command string, args []string, stdinContent string) (*ExecutionResult, error) {
	// Build isolate command
	isolateArgs := []string{
		"--box-id", strconv.Itoa(sm.config.BoxID),
		"--time", fmt.Sprintf("%.3f", sm.config.TimeLimit.Seconds()),
		"--wall-time", fmt.Sprintf("%.3f", sm.config.TimeLimit.Seconds()*2), // Wall time is usually 2x CPU time
		"--memory", strconv.Itoa(sm.config.MemoryLimit * 1024), // Convert MB to KB
		"--processes", strconv.Itoa(sm.config.ProcessLimit),
		"--run",
	}
	
	// Add the command and its arguments
	isolateArgs = append(isolateArgs, command)
	isolateArgs = append(isolateArgs, args...)
	
	cmd := exec.Command("isolate", isolateArgs...)
	
	// Set stdin if provided
	if stdinContent != "" {
		cmd.Stdin = stringReader(stdinContent)
	}
	
	// Execute the command
	output, err := cmd.CombinedOutput()
	
	result := &ExecutionResult{
		ExitCode: cmd.ProcessState.ExitCode(),
		Stdout:   string(output),
		Status:   "OK",
	}
	
	if err != nil {
		result.Status = "RE" // Runtime error
		result.Message = err.Error()
	}
	
	// Parse isolate output for additional information
	// This is a simplified version - in a real implementation, you'd parse the meta file
	if cmd.ProcessState.ExitCode() != 0 {
		result.Status = "RE"
	}
	
	return result, nil
}

// CompileAndExecute compiles and executes code in the sandbox
func (sm *SandboxManager) CompileAndExecute(language, sourceCode, input string) (*ExecutionResult, error) {
	// Initialize sandbox
	if err := sm.InitializeSandbox(); err != nil {
		return nil, err
	}
	defer sm.CleanupSandbox()
	
	langConfig, exists := SupportedLanguages[language]
	if !exists {
		return &ExecutionResult{
			Status:  "CE",
			Message: fmt.Sprintf("Unsupported language: %s", language),
		}, nil
	}
	
	// Write source code to sandbox
	sourceFile := "main" + langConfig.FileExtension
	if err := sm.WriteFile(sourceFile, sourceCode); err != nil {
		return &ExecutionResult{
			Status:  "IE",
			Message: fmt.Sprintf("Failed to write source file: %v", err),
		}, nil
	}
	
	// Compile if needed
	if langConfig.CompileCommand != "" {
		compileResult, err := sm.compile(langConfig, sourceFile)
		if err != nil || compileResult.ExitCode != 0 {
			return &ExecutionResult{
				Status:  "CE",
				Message: fmt.Sprintf("Compilation failed: %s", compileResult.Stderr),
				Stderr:  compileResult.Stderr,
			}, nil
		}
	}
	
	// Execute
	return sm.execute(langConfig, sourceFile, input)
}

// compile compiles the source code
func (sm *SandboxManager) compile(langConfig Language, sourceFile string) (*ExecutionResult, error) {
	// Parse compile command
	compileCmd := langConfig.CompileCommand
	compileCmd = replacePlaceholders(compileCmd, map[string]string{
		"{source}": sourceFile,
		"{output}": "program",
	})
	
	// Split command into parts
	parts := splitCommand(compileCmd)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty compile command")
	}
	
	// Execute compilation command with a timeout
	isolateArgs := []string{
		"--box-id", strconv.Itoa(sm.config.BoxID),
		"--time", "10", // 10 seconds for compilation
		"--wall-time", "20",
		"--memory", "512000", // 512MB for compilation
		"--processes", "10",
		"--run",
	}
	
	isolateArgs = append(isolateArgs, parts...)
	
	cmd := exec.Command("isolate", isolateArgs...)
	output, err := cmd.CombinedOutput()
	
	result := &ExecutionResult{
		ExitCode: cmd.ProcessState.ExitCode(),
		Stdout:   string(output),
		Status:   "OK",
	}
	
	if err != nil {
		result.Status = "CE"
		result.Message = err.Error()
		result.Stderr = string(output)
	}
	
	return result, nil
}

// execute executes the compiled program
func (sm *SandboxManager) execute(langConfig Language, sourceFile string, input string) (*ExecutionResult, error) {
	// Parse run command
	runCmd := langConfig.RunCommand
	runCmd = replacePlaceholders(runCmd, map[string]string{
		"{source}": sourceFile,
		"{output}": "program",
		"{class}":  getClassNameFromFile(sourceFile),
	})
	
	// Split command into parts
	parts := splitCommand(runCmd)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty run command")
	}
	
	return sm.ExecuteCommand(parts[0], parts[1:], input)
}

// Helper functions

func stringReader(s string) *stringReadCloser {
	return &stringReadCloser{s: s}
}

type stringReadCloser struct {
	s string
	i int
}

func (r *stringReadCloser) Read(p []byte) (int, error) {
	if r.i >= len(r.s) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, r.s[r.i:])
	r.i += n
	return n, nil
}

func (r *stringReadCloser) Close() error {
	return nil
}

func replacePlaceholders(cmd string, replacements map[string]string) string {
	for placeholder, replacement := range replacements {
		cmd = strings.Replace(cmd, placeholder, replacement, -1)
	}
	return cmd
}

func splitCommand(cmd string) []string {
	// Simple command splitting by spaces
	// In production, you'd want to handle quoted strings properly
	parts := strings.Fields(cmd)
	return parts
}

func getClassNameFromFile(filename string) string {
	// Extract class name from Java file
	base := filepath.Base(filename)
	return base[:len(base)-len(filepath.Ext(base))]
}