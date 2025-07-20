package recommendation

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"competitive-programming-platform/pkg/database"

	"github.com/google/uuid"
)

// DataPipeline manages the data processing pipeline for recommendations
type DataPipeline struct {
	db              *database.DB
	featureEngineer *FeatureEngineer
	config          *PipelineConfig
	
	// State management
	isRunning       bool
	stopChan        chan bool
	mu              sync.RWMutex
	
	// Processing state
	lastProcessedTime time.Time
	batchNumber       int64
	
	// Metrics
	processedInteractions int64
	processedProfiles     int64
	processedFeatures     int64
	errors                []error
}

// NewDataPipeline creates a new data pipeline
func NewDataPipeline(db *database.DB, config *PipelineConfig) *DataPipeline {
	if config == nil {
		config = DefaultPipelineConfig()
	}

	return &DataPipeline{
		db:                    db,
		featureEngineer:      NewFeatureEngineer(db, config),
		config:               config,
		stopChan:             make(chan bool),
		lastProcessedTime:    time.Now().Add(-24 * time.Hour), // Start from 24 hours ago
	}
}

// Start starts the data pipeline
func (dp *DataPipeline) Start(ctx context.Context) error {
	dp.mu.Lock()
	if dp.isRunning {
		dp.mu.Unlock()
		return fmt.Errorf("pipeline is already running")
	}
	dp.isRunning = true
	dp.mu.Unlock()

	log.Println("Starting recommendation data pipeline...")

	// Start periodic processing
	ticker := time.NewTicker(dp.config.ProcessingInterval)
	defer ticker.Stop()

	// Initial processing
	if err := dp.processNewData(ctx); err != nil {
		log.Printf("Initial data processing failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Pipeline stopped due to context cancellation")
			return ctx.Err()
		case <-dp.stopChan:
			log.Println("Pipeline stopped")
			return nil
		case <-ticker.C:
			if err := dp.processNewData(ctx); err != nil {
				log.Printf("Data processing error: %v", err)
				dp.addError(err)
			}
		}
	}
}

// Stop stops the data pipeline
func (dp *DataPipeline) Stop() {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	
	if dp.isRunning {
		dp.isRunning = false
		close(dp.stopChan)
	}
}

// processNewData processes new data since last processing
func (dp *DataPipeline) processNewData(ctx context.Context) error {
	now := time.Now()
	log.Printf("Processing data from %v to %v", dp.lastProcessedTime, now)

	// Process user interactions
	err := dp.processUserInteractions(ctx, dp.lastProcessedTime, now)
	if err != nil {
		return fmt.Errorf("failed to process user interactions: %w", err)
	}

	// Update user profiles
	err = dp.updateUserProfiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to update user profiles: %w", err)
	}

	// Update problem features
	err = dp.updateProblemFeatures(ctx)
	if err != nil {
		return fmt.Errorf("failed to update problem features: %w", err)
	}

	// Calculate similarities if needed
	if dp.batchNumber%10 == 0 { // Every 10 batches
		err = dp.calculateSimilarities(ctx)
		if err != nil {
			log.Printf("Failed to calculate similarities: %v", err)
		}
	}

	dp.lastProcessedTime = now
	dp.batchNumber++

	log.Printf("Data processing completed. Batch: %d", dp.batchNumber)
	return nil
}

// processUserInteractions processes new user interactions
func (dp *DataPipeline) processUserInteractions(ctx context.Context, startTime, endTime time.Time) error {
	// Query for new interactions
	query := `
		SELECT user_id, problem_id, interaction_type, duration, success,
		       attempt_count, language_used, solution_quality, difficulty_rating, timestamp
		FROM user_interactions
		WHERE timestamp BETWEEN $1 AND $2
		ORDER BY timestamp
	`

	rows, err := dp.db.Pool.Query(ctx, query, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to query interactions: %w", err)
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
			log.Printf("Failed to scan interaction: %v", err)
			continue
		}
		interactions = append(interactions, interaction)
	}

	// Process interactions in batches
	for i := 0; i < len(interactions); i += dp.config.BatchSize {
		end := i + dp.config.BatchSize
		if end > len(interactions) {
			end = len(interactions)
		}

		batch := interactions[i:end]
		err := dp.processBatchInteractions(ctx, batch)
		if err != nil {
			log.Printf("Failed to process interaction batch: %v", err)
			continue
		}

		dp.processedInteractions += int64(len(batch))
	}

	return nil
}

// processBatchInteractions processes a batch of interactions
func (dp *DataPipeline) processBatchInteractions(ctx context.Context, interactions []UserInteraction) error {
	// Group interactions by user and problem for efficient processing
	userInteractions := make(map[uuid.UUID][]UserInteraction)
	problemInteractions := make(map[uuid.UUID][]UserInteraction)

	for _, interaction := range interactions {
		userInteractions[interaction.UserID] = append(userInteractions[interaction.UserID], interaction)
		problemInteractions[interaction.ProblemID] = append(problemInteractions[interaction.ProblemID], interaction)
	}

	// Update user statistics
	for userID, userInts := range userInteractions {
		err := dp.updateUserStatistics(ctx, userID, userInts)
		if err != nil {
			log.Printf("Failed to update user statistics for %v: %v", userID, err)
		}
	}

	// Update problem statistics
	for problemID, probInts := range problemInteractions {
		err := dp.updateProblemStatistics(ctx, problemID, probInts)
		if err != nil {
			log.Printf("Failed to update problem statistics for %v: %v", problemID, err)
		}
	}

	return nil
}

// updateUserProfiles updates user profiles for active users
func (dp *DataPipeline) updateUserProfiles(ctx context.Context) error {
	// Get users who had interactions recently
	cutoffTime := time.Now().Add(-24 * time.Hour)
	query := `
		SELECT DISTINCT user_id 
		FROM user_interactions 
		WHERE timestamp >= $1
	`

	rows, err := dp.db.Pool.Query(ctx, query, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to query active users: %w", err)
	}
	defer rows.Close()

	var userIDs []uuid.UUID
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			continue
		}
		userIDs = append(userIDs, userID)
	}

	// Update profiles concurrently with limited concurrency
	semaphore := make(chan struct{}, 10) // Limit to 10 concurrent updates
	var wg sync.WaitGroup

	for _, userID := range userIDs {
		wg.Add(1)
		go func(uid uuid.UUID) {
			defer wg.Done()
			semaphore <- struct{}{} // Acquire
			defer func() { <-semaphore }() // Release

			err := dp.updateSingleUserProfile(ctx, uid)
			if err != nil {
				log.Printf("Failed to update profile for user %v: %v", uid, err)
			} else {
				dp.processedProfiles++
			}
		}(userID)
	}

	wg.Wait()
	return nil
}

// updateSingleUserProfile updates a single user's profile
func (dp *DataPipeline) updateSingleUserProfile(ctx context.Context, userID uuid.UUID) error {
	profile, err := dp.featureEngineer.ExtractUserProfile(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to extract user profile: %w", err)
	}

	// Store or update the profile in database
	err = dp.storeUserProfile(ctx, profile)
	if err != nil {
		return fmt.Errorf("failed to store user profile: %w", err)
	}

	return nil
}

// updateProblemFeatures updates problem features for problems with new interactions
func (dp *DataPipeline) updateProblemFeatures(ctx context.Context) error {
	// Get problems that had interactions recently
	cutoffTime := time.Now().Add(-24 * time.Hour)
	query := `
		SELECT DISTINCT problem_id 
		FROM user_interactions 
		WHERE timestamp >= $1
	`

	rows, err := dp.db.Pool.Query(ctx, query, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to query active problems: %w", err)
	}
	defer rows.Close()

	var problemIDs []uuid.UUID
	for rows.Next() {
		var problemID uuid.UUID
		if err := rows.Scan(&problemID); err != nil {
			continue
		}
		problemIDs = append(problemIDs, problemID)
	}

	// Update features concurrently
	semaphore := make(chan struct{}, 5) // Limit to 5 concurrent updates
	var wg sync.WaitGroup

	for _, problemID := range problemIDs {
		wg.Add(1)
		go func(pid uuid.UUID) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			err := dp.updateSingleProblemFeatures(ctx, pid)
			if err != nil {
				log.Printf("Failed to update features for problem %v: %v", pid, err)
			} else {
				dp.processedFeatures++
			}
		}(problemID)
	}

	wg.Wait()
	return nil
}

// updateSingleProblemFeatures updates features for a single problem
func (dp *DataPipeline) updateSingleProblemFeatures(ctx context.Context, problemID uuid.UUID) error {
	features, err := dp.featureEngineer.ExtractProblemFeatures(ctx, problemID)
	if err != nil {
		return fmt.Errorf("failed to extract problem features: %w", err)
	}

	// Store or update the features in database
	err = dp.storeProblemFeatures(ctx, features)
	if err != nil {
		return fmt.Errorf("failed to store problem features: %w", err)
	}

	return nil
}

// calculateSimilarities calculates user and problem similarities
func (dp *DataPipeline) calculateSimilarities(ctx context.Context) error {
	log.Println("Calculating similarities...")

	// Calculate user similarities
	err := dp.calculateUserSimilarities(ctx)
	if err != nil {
		log.Printf("Failed to calculate user similarities: %v", err)
	}

	// Calculate problem similarities
	err = dp.calculateProblemSimilarities(ctx)
	if err != nil {
		log.Printf("Failed to calculate problem similarities: %v", err)
	}

	return nil
}

// calculateUserSimilarities calculates similarities between users
func (dp *DataPipeline) calculateUserSimilarities(ctx context.Context) error {
	// This is a simplified implementation
	// In practice, you'd use more sophisticated algorithms like cosine similarity
	// on user feature vectors or collaborative filtering approaches

	query := `
		SELECT user_id, solved_problems, skill_vector
		FROM user_profiles
		WHERE updated_at >= $1
		LIMIT 1000
	`

	cutoffTime := time.Now().Add(-7 * 24 * time.Hour)
	rows, err := dp.db.Pool.Query(ctx, query, cutoffTime)
	if err != nil {
		return err
	}
	defer rows.Close()

	type userFeatures struct {
		userID         uuid.UUID
		solvedProblems []uuid.UUID
		skillVector    map[string]float64
	}

	var users []userFeatures
	for rows.Next() {
		var user userFeatures
		// Simplified: in real implementation, you'd properly unmarshal JSON fields
		err := rows.Scan(&user.userID, &user.solvedProblems, &user.skillVector)
		if err != nil {
			continue
		}
		users = append(users, user)
	}

	// Calculate pairwise similarities for a subset of users
	for i := 0; i < len(users) && i < 100; i++ {
		for j := i + 1; j < len(users) && j < 100; j++ {
			similarity := dp.calculateJaccardSimilarity(users[i].solvedProblems, users[j].solvedProblems)
			
			if similarity > 0.1 { // Only store meaningful similarities
				err := dp.storeUserSimilarity(ctx, users[i].userID, users[j].userID, similarity)
				if err != nil {
					log.Printf("Failed to store user similarity: %v", err)
				}
			}
		}
	}

	return nil
}

// calculateProblemSimilarities calculates similarities between problems
func (dp *DataPipeline) calculateProblemSimilarities(ctx context.Context) error {
	// Simplified implementation
	query := `
		SELECT problem_id, tags, difficulty, topic_vector
		FROM problem_features
		WHERE updated_at >= $1
		LIMIT 500
	`

	cutoffTime := time.Now().Add(-7 * 24 * time.Hour)
	rows, err := dp.db.Pool.Query(ctx, query, cutoffTime)
	if err != nil {
		return err
	}
	defer rows.Close()

	type problemFeatures struct {
		problemID   uuid.UUID
		tags        []string
		difficulty  int
		topicVector map[string]float64
	}

	var problems []problemFeatures
	for rows.Next() {
		var problem problemFeatures
		err := rows.Scan(&problem.problemID, &problem.tags, &problem.difficulty, &problem.topicVector)
		if err != nil {
			continue
		}
		problems = append(problems, problem)
	}

	// Calculate pairwise similarities
	for i := 0; i < len(problems); i++ {
		for j := i + 1; j < len(problems); j++ {
			tagSim := dp.calculateJaccardSimilarityStr(problems[i].tags, problems[j].tags)
			diffSim := 1.0 - math.Abs(float64(problems[i].difficulty-problems[j].difficulty))/3500.0
			
			similarity := (tagSim*0.7 + diffSim*0.3)
			
			if similarity > 0.3 {
				err := dp.storeProblemSimilarity(ctx, problems[i].problemID, problems[j].problemID, similarity)
				if err != nil {
					log.Printf("Failed to store problem similarity: %v", err)
				}
			}
		}
	}

	return nil
}

// Helper methods for storing data
func (dp *DataPipeline) storeUserProfile(ctx context.Context, profile *UserProfile) error {
	query := `
		INSERT INTO user_profiles (user_id, skill_vector, preferred_difficulty, preferred_tags, 
		                          preferred_languages, solved_problems, attempted_problems, 
		                          weak_areas, learning_goals, activity_pattern, last_active, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (user_id) DO UPDATE SET
			skill_vector = $2, preferred_difficulty = $3, preferred_tags = $4,
			preferred_languages = $5, solved_problems = $6, attempted_problems = $7,
			weak_areas = $8, learning_goals = $9, activity_pattern = $10,
			last_active = $11, updated_at = $12
	`

	_, err := dp.db.Pool.Exec(ctx, query,
		profile.UserID, profile.SkillVector, profile.PreferredDifficulty,
		profile.PreferredTags, profile.PreferredLanguages, profile.SolvedProblems,
		profile.AttemptedProblems, profile.WeakAreas, profile.LearningGoals,
		profile.ActivityPattern, profile.LastActive, profile.UpdatedAt,
	)

	return err
}

func (dp *DataPipeline) storeProblemFeatures(ctx context.Context, features *ProblemFeatures) error {
	query := `
		INSERT INTO problem_features (problem_id, title, difficulty, tags, acceptance_rate,
		                             average_attempts, average_solve_time, topic_vector,
		                             complexity_score, popularity_score, similar_problems,
		                             prerequisites, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (problem_id) DO UPDATE SET
			title = $2, difficulty = $3, tags = $4, acceptance_rate = $5,
			average_attempts = $6, average_solve_time = $7, topic_vector = $8,
			complexity_score = $9, popularity_score = $10, similar_problems = $11,
			prerequisites = $12, updated_at = $13
	`

	_, err := dp.db.Pool.Exec(ctx, query,
		features.ProblemID, features.Title, features.Difficulty, features.Tags,
		features.AcceptanceRate, features.AverageAttempts, features.AverageSolveTime,
		features.TopicVector, features.ComplexityScore, features.PopularityScore,
		features.SimilarProblems, features.Prerequisites, features.UpdatedAt,
	)

	return err
}

func (dp *DataPipeline) storeUserSimilarity(ctx context.Context, userID1, userID2 uuid.UUID, score float64) error {
	query := `
		INSERT INTO user_similarities (user_id_1, user_id_2, similarity_type, score, computed_at)
		VALUES ($1, $2, 'jaccard', $3, $4)
		ON CONFLICT (user_id_1, user_id_2, similarity_type) DO UPDATE SET
			score = $3, computed_at = $4
	`

	_, err := dp.db.Pool.Exec(ctx, query, userID1, userID2, score, time.Now())
	return err
}

func (dp *DataPipeline) storeProblemSimilarity(ctx context.Context, problemID1, problemID2 uuid.UUID, score float64) error {
	query := `
		INSERT INTO problem_similarities (problem_id_1, problem_id_2, similarity_type, score, computed_at)
		VALUES ($1, $2, 'content', $3, $4)
		ON CONFLICT (problem_id_1, problem_id_2, similarity_type) DO UPDATE SET
			score = $3, computed_at = $4
	`

	_, err := dp.db.Pool.Exec(ctx, query, problemID1, problemID2, score, time.Now())
	return err
}

func (dp *DataPipeline) updateUserStatistics(ctx context.Context, userID uuid.UUID, interactions []UserInteraction) error {
	// Update user-specific statistics based on new interactions
	// This could include updating skill assessments, solving streaks, etc.
	// Simplified implementation
	return nil
}

func (dp *DataPipeline) updateProblemStatistics(ctx context.Context, problemID uuid.UUID, interactions []UserInteraction) error {
	// Update problem-specific statistics
	// This could include acceptance rate, average solve time, etc.
	// Simplified implementation
	return nil
}

// Utility functions
func (dp *DataPipeline) calculateJaccardSimilarity(set1, set2 []uuid.UUID) float64 {
	if len(set1) == 0 && len(set2) == 0 {
		return 1.0
	}

	set1Map := make(map[uuid.UUID]bool)
	for _, item := range set1 {
		set1Map[item] = true
	}

	intersection := 0
	for _, item := range set2 {
		if set1Map[item] {
			intersection++
		}
	}

	union := len(set1) + len(set2) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

func (dp *DataPipeline) calculateJaccardSimilarityStr(set1, set2 []string) float64 {
	if len(set1) == 0 && len(set2) == 0 {
		return 1.0
	}

	set1Map := make(map[string]bool)
	for _, item := range set1 {
		set1Map[item] = true
	}

	intersection := 0
	for _, item := range set2 {
		if set1Map[item] {
			intersection++
		}
	}

	union := len(set1) + len(set2) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

func (dp *DataPipeline) addError(err error) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	
	dp.errors = append(dp.errors, err)
	
	// Keep only the last 100 errors
	if len(dp.errors) > 100 {
		dp.errors = dp.errors[len(dp.errors)-100:]
	}
}

// GetMetrics returns pipeline metrics
func (dp *DataPipeline) GetMetrics() map[string]interface{} {
	dp.mu.RLock()
	defer dp.mu.RUnlock()

	return map[string]interface{}{
		"is_running":              dp.isRunning,
		"last_processed_time":     dp.lastProcessedTime,
		"batch_number":            dp.batchNumber,
		"processed_interactions":  dp.processedInteractions,
		"processed_profiles":      dp.processedProfiles,
		"processed_features":      dp.processedFeatures,
		"error_count":            len(dp.errors),
	}
}