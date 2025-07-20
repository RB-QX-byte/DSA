package recommendation

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"competitive-programming-platform/pkg/database"

	"github.com/google/uuid"
)

// HybridRecommendationEngine combines content-based and collaborative filtering
type HybridRecommendationEngine struct {
	db                 *database.DB
	contentBasedFilter *ContentBasedFilter
	collaborativeFilter *CollaborativeFilter
	
	// Hybrid model configuration
	hybridModel        *HybridModel
	mu                 sync.RWMutex
	
	// Caching
	cache              map[string]*RecommendationResponse
	cacheExpiration    time.Duration
	
	// Performance tracking
	lastModelUpdate    time.Time
	recommendationCount int64
	averageLatency     time.Duration
}

// NewHybridRecommendationEngine creates a new hybrid recommendation engine
func NewHybridRecommendationEngine(db *database.DB) *HybridRecommendationEngine {
	return &HybridRecommendationEngine{
		db:                  db,
		contentBasedFilter:  NewContentBasedFilter(db),
		collaborativeFilter: NewCollaborativeFilter(db),
		cache:              make(map[string]*RecommendationResponse),
		cacheExpiration:    1 * time.Hour, // Default cache expiration
	}
}

// Train trains both component models and creates the hybrid model
func (hre *HybridRecommendationEngine) Train(ctx context.Context, trainingData *TrainingData) error {
	startTime := time.Now()
	fmt.Println("Starting hybrid model training...")

	// Train both models concurrently
	var contentErr, collaborativeErr error
	var wg sync.WaitGroup

	wg.Add(2)

	// Train content-based model
	go func() {
		defer wg.Done()
		fmt.Println("Training content-based model...")
		contentErr = hre.contentBasedFilter.Train(ctx, trainingData)
		if contentErr != nil {
			fmt.Printf("Content-based training failed: %v\n", contentErr)
		} else {
			fmt.Println("Content-based model training completed")
		}
	}()

	// Train collaborative filtering model
	go func() {
		defer wg.Done()
		fmt.Println("Training collaborative filtering model...")
		collaborativeErr = hre.collaborativeFilter.Train(ctx, trainingData)
		if collaborativeErr != nil {
			fmt.Printf("Collaborative filtering training failed: %v\n", collaborativeErr)
		} else {
			fmt.Println("Collaborative filtering model training completed")
		}
	}()

	wg.Wait()

	// Check training results
	if contentErr != nil && collaborativeErr != nil {
		return fmt.Errorf("both models failed to train: content-based: %v, collaborative: %v", contentErr, collaborativeErr)
	}

	// Create hybrid model configuration
	err := hre.createHybridModel(ctx, trainingData)
	if err != nil {
		return fmt.Errorf("failed to create hybrid model: %w", err)
	}

	hre.lastModelUpdate = time.Now()
	trainingDuration := time.Since(startTime)
	fmt.Printf("Hybrid model training completed in %v\n", trainingDuration)

	return nil
}

// GetRecommendations generates hybrid recommendations for a user
func (hre *HybridRecommendationEngine) GetRecommendations(ctx context.Context, request *RecommendationRequest) (*RecommendationResponse, error) {
	startTime := time.Now()
	defer func() {
		hre.mu.Lock()
		hre.recommendationCount++
		hre.averageLatency = (hre.averageLatency + time.Since(startTime)) / 2
		hre.mu.Unlock()
	}()

	// Check cache first
	cacheKey := hre.buildCacheKey(request)
	if cachedResponse := hre.getFromCache(cacheKey); cachedResponse != nil {
		return cachedResponse, nil
	}

	// Get user's solved problems to exclude
	solvedProblems, err := hre.getUserSolvedProblems(ctx, request.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user solved problems: %w", err)
	}

	// Combine with explicitly excluded problems
	excludeProblems := append(solvedProblems, request.ExcludeTags...)
	if !request.IncludeSolved {
		excludeProblems = solvedProblems
	}

	// Generate recommendations from both models
	var contentRecs, collaborativeRecs []RecommendationResult
	var contentErr, collaborativeErr error

	// Get content-based recommendations
	if hre.contentBasedFilter != nil {
		contentRecs, contentErr = hre.contentBasedFilter.GetRecommendations(
			ctx, request.UserID, request.Count*2, excludeProblems)
	}

	// Get collaborative filtering recommendations
	if hre.collaborativeFilter != nil {
		collaborativeRecs, collaborativeErr = hre.collaborativeFilter.GetRecommendations(
			ctx, request.UserID, request.Count*2, excludeProblems)
	}

	// If both models failed, return error
	if contentErr != nil && collaborativeErr != nil {
		return nil, fmt.Errorf("both recommendation models failed: content: %v, collaborative: %v", contentErr, collaborativeErr)
	}

	// Combine recommendations
	combinedRecs := hre.combineRecommendations(contentRecs, collaborativeRecs, request)

	// Apply filters
	filteredRecs := hre.applyFilters(combinedRecs, request)

	// Take top N recommendations
	if len(filteredRecs) > request.Count {
		filteredRecs = filteredRecs[:request.Count]
	}

	// Create response
	response := &RecommendationResponse{
		UserID:          request.UserID,
		Recommendations: filteredRecs,
		TotalCount:      len(filteredRecs),
		ModelVersion:    hre.getModelVersion(),
		GeneratedAt:     time.Now(),
		RefreshIn:       hre.cacheExpiration,
	}

	// Cache the response
	hre.setCache(cacheKey, response)

	return response, nil
}

// createHybridModel creates the hybrid model configuration
func (hre *HybridRecommendationEngine) createHybridModel(ctx context.Context, trainingData *TrainingData) error {
	// Determine optimal weights through validation
	weights, err := hre.optimizeWeights(ctx, trainingData)
	if err != nil {
		// Use default weights if optimization fails
		weights = map[string]float64{
			"content":       0.6,
			"collaborative": 0.4,
			"popularity":    0.0,
		}
	}

	// Create hybrid model
	hre.hybridModel = &HybridModel{
		ModelID:              uuid.New(),
		Version:              "1.0",
		ContentWeight:        weights["content"],
		CollaborativeWeight:  weights["collaborative"],
		PopularityWeight:     weights["popularity"],
		CombinationStrategy:  "weighted_sum",
		TrainedAt:           time.Now(),
		Status:              ModelStatusReady,
	}

	// Store model metadata in database
	err = hre.storeHybridModel(ctx)
	if err != nil {
		return fmt.Errorf("failed to store hybrid model: %w", err)
	}

	return nil
}

// optimizeWeights optimizes the combination weights using validation data
func (hre *HybridRecommendationEngine) optimizeWeights(ctx context.Context, trainingData *TrainingData) (map[string]float64, error) {
	// Simple grid search for optimal weights
	bestWeights := map[string]float64{"content": 0.5, "collaborative": 0.5, "popularity": 0.0}
	bestScore := 0.0

	// Test different weight combinations
	weightCombinations := [][]float64{
		{0.8, 0.2, 0.0}, // Content-heavy
		{0.6, 0.4, 0.0}, // Content-leaning
		{0.5, 0.5, 0.0}, // Balanced
		{0.4, 0.6, 0.0}, // Collaborative-leaning
		{0.2, 0.8, 0.0}, // Collaborative-heavy
		{0.4, 0.4, 0.2}, // Include popularity
		{0.3, 0.5, 0.2}, // Collaborative with popularity
		{0.5, 0.3, 0.2}, // Content with popularity
	}

	// Use a subset of users for validation
	validationUsers := hre.getValidationUsers(trainingData, 50)

	for _, weights := range weightCombinations {
		score := hre.evaluateWeights(ctx, weights, validationUsers)
		if score > bestScore {
			bestScore = score
			bestWeights = map[string]float64{
				"content":       weights[0],
				"collaborative": weights[1],
				"popularity":    weights[2],
			}
		}
	}

	fmt.Printf("Optimal weights found: content=%.2f, collaborative=%.2f, popularity=%.2f (score=%.4f)\n",
		bestWeights["content"], bestWeights["collaborative"], bestWeights["popularity"], bestScore)

	return bestWeights, nil
}

// evaluateWeights evaluates a weight combination using validation metrics
func (hre *HybridRecommendationEngine) evaluateWeights(ctx context.Context, weights []float64, validationUsers []uuid.UUID) float64 {
	totalScore := 0.0
	validUsers := 0

	for _, userID := range validationUsers {
		// Get user's known positive interactions
		positiveItems, err := hre.getUserPositiveInteractions(ctx, userID)
		if err != nil || len(positiveItems) < 2 {
			continue
		}

		// Split into train/test (use first 80% for training, last 20% for testing)
		splitIndex := int(float64(len(positiveItems)) * 0.8)
		if splitIndex <= 0 {
			continue
		}

		testItems := positiveItems[splitIndex:]
		excludeItems := positiveItems[:splitIndex]

		// Generate recommendations
		request := &RecommendationRequest{
			UserID:        userID,
			Count:         20,
			ExcludeTags:   excludeItems,
		}

		// Simulate hybrid recommendation with these weights
		score := hre.simulateRecommendations(ctx, request, weights, testItems)
		if score > 0 {
			totalScore += score
			validUsers++
		}
	}

	if validUsers == 0 {
		return 0.0
	}

	return totalScore / float64(validUsers)
}

// combineRecommendations combines recommendations from multiple models
func (hre *HybridRecommendationEngine) combineRecommendations(contentRecs, collaborativeRecs []RecommendationResult, request *RecommendationRequest) []RecommendationResult {
	if hre.hybridModel == nil {
		// Default weights if hybrid model not available
		hre.hybridModel = &HybridModel{
			ContentWeight:       0.6,
			CollaborativeWeight: 0.4,
			PopularityWeight:    0.0,
		}
	}

	// Create a map to combine scores for the same problems
	combinedScores := make(map[uuid.UUID]*RecommendationResult)

	// Add content-based recommendations
	for _, rec := range contentRecs {
		if existing, exists := combinedScores[rec.ProblemID]; exists {
			// Combine scores
			existing.Score = existing.Score + rec.Score*hre.hybridModel.ContentWeight
			existing.Confidence = math.Max(existing.Confidence, rec.Confidence)
			
			// Merge reasoning factors
			for factor, value := range rec.ReasoningFactors {
				existing.ReasoningFactors["content_"+factor] = value
			}
		} else {
			// Create new recommendation
			newRec := rec
			newRec.Score = rec.Score * hre.hybridModel.ContentWeight
			newRec.ReasoningFactors = make(map[string]float64)
			for factor, value := range rec.ReasoningFactors {
				newRec.ReasoningFactors["content_"+factor] = value
			}
			combinedScores[rec.ProblemID] = &newRec
		}
	}

	// Add collaborative filtering recommendations
	for _, rec := range collaborativeRecs {
		if existing, exists := combinedScores[rec.ProblemID]; exists {
			// Combine scores
			existing.Score = existing.Score + rec.Score*hre.hybridModel.CollaborativeWeight
			existing.Confidence = math.Max(existing.Confidence, rec.Confidence)
			
			// Merge reasoning factors
			for factor, value := range rec.ReasoningFactors {
				existing.ReasoningFactors["collaborative_"+factor] = value
			}
		} else {
			// Create new recommendation
			newRec := rec
			newRec.Score = rec.Score * hre.hybridModel.CollaborativeWeight
			newRec.ReasoningFactors = make(map[string]float64)
			for factor, value := range rec.ReasoningFactors {
				newRec.ReasoningFactors["collaborative_"+factor] = value
			}
			combinedScores[rec.ProblemID] = &newRec
		}
	}

	// Add popularity boost if needed
	if hre.hybridModel.PopularityWeight > 0 {
		hre.addPopularityBoost(combinedScores, request)
	}

	// Convert map to slice and sort by score
	var combined []RecommendationResult
	for _, rec := range combinedScores {
		// Normalize confidence and ensure reasonable values
		rec.Confidence = math.Min(rec.Confidence, 1.0)
		rec.Score = math.Max(0.0, rec.Score)
		
		combined = append(combined, *rec)
	}

	// Sort by score (descending)
	sort.Slice(combined, func(i, j int) bool {
		return combined[i].Score > combined[j].Score
	})

	return combined
}

// applyFilters applies request filters to recommendations
func (hre *HybridRecommendationEngine) applyFilters(recommendations []RecommendationResult, request *RecommendationRequest) []RecommendationResult {
	var filtered []RecommendationResult

	for _, rec := range recommendations {
		// Apply difficulty filter
		if request.MaxDifficulty != nil || request.MinDifficulty != nil {
			problemDifficulty := hre.getProblemDifficulty(rec.ProblemID)
			
			if request.MinDifficulty != nil && problemDifficulty < *request.MinDifficulty {
				continue
			}
			if request.MaxDifficulty != nil && problemDifficulty > *request.MaxDifficulty {
				continue
			}
		}

		// Apply tag filters
		problemTags := hre.getProblemTags(rec.ProblemID)
		
		// Check required tags
		if len(request.RequiredTags) > 0 {
			hasAllRequired := true
			for _, requiredTag := range request.RequiredTags {
				found := false
				for _, problemTag := range problemTags {
					if problemTag == requiredTag {
						found = true
						break
					}
				}
				if !found {
					hasAllRequired = false
					break
				}
			}
			if !hasAllRequired {
				continue
			}
		}

		// Check excluded tags
		if len(request.ExcludeTags) > 0 {
			hasExcluded := false
			for _, excludeTag := range request.ExcludeTags {
				for _, problemTag := range problemTags {
					if problemTag == excludeTag {
						hasExcluded = true
						break
					}
				}
				if hasExcluded {
					break
				}
			}
			if hasExcluded {
				continue
			}
		}

		// Apply time limit filter
		if request.TimeLimit != nil && rec.EstimatedTime > *request.TimeLimit {
			continue
		}

		// Apply recommendation type specific logic
		if request.RecommendationType != "" {
			if !hre.matchesRecommendationType(rec, request.RecommendationType) {
				continue
			}
		}

		filtered = append(filtered, rec)
	}

	return filtered
}

// Helper functions for caching
func (hre *HybridRecommendationEngine) buildCacheKey(request *RecommendationRequest) string {
	return fmt.Sprintf("user_%s_count_%d_type_%s", request.UserID, request.Count, request.RecommendationType)
}

func (hre *HybridRecommendationEngine) getFromCache(cacheKey string) *RecommendationResponse {
	hre.mu.RLock()
	defer hre.mu.RUnlock()
	
	if cached, exists := hre.cache[cacheKey]; exists {
		if time.Since(cached.GeneratedAt) < hre.cacheExpiration {
			return cached
		}
		// Remove expired cache entry
		delete(hre.cache, cacheKey)
	}
	return nil
}

func (hre *HybridRecommendationEngine) setCache(cacheKey string, response *RecommendationResponse) {
	hre.mu.Lock()
	defer hre.mu.Unlock()
	
	hre.cache[cacheKey] = response
	
	// Simple cache size management
	if len(hre.cache) > 1000 {
		// Remove oldest entries (simplified approach)
		for k := range hre.cache {
			delete(hre.cache, k)
			if len(hre.cache) <= 800 {
				break
			}
		}
	}
}

// Helper database operations
func (hre *HybridRecommendationEngine) getUserSolvedProblems(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	query := `
		SELECT DISTINCT problem_id 
		FROM submissions 
		WHERE user_id = $1 AND status = 'AC'
	`
	
	rows, err := hre.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var problems []uuid.UUID
	for rows.Next() {
		var problemID uuid.UUID
		if err := rows.Scan(&problemID); err != nil {
			continue
		}
		problems = append(problems, problemID)
	}
	
	return problems, nil
}

func (hre *HybridRecommendationEngine) getProblemDifficulty(problemID uuid.UUID) int {
	// This would query the database for problem difficulty
	// Simplified implementation
	return 1200 // Default difficulty
}

func (hre *HybridRecommendationEngine) getProblemTags(problemID uuid.UUID) []string {
	// This would query the database for problem tags
	// Simplified implementation
	return []string{} // Default no tags
}

func (hre *HybridRecommendationEngine) getModelVersion() string {
	if hre.hybridModel != nil {
		return hre.hybridModel.Version
	}
	return "1.0"
}

func (hre *HybridRecommendationEngine) getValidationUsers(trainingData *TrainingData, maxUsers int) []uuid.UUID {
	userSet := make(map[uuid.UUID]bool)
	for _, interaction := range trainingData.UserInteractions {
		userSet[interaction.UserID] = true
	}
	
	var users []uuid.UUID
	for userID := range userSet {
		users = append(users, userID)
		if len(users) >= maxUsers {
			break
		}
	}
	
	return users
}

func (hre *HybridRecommendationEngine) getUserPositiveInteractions(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	query := `
		SELECT DISTINCT problem_id 
		FROM user_interactions 
		WHERE user_id = $1 AND success = true
		ORDER BY timestamp DESC
		LIMIT 20
	`
	
	rows, err := hre.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var problems []uuid.UUID
	for rows.Next() {
		var problemID uuid.UUID
		if err := rows.Scan(&problemID); err != nil {
			continue
		}
		problems = append(problems, problemID)
	}
	
	return problems, nil
}

func (hre *HybridRecommendationEngine) simulateRecommendations(ctx context.Context, request *RecommendationRequest, weights []float64, testItems []uuid.UUID) float64 {
	// Simplified simulation - in practice, you'd generate actual recommendations
	// and calculate precision/recall metrics
	return 0.5 // Placeholder score
}

func (hre *HybridRecommendationEngine) addPopularityBoost(combinedScores map[uuid.UUID]*RecommendationResult, request *RecommendationRequest) {
	// Add popularity boost to recommendations
	// This is a simplified implementation
	for problemID, rec := range combinedScores {
		popularityScore := 0.1 // This would be calculated from actual popularity metrics
		rec.Score += popularityScore * hre.hybridModel.PopularityWeight
		rec.ReasoningFactors["popularity_boost"] = popularityScore
	}
}

func (hre *HybridRecommendationEngine) matchesRecommendationType(rec RecommendationResult, recType string) bool {
	// Implement logic to match recommendations to specific types
	switch recType {
	case RecommendationSkillBuilding:
		return rec.LearningValue > 0.6
	case RecommendationChallenge:
		return rec.DifficultyMatch < 0.5 // More challenging
	case RecommendationPractice:
		return rec.DifficultyMatch > 0.7 // More practice-friendly
	case RecommendationContestPrep:
		return rec.TopicRelevance > 0.8 // Highly relevant
	default:
		return true
	}
}

func (hre *HybridRecommendationEngine) storeHybridModel(ctx context.Context) error {
	query := `
		INSERT INTO recommendation_models (model_type, version, status, model_data, trained_at)
		VALUES ('hybrid', $1, $2, $3, $4)
		ON CONFLICT (model_type, version) DO UPDATE SET
			status = $2, model_data = $3, trained_at = $4, updated_at = NOW()
	`
	
	modelData := map[string]interface{}{
		"content_weight":       hre.hybridModel.ContentWeight,
		"collaborative_weight": hre.hybridModel.CollaborativeWeight,
		"popularity_weight":    hre.hybridModel.PopularityWeight,
		"combination_strategy": hre.hybridModel.CombinationStrategy,
	}
	
	_, err := hre.db.Pool.Exec(ctx, query,
		hre.hybridModel.Version,
		hre.hybridModel.Status,
		modelData,
		hre.hybridModel.TrainedAt,
	)
	
	return err
}

// GetEngineInfo returns information about the hybrid engine
func (hre *HybridRecommendationEngine) GetEngineInfo() map[string]interface{} {
	hre.mu.RLock()
	defer hre.mu.RUnlock()
	
	info := map[string]interface{}{
		"engine_type":         "hybrid_recommendation_engine",
		"last_model_update":   hre.lastModelUpdate,
		"recommendation_count": hre.recommendationCount,
		"average_latency":     hre.averageLatency,
		"cache_size":          len(hre.cache),
		"cache_expiration":    hre.cacheExpiration,
	}
	
	if hre.hybridModel != nil {
		info["hybrid_model"] = map[string]interface{}{
			"model_id":            hre.hybridModel.ModelID,
			"version":             hre.hybridModel.Version,
			"content_weight":      hre.hybridModel.ContentWeight,
			"collaborative_weight": hre.hybridModel.CollaborativeWeight,
			"popularity_weight":   hre.hybridModel.PopularityWeight,
			"combination_strategy": hre.hybridModel.CombinationStrategy,
			"status":              hre.hybridModel.Status,
			"trained_at":          hre.hybridModel.TrainedAt,
		}
	}
	
	// Add component model info
	if hre.contentBasedFilter != nil {
		info["content_based_model"] = hre.contentBasedFilter.GetModelInfo()
	}
	
	if hre.collaborativeFilter != nil {
		info["collaborative_model"] = hre.collaborativeFilter.GetModelInfo()
	}
	
	return info
}

// ClearCache clears the recommendation cache
func (hre *HybridRecommendationEngine) ClearCache() {
	hre.mu.Lock()
	defer hre.mu.Unlock()
	
	hre.cache = make(map[string]*RecommendationResponse)
}