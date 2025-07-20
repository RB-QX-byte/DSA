package contest

import (
	"fmt"
	"time"
)

// Contest represents a competitive programming contest
type Contest struct {
	ID                string     `json:"id" db:"id"`
	Title             string     `json:"title" db:"title" validate:"required,min=1,max=255"`
	Description       *string    `json:"description,omitempty" db:"description"`
	Rules             *string    `json:"rules,omitempty" db:"rules"`
	StartTime         time.Time  `json:"start_time" db:"start_time" validate:"required"`
	EndTime           time.Time  `json:"end_time" db:"end_time" validate:"required"`
	RegistrationStart *time.Time `json:"registration_start,omitempty" db:"registration_start"`
	RegistrationEnd   *time.Time `json:"registration_end,omitempty" db:"registration_end"`
	MaxParticipants   *int       `json:"max_participants,omitempty" db:"max_participants"`
	Status            string     `json:"status" db:"status"` // upcoming, live, ended, cancelled
	CreatedBy         *string    `json:"created_by,omitempty" db:"created_by"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
	
	// Calculated fields
	IsRegistrationOpenFlag bool `json:"is_registration_open"`
	ParticipantCount       int  `json:"participant_count"`
	ProblemCount           int  `json:"problem_count"`
}

// ContestProblem represents a problem in a contest
type ContestProblem struct {
	ID           string    `json:"id" db:"id"`
	ContestID    string    `json:"contest_id" db:"contest_id" validate:"required"`
	ProblemID    string    `json:"problem_id" db:"problem_id" validate:"required"`
	ProblemOrder int       `json:"problem_order" db:"problem_order" validate:"required,min=1"`
	Points       int       `json:"points" db:"points" validate:"min=0"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	
	// Joined fields from problems table
	ProblemTitle       string   `json:"problem_title,omitempty" db:"problem_title"`
	ProblemDescription string   `json:"problem_description,omitempty" db:"problem_description"`
	ProblemDifficulty  int      `json:"problem_difficulty,omitempty" db:"problem_difficulty"`
	ProblemTags        []string `json:"problem_tags,omitempty" db:"problem_tags"`
}

// ContestRegistration represents a user's registration for a contest
type ContestRegistration struct {
	ID           string    `json:"id" db:"id"`
	ContestID    string    `json:"contest_id" db:"contest_id" validate:"required"`
	UserID       string    `json:"user_id" db:"user_id" validate:"required"`
	RegisteredAt time.Time `json:"registered_at" db:"registered_at"`
	
	// Joined fields from users table
	Username string `json:"username,omitempty" db:"username"`
	FullName string `json:"full_name,omitempty" db:"full_name"`
}

// ContestSubmission represents a submission made during a contest
type ContestSubmission struct {
	ID             string    `json:"id" db:"id"`
	ContestID      string    `json:"contest_id" db:"contest_id" validate:"required"`
	UserID         string    `json:"user_id" db:"user_id" validate:"required"`
	ProblemID      string    `json:"problem_id" db:"problem_id" validate:"required"`
	SubmissionID   string    `json:"submission_id" db:"submission_id" validate:"required"`
	SubmittedAt    time.Time `json:"submitted_at" db:"submitted_at"`
	Verdict        string    `json:"verdict" db:"verdict"`
	Points         int       `json:"points" db:"points"`
	PenaltyMinutes int       `json:"penalty_minutes" db:"penalty_minutes"`
	
	// Joined fields from submissions table
	Language     string `json:"language,omitempty" db:"language"`
	ExecutionTime int   `json:"execution_time,omitempty" db:"execution_time"`
	MemoryUsage  int    `json:"memory_usage,omitempty" db:"memory_usage"`
}

// ContestStanding represents a user's standing in a contest
type ContestStanding struct {
	ContestID      string `json:"contest_id"`
	UserID         string `json:"user_id"`
	Username       string `json:"username"`
	FullName       string `json:"full_name"`
	Rank           int    `json:"rank"`
	TotalPoints    int    `json:"total_points"`
	TotalPenalty   int    `json:"total_penalty"`
	ProblemsSolved int    `json:"problems_solved"`
	
	// Problem-specific results
	ProblemResults []ContestProblemResult `json:"problem_results"`
}

// ContestProblemResult represents a user's result for a specific problem
type ContestProblemResult struct {
	ProblemID      string    `json:"problem_id"`
	ProblemOrder   int       `json:"problem_order"`
	Points         int       `json:"points"`
	Attempts       int       `json:"attempts"`
	Solved         bool      `json:"solved"`
	SolveTime      *time.Time `json:"solve_time,omitempty"`
	PenaltyMinutes int       `json:"penalty_minutes"`
}

// CreateContestRequest represents the request to create a contest
type CreateContestRequest struct {
	Title             string     `json:"title" validate:"required,min=1,max=255"`
	Description       *string    `json:"description,omitempty"`
	Rules             *string    `json:"rules,omitempty"`
	StartTime         time.Time  `json:"start_time" validate:"required"`
	EndTime           time.Time  `json:"end_time" validate:"required"`
	RegistrationStart *time.Time `json:"registration_start,omitempty"`
	RegistrationEnd   *time.Time `json:"registration_end,omitempty"`
	MaxParticipants   *int       `json:"max_participants,omitempty"`
	ProblemIDs        []string   `json:"problem_ids,omitempty"`
}

// UpdateContestRequest represents the request to update a contest
type UpdateContestRequest struct {
	Title             *string    `json:"title,omitempty" validate:"omitempty,min=1,max=255"`
	Description       *string    `json:"description,omitempty"`
	Rules             *string    `json:"rules,omitempty"`
	StartTime         *time.Time `json:"start_time,omitempty"`
	EndTime           *time.Time `json:"end_time,omitempty"`
	RegistrationStart *time.Time `json:"registration_start,omitempty"`
	RegistrationEnd   *time.Time `json:"registration_end,omitempty"`
	MaxParticipants   *int       `json:"max_participants,omitempty"`
}

// ContestFilters represents filters for contest listing
type ContestFilters struct {
	Status     string `json:"status,omitempty"`     // upcoming, live, ended
	CreatedBy  string `json:"created_by,omitempty"` // filter by creator
	Page       int    `json:"page,omitempty"`       // pagination
	Limit      int    `json:"limit,omitempty"`      // pagination
	Search     string `json:"search,omitempty"`     // search by title
	Registered *bool  `json:"registered,omitempty"` // filter by user registration
}

// ContestStats represents statistics for a contest
type ContestStats struct {
	ContestID         string `json:"contest_id"`
	TotalParticipants int    `json:"total_participants"`
	TotalSubmissions  int    `json:"total_submissions"`
	TotalProblems     int    `json:"total_problems"`
	AverageScore      float64 `json:"average_score"`
	CompletionRate    float64 `json:"completion_rate"`
}

// Validation methods

// IsValid checks if the contest has valid time constraints
func (c *Contest) IsValid() error {
	if c.EndTime.Before(c.StartTime) {
		return ErrInvalidContestTimes
	}
	
	if c.RegistrationStart != nil && c.RegistrationEnd != nil {
		if c.RegistrationEnd.Before(*c.RegistrationStart) {
			return ErrInvalidRegistrationTimes
		}
	}
	
	return nil
}

// IsRegistrationOpen checks if registration is currently open
func (c *Contest) IsRegistrationOpen() bool {
	now := time.Now()
	
	// Check if registration period is defined
	if c.RegistrationStart != nil && now.Before(*c.RegistrationStart) {
		return false
	}
	
	if c.RegistrationEnd != nil && now.After(*c.RegistrationEnd) {
		return false
	}
	
	// Check if contest hasn't started yet
	if now.After(c.StartTime) {
		return false
	}
	
	return true
}

// GetStatus returns the current status of the contest
func (c *Contest) GetStatus() string {
	now := time.Now()
	
	if now.Before(c.StartTime) {
		return "upcoming"
	} else if now.After(c.EndTime) {
		return "ended"
	} else {
		return "live"
	}
}

// CanUserAccess checks if a user can access the contest workspace
func (c *Contest) CanUserAccess(userID string, isRegistered bool) bool {
	// Must be registered to access
	if !isRegistered {
		return false
	}
	
	// Contest must be live
	return c.GetStatus() == "live"
}

// Custom errors
var (
	ErrInvalidContestTimes      = fmt.Errorf("contest end time must be after start time")
	ErrInvalidRegistrationTimes = fmt.Errorf("registration end time must be after start time")
	ErrContestNotFound          = fmt.Errorf("contest not found")
	ErrAlreadyRegistered        = fmt.Errorf("user is already registered for this contest")
	ErrRegistrationClosed       = fmt.Errorf("registration is closed for this contest")
	ErrContestFull              = fmt.Errorf("contest has reached maximum participants")
	ErrUnauthorized             = fmt.Errorf("unauthorized access")
)