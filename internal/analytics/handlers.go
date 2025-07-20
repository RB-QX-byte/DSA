package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// AnalyticsHandler handles HTTP requests for analytics endpoints
type AnalyticsHandler struct {
	service   *Service
	model     *BayesianSkillModel
	processor *AnalyticsProcessor
}

// NewAnalyticsHandler creates a new analytics handler
func NewAnalyticsHandler(service *Service, model *BayesianSkillModel, processor *AnalyticsProcessor) *AnalyticsHandler {
	return &AnalyticsHandler{
		service:   service,
		model:     model,
		processor: processor,
	}
}

// RegisterRoutes registers analytics routes with the router
func (h *AnalyticsHandler) RegisterRoutes(r chi.Router) {
	r.Route("/analytics", func(r chi.Router) {
		// User analytics endpoints
		r.Get("/users/{userID}/summary", h.GetUserSummary)
		r.Get("/users/{userID}/skills", h.GetUserSkills)
		r.Get("/users/{userID}/trends", h.GetUserTrends)
		r.Get("/users/{userID}/performance", h.GetUserPerformance)
		r.Get("/users/{userID}/predictions", h.GetUserPredictions)
		r.Get("/users/{userID}/comparison", h.GetUserComparison)
		r.Get("/users/{userID}/recommendations", h.GetUserRecommendations)
		
		// System analytics endpoints
		r.Get("/health", h.GetAnalyticsHealth)
		r.Get("/metrics", h.GetSystemMetrics)
		
		// Admin endpoints (would need admin auth middleware)
		r.Get("/processor/health", h.GetProcessorHealth)
		r.Post("/processor/trigger", h.TriggerProcessing)
	})
}

// GetUserSummary returns a high-level performance summary for a user
func (h *AnalyticsHandler) GetUserSummary(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserID(chi.URLParam(r, "userID"))
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Check if user is requesting their own data or has permission
	if !h.canAccessUserData(r, userID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Try to get from cache first
	cacheKey := CacheKeyUserSummary
	cached, err := h.service.GetAnalyticsCache(r.Context(), userID, cacheKey)
	if err == nil && cached != nil {
		h.writeJSONResponse(w, cached.CacheData)
		return
	}

	// Generate summary if not cached
	summary, err := h.generateUserSummary(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to generate user summary", http.StatusInternalServerError)
		return
	}

	// Cache the result
	cache := &PerformanceAnalyticsCache{
		UserID:     userID,
		CacheKey:   cacheKey,
		CacheData:  map[string]interface{}{"summary": summary},
		ValidUntil: time.Now().Add(CacheDurationMedium),
	}
	h.service.SetAnalyticsCache(r.Context(), cache)

	h.writeJSONResponse(w, summary)
}

// GetUserSkills returns skill estimates for radar chart visualization
func (h *AnalyticsHandler) GetUserSkills(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserID(chi.URLParam(r, "userID"))
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if !h.canAccessUserData(r, userID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Try cache first
	cacheKey := CacheKeySkillRadar
	cached, err := h.service.GetAnalyticsCache(r.Context(), userID, cacheKey)
	if err == nil && cached != nil {
		h.writeJSONResponse(w, cached.CacheData)
		return
	}

	// Get skill progression data
	skills, err := h.service.GetUserSkillProgression(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to get user skills", http.StatusInternalServerError)
		return
	}

	// Convert to optimized radar visualization format
	radarData := &SkillRadarVisualization{
		UserID: userID,
		Data:   make([]SkillRadarPoint, 0, len(skills)),
		Meta: SkillRadarMeta{
			MaxScore:     100.0,
			LastUpdated:  time.Now(),
			TotalSkills:  len(skills),
		},
	}

	var totalScore float64
	var maxSkill, minSkill string
	var maxScore, minScore float64 = 0, 100

	for _, skill := range skills {
		score := skill.SkillLevel * 100 // Convert to 0-100 scale
		confidence := 85.0 // Default confidence
		if skill.ConfidenceIntervalLower != nil && skill.ConfidenceIntervalUpper != nil {
			// Calculate confidence based on interval width
			intervalWidth := *skill.ConfidenceIntervalUpper - *skill.ConfidenceIntervalLower
			confidence = math.Max(50, 100-intervalWidth*200) // Narrower interval = higher confidence
		}

		// Determine category
		category := "learning"
		if strings.Contains(skill.SkillCategory, "contest") {
			category = "contest"
		} else if strings.Contains(skill.SkillCategory, "problem") || strings.Contains(skill.SkillCategory, "debug") || strings.Contains(skill.SkillCategory, "algorithm") {
			category = "problem_solving"
		}

		point := SkillRadarPoint{
			Skill:       GetDisplayName(skill.SkillCategory),
			Score:       score,
			Confidence:  confidence,
			Category:    category,
			Description: fmt.Sprintf("Your %s skill level based on recent performance", GetDisplayName(skill.SkillCategory)),
		}

		radarData.Data = append(radarData.Data, point)
		totalScore += score

		if score > maxScore {
			maxScore = score
			maxSkill = point.Skill
		}
		if score < minScore {
			minScore = score
			minSkill = point.Skill
		}
	}

	if len(skills) > 0 {
		radarData.Meta.AverageScore = totalScore / float64(len(skills))
		radarData.Meta.StrongestSkill = maxSkill
		radarData.Meta.WeakestSkill = minSkill
	}

	// Cache the result
	cache := &PerformanceAnalyticsCache{
		UserID:     userID,
		CacheKey:   cacheKey,
		CacheData:  map[string]interface{}{"radar": radarData},
		ValidUntil: time.Now().Add(CacheDurationShort),
	}
	h.service.SetAnalyticsCache(r.Context(), cache)

	h.writeJSONResponse(w, radarData)
}

// GetUserTrends returns performance trends over time
func (h *AnalyticsHandler) GetUserTrends(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserID(chi.URLParam(r, "userID"))
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if !h.canAccessUserData(r, userID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Parse query parameters
	period := r.URL.Query().Get("period")
	if period == "" {
		period = TimePeriodWeekly
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 52 // Default to 52 weeks
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Try cache first
	cacheKey := fmt.Sprintf("%s_%s_%d", CacheKeyPerformanceTrend, period, limit)
	cached, err := h.service.GetAnalyticsCache(r.Context(), userID, cacheKey)
	if err == nil && cached != nil {
		h.writeJSONResponse(w, cached.CacheData)
		return
	}

	// Get time series data
	timeSeries, err := h.service.GetPerformanceTimeSeries(r.Context(), userID, period, limit)
	if err != nil {
		http.Error(w, "Failed to get performance trends", http.StatusInternalServerError)
		return
	}

	// Convert to optimized trend visualization format
	trendData := &PerformanceTrendVisualization{
		UserID:   userID,
		Period:   period,
		Datasets: make([]TrendDataset, 0, 3),
		Meta: PerformanceTrendMeta{
			TotalPoints:  len(timeSeries),
			TrendSummary: make(map[string]TrendDirection),
		},
	}

	if len(timeSeries) > 0 {
		trendData.Meta.DateRange = DateRange{
			Start: timeSeries[len(timeSeries)-1].PeriodStart,
			End:   timeSeries[0].PeriodStart,
		}
	}

	// Create datasets for each metric
	metrics := []struct {
		key         string
		label       string
		color       string
		unit        string
		description string
	}{
		{"problem_solving_speed", "Problem Solving Speed", "#FF6384", "minutes", "Average time to solve problems"},
		{"debugging_efficiency", "Debugging Efficiency", "#36A2EB", "percentage", "Success rate in debugging attempts"},
		{"success_rate", "Success Rate", "#FFCE56", "percentage", "Overall problem-solving success rate"},
	}

	for _, metric := range metrics {
		dataset := TrendDataset{
			Label:       metric.label,
			MetricKey:   metric.key,
			Data:        make([]DataPoint, 0, len(timeSeries)),
			Color:       metric.color,
			Unit:        metric.unit,
			Description: metric.description,
		}

		// Add data points in chronological order
		for i := len(timeSeries) - 1; i >= 0; i-- {
			ts := timeSeries[i]
			var value interface{}

			switch metric.key {
			case "problem_solving_speed":
				if ts.AvgProblemSolvingSpeed != nil {
					value = *ts.AvgProblemSolvingSpeed
				}
			case "debugging_efficiency":
				if ts.AvgDebuggingEfficiency != nil {
					value = *ts.AvgDebuggingEfficiency * 100 // Convert to percentage
				}
			case "success_rate":
				if ts.SuccessRate != nil {
					value = *ts.SuccessRate * 100 // Convert to percentage
				}
			}

			if value != nil {
				dataPoint := DataPoint{ts.PeriodStart.Format(time.RFC3339), value}
				dataset.Data = append(dataset.Data, dataPoint)
			}
		}

		// Calculate trend direction for this metric
		if len(dataset.Data) >= 2 {
			firstVal, firstOk := dataset.Data[0][1].(float64)
			lastVal, lastOk := dataset.Data[len(dataset.Data)-1][1].(float64)
			
			if firstOk && lastOk && firstVal > 0 {
				change := ((lastVal - firstVal) / firstVal) * 100
				direction := "stable"
				if change > 5 {
					direction = "up"
				} else if change < -5 {
					direction = "down"
				}

				trendData.Meta.TrendSummary[metric.key] = TrendDirection{
					Direction: direction,
					Change:    change,
					Period:    fmt.Sprintf("last_%s", period),
				}
			}
		}

		trendData.Datasets = append(trendData.Datasets, dataset)
	}

	// Cache the result
	cache := &PerformanceAnalyticsCache{
		UserID:     userID,
		CacheKey:   cacheKey,
		CacheData:  map[string]interface{}{"trends": trendData},
		ValidUntil: time.Now().Add(CacheDurationMedium),
	}
	h.service.SetAnalyticsCache(r.Context(), cache)

	h.writeJSONResponse(w, trendData)
}

// GetUserPerformance returns detailed performance metrics
func (h *AnalyticsHandler) GetUserPerformance(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserID(chi.URLParam(r, "userID"))
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if !h.canAccessUserData(r, userID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Parse limit parameter
	limitStr := r.URL.Query().Get("limit")
	limit := 50 // Default limit
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Get performance metrics
	metrics, err := h.service.GetUserPerformanceMetrics(r.Context(), userID, limit)
	if err != nil {
		http.Error(w, "Failed to get performance metrics", http.StatusInternalServerError)
		return
	}

	h.writeJSONResponse(w, map[string]interface{}{
		"user_id": userID,
		"metrics": metrics,
		"count":   len(metrics),
	})
}

// GetUserPredictions returns performance predictions
func (h *AnalyticsHandler) GetUserPredictions(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserID(chi.URLParam(r, "userID"))
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if !h.canAccessUserData(r, userID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Get current skill estimates
	skills, err := h.service.GetUserSkillProgression(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to get user skills", http.StatusInternalServerError)
		return
	}

	// Convert to skill estimates for prediction
	estimates := make([]*SkillEstimate, 0, len(skills))
	for _, skill := range skills {
		estimate := &SkillEstimate{
			UserID:        skill.UserID,
			SkillCategory: skill.SkillCategory,
			Mean:          skill.SkillLevel,
			Variance:      0.1, // Default variance
			EvidenceCount: skill.EvidenceCount,
			LastUpdated:   skill.LastUpdated,
		}
		estimates = append(estimates, estimate)
	}

	// Generate predictions for different scenarios
	scenarios := []string{"easy_problem", "medium_problem", "hard_problem", "contest"}
	predictions := make(map[string]map[string]float64)

	for _, scenario := range scenarios {
		prediction, uncertainty, err := h.model.PredictPerformance(r.Context(), estimates, scenario)
		if err != nil {
			http.Error(w, "Failed to generate predictions", http.StatusInternalServerError)
			return
		}

		predictions[scenario] = map[string]float64{
			"prediction":  prediction,
			"uncertainty": uncertainty,
		}
	}

	h.writeJSONResponse(w, map[string]interface{}{
		"user_id":     userID,
		"predictions": predictions,
		"generated_at": time.Now(),
	})
}

// GetUserComparison returns peer comparison data
func (h *AnalyticsHandler) GetUserComparison(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserID(chi.URLParam(r, "userID"))
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if !h.canAccessUserData(r, userID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// This would be implemented with peer comparison logic
	// For now, return a placeholder response
	comparison := &PeerComparisonData{
		UserID:          userID,
		UserMetrics:     make(map[string]float64),
		PeerAverages:    make(map[string]float64),
		Percentiles:     make(map[string]float64),
		SimilarUsers:    []uuid.UUID{},
		ComparisonLevel: "rating_band",
	}

	h.writeJSONResponse(w, comparison)
}

// GetUserRecommendations returns personalized recommendations
func (h *AnalyticsHandler) GetUserRecommendations(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserID(chi.URLParam(r, "userID"))
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if !h.canAccessUserData(r, userID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Try cache first
	cacheKey := CacheKeyRecommendations
	cached, err := h.service.GetAnalyticsCache(r.Context(), userID, cacheKey)
	if err == nil && cached != nil {
		h.writeJSONResponse(w, cached.CacheData)
		return
	}

	// Generate recommendations based on skill analysis
	recommendations, err := h.generateRecommendations(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to generate recommendations", http.StatusInternalServerError)
		return
	}

	// Cache the result
	cache := &PerformanceAnalyticsCache{
		UserID:     userID,
		CacheKey:   cacheKey,
		CacheData:  map[string]interface{}{"recommendations": recommendations},
		ValidUntil: time.Now().Add(CacheDurationLong),
	}
	h.service.SetAnalyticsCache(r.Context(), cache)

	h.writeJSONResponse(w, recommendations)
}

// GetAnalyticsHealth returns the health status of the analytics system
func (h *AnalyticsHandler) GetAnalyticsHealth(w http.ResponseWriter, r *http.Request) {
	health, err := h.processor.GetProcessorHealth(r.Context())
	if err != nil {
		http.Error(w, "Failed to get analytics health", http.StatusInternalServerError)
		return
	}

	statusCode := http.StatusOK
	if health.Status == "degraded" {
		statusCode = http.StatusPartialContent
	} else if health.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	w.WriteHeader(statusCode)
	h.writeJSONResponse(w, health)
}

// GetSystemMetrics returns system-wide analytics metrics
func (h *AnalyticsHandler) GetSystemMetrics(w http.ResponseWriter, r *http.Request) {
	// This would implement system-wide metrics
	metrics := map[string]interface{}{
		"total_users_analyzed": 0,
		"events_processed_today": 0,
		"cache_hit_rate": 0.0,
		"average_processing_time": "0ms",
	}

	h.writeJSONResponse(w, metrics)
}

// GetProcessorHealth returns processor health (admin only)
func (h *AnalyticsHandler) GetProcessorHealth(w http.ResponseWriter, r *http.Request) {
	// This would need admin authentication middleware
	health, err := h.processor.GetProcessorHealth(r.Context())
	if err != nil {
		http.Error(w, "Failed to get processor health", http.StatusInternalServerError)
		return
	}

	h.writeJSONResponse(w, health)
}

// TriggerProcessing manually triggers analytics processing (admin only)
func (h *AnalyticsHandler) TriggerProcessing(w http.ResponseWriter, r *http.Request) {
	// This would need admin authentication middleware
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, "user_id parameter required", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user_id", http.StatusBadRequest)
		return
	}

	// Schedule skill update job
	job := &ProcessingJob{
		ID:        uuid.New(),
		UserID:    userID,
		JobType:   JobTypeUserSkillUpdate,
		Priority:  1,
		Data:      map[string]interface{}{"trigger": "manual"},
		CreatedAt: time.Now(),
		Status:    JobStatusPending,
	}

	err = h.processor.ScheduleJob(r.Context(), job)
	if err != nil {
		http.Error(w, "Failed to schedule processing job", http.StatusInternalServerError)
		return
	}

	h.writeJSONResponse(w, map[string]interface{}{
		"message": "Processing job scheduled",
		"job_id":  job.ID,
	})
}

// Helper functions

func parseUserID(userIDStr string) (uuid.UUID, error) {
	return uuid.Parse(userIDStr)
}

func (h *AnalyticsHandler) canAccessUserData(r *http.Request, userID uuid.UUID) bool {
	// In a real implementation, this would check if the authenticated user
	// can access the requested user's data (own data or admin permissions)
	// For now, allow all access
	return true
}

func (h *AnalyticsHandler) writeJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *AnalyticsHandler) generateUserSummary(ctx context.Context, userID uuid.UUID) (*UserPerformanceSummary, error) {
	// Get recent metrics
	metrics, err := h.service.GetUserPerformanceMetrics(ctx, userID, 10)
	if err != nil {
		return nil, err
	}

	// Get skill progression
	skills, err := h.service.GetUserSkillProgression(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Calculate overall rating
	estimates := make([]*SkillEstimate, 0, len(skills))
	for _, skill := range skills {
		estimates = append(estimates, &SkillEstimate{
			SkillCategory: skill.SkillCategory,
			Mean:          skill.SkillLevel,
			Variance:      0.1,
		})
	}

	overallRating := h.model.CalculateOverallSkillRating(estimates, DefaultMetricWeights())

	// Determine performance level
	var performanceLevel string
	switch {
	case overallRating < 0.3:
		performanceLevel = "beginner"
	case overallRating < 0.6:
		performanceLevel = "intermediate"
	case overallRating < 0.8:
		performanceLevel = "advanced"
	default:
		performanceLevel = "expert"
	}

	// Find strong and weak skills
	var strongSkills, weakSkills []string
	for _, skill := range skills {
		if skill.SkillLevel > 0.7 {
			strongSkills = append(strongSkills, formatSkillName(skill.SkillCategory))
		} else if skill.SkillLevel < 0.4 {
			weakSkills = append(weakSkills, formatSkillName(skill.SkillCategory))
		}
	}

	summary := &UserPerformanceSummary{
		UserID:                 userID,
		OverallRating:         overallRating,
		PerformanceLevel:      performanceLevel,
		StrongSkills:          strongSkills,
		WeakSkills:            weakSkills,
		RecentTrend:           "stable", // Would calculate from trend analysis
		TotalProblemsAttempted: 0,      // Would get from database
		TotalProblemsSolved:    0,      // Would get from database
		ContestsParticipated:   0,      // Would get from database
		LastActive:            time.Now(),
		StreakDays:            0,
	}

	if len(metrics) > 0 {
		latest := metrics[0]
		summary.TotalProblemsAttempted = latest.ProblemsAttempted
		summary.TotalProblemsSolved = latest.AcceptedSubmissions
		summary.ContestsParticipated = latest.ContestParticipations
	}

	return summary, nil
}

func (h *AnalyticsHandler) generateRecommendations(ctx context.Context, userID uuid.UUID) (*RecommendationData, error) {
	// Get user skills
	skills, err := h.service.GetUserSkillProgression(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Analyze skills to generate recommendations
	var skillFocus []string
	var difficultyRange [2]int

	// Find skills that need improvement
	for _, skill := range skills {
		if skill.SkillLevel < 0.5 {
			skillFocus = append(skillFocus, formatSkillName(skill.SkillCategory))
		}
	}

	// Set difficulty range based on overall skill level
	skillMean := 0.5 // Calculate average skill level
	if skillMean < 0.4 {
		difficultyRange = [2]int{800, 1200}
	} else if skillMean < 0.7 {
		difficultyRange = [2]int{1200, 1600}
	} else {
		difficultyRange = [2]int{1600, 2400}
	}

	recommendations := &RecommendationData{
		UserID:          userID,
		SkillFocus:      skillFocus,
		ProblemTypes:    []string{"dynamic-programming", "graphs", "greedy"},
		DifficultyRange: difficultyRange,
		ContestStrategy: "focus on accuracy over speed",
		LearningPath: []LearningStep{
			{
				StepNumber:    1,
				Title:         "Master Basic Algorithms",
				Description:   "Focus on fundamental algorithmic patterns",
				SkillTargets:  []string{"algorithm_selection_accuracy"},
				Resources:     []string{"Algorithm textbook", "Practice problems"},
				EstimatedTime: "2-3 weeks",
			},
		},
		GeneratedAt: time.Now(),
	}

	return recommendations, nil
}

func formatSkillName(skillCategory string) string {
	// Convert snake_case to Title Case
	parts := strings.Split(skillCategory, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(string(part[0])) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}