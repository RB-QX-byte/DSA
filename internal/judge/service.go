package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// JudgeService handles judging operations
type JudgeService struct {
	db              *pgxpool.Pool
	queueManager    *QueueManager
	testCaseManager *TestCaseManager
}

// NewJudgeService creates a new judge service
func NewJudgeService(db *pgxpool.Pool, qm *QueueManager) *JudgeService {
	return &JudgeService{
		db:              db,
		queueManager:    qm,
		testCaseManager: NewTestCaseManager(db),
	}
}

// SubmitForJudging submits a solution for judging
func (js *JudgeService) SubmitForJudging(ctx context.Context, payload *SubmissionPayload) error {
	tracer := otel.Tracer("judge-service")
	ctx, span := tracer.Start(ctx, "judge.submit_for_judging")
	defer span.End()

	span.SetAttributes(
		attribute.String("submission.user_id", payload.UserID),
		attribute.String("submission.problem_id", payload.ProblemID),
		attribute.String("submission.language", payload.Language),
		attribute.Int("submission.time_limit", payload.TimeLimit),
		attribute.Int("submission.memory_limit", payload.MemoryLimit),
	)

	// First, create a submission record in the database
	submissionID, err := js.createSubmissionRecord(ctx, payload)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to create submission record: %w", err)
	}

	span.SetAttributes(attribute.String("submission.id", submissionID))

	// Update the payload with the database-generated submission ID
	payload.SubmissionID = submissionID

	// Enqueue the submission for judging
	if err := js.queueManager.EnqueueSubmission(ctx, payload); err != nil {
		// If enqueuing fails, update the submission status to error
		js.updateSubmissionStatus(ctx, submissionID, VerdictInternalError, "Failed to enqueue submission")
		span.RecordError(err)
		return fmt.Errorf("failed to enqueue submission: %w", err)
	}

	log.Printf("Submission %s queued for judging", submissionID)
	return nil
}

// HandleSubmissionTask handles a submission task from the queue
func (js *JudgeService) HandleSubmissionTask(ctx context.Context, t *asynq.Task) error {
	tracer := otel.Tracer("judge-service")
	ctx, span := tracer.Start(ctx, "judge.handle_submission")
	defer span.End()

	span.SetAttributes(
		attribute.String("queue.task_type", t.Type()),
		attribute.String("queue.task_id", t.ResultWriter().TaskID()),
	)

	var payload SubmissionPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to unmarshal submission payload: %w", err)
	}

	span.SetAttributes(
		attribute.String("submission.id", payload.SubmissionID),
		attribute.String("submission.user_id", payload.UserID),
		attribute.String("submission.problem_id", payload.ProblemID),
		attribute.String("submission.language", payload.Language),
	)

	log.Printf("Processing submission %s", payload.SubmissionID)

	// Update submission status to in progress
	if err := js.updateSubmissionStatus(ctx, payload.SubmissionID, VerdictPending, "Judging in progress"); err != nil {
		log.Printf("Failed to update submission status: %v", err)
		span.RecordError(err)
	}

	// Initialize sandbox
	sandboxConfig := SandboxConfig{
		BoxID:        0, // Use box ID 0 for now, in production this should be unique per worker
		TimeLimit:    time.Duration(payload.TimeLimit) * time.Millisecond,
		MemoryLimit:  payload.MemoryLimit,
		ProcessLimit: 1,
	}
	
	sandbox := NewSandbox(sandboxConfig)
	
	// Judge with test cases
	result, err := js.testCaseManager.JudgeWithTestCases(ctx, &payload, sandbox)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("judging failed: %v", err)
	}

	span.SetAttributes(
		attribute.String("submission.verdict", string(result.Verdict)),
		attribute.Int("submission.test_cases_run", result.TestCasesRun),
		attribute.Int("submission.total_test_cases", result.TotalTestCases),
		attribute.Int64("submission.execution_time_ms", result.ExecutionTime.Milliseconds()),
		attribute.Int("submission.memory_usage_kb", result.MemoryUsage),
	)

	// Update the submission record with the result
	if err := js.updateSubmissionResult(ctx, result); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update submission result: %w", err)
	}

	log.Printf("Submission %s judged successfully: %s", payload.SubmissionID, result.Verdict)
	return nil
}

// createSubmissionRecord creates a new submission record in the database
func (js *JudgeService) createSubmissionRecord(ctx context.Context, payload *SubmissionPayload) (string, error) {
	query := `
		INSERT INTO submissions (user_id, problem_id, language, source_code, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	
	var submissionID string
	err := js.db.QueryRow(ctx, query, 
		payload.UserID, 
		payload.ProblemID, 
		payload.Language, 
		payload.SourceCode, 
		VerdictPending, 
		time.Now(),
	).Scan(&submissionID)
	
	if err != nil {
		return "", fmt.Errorf("failed to create submission record: %w", err)
	}
	
	return submissionID, nil
}

// updateSubmissionStatus updates the status of a submission
func (js *JudgeService) updateSubmissionStatus(ctx context.Context, submissionID string, verdict Verdict, message string) error {
	query := `
		UPDATE submissions 
		SET status = $1, error_message = $2, updated_at = $3
		WHERE id = $4
	`
	
	_, err := js.db.Exec(ctx, query, verdict, message, time.Now(), submissionID)
	if err != nil {
		return fmt.Errorf("failed to update submission status: %w", err)
	}
	
	return nil
}

// updateSubmissionResult updates a submission with the judging result
func (js *JudgeService) updateSubmissionResult(ctx context.Context, result *JudgeResult) error {
	query := `
		UPDATE submissions 
		SET 
			status = $1,
			execution_time = $2,
			memory_usage = $3,
			test_cases_run = $4,
			total_test_cases = $5,
			error_message = $6,
			updated_at = $7
		WHERE id = $8
	`
	
	_, err := js.db.Exec(ctx, query,
		result.Verdict,
		result.ExecutionTime.Milliseconds(),
		result.MemoryUsage,
		result.TestCasesRun,
		result.TotalTestCases,
		result.ErrorMessage,
		time.Now(),
		result.SubmissionID,
	)
	
	if err != nil {
		return fmt.Errorf("failed to update submission result: %w", err)
	}
	
	return nil
}

// GetSubmission retrieves a submission by ID
func (js *JudgeService) GetSubmission(ctx context.Context, submissionID string) (*JudgeResult, error) {
	query := `
		SELECT id, status, execution_time, memory_usage, test_cases_run, total_test_cases, error_message
		FROM submissions
		WHERE id = $1
	`
	
	var result JudgeResult
	var executionTimeMs int64
	
	err := js.db.QueryRow(ctx, query, submissionID).Scan(
		&result.SubmissionID,
		&result.Verdict,
		&executionTimeMs,
		&result.MemoryUsage,
		&result.TestCasesRun,
		&result.TotalTestCases,
		&result.ErrorMessage,
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get submission: %w", err)
	}
	
	result.ExecutionTime = time.Duration(executionTimeMs) * time.Millisecond
	return &result, nil
}