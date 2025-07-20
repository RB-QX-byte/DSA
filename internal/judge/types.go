package judge

import (
	"time"
)

// SubmissionPayload represents the data sent to the judge worker
type SubmissionPayload struct {
	SubmissionID string `json:"submission_id"`
	UserID       string `json:"user_id"`
	ProblemID    string `json:"problem_id"`
	Language     string `json:"language"`
	SourceCode   string `json:"source_code"`
	TimeLimit    int    `json:"time_limit"`    // in seconds
	MemoryLimit  int    `json:"memory_limit"`  // in MB
}

// JudgeResult represents the result of judging a submission
type JudgeResult struct {
	SubmissionID string        `json:"submission_id"`
	Verdict      Verdict       `json:"verdict"`
	ExecutionTime time.Duration `json:"execution_time"`
	MemoryUsage  int           `json:"memory_usage"`   // in KB
	TestCasesRun int           `json:"test_cases_run"`
	TotalTestCases int         `json:"total_test_cases"`
	ErrorMessage string        `json:"error_message,omitempty"`
}

// Verdict represents the possible verdicts for a submission
type Verdict string

const (
	VerdictAccepted           Verdict = "AC"  // Accepted
	VerdictWrongAnswer        Verdict = "WA"  // Wrong Answer
	VerdictTimeLimitExceeded  Verdict = "TLE" // Time Limit Exceeded
	VerdictMemoryLimitExceeded Verdict = "MLE" // Memory Limit Exceeded
	VerdictRuntimeError       Verdict = "RE"  // Runtime Error
	VerdictCompilationError   Verdict = "CE"  // Compilation Error
	VerdictPending            Verdict = "PE"  // Pending
	VerdictInternalError      Verdict = "IE"  // Internal Error
)

// TestCase represents a single test case for a problem
type TestCase struct {
	ID       string `json:"id"`
	Input    string `json:"input"`
	Expected string `json:"expected"`
}

// SupportedLanguages defines the languages supported by the judge
var SupportedLanguages = map[string]Language{
	"cpp": {
		Name:           "C++",
		FileExtension:  ".cpp",
		CompileCommand: "g++ -o {output} {source} -std=c++17 -O2",
		RunCommand:     "./{output}",
	},
	"java": {
		Name:           "Java",
		FileExtension:  ".java",
		CompileCommand: "javac {source}",
		RunCommand:     "java {class}",
	},
	"python": {
		Name:           "Python",
		FileExtension:  ".py",
		CompileCommand: "", // No compilation needed
		RunCommand:     "python3 {source}",
	},
	"go": {
		Name:           "Go",
		FileExtension:  ".go",
		CompileCommand: "go build -o {output} {source}",
		RunCommand:     "./{output}",
	},
}

// Language represents configuration for a programming language
type Language struct {
	Name           string
	FileExtension  string
	CompileCommand string
	RunCommand     string
}