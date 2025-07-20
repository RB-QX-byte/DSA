package judge

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// APIHandler handles judge-related API endpoints
type APIHandler struct {
	judgeService *JudgeService
	db           *pgxpool.Pool
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(judgeService *JudgeService, db *pgxpool.Pool) *APIHandler {
	return &APIHandler{
		judgeService: judgeService,
		db:           db,
	}
}

// GetSubmission retrieves a submission by ID
func (ah *APIHandler) GetSubmission(w http.ResponseWriter, r *http.Request) {
	submissionID := chi.URLParam(r, "id")
	if submissionID == "" {
		http.Error(w, "Submission ID is required", http.StatusBadRequest)
		return
	}

	result, err := ah.judgeService.GetSubmission(r.Context(), submissionID)
	if err != nil {
		http.Error(w, "Submission not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GetSubmissions retrieves all submissions for a user
func (ah *APIHandler) GetSubmissions(w http.ResponseWriter, r *http.Request) {
	// This would be implemented to get user submissions
	// For now, return empty array
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]interface{}{})
}

// GetQueueStats returns queue statistics
func (ah *APIHandler) GetQueueStats(w http.ResponseWriter, r *http.Request) {
	stats, err := ah.judgeService.queueManager.GetQueueStats(r.Context())
	if err != nil {
		http.Error(w, "Failed to get queue stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}