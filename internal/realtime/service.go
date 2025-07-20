package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"competitive-programming-platform/pkg/database"
	"competitive-programming-platform/pkg/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Service handles real-time operations
type Service struct {
	db  *database.DB
	hub *Hub
}

// NewService creates a new real-time service
func NewService(db *database.DB) *Service {
	return &Service{
		db:  db,
		hub: NewHub(),
	}
}

// GetHub returns the SSE hub
func (s *Service) GetHub() *Hub {
	return s.hub
}

// StartHub starts the SSE hub
func (s *Service) StartHub(ctx context.Context) {
	go s.hub.Run(ctx)
}

// HandleSSE handles Server-Sent Events connections
func (s *Service) HandleSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	// Get user ID from context
	userID, isAuthenticated := middleware.GetUserIDFromContext(r.Context())
	if !isAuthenticated {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Get contest ID from query parameters
	contestID := r.URL.Query().Get("contest_id")

	// Create client context with contest information
	ctx, cancel := context.WithCancel(r.Context())
	if contestID != "" {
		ctx = context.WithValue(ctx, "contestID", contestID)
	}

	// Create new client
	client := &Client{
		ID:      uuid.New().String(),
		UserID:  userID,
		Channel: make(chan Event, 100), // Buffer to prevent blocking
		Request: r.WithContext(ctx),
		Writer:  w,
		Context: ctx,
		Cancel:  cancel,
	}

	// Register client
	s.hub.RegisterClient(client)

	// Cleanup on disconnect
	defer func() {
		s.hub.UnregisterClient(client)
		cancel()
	}()

	// Start listening for events
	client.Listen()
}

// HandleContestSSE handles SSE connections for a specific contest
func (s *Service) HandleContestSSE(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "id")
	if contestID == "" {
		http.Error(w, "Contest ID is required", http.StatusBadRequest)
		return
	}

	// Set contest ID in query parameters
	q := r.URL.Query()
	q.Set("contest_id", contestID)
	r.URL.RawQuery = q.Encode()

	s.HandleSSE(w, r)
}

// GetSSEStats returns SSE connection statistics
func (s *Service) GetSSEStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{
		"total_clients":    s.hub.GetClientCount(),
		"timestamp":        time.Now(),
		"server_uptime":    time.Since(time.Now()).String(), // This would be tracked from server start
	}

	// Get contest-specific stats if contest ID is provided
	contestID := r.URL.Query().Get("contest_id")
	if contestID != "" {
		stats["contest_clients"] = s.hub.GetContestClientCount(contestID)
		stats["contest_id"] = contestID
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// SubmissionStatusUpdate represents a submission status update
type SubmissionStatusUpdate struct {
	SubmissionID string `json:"submission_id"`
	UserID       string `json:"user_id"`
	ProblemID    string `json:"problem_id"`
	ContestID    string `json:"contest_id,omitempty"`
	Status       string `json:"status"`
	Verdict      string `json:"verdict"`
	ExecutionTime int   `json:"execution_time,omitempty"`
	MemoryUsage  int    `json:"memory_usage,omitempty"`
	Score        int    `json:"score"`
	TestCasesPassed int `json:"test_cases_passed"`
	TotalTestCases  int `json:"total_test_cases"`
	Language     string `json:"language"`
	Timestamp    time.Time `json:"timestamp"`
}

// LeaderboardUpdate represents a leaderboard update
type LeaderboardUpdate struct {
	ContestID string                 `json:"contest_id"`
	Rankings  []LeaderboardEntry     `json:"rankings"`
	Timestamp time.Time              `json:"timestamp"`
	UpdateType string                `json:"update_type"` // "full" or "incremental"
}

// LeaderboardEntry represents a single leaderboard entry
type LeaderboardEntry struct {
	Rank           int                        `json:"rank"`
	UserID         string                     `json:"user_id"`
	Username       string                     `json:"username"`
	FullName       string                     `json:"full_name"`
	TotalPoints    int                        `json:"total_points"`
	TotalPenalty   int                        `json:"total_penalty"`
	ProblemsSolved int                        `json:"problems_solved"`
	LastSubmission time.Time                  `json:"last_submission"`
	ProblemResults []LeaderboardProblemResult `json:"problem_results"`
}

// LeaderboardProblemResult represents a problem result in the leaderboard
type LeaderboardProblemResult struct {
	ProblemID      string     `json:"problem_id"`
	ProblemOrder   int        `json:"problem_order"`
	Points         int        `json:"points"`
	Attempts       int        `json:"attempts"`
	Solved         bool       `json:"solved"`
	SolveTime      *time.Time `json:"solve_time,omitempty"`
	PenaltyMinutes int        `json:"penalty_minutes"`
}

// BroadcastSubmissionUpdate broadcasts a submission status update
func (s *Service) BroadcastSubmissionUpdate(update SubmissionStatusUpdate) {
	// Broadcast to the specific user
	s.hub.BroadcastToUser(update.UserID, "submission_update", update)

	// If it's a contest submission, also broadcast to contest viewers
	if update.ContestID != "" {
		s.hub.BroadcastToContest(update.ContestID, "contest_submission_update", update)
	}

	// Broadcast general submission update
	s.hub.BroadcastEvent("global_submission_update", update)
}

// BroadcastLeaderboardUpdate broadcasts a leaderboard update
func (s *Service) BroadcastLeaderboardUpdate(update LeaderboardUpdate) {
	s.hub.BroadcastToContest(update.ContestID, "leaderboard_update", update)
}

// BroadcastContestUpdate broadcasts a contest status update
func (s *Service) BroadcastContestUpdate(contestID string, status string, data interface{}) {
	update := map[string]interface{}{
		"contest_id": contestID,
		"status":     status,
		"data":       data,
		"timestamp":  time.Now(),
	}
	
	s.hub.BroadcastToContest(contestID, "contest_update", update)
}

// BroadcastSystemNotification broadcasts a system-wide notification
func (s *Service) BroadcastSystemNotification(message string, level string) {
	notification := map[string]interface{}{
		"message":   message,
		"level":     level, // "info", "warning", "error"
		"timestamp": time.Now(),
	}
	
	s.hub.BroadcastEvent("system_notification", notification)
}

// Health check for real-time service
func (s *Service) HealthCheck(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":         "healthy",
		"connected_clients": s.hub.GetClientCount(),
		"timestamp":      time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}