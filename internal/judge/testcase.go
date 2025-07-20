package judge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestCaseManager manages test cases for problems
type TestCaseManager struct {
	db *pgxpool.Pool
}

// NewTestCaseManager creates a new test case manager
func NewTestCaseManager(db *pgxpool.Pool) *TestCaseManager {
	return &TestCaseManager{db: db}
}

// GetTestCases retrieves all test cases for a problem
func (tcm *TestCaseManager) GetTestCases(ctx context.Context, problemID string) ([]TestCase, error) {
	query := `
		SELECT id, input_data, expected_output 
		FROM test_cases 
		WHERE problem_id = $1 
		ORDER BY is_sample DESC, created_at ASC
	`
	
	rows, err := tcm.db.Query(ctx, query, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to query test cases: %w", err)
	}
	defer rows.Close()
	
	var testCases []TestCase
	for rows.Next() {
		var tc TestCase
		if err := rows.Scan(&tc.ID, &tc.Input, &tc.Expected); err != nil {
			return nil, fmt.Errorf("failed to scan test case: %w", err)
		}
		testCases = append(testCases, tc)
	}
	
	return testCases, nil
}

// JudgeWithTestCases runs a submission against all test cases
func (tcm *TestCaseManager) JudgeWithTestCases(ctx context.Context, payload *SubmissionPayload, sandbox SandboxInterface) (*JudgeResult, error) {
	// Get test cases for the problem
	testCases, err := tcm.GetTestCases(ctx, payload.ProblemID)
	if err != nil {
		return &JudgeResult{
			SubmissionID: payload.SubmissionID,
			Verdict:      VerdictInternalError,
			ErrorMessage: fmt.Sprintf("Failed to get test cases: %v", err),
		}, nil
	}
	
	if len(testCases) == 0 {
		return &JudgeResult{
			SubmissionID: payload.SubmissionID,
			Verdict:      VerdictInternalError,
			ErrorMessage: "No test cases found for problem",
		}, nil
	}
	
	result := &JudgeResult{
		SubmissionID:   payload.SubmissionID,
		Verdict:        VerdictAccepted,
		TotalTestCases: len(testCases),
	}
	
	var totalTime time.Duration
	var maxMemory int
	
	// Run against each test case
	for i, testCase := range testCases {
		execResult, err := sandbox.CompileAndExecute(payload.Language, payload.SourceCode, testCase.Input)
		if err != nil {
			result.Verdict = VerdictInternalError
			result.ErrorMessage = fmt.Sprintf("Execution failed: %v", err)
			result.TestCasesRun = i
			return result, nil
		}
		
		// Update timing and memory stats
		totalTime += execResult.TimeUsed
		if execResult.MemoryUsed > maxMemory {
			maxMemory = execResult.MemoryUsed
		}
		
		// Check for compilation errors
		if execResult.Status == "CE" {
			result.Verdict = VerdictCompilationError
			result.ErrorMessage = execResult.Message
			result.TestCasesRun = i
			return result, nil
		}
		
		// Check for runtime errors
		if execResult.Status == "RE" {
			result.Verdict = VerdictRuntimeError
			result.ErrorMessage = execResult.Message
			result.TestCasesRun = i + 1
			return result, nil
		}
		
		// Check for time limit exceeded
		if execResult.Status == "TLE" || execResult.TimeUsed > time.Duration(payload.TimeLimit)*time.Millisecond {
			result.Verdict = VerdictTimeLimitExceeded
			result.ErrorMessage = "Time limit exceeded"
			result.TestCasesRun = i + 1
			return result, nil
		}
		
		// Check for memory limit exceeded
		if execResult.MemoryUsed > payload.MemoryLimit*1024 { // Convert MB to KB
			result.Verdict = VerdictMemoryLimitExceeded
			result.ErrorMessage = "Memory limit exceeded"
			result.TestCasesRun = i + 1
			return result, nil
		}
		
		// Check output
		if !compareOutput(execResult.Stdout, testCase.Expected) {
			result.Verdict = VerdictWrongAnswer
			result.ErrorMessage = fmt.Sprintf("Wrong answer on test case %d", i+1)
			result.TestCasesRun = i + 1
			return result, nil
		}
		
		result.TestCasesRun = i + 1
	}
	
	// All test cases passed
	result.ExecutionTime = totalTime
	result.MemoryUsage = maxMemory
	
	return result, nil
}

// compareOutput compares the actual output with expected output
func compareOutput(actual, expected string) bool {
	// Normalize whitespace
	actual = normalizeOutput(actual)
	expected = normalizeOutput(expected)
	
	return actual == expected
}

// normalizeOutput normalizes output by trimming whitespace and converting line endings
func normalizeOutput(output string) string {
	// Split into lines and trim each line
	lines := strings.Split(output, "\n")
	var normalizedLines []string
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			normalizedLines = append(normalizedLines, trimmed)
		}
	}
	
	return strings.Join(normalizedLines, "\n")
}

// CreateTestCase creates a new test case for a problem
func (tcm *TestCaseManager) CreateTestCase(ctx context.Context, problemID, input, expected string, isSample bool) error {
	query := `
		INSERT INTO test_cases (problem_id, input_data, expected_output, is_sample)
		VALUES ($1, $2, $3, $4)
	`
	
	_, err := tcm.db.Exec(ctx, query, problemID, input, expected, isSample)
	if err != nil {
		return fmt.Errorf("failed to create test case: %w", err)
	}
	
	return nil
}