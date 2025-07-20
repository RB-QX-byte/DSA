package contest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"competitive-programming-platform/pkg/database"
	"competitive-programming-platform/pkg/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Service handles contest operations
type Service struct {
	db *database.DB
}

// NewService creates a new contest service
func NewService(db *database.DB) *Service {
	return &Service{db: db}
}

// GetContests returns a list of contests with pagination and filtering
func (s *Service) GetContests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Parse query parameters
	filters := ContestFilters{
		Status: r.URL.Query().Get("status"),
		Search: r.URL.Query().Get("search"),
		Page:   1,
		Limit:  20,
	}
	
	if page, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && page > 0 {
		filters.Page = page
	}
	
	if limit, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && limit > 0 && limit <= 100 {
		filters.Limit = limit
	}
	
	// Get user ID if authenticated
	userID, isAuthenticated := middleware.GetUserIDFromContext(ctx)
	if isAuthenticated && r.URL.Query().Get("registered") == "true" {
		registered := true
		filters.Registered = &registered
	}
	
	contests, err := s.getContests(ctx, filters, userID)
	if err != nil {
		http.Error(w, "Failed to fetch contests", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(contests)
}

// GetContest returns a specific contest by ID
func (s *Service) GetContest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	contestID := chi.URLParam(r, "id")
	
	if contestID == "" {
		http.Error(w, "Contest ID is required", http.StatusBadRequest)
		return
	}
	
	userID, _ := middleware.GetUserIDFromContext(ctx)
	
	contest, err := s.getContestByID(ctx, contestID, userID)
	if err != nil {
		if err == ErrContestNotFound {
			http.Error(w, "Contest not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to fetch contest", http.StatusInternalServerError)
		}
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(contest)
}

// CreateContest creates a new contest
func (s *Service) CreateContest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}
	
	var req CreateContestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	// Validate request
	if err := s.validateCreateContestRequest(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	contest, err := s.createContest(ctx, &req, userID)
	if err != nil {
		http.Error(w, "Failed to create contest", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(contest)
}

// UpdateContest updates an existing contest
func (s *Service) UpdateContest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	contestID := chi.URLParam(r, "id")
	
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}
	
	if contestID == "" {
		http.Error(w, "Contest ID is required", http.StatusBadRequest)
		return
	}
	
	var req UpdateContestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	contest, err := s.updateContest(ctx, contestID, &req, userID)
	if err != nil {
		if err == ErrContestNotFound {
			http.Error(w, "Contest not found", http.StatusNotFound)
		} else if err == ErrUnauthorized {
			http.Error(w, "Unauthorized access", http.StatusForbidden)
		} else {
			http.Error(w, "Failed to update contest", http.StatusInternalServerError)
		}
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(contest)
}

// DeleteContest deletes a contest
func (s *Service) DeleteContest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	contestID := chi.URLParam(r, "id")
	
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}
	
	if contestID == "" {
		http.Error(w, "Contest ID is required", http.StatusBadRequest)
		return
	}
	
	err := s.deleteContest(ctx, contestID, userID)
	if err != nil {
		if err == ErrContestNotFound {
			http.Error(w, "Contest not found", http.StatusNotFound)
		} else if err == ErrUnauthorized {
			http.Error(w, "Unauthorized access", http.StatusForbidden)
		} else {
			http.Error(w, "Failed to delete contest", http.StatusInternalServerError)
		}
		return
	}
	
	w.WriteHeader(http.StatusNoContent)
}

// RegisterForContest registers a user for a contest
func (s *Service) RegisterForContest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	contestID := chi.URLParam(r, "id")
	
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}
	
	if contestID == "" {
		http.Error(w, "Contest ID is required", http.StatusBadRequest)
		return
	}
	
	registration, err := s.registerForContest(ctx, contestID, userID)
	if err != nil {
		if err == ErrContestNotFound {
			http.Error(w, "Contest not found", http.StatusNotFound)
		} else if err == ErrAlreadyRegistered {
			http.Error(w, "Already registered for this contest", http.StatusConflict)
		} else if err == ErrRegistrationClosed {
			http.Error(w, "Registration is closed for this contest", http.StatusBadRequest)
		} else if err == ErrContestFull {
			http.Error(w, "Contest has reached maximum participants", http.StatusBadRequest)
		} else {
			http.Error(w, "Failed to register for contest", http.StatusInternalServerError)
		}
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(registration)
}

// GetContestProblems returns problems for a contest
func (s *Service) GetContestProblems(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	contestID := chi.URLParam(r, "id")
	
	userID, isAuthenticated := middleware.GetUserIDFromContext(ctx)
	
	if contestID == "" {
		http.Error(w, "Contest ID is required", http.StatusBadRequest)
		return
	}
	
	// Check if user can access contest problems
	if isAuthenticated {
		canAccess, err := s.canUserAccessContest(ctx, contestID, userID)
		if err != nil {
			http.Error(w, "Failed to check access", http.StatusInternalServerError)
			return
		}
		
		if !canAccess {
			// Check if contest is live for public access
			contest, err := s.getContestByID(ctx, contestID, userID)
			if err != nil || contest.GetStatus() != "live" {
				http.Error(w, "Access denied", http.StatusForbidden)
				return
			}
		}
	}
	
	problems, err := s.getContestProblems(ctx, contestID)
	if err != nil {
		http.Error(w, "Failed to fetch contest problems", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(problems)
}

// GetContestStandings returns the leaderboard for a contest
func (s *Service) GetContestStandings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	contestID := chi.URLParam(r, "id")
	
	if contestID == "" {
		http.Error(w, "Contest ID is required", http.StatusBadRequest)
		return
	}
	
	standings, err := s.getContestStandings(ctx, contestID)
	if err != nil {
		http.Error(w, "Failed to fetch contest standings", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(standings)
}

// Private helper methods

func (s *Service) getContests(ctx context.Context, filters ContestFilters, userID string) ([]Contest, error) {
	query := `
		SELECT c.id, c.title, c.description, c.rules, c.start_time, c.end_time, 
		       c.registration_start, c.registration_end, c.max_participants, 
		       c.status, c.created_by, c.created_at, c.updated_at,
		       COALESCE(reg_count.count, 0) as participant_count,
		       COALESCE(prob_count.count, 0) as problem_count
		FROM contests c
		LEFT JOIN (
			SELECT contest_id, COUNT(*) as count 
			FROM contest_registrations 
			GROUP BY contest_id
		) reg_count ON c.id = reg_count.contest_id
		LEFT JOIN (
			SELECT contest_id, COUNT(*) as count 
			FROM contest_problems 
			GROUP BY contest_id
		) prob_count ON c.id = prob_count.contest_id
		WHERE 1=1
	`
	
	args := []interface{}{}
	argIndex := 1
	
	// Add filters
	if filters.Status != "" {
		query += fmt.Sprintf(" AND c.status = $%d", argIndex)
		args = append(args, filters.Status)
		argIndex++
	}
	
	if filters.Search != "" {
		query += fmt.Sprintf(" AND c.title ILIKE $%d", argIndex)
		args = append(args, "%"+filters.Search+"%")
		argIndex++
	}
	
	if filters.Registered != nil && *filters.Registered && userID != "" {
		query += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM contest_registrations WHERE contest_id = c.id AND user_id = $%d)", argIndex)
		args = append(args, userID)
		argIndex++
	}
	
	// Add ordering and pagination
	query += " ORDER BY c.start_time DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, filters.Limit, (filters.Page-1)*filters.Limit)
	
	rows, err := s.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var contests []Contest
	for rows.Next() {
		var contest Contest
		err := rows.Scan(
			&contest.ID, &contest.Title, &contest.Description, &contest.Rules,
			&contest.StartTime, &contest.EndTime, &contest.RegistrationStart,
			&contest.RegistrationEnd, &contest.MaxParticipants, &contest.Status,
			&contest.CreatedBy, &contest.CreatedAt, &contest.UpdatedAt,
			&contest.ParticipantCount, &contest.ProblemCount,
		)
		if err != nil {
			return nil, err
		}
		
		contest.IsRegistrationOpenFlag = contest.IsRegistrationOpen()
		contests = append(contests, contest)
	}
	
	return contests, nil
}

func (s *Service) getContestByID(ctx context.Context, contestID string, userID string) (*Contest, error) {
	query := `
		SELECT c.id, c.title, c.description, c.rules, c.start_time, c.end_time, 
		       c.registration_start, c.registration_end, c.max_participants, 
		       c.status, c.created_by, c.created_at, c.updated_at,
		       COALESCE(reg_count.count, 0) as participant_count,
		       COALESCE(prob_count.count, 0) as problem_count
		FROM contests c
		LEFT JOIN (
			SELECT contest_id, COUNT(*) as count 
			FROM contest_registrations 
			GROUP BY contest_id
		) reg_count ON c.id = reg_count.contest_id
		LEFT JOIN (
			SELECT contest_id, COUNT(*) as count 
			FROM contest_problems 
			GROUP BY contest_id
		) prob_count ON c.id = prob_count.contest_id
		WHERE c.id = $1
	`
	
	var contest Contest
	err := s.db.Pool.QueryRow(ctx, query, contestID).Scan(
		&contest.ID, &contest.Title, &contest.Description, &contest.Rules,
		&contest.StartTime, &contest.EndTime, &contest.RegistrationStart,
		&contest.RegistrationEnd, &contest.MaxParticipants, &contest.Status,
		&contest.CreatedBy, &contest.CreatedAt, &contest.UpdatedAt,
		&contest.ParticipantCount, &contest.ProblemCount,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrContestNotFound
		}
		return nil, err
	}
	
	contest.IsRegistrationOpenFlag = contest.IsRegistrationOpen()
	return &contest, nil
}

func (s *Service) createContest(ctx context.Context, req *CreateContestRequest, createdBy string) (*Contest, error) {
	contestID := uuid.New().String()
	
	query := `
		INSERT INTO contests (id, title, description, rules, start_time, end_time, 
		                     registration_start, registration_end, max_participants, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, title, description, rules, start_time, end_time, 
		          registration_start, registration_end, max_participants, 
		          status, created_by, created_at, updated_at
	`
	
	var contest Contest
	err := s.db.Pool.QueryRow(ctx, query,
		contestID, req.Title, req.Description, req.Rules,
		req.StartTime, req.EndTime, req.RegistrationStart, req.RegistrationEnd,
		req.MaxParticipants, createdBy,
	).Scan(
		&contest.ID, &contest.Title, &contest.Description, &contest.Rules,
		&contest.StartTime, &contest.EndTime, &contest.RegistrationStart,
		&contest.RegistrationEnd, &contest.MaxParticipants, &contest.Status,
		&contest.CreatedBy, &contest.CreatedAt, &contest.UpdatedAt,
	)
	
	if err != nil {
		return nil, err
	}
	
	// Add problems if provided
	if len(req.ProblemIDs) > 0 {
		for i, problemID := range req.ProblemIDs {
			_, err := s.db.Pool.Exec(ctx, `
				INSERT INTO contest_problems (contest_id, problem_id, problem_order)
				VALUES ($1, $2, $3)
			`, contestID, problemID, i+1)
			if err != nil {
				// Continue with other problems if one fails
				continue
			}
		}
	}
	
	contest.IsRegistrationOpenFlag = contest.IsRegistrationOpen()
	return &contest, nil
}

func (s *Service) updateContest(ctx context.Context, contestID string, req *UpdateContestRequest, userID string) (*Contest, error) {
	// Check if user can update this contest
	existing, err := s.getContestByID(ctx, contestID, userID)
	if err != nil {
		return nil, err
	}
	
	if existing.CreatedBy == nil || *existing.CreatedBy != userID {
		return nil, ErrUnauthorized
	}
	
	// Build update query dynamically
	setParts := []string{}
	args := []interface{}{}
	argIndex := 1
	
	if req.Title != nil {
		setParts = append(setParts, fmt.Sprintf("title = $%d", argIndex))
		args = append(args, *req.Title)
		argIndex++
	}
	
	if req.Description != nil {
		setParts = append(setParts, fmt.Sprintf("description = $%d", argIndex))
		args = append(args, *req.Description)
		argIndex++
	}
	
	if req.Rules != nil {
		setParts = append(setParts, fmt.Sprintf("rules = $%d", argIndex))
		args = append(args, *req.Rules)
		argIndex++
	}
	
	if req.StartTime != nil {
		setParts = append(setParts, fmt.Sprintf("start_time = $%d", argIndex))
		args = append(args, *req.StartTime)
		argIndex++
	}
	
	if req.EndTime != nil {
		setParts = append(setParts, fmt.Sprintf("end_time = $%d", argIndex))
		args = append(args, *req.EndTime)
		argIndex++
	}
	
	if req.RegistrationStart != nil {
		setParts = append(setParts, fmt.Sprintf("registration_start = $%d", argIndex))
		args = append(args, *req.RegistrationStart)
		argIndex++
	}
	
	if req.RegistrationEnd != nil {
		setParts = append(setParts, fmt.Sprintf("registration_end = $%d", argIndex))
		args = append(args, *req.RegistrationEnd)
		argIndex++
	}
	
	if req.MaxParticipants != nil {
		setParts = append(setParts, fmt.Sprintf("max_participants = $%d", argIndex))
		args = append(args, *req.MaxParticipants)
		argIndex++
	}
	
	if len(setParts) == 0 {
		return existing, nil
	}
	
	setParts = append(setParts, "updated_at = NOW()")
	query := fmt.Sprintf(`
		UPDATE contests 
		SET %s
		WHERE id = $%d
		RETURNING id, title, description, rules, start_time, end_time, 
		          registration_start, registration_end, max_participants, 
		          status, created_by, created_at, updated_at
	`, strings.Join(setParts, ", "), argIndex)
	
	args = append(args, contestID)
	
	var contest Contest
	err = s.db.Pool.QueryRow(ctx, query, args...).Scan(
		&contest.ID, &contest.Title, &contest.Description, &contest.Rules,
		&contest.StartTime, &contest.EndTime, &contest.RegistrationStart,
		&contest.RegistrationEnd, &contest.MaxParticipants, &contest.Status,
		&contest.CreatedBy, &contest.CreatedAt, &contest.UpdatedAt,
	)
	
	if err != nil {
		return nil, err
	}
	
	contest.IsRegistrationOpenFlag = contest.IsRegistrationOpen()
	return &contest, nil
}

func (s *Service) deleteContest(ctx context.Context, contestID string, userID string) error {
	// Check if user can delete this contest
	existing, err := s.getContestByID(ctx, contestID, userID)
	if err != nil {
		return err
	}
	
	if existing.CreatedBy == nil || *existing.CreatedBy != userID {
		return ErrUnauthorized
	}
	
	_, err = s.db.Pool.Exec(ctx, "DELETE FROM contests WHERE id = $1", contestID)
	return err
}

func (s *Service) registerForContest(ctx context.Context, contestID string, userID string) (*ContestRegistration, error) {
	// Check if contest exists and registration is open
	contest, err := s.getContestByID(ctx, contestID, userID)
	if err != nil {
		return nil, err
	}
	
	if !contest.IsRegistrationOpen() {
		return nil, ErrRegistrationClosed
	}
	
	// Check if already registered
	var existingID string
	err = s.db.Pool.QueryRow(ctx, `
		SELECT id FROM contest_registrations 
		WHERE contest_id = $1 AND user_id = $2
	`, contestID, userID).Scan(&existingID)
	
	if err == nil {
		return nil, ErrAlreadyRegistered
	} else if err != pgx.ErrNoRows {
		return nil, err
	}
	
	// Check if contest is full
	if contest.MaxParticipants != nil && contest.ParticipantCount >= *contest.MaxParticipants {
		return nil, ErrContestFull
	}
	
	// Create registration
	registrationID := uuid.New().String()
	query := `
		INSERT INTO contest_registrations (id, contest_id, user_id)
		VALUES ($1, $2, $3)
		RETURNING id, contest_id, user_id, registered_at
	`
	
	var registration ContestRegistration
	err = s.db.Pool.QueryRow(ctx, query, registrationID, contestID, userID).Scan(
		&registration.ID, &registration.ContestID, &registration.UserID, &registration.RegisteredAt,
	)
	
	if err != nil {
		return nil, err
	}
	
	return &registration, nil
}

func (s *Service) canUserAccessContest(ctx context.Context, contestID string, userID string) (bool, error) {
	var count int
	err := s.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM contest_registrations 
		WHERE contest_id = $1 AND user_id = $2
	`, contestID, userID).Scan(&count)
	
	if err != nil {
		return false, err
	}
	
	return count > 0, nil
}

func (s *Service) getContestProblems(ctx context.Context, contestID string) ([]ContestProblem, error) {
	query := `
		SELECT cp.id, cp.contest_id, cp.problem_id, cp.problem_order, cp.points, cp.created_at,
		       p.title, p.description, p.difficulty, p.tags
		FROM contest_problems cp
		JOIN problems p ON cp.problem_id = p.id
		WHERE cp.contest_id = $1
		ORDER BY cp.problem_order
	`
	
	rows, err := s.db.Pool.Query(ctx, query, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var problems []ContestProblem
	for rows.Next() {
		var problem ContestProblem
		err := rows.Scan(
			&problem.ID, &problem.ContestID, &problem.ProblemID, &problem.ProblemOrder,
			&problem.Points, &problem.CreatedAt, &problem.ProblemTitle,
			&problem.ProblemDescription, &problem.ProblemDifficulty, &problem.ProblemTags,
		)
		if err != nil {
			return nil, err
		}
		problems = append(problems, problem)
	}
	
	return problems, nil
}

func (s *Service) getContestStandings(ctx context.Context, contestID string) ([]ContestStanding, error) {
	// This is a simplified version - in production, you'd have a more complex query
	query := `
		SELECT DISTINCT cr.user_id, u.username, u.full_name
		FROM contest_registrations cr
		JOIN users u ON cr.user_id = u.id
		WHERE cr.contest_id = $1
		ORDER BY u.username
	`
	
	rows, err := s.db.Pool.Query(ctx, query, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var standings []ContestStanding
	rank := 1
	for rows.Next() {
		var standing ContestStanding
		err := rows.Scan(&standing.UserID, &standing.Username, &standing.FullName)
		if err != nil {
			return nil, err
		}
		
		standing.ContestID = contestID
		standing.Rank = rank
		// In a real implementation, you'd calculate these values from contest_submissions
		standing.TotalPoints = 0
		standing.TotalPenalty = 0
		standing.ProblemsSolved = 0
		standing.ProblemResults = []ContestProblemResult{}
		
		standings = append(standings, standing)
		rank++
	}
	
	return standings, nil
}

func (s *Service) validateCreateContestRequest(req *CreateContestRequest) error {
	if req.Title == "" {
		return fmt.Errorf("contest title is required")
	}
	
	if req.EndTime.Before(req.StartTime) {
		return fmt.Errorf("contest end time must be after start time")
	}
	
	if req.RegistrationStart != nil && req.RegistrationEnd != nil {
		if req.RegistrationEnd.Before(*req.RegistrationStart) {
			return fmt.Errorf("registration end time must be after start time")
		}
	}
	
	if req.MaxParticipants != nil && *req.MaxParticipants <= 0 {
		return fmt.Errorf("max participants must be positive")
	}
	
	return nil
}