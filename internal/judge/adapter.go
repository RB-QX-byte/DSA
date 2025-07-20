package judge

import (
	"context"
)

// ProblemSubmissionPayload represents the data sent from problem service
type ProblemSubmissionPayload struct {
	SubmissionID string `json:"submission_id"`
	UserID       string `json:"user_id"`
	ProblemID    string `json:"problem_id"`
	Language     string `json:"language"`
	SourceCode   string `json:"source_code"`
	TimeLimit    int    `json:"time_limit"`
	MemoryLimit  int    `json:"memory_limit"`
}

// JudgeAdapter adapts the judge service to the problem service interface
type JudgeAdapter struct {
	judgeService *JudgeService
}

// NewJudgeAdapter creates a new judge adapter
func NewJudgeAdapter(judgeService *JudgeService) *JudgeAdapter {
	return &JudgeAdapter{
		judgeService: judgeService,
	}
}

// SubmitForJudging adapts the call to the judge service
func (ja *JudgeAdapter) SubmitForJudging(ctx context.Context, payload *ProblemSubmissionPayload) error {
	// Convert the payload to the judge service format
	judgePayload := &SubmissionPayload{
		SubmissionID: payload.SubmissionID,
		UserID:       payload.UserID,
		ProblemID:    payload.ProblemID,
		Language:     payload.Language,
		SourceCode:   payload.SourceCode,
		TimeLimit:    payload.TimeLimit,
		MemoryLimit:  payload.MemoryLimit,
	}
	
	return ja.judgeService.SubmitForJudging(ctx, judgePayload)
}