package recommendation

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"competitive-programming-platform/pkg/database"

	"github.com/google/uuid"
)

// Service provides the main recommendation service interface
type Service struct {
	db             *database.DB
	hybridEngine   *HybridRecommendationEngine
	dataPipeline   *DataPipeline
	
	// Configuration
	config         *PipelineConfig
	
	// State management
	isInitialized  bool
	mu             sync.RWMutex
	
	// Background processes
	pipelineRunning bool
	stopChan        chan struct{}
	
	// Metrics
	totalRequests   int64
	successfulRecs  int64
	avgResponseTime time.Duration
}

// NewService creates a new recommendation service
func NewService(db *database.DB) *Service {
	config := DefaultPipelineConfig()
	
	return &Service{
		db:           db,
		hybridEngine: NewHybridRecommendationEngine(db),
		dataPipeline: NewDataPipeline(db, config),
		config:       config,
		stopChan:     make(chan struct{}),
	}
}

// Initialize initializes the recommendation service
func (s *Service) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.isInitialized {
		return fmt.Errorf("service is already initialized")
	}
	
	log.Println("Initializing recommendation service...")
	
	// Check if we have existing models
	existingModels, err := s.loadExistingModels(ctx)
	if err != nil {
		log.Printf("Failed to load existing models: %v", err)
	}
	
	// If we don't have trained models, train them
	if len(existingModels) == 0 {
		log.Println("No existing models found, training new models...")
		err = s.trainModels(ctx)
		if err != nil {
			return fmt.Errorf("failed to train models: %w", err)
		}
	} else {
		log.Printf("Loaded %d existing models", len(existingModels))
	}
	
	// Start the data pipeline
	go func() {
		if err := s.dataPipeline.Start(ctx); err != nil {
			log.Printf("Data pipeline error: %v", err)
		}
	}()
	s.pipelineRunning = true
	
	s.isInitialized = true
	log.Println("Recommendation service initialized successfully")
	
	return nil
}

// GetRecommendations provides personalized problem recommendations for a user
func (s *Service) GetRecommendations(ctx context.Context, request *RecommendationRequest) (*RecommendationResponse, error) {
	startTime := time.Now()
	
	defer func() {
		s.mu.Lock()
		s.totalRequests++
		responseTime := time.Since(startTime)
		s.avgResponseTime = (s.avgResponseTime + responseTime) / 2
		s.mu.Unlock()
	}()
	
	s.mu.RLock()
	initialized := s.isInitialized
	s.mu.RUnlock()
	
	if !initialized {
		return nil, fmt.Errorf("recommendation service not initialized")
	}
	
	// Validate request
	if err := s.validateRequest(request); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	
	// Generate recommendations using hybrid engine
	response, err := s.hybridEngine.GetRecommendations(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to generate recommendations: %w", err)
	}
	
	// Update success metrics
	s.mu.Lock()
	s.successfulRecs++
	s.mu.Unlock()
	
	// Log recommendation for feedback learning
	go s.logRecommendation(ctx, request, response)
	
	return response, nil
}

// GetUserProfile returns the user's extracted profile information
func (s *Service) GetUserProfile(ctx context.Context, userID uuid.UUID) (*UserProfile, error) {
	s.mu.RLock()
	initialized := s.isInitialized
	s.mu.RUnlock()
	
	if !initialized {
		return nil, fmt.Errorf("recommendation service not initialized")
	}
	
	// Use feature engineer to extract current profile
	profile, err := s.dataPipeline.featureEngineer.ExtractUserProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to extract user profile: %w", err)
	}
	
	return profile, nil
}

// GetProblemFeatures returns the extracted features for a problem
func (s *Service) GetProblemFeatures(ctx context.Context, problemID uuid.UUID) (*ProblemFeatures, error) {
	s.mu.RLock()
	initialized := s.isInitialized
	s.mu.RUnlock()
	
	if !initialized {
		return nil, fmt.Errorf("recommendation service not initialized")
	}
	
	// Use feature engineer to extract current features
	features, err := s.dataPipeline.featureEngineer.ExtractProblemFeatures(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to extract problem features: %w", err)
	}
	
	return features, nil
}

// RecordUserFeedback records user feedback on recommendations
func (s *Service) RecordUserFeedback(ctx context.Context, userID, problemID uuid.UUID, feedbackType string, feedbackValue *float64, feedbackText *string) error {
	s.mu.RLock()
	initialized := s.isInitialized
	s.mu.RUnlock()
	
	if !initialized {
		return fmt.Errorf("recommendation service not initialized")
	}
	
	// Store feedback in database
	query := `
		INSERT INTO recommendation_feedback (user_id, problem_id, feedback_type, feedback_value, feedback_text, model_version)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	
	modelVersion := s.hybridEngine.getModelVersion()
	
	_, err := s.db.Pool.Exec(ctx, query, userID, problemID, feedbackType, feedbackValue, feedbackText, modelVersion)
	if err != nil {
		return fmt.Errorf("failed to record feedback: %w", err)
	}
	
	log.Printf("Recorded feedback: user=%v, problem=%v, type=%s", userID, problemID, feedbackType)
	return nil
}

// RetrainModels triggers retraining of the recommendation models
func (s *Service) RetrainModels(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.isInitialized {
		return fmt.Errorf("service not initialized")
	}
	
	log.Println("Starting model retraining...")
	
	err := s.trainModels(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrain models: %w", err)
	}
	
	// Clear cache to ensure fresh recommendations
	s.hybridEngine.ClearCache()
	
	log.Println("Model retraining completed successfully")
	return nil
}

// GetServiceStatus returns the current status of the recommendation service
func (s *Service) GetServiceStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	status := map[string]interface{}{
		"is_initialized":      s.isInitialized,
		"pipeline_running":    s.pipelineRunning,
		"total_requests":      s.totalRequests,
		"successful_recommendations": s.successfulRecs,
		"average_response_time":      s.avgResponseTime,
	}
	
	// Add pipeline metrics
	if s.dataPipeline != nil {
		status["pipeline_metrics"] = s.dataPipeline.GetMetrics()
	}
	
	// Add engine metrics
	if s.hybridEngine != nil {
		status["engine_info"] = s.hybridEngine.GetEngineInfo()
	}
	
	return status
}

// GetModelPerformanceMetrics returns performance metrics for the models
func (s *Service) GetModelPerformanceMetrics(ctx context.Context) (map[string]interface{}, error) {
	query := `
		SELECT model_type, metric_name, metric_value, evaluation_set, evaluated_at
		FROM model_performance_metrics 
		WHERE evaluated_at >= $1
		ORDER BY evaluated_at DESC
	`
	
	cutoffTime := time.Now().Add(-7 * 24 * time.Hour) // Last 7 days
	rows, err := s.db.Pool.Query(ctx, query, cutoffTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}
	defer rows.Close()
	
	metrics := make(map[string]interface{})
	modelMetrics := make(map[string]map[string]float64)
	
	for rows.Next() {
		var modelType, metricName, evalSet string
		var metricValue float64
		var evaluatedAt time.Time
		
		err := rows.Scan(&modelType, &metricName, &metricValue, &evalSet, &evaluatedAt)
		if err != nil {
			continue
		}
		
		key := fmt.Sprintf("%s_%s", modelType, evalSet)
		if _, exists := modelMetrics[key]; !exists {
			modelMetrics[key] = make(map[string]float64)
		}
		modelMetrics[key][metricName] = metricValue
	}
	
	metrics["model_metrics"] = modelMetrics
	metrics["last_updated"] = time.Now()
	
	return metrics, nil
}

// Stop stops the recommendation service
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.pipelineRunning {
		s.dataPipeline.Stop()
		s.pipelineRunning = false
	}
	
	close(s.stopChan)
	log.Println("Recommendation service stopped")
}

// Private helper methods

func (s *Service) validateRequest(request *RecommendationRequest) error {
	if request.UserID == uuid.Nil {
		return fmt.Errorf("user ID is required")
	}
	
	if request.Count <= 0 {
		request.Count = 10 // Default to 10 recommendations
	}
	
	if request.Count > 100 {
		return fmt.Errorf("count cannot exceed 100")
	}
	
	return nil
}

func (s *Service) loadExistingModels(ctx context.Context) ([]string, error) {
	query := `
		SELECT model_type, version, status 
		FROM recommendation_models 
		WHERE status = 'ready'
		ORDER BY trained_at DESC
	`
	
	rows, err := s.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var models []string
	for rows.Next() {
		var modelType, version, status string
		err := rows.Scan(&modelType, &version, &status)
		if err != nil {
			continue
		}
		models = append(models, fmt.Sprintf("%s:%s", modelType, version))
	}
	
	return models, nil
}

func (s *Service) trainModels(ctx context.Context) error {
	// Prepare training data
	trainingData, err := s.prepareTrainingData(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare training data: %w", err)
	}
	
	log.Printf("Training with %d interactions, %d user profiles, %d problem features",
		len(trainingData.UserInteractions), len(trainingData.UserProfiles), len(trainingData.ProblemFeatures))
	
	// Train the hybrid model
	err = s.hybridEngine.Train(ctx, trainingData)
	if err != nil {
		return fmt.Errorf("failed to train hybrid model: %w", err)
	}
	
	// Store training metadata
	err = s.storeTrainingMetadata(ctx, trainingData)
	if err != nil {
		log.Printf("Failed to store training metadata: %v", err)
	}
	
	return nil
}

func (s *Service) prepareTrainingData(ctx context.Context) (*TrainingData, error) {
	endDate := time.Now()
	startDate := endDate.Add(-90 * 24 * time.Hour) // Last 90 days
	
	trainingData := &TrainingData{
		StartDate:       startDate,
		EndDate:         endDate,
		ValidationSplit: 0.2,
		TestSplit:       0.1,
	}
	
	// Get user interactions
	interactions, err := s.getUserInteractions(ctx, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get user interactions: %w", err)
	}
	trainingData.UserInteractions = interactions
	
	// Get user profiles
	profiles, err := s.getUserProfiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user profiles: %w", err)
	}
	trainingData.UserProfiles = profiles
	
	// Get problem features
	features, err := s.getProblemFeatures(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get problem features: %w", err)
	}
	trainingData.ProblemFeatures = features
	
	return trainingData, nil
}

func (s *Service) getUserInteractions(ctx context.Context, startDate, endDate time.Time) ([]UserInteraction, error) {
	query := `
		SELECT user_id, problem_id, interaction_type, duration, success,
		       attempt_count, language_used, solution_quality, difficulty_rating, timestamp
		FROM user_interactions
		WHERE timestamp BETWEEN $1 AND $2
		ORDER BY timestamp
	`
	
	rows, err := s.db.Pool.Query(ctx, query, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var interactions []UserInteraction
	for rows.Next() {
		var interaction UserInteraction
		err := rows.Scan(
			&interaction.UserID, &interaction.ProblemID, &interaction.InteractionType,
			&interaction.Duration, &interaction.Success, &interaction.AttemptCount,
			&interaction.LanguageUsed, &interaction.SolutionQuality,
			&interaction.DifficultyRating, &interaction.Timestamp,
		)
		if err != nil {
			continue
		}
		interactions = append(interactions, interaction)
	}
	
	return interactions, nil
}

func (s *Service) getUserProfiles(ctx context.Context) ([]UserProfile, error) {
	query := `
		SELECT user_id, skill_vector, preferred_difficulty, preferred_tags,
		       preferred_languages, solved_problems, attempted_problems,
		       weak_areas, learning_goals, activity_pattern, last_active, updated_at
		FROM user_profiles
		WHERE updated_at >= $1
	`
	
	cutoffTime := time.Now().Add(-30 * 24 * time.Hour)
	rows, err := s.db.Pool.Query(ctx, query, cutoffTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var profiles []UserProfile
	for rows.Next() {
		var profile UserProfile
		// Simplified scanning - in practice, you'd properly handle JSON fields
		err := rows.Scan(
			&profile.UserID, &profile.SkillVector, &profile.PreferredDifficulty,
			&profile.PreferredTags, &profile.PreferredLanguages, &profile.SolvedProblems,
			&profile.AttemptedProblems, &profile.WeakAreas, &profile.LearningGoals,
			&profile.ActivityPattern, &profile.LastActive, &profile.UpdatedAt,
		)
		if err != nil {
			continue
		}
		profiles = append(profiles, profile)
	}
	
	return profiles, nil
}

func (s *Service) getProblemFeatures(ctx context.Context) ([]ProblemFeatures, error) {
	query := `
		SELECT problem_id, title, difficulty, tags, acceptance_rate,
		       average_attempts, average_solve_time, topic_vector,
		       complexity_score, popularity_score, similar_problems,
		       prerequisites, updated_at
		FROM problem_features
		WHERE updated_at >= $1
	`
	
	cutoffTime := time.Now().Add(-30 * 24 * time.Hour)
	rows, err := s.db.Pool.Query(ctx, query, cutoffTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var features []ProblemFeatures
	for rows.Next() {
		var feature ProblemFeatures
		err := rows.Scan(
			&feature.ProblemID, &feature.Title, &feature.Difficulty, &feature.Tags,
			&feature.AcceptanceRate, &feature.AverageAttempts, &feature.AverageSolveTime,
			&feature.TopicVector, &feature.ComplexityScore, &feature.PopularityScore,
			&feature.SimilarProblems, &feature.Prerequisites, &feature.UpdatedAt,
		)
		if err != nil {
			continue
		}
		features = append(features, feature)
	}
	
	return features, nil
}

func (s *Service) storeTrainingMetadata(ctx context.Context, trainingData *TrainingData) error {
	query := `
		INSERT INTO training_data_snapshots (snapshot_name, start_date, end_date, 
		                                     validation_split, test_split, interaction_count,
		                                     user_count, problem_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	
	snapshotName := fmt.Sprintf("training_%s", time.Now().Format("20060102_150405"))
	
	_, err := s.db.Pool.Exec(ctx, query,
		snapshotName, trainingData.StartDate, trainingData.EndDate,
		trainingData.ValidationSplit, trainingData.TestSplit,
		len(trainingData.UserInteractions), len(trainingData.UserProfiles),
		len(trainingData.ProblemFeatures),
	)
	
	return err
}

func (s *Service) logRecommendation(ctx context.Context, request *RecommendationRequest, response *RecommendationResponse) {
	// This is a background operation, so we don't want to fail the main request
	// if this fails
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Error logging recommendation: %v", r)
		}
	}()
	
	// Store recommendation log for future analysis
	query := `
		INSERT INTO recommendation_cache (user_id, cache_key, recommendations, model_version, generated_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, cache_key) DO UPDATE SET
			recommendations = $3, model_version = $4, generated_at = $5, expires_at = $6, hit_count = recommendation_cache.hit_count + 1
	`
	
	cacheKey := fmt.Sprintf("log_%s_%d", request.RecommendationType, request.Count)
	expiresAt := response.GeneratedAt.Add(24 * time.Hour) // Log expires in 24 hours
	
	_, err := s.db.Pool.Exec(ctx, query,
		request.UserID, cacheKey, response.Recommendations,
		response.ModelVersion, response.GeneratedAt, expiresAt,
	)
	
	if err != nil {
		log.Printf("Failed to log recommendation: %v", err)
	}
}