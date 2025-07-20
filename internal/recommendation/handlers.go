package recommendation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handlers provides HTTP handlers for the recommendation API
type Handlers struct {
	service *Service
}

// NewHandlers creates new recommendation handlers
func NewHandlers(service *Service) *Handlers {
	return &Handlers{
		service: service,
	}
}

// RegisterRoutes registers recommendation routes with the router
func (h *Handlers) RegisterRoutes(r chi.Router) {
	// Main recommendation endpoints
	r.Get("/recommendations", h.GetRecommendations)
	r.Post("/recommendations", h.GetRecommendationsPost)
	
	// User profile endpoints
	r.Get("/users/{userId}/profile", h.GetUserProfile)
	r.Post("/users/{userId}/feedback", h.RecordFeedback)
	
	// Problem features endpoints
	r.Get("/problems/{problemId}/features", h.GetProblemFeatures)
	
	// Service management endpoints
	r.Get("/recommendations/status", h.GetServiceStatus)
	r.Get("/recommendations/metrics", h.GetPerformanceMetrics)
	r.Post("/recommendations/retrain", h.RetrainModels)
}

// GetRecommendations handles GET /api/v1/recommendations
func (h *Handlers) GetRecommendations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Parse query parameters
	request, err := h.parseRecommendationRequest(r)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request parameters", err)
		return
	}
	
	// Get recommendations
	response, err := h.service.GetRecommendations(ctx, request)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get recommendations", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, response)
}

// GetRecommendationsPost handles POST /api/v1/recommendations
func (h *Handlers) GetRecommendationsPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Parse JSON request body
	var request RecommendationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON request body", err)
		return
	}
	
	// Get recommendations
	response, err := h.service.GetRecommendations(ctx, &request)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get recommendations", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, response)
}

// GetUserProfile handles GET /api/v1/users/{userId}/profile
func (h *Handlers) GetUserProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Parse user ID from URL
	userID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid user ID", err)
		return
	}
	
	// Get user profile
	profile, err := h.service.GetUserProfile(ctx, userID)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get user profile", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, profile)
}

// GetProblemFeatures handles GET /api/v1/problems/{problemId}/features
func (h *Handlers) GetProblemFeatures(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Parse problem ID from URL
	problemID, err := uuid.Parse(chi.URLParam(r, "problemId"))
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid problem ID", err)
		return
	}
	
	// Get problem features
	features, err := h.service.GetProblemFeatures(ctx, problemID)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get problem features", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, features)
}

// RecordFeedback handles POST /api/v1/users/{userId}/feedback
func (h *Handlers) RecordFeedback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Parse user ID from URL
	userID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid user ID", err)
		return
	}
	
	// Parse feedback request
	var feedbackReq FeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&feedbackReq); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON request body", err)
		return
	}
	
	// Validate feedback request
	if feedbackReq.ProblemID == uuid.Nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Problem ID is required", nil)
		return
	}
	
	if feedbackReq.FeedbackType == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Feedback type is required", nil)
		return
	}
	
	// Record feedback
	err = h.service.RecordUserFeedback(
		ctx, userID, feedbackReq.ProblemID, 
		feedbackReq.FeedbackType, feedbackReq.FeedbackValue, feedbackReq.FeedbackText,
	)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to record feedback", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, map[string]string{"status": "feedback recorded"})
}

// GetServiceStatus handles GET /api/v1/recommendations/status
func (h *Handlers) GetServiceStatus(w http.ResponseWriter, r *http.Request) {
	status := h.service.GetServiceStatus()
	h.writeJSONResponse(w, http.StatusOK, status)
}

// GetPerformanceMetrics handles GET /api/v1/recommendations/metrics
func (h *Handlers) GetPerformanceMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	metrics, err := h.service.GetModelPerformanceMetrics(ctx)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get performance metrics", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, metrics)
}

// RetrainModels handles POST /api/v1/recommendations/retrain
func (h *Handlers) RetrainModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Start retraining (this is async, so we return immediately)
	go func() {
		if err := h.service.RetrainModels(ctx); err != nil {
			// Log error but don't fail the request since it's async
			fmt.Printf("Model retraining failed: %v\n", err)
		}
	}()
	
	h.writeJSONResponse(w, http.StatusAccepted, map[string]string{
		"status": "retraining started",
		"message": "Model retraining has been initiated in the background",
	})
}

// Helper methods

// parseRecommendationRequest parses a recommendation request from query parameters
func (h *Handlers) parseRecommendationRequest(r *http.Request) (*RecommendationRequest, error) {
	query := r.URL.Query()
	
	// Parse user ID (required)
	userIDStr := query.Get("user_id")
	if userIDStr == "" {
		return nil, fmt.Errorf("user_id parameter is required")
	}
	
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id: %w", err)
	}
	
	request := &RecommendationRequest{
		UserID: userID,
		Count:  10, // Default count
	}
	
	// Parse optional count
	if countStr := query.Get("count"); countStr != "" {
		count, err := strconv.Atoi(countStr)
		if err != nil {
			return nil, fmt.Errorf("invalid count parameter: %w", err)
		}
		if count <= 0 || count > 100 {
			return nil, fmt.Errorf("count must be between 1 and 100")
		}
		request.Count = count
	}
	
	// Parse optional difficulty filters
	if minDiffStr := query.Get("min_difficulty"); minDiffStr != "" {
		minDiff, err := strconv.Atoi(minDiffStr)
		if err != nil {
			return nil, fmt.Errorf("invalid min_difficulty: %w", err)
		}
		request.MinDifficulty = &minDiff
	}
	
	if maxDiffStr := query.Get("max_difficulty"); maxDiffStr != "" {
		maxDiff, err := strconv.Atoi(maxDiffStr)
		if err != nil {
			return nil, fmt.Errorf("invalid max_difficulty: %w", err)
		}
		request.MaxDifficulty = &maxDiff
	}
	
	// Parse optional tag filters
	if requiredTagsStr := query.Get("required_tags"); requiredTagsStr != "" {
		request.RequiredTags = strings.Split(requiredTagsStr, ",")
		// Trim whitespace
		for i, tag := range request.RequiredTags {
			request.RequiredTags[i] = strings.TrimSpace(tag)
		}
	}
	
	if excludeTagsStr := query.Get("exclude_tags"); excludeTagsStr != "" {
		request.ExcludeTags = strings.Split(excludeTagsStr, ",")
		// Trim whitespace
		for i, tag := range request.ExcludeTags {
			request.ExcludeTags[i] = strings.TrimSpace(tag)
		}
	}
	
	// Parse optional focus areas
	if focusAreasStr := query.Get("focus_areas"); focusAreasStr != "" {
		request.FocusAreas = strings.Split(focusAreasStr, ",")
		// Trim whitespace
		for i, area := range request.FocusAreas {
			request.FocusAreas[i] = strings.TrimSpace(area)
		}
	}
	
	// Parse optional time limit
	if timeLimitStr := query.Get("time_limit"); timeLimitStr != "" {
		timeLimit, err := strconv.Atoi(timeLimitStr)
		if err != nil {
			return nil, fmt.Errorf("invalid time_limit: %w", err)
		}
		request.TimeLimit = &timeLimit
	}
	
	// Parse optional include solved flag
	if includeSolvedStr := query.Get("include_solved"); includeSolvedStr != "" {
		includeSolved, err := strconv.ParseBool(includeSolvedStr)
		if err != nil {
			return nil, fmt.Errorf("invalid include_solved: %w", err)
		}
		request.IncludeSolved = includeSolved
	}
	
	// Parse optional recommendation type
	if recType := query.Get("recommendation_type"); recType != "" {
		validTypes := map[string]bool{
			RecommendationSkillBuilding: true,
			RecommendationChallenge:     true,
			RecommendationPractice:      true,
			RecommendationContestPrep:   true,
		}
		
		if !validTypes[recType] {
			return nil, fmt.Errorf("invalid recommendation_type: %s", recType)
		}
		request.RecommendationType = recType
	}
	
	return request, nil
}

// writeJSONResponse writes a JSON response
func (h *Handlers) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Printf("Error encoding JSON response: %v\n", err)
	}
}

// writeErrorResponse writes an error response
func (h *Handlers) writeErrorResponse(w http.ResponseWriter, statusCode int, message string, err error) {
	errorResponse := ErrorResponse{
		Error:   message,
		Status:  statusCode,
		Details: "",
	}
	
	if err != nil {
		errorResponse.Details = err.Error()
	}
	
	h.writeJSONResponse(w, statusCode, errorResponse)
}

// Request/Response types for API

// FeedbackRequest represents a user feedback request
type FeedbackRequest struct {
	ProblemID     uuid.UUID `json:"problem_id"`
	FeedbackType  string    `json:"feedback_type"`  // 'clicked', 'solved', 'dismissed', 'rated'
	FeedbackValue *float64  `json:"feedback_value"` // optional numeric feedback
	FeedbackText  *string   `json:"feedback_text"`  // optional text feedback
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Status  int    `json:"status"`
	Details string `json:"details,omitempty"`
}

// Middleware for authentication (simplified)
func (h *Handlers) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// In a real implementation, you'd verify JWT tokens or session auth
		// For now, this is a placeholder
		
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			h.writeErrorResponse(w, http.StatusUnauthorized, "Authorization required", nil)
			return
		}
		
		// Simple bearer token validation (in production, use proper JWT validation)
		if !strings.HasPrefix(authHeader, "Bearer ") {
			h.writeErrorResponse(w, http.StatusUnauthorized, "Invalid authorization format", nil)
			return
		}
		
		// Continue to the next handler
		next.ServeHTTP(w, r)
	}
}

// Middleware for CORS
func (h *Handlers) CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// Middleware for rate limiting (simplified)
func (h *Handlers) RateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// In a real implementation, you'd implement proper rate limiting
		// using Redis or in-memory counters with time windows
		
		// For now, this is a placeholder that always allows requests
		next.ServeHTTP(w, r)
	}
}