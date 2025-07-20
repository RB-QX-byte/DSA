package problem

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"competitive-programming-platform/pkg/database"
	"competitive-programming-platform/pkg/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// JudgeService defines the interface for judge operations  
type JudgeService interface {
	SubmitForJudging(ctx context.Context, payload *SubmissionPayload) error
}

// SubmissionPayload represents the data sent to the judge worker
type SubmissionPayload struct {
	SubmissionID string `json:"submission_id"`
	UserID       string `json:"user_id"`
	ProblemID    string `json:"problem_id"`
	Language     string `json:"language"`
	SourceCode   string `json:"source_code"`
	TimeLimit    int    `json:"time_limit"`
	MemoryLimit  int    `json:"memory_limit"`
}

// Service handles problem operations
type Service struct {
	db           *database.DB
	judgeService JudgeService
}

// NewService creates a new problem service
func NewService(db *database.DB) *Service {
	return &Service{db: db}
}

// SetJudgeService sets the judge service for handling submissions
func (s *Service) SetJudgeService(judgeService JudgeService) {
	s.judgeService = judgeService
}

// Problem represents a competitive programming problem
type Problem struct {
	ID                   string    `json:"id"`
	Title                string    `json:"title"`
	Slug                 string    `json:"slug"`
	Description          string    `json:"description"`
	Difficulty           int       `json:"difficulty"`
	TimeLimit            int       `json:"time_limit"`
	MemoryLimit          int       `json:"memory_limit"`
	Tags                 []string  `json:"tags"`
	AcceptanceRate       float64   `json:"acceptance_rate"`
	TotalSubmissions     int       `json:"total_submissions"`
	AcceptedSubmissions  int       `json:"accepted_submissions"`
	CreatedBy            *string   `json:"created_by"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// CreateProblemRequest represents the request to create a problem
type CreateProblemRequest struct {
	Title       string   `json:"title"`
	Slug        string   `json:"slug"`
	Description string   `json:"description"`
	Difficulty  int      `json:"difficulty"`
	TimeLimit   int      `json:"time_limit"`
	MemoryLimit int      `json:"memory_limit"`
	Tags        []string `json:"tags"`
}

// UpdateProblemRequest represents the request to update a problem
type UpdateProblemRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Difficulty  int      `json:"difficulty"`
	TimeLimit   int      `json:"time_limit"`
	MemoryLimit int      `json:"memory_limit"`
	Tags        []string `json:"tags"`
}

// SubmissionRequest represents a code submission
type SubmissionRequest struct {
	SourceCode string `json:"source_code"`
	Language   string `json:"language"`
}

// GetProblems returns a list of problems with pagination and filtering
func (s *Service) GetProblems(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	difficulty := r.URL.Query().Get("difficulty")
	tags := r.URL.Query().Get("tags")

	// Build query
	query := `
		SELECT id, title, slug, description, difficulty, time_limit, memory_limit, 
		       tags, acceptance_rate, total_submissions, accepted_submissions, 
		       created_by, created_at, updated_at
		FROM problems
		WHERE 1=1
	`
	args := []interface{}{}
	argIndex := 1

	if difficulty != "" {
		query += " AND difficulty = $" + strconv.Itoa(argIndex)
		args = append(args, difficulty)
		argIndex++
	}

	if tags != "" {
		query += " AND $" + strconv.Itoa(argIndex) + " = ANY(tags)"
		args = append(args, tags)
		argIndex++
	}

	query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(argIndex) + " OFFSET $" + strconv.Itoa(argIndex+1)
	args = append(args, limit, (page-1)*limit)

	rows, err := s.db.Pool.Query(r.Context(), query, args...)
	if err != nil {
		http.Error(w, "Failed to fetch problems", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var problems []Problem
	for rows.Next() {
		var problem Problem
		err := rows.Scan(
			&problem.ID, &problem.Title, &problem.Slug, &problem.Description,
			&problem.Difficulty, &problem.TimeLimit, &problem.MemoryLimit,
			&problem.Tags, &problem.AcceptanceRate, &problem.TotalSubmissions,
			&problem.AcceptedSubmissions, &problem.CreatedBy, &problem.CreatedAt, &problem.UpdatedAt,
		)
		if err != nil {
			http.Error(w, "Failed to scan problem", http.StatusInternalServerError)
			return
		}
		problems = append(problems, problem)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(problems)
}

// GetProblem returns a specific problem by ID
func (s *Service) GetProblem(w http.ResponseWriter, r *http.Request) {
	problemID := chi.URLParam(r, "id")
	if problemID == "" {
		http.Error(w, "Problem ID is required", http.StatusBadRequest)
		return
	}

	problem, err := s.getProblemByID(r.Context(), problemID)
	if err != nil {
		http.Error(w, "Problem not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(problem)
}

// CreateProblem creates a new problem
func (s *Service) CreateProblem(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	var req CreateProblemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Title == "" || req.Slug == "" || req.Description == "" {
		http.Error(w, "Title, slug, and description are required", http.StatusBadRequest)
		return
	}

	if req.Difficulty < 800 || req.Difficulty > 3500 {
		http.Error(w, "Difficulty must be between 800 and 3500", http.StatusBadRequest)
		return
	}

	problemID := uuid.New().String()

	query := `
		INSERT INTO problems (id, title, slug, description, difficulty, time_limit, memory_limit, tags, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, title, slug, description, difficulty, time_limit, memory_limit, tags, acceptance_rate, total_submissions, accepted_submissions, created_by, created_at, updated_at
	`

	var problem Problem
	err := s.db.Pool.QueryRow(r.Context(), query,
		problemID, req.Title, req.Slug, req.Description, req.Difficulty,
		req.TimeLimit, req.MemoryLimit, req.Tags, userID,
	).Scan(
		&problem.ID, &problem.Title, &problem.Slug, &problem.Description,
		&problem.Difficulty, &problem.TimeLimit, &problem.MemoryLimit,
		&problem.Tags, &problem.AcceptanceRate, &problem.TotalSubmissions,
		&problem.AcceptedSubmissions, &problem.CreatedBy, &problem.CreatedAt, &problem.UpdatedAt,
	)
	if err != nil {
		http.Error(w, "Failed to create problem", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(problem)
}

// UpdateProblem updates an existing problem
func (s *Service) UpdateProblem(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	problemID := chi.URLParam(r, "id")
	if problemID == "" {
		http.Error(w, "Problem ID is required", http.StatusBadRequest)
		return
	}

	var req UpdateProblemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check if user owns the problem or is admin
	// For now, we'll allow any authenticated user to update
	query := `
		UPDATE problems 
		SET title = $1, description = $2, difficulty = $3, time_limit = $4, memory_limit = $5, tags = $6, updated_at = NOW()
		WHERE id = $7
		RETURNING id, title, slug, description, difficulty, time_limit, memory_limit, tags, acceptance_rate, total_submissions, accepted_submissions, created_by, created_at, updated_at
	`

	var problem Problem
	err := s.db.Pool.QueryRow(r.Context(), query,
		req.Title, req.Description, req.Difficulty, req.TimeLimit, req.MemoryLimit, req.Tags, problemID,
	).Scan(
		&problem.ID, &problem.Title, &problem.Slug, &problem.Description,
		&problem.Difficulty, &problem.TimeLimit, &problem.MemoryLimit,
		&problem.Tags, &problem.AcceptanceRate, &problem.TotalSubmissions,
		&problem.AcceptedSubmissions, &problem.CreatedBy, &problem.CreatedAt, &problem.UpdatedAt,
	)
	if err != nil {
		http.Error(w, "Failed to update problem", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(problem)
}

// DeleteProblem deletes a problem
func (s *Service) DeleteProblem(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	problemID := chi.URLParam(r, "id")
	if problemID == "" {
		http.Error(w, "Problem ID is required", http.StatusBadRequest)
		return
	}

	// Check if user owns the problem or is admin
	query := `DELETE FROM problems WHERE id = $1`
	
	result, err := s.db.Pool.Exec(r.Context(), query, problemID)
	if err != nil {
		http.Error(w, "Failed to delete problem", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected() == 0 {
		http.Error(w, "Problem not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SubmitSolution handles code submission for a problem
func (s *Service) SubmitSolution(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	problemID := chi.URLParam(r, "id")
	if problemID == "" {
		http.Error(w, "Problem ID is required", http.StatusBadRequest)
		return
	}

	var req SubmissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.SourceCode == "" || req.Language == "" {
		http.Error(w, "Source code and language are required", http.StatusBadRequest)
		return
	}

	// Get problem details for time/memory limits
	problem, err := s.getProblemByID(r.Context(), problemID)
	if err != nil {
		http.Error(w, "Problem not found", http.StatusNotFound)
		return
	}

	if s.judgeService == nil {
		http.Error(w, "Judge service not available", http.StatusServiceUnavailable)
		return
	}

	// Create submission payload
	payload := &SubmissionPayload{
		UserID:      userID,
		ProblemID:   problemID,
		Language:    req.Language,
		SourceCode:  req.SourceCode,
		TimeLimit:   problem.TimeLimit,
		MemoryLimit: problem.MemoryLimit,
	}

	// Submit for judging
	if err := s.judgeService.SubmitForJudging(r.Context(), payload); err != nil {
		http.Error(w, "Failed to submit solution for judging", http.StatusInternalServerError)
		return
	}

	// Return success response
	response := map[string]interface{}{
		"message": "Solution submitted successfully",
		"status":  "submitted",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// getProblemByID is a helper function to get a problem by ID
func (s *Service) getProblemByID(ctx context.Context, problemID string) (*Problem, error) {
	query := `
		SELECT id, title, slug, description, difficulty, time_limit, memory_limit, 
		       tags, acceptance_rate, total_submissions, accepted_submissions, 
		       created_by, created_at, updated_at
		FROM problems
		WHERE id = $1
	`

	var problem Problem
	err := s.db.Pool.QueryRow(ctx, query, problemID).Scan(
		&problem.ID, &problem.Title, &problem.Slug, &problem.Description,
		&problem.Difficulty, &problem.TimeLimit, &problem.MemoryLimit,
		&problem.Tags, &problem.AcceptanceRate, &problem.TotalSubmissions,
		&problem.AcceptedSubmissions, &problem.CreatedBy, &problem.CreatedAt, &problem.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &problem, nil
}