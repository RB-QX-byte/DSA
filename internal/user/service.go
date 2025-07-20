package user

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"competitive-programming-platform/pkg/database"
	"competitive-programming-platform/pkg/middleware"

	"github.com/go-chi/chi/v5"
)

// Service handles user operations
type Service struct {
	db *database.DB
}

// NewService creates a new user service
func NewService(db *database.DB) *Service {
	return &Service{db: db}
}

// User represents a user profile
type User struct {
	ID                   string    `json:"id"`
	Username             string    `json:"username"`
	FullName             string    `json:"full_name"`
	Email                string    `json:"email"`
	Rating               int       `json:"rating"`
	MaxRating            int       `json:"max_rating"`
	ProblemsSolved       int       `json:"problems_solved"`
	ContestsParticipated int       `json:"contests_participated"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// UpdateUserRequest represents the user update request
type UpdateUserRequest struct {
	Username string `json:"username"`
	FullName string `json:"full_name"`
}

// Submission represents a user submission
type Submission struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	ProblemID        string    `json:"problem_id"`
	ProblemTitle     string    `json:"problem_title"`
	Language         string    `json:"language"`
	Verdict          string    `json:"verdict"`
	ExecutionTime    *int      `json:"execution_time"`
	MemoryUsed       *int      `json:"memory_used"`
	Score            int       `json:"score"`
	TestCasesPassed  int       `json:"test_cases_passed"`
	TotalTestCases   int       `json:"total_test_cases"`
	CreatedAt        time.Time `json:"created_at"`
}

// GetCurrentUser returns the current user's profile
func (s *Service) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	user, err := s.getUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// GetUser returns a user profile by ID
func (s *Service) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	user, err := s.getUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// UpdateCurrentUser updates the current user's profile
func (s *Service) UpdateCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Update user in database
	query := `
		UPDATE users 
		SET username = $1, full_name = $2, updated_at = NOW()
		WHERE id = $3
		RETURNING id, username, full_name, email, rating, max_rating, problems_solved, contests_participated, created_at, updated_at
	`

	var user User
	err := s.db.Pool.QueryRow(r.Context(), query, req.Username, req.FullName, userID).Scan(
		&user.ID, &user.Username, &user.FullName, &user.Email, &user.Rating,
		&user.MaxRating, &user.ProblemsSolved, &user.ContestsParticipated,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// GetSubmissions returns the user's submissions
func (s *Service) GetSubmissions(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	query := `
		SELECT s.id, s.user_id, s.problem_id, p.title, s.language, s.verdict, 
		       s.execution_time, s.memory_used, s.score, s.test_cases_passed, 
		       s.total_test_cases, s.created_at
		FROM submissions s
		JOIN problems p ON s.problem_id = p.id
		WHERE s.user_id = $1
		ORDER BY s.created_at DESC
		LIMIT 50
	`

	rows, err := s.db.Pool.Query(r.Context(), query, userID)
	if err != nil {
		http.Error(w, "Failed to fetch submissions", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var submissions []Submission
	for rows.Next() {
		var submission Submission
		err := rows.Scan(
			&submission.ID, &submission.UserID, &submission.ProblemID, &submission.ProblemTitle,
			&submission.Language, &submission.Verdict, &submission.ExecutionTime, &submission.MemoryUsed,
			&submission.Score, &submission.TestCasesPassed, &submission.TotalTestCases, &submission.CreatedAt,
		)
		if err != nil {
			http.Error(w, "Failed to scan submission", http.StatusInternalServerError)
			return
		}
		submissions = append(submissions, submission)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(submissions)
}

// GetSubmission returns a specific submission
func (s *Service) GetSubmission(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	submissionID := chi.URLParam(r, "id")
	if submissionID == "" {
		http.Error(w, "Submission ID is required", http.StatusBadRequest)
		return
	}

	query := `
		SELECT s.id, s.user_id, s.problem_id, p.title, s.language, s.verdict, 
		       s.execution_time, s.memory_used, s.score, s.test_cases_passed, 
		       s.total_test_cases, s.created_at
		FROM submissions s
		JOIN problems p ON s.problem_id = p.id
		WHERE s.id = $1 AND s.user_id = $2
	`

	var submission Submission
	err := s.db.Pool.QueryRow(r.Context(), query, submissionID, userID).Scan(
		&submission.ID, &submission.UserID, &submission.ProblemID, &submission.ProblemTitle,
		&submission.Language, &submission.Verdict, &submission.ExecutionTime, &submission.MemoryUsed,
		&submission.Score, &submission.TestCasesPassed, &submission.TotalTestCases, &submission.CreatedAt,
	)
	if err != nil {
		http.Error(w, "Submission not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(submission)
}

// getUserByID is a helper function to get a user by ID
func (s *Service) getUserByID(ctx context.Context, userID string) (*User, error) {
	query := `
		SELECT id, username, full_name, email, rating, max_rating, problems_solved, contests_participated, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user User
	err := s.db.Pool.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.Username, &user.FullName, &user.Email, &user.Rating,
		&user.MaxRating, &user.ProblemsSolved, &user.ContestsParticipated,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &user, nil
}